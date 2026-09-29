// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

// Package cluster implements einterfaces.ClusterInterface, allowing several
// Mattermost servers sharing a database to work as a high-availability
// cluster.
//
// Membership, failure detection and transport are provided by
// hashicorp/memberlist (SWIM gossip over the configured gossip port, UDP and
// TCP). Nodes discover each other through the ClusterDiscovery table.
// On top of memberlist user messages this package implements:
//   - best-effort (UDP) and reliable (TCP, ordered per peer, batched)
//     delivery of model.ClusterMessage,
//   - a request/response mechanism used by the methods that aggregate data
//     from all the nodes (logs, stats, plugin statuses, support packets,
//     websocket queues...),
//   - deterministic leader election: the leader is the live node that
//     started first (ties broken by node id).
package cluster

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hashicorp/memberlist"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/v8/channels/app/platform"
	"github.com/mattermost/mattermost/server/v8/einterfaces"
)

func init() {
	platform.RegisterClusterInterface(func(ps *platform.PlatformService) einterfaces.ClusterInterface {
		return New(newPlatformHost(ps))
	})
}

const (
	// memberlist label prefix. Nodes only talk to nodes using the same label,
	// which keeps different clusters (and incompatible implementations) apart.
	labelPrefix = "mmlibre1:"

	defaultTCPTimeout       = 10 * time.Second
	defaultDiscoveryPing    = platform.DiscoveryServiceWritePing
	defaultJoinInterval     = 60 * time.Second
	defaultJoinIntervalLone = 10 * time.Second
	initialJoinTimeout      = 15 * time.Second
	leaveTimeout            = 3 * time.Second
	updateMetaTimeout       = 5 * time.Second
)

var (
	errNotRunning = errors.New("inter-node communication is not running")
	errNodeGone   = errors.New("cluster node is not a member of the cluster")
)

// nodeMeta is the metadata each node gossips about itself (limited to
// memberlist.MetaMaxSize bytes).
type nodeMeta struct {
	ID         string `json:"i"`
	Hostname   string `json:"h"`
	IPAddress  string `json:"a"`
	Version    string `json:"v"`
	Schema     string `json:"s"`
	ConfigHash string `json:"c"`
	StartAt    int64  `json:"t"`
}

func (m nodeMeta) clusterInfo() *model.ClusterInfo {
	return &model.ClusterInfo{
		Id:            m.ID,
		Version:       m.Version,
		SchemaVersion: m.Schema,
		ConfigHash:    m.ConfigHash,
		IPAddress:     m.IPAddress,
		Hostname:      m.Hostname,
	}
}

type peer struct {
	node   memberlist.Node
	meta   nodeMeta
	metaOK bool
}

// Cluster implements einterfaces.ClusterInterface.
type Cluster struct {
	host Host
	id   string

	handlersMu sync.RWMutex
	handlers   map[model.ClusterEvent]einterfaces.ClusterMessageHandler

	// lifecycleMu serializes start and stop.
	lifecycleMu sync.Mutex
	running     atomic.Bool
	list        *memberlist.Memberlist
	stopCh      chan struct{}
	wg          sync.WaitGroup
	cfgListener string
	discovery   *model.ClusterDiscovery
	clusterName string
	advertise   string // host:port other nodes use to reach this node

	metaMu sync.RWMutex
	meta   nodeMeta

	nodesMu   sync.RWMutex
	nodes     map[string]*peer
	senders   map[string]*peerSender
	membersCh chan struct{}

	leaderMu sync.Mutex
	leaderID string
	isLeader atomic.Bool

	dispatcher *dispatcher
	fragments  *reassembler
	seq        *sequencer
	epochs     atomic.Uint64
	rpc        *rpcManager

	// Tunables, overridden in tests.
	tcpTimeout       time.Duration
	discoveryPing    time.Duration
	joinInterval     time.Duration
	joinIntervalLone time.Duration
	timeouts         rpcTimeouts
	tuneMemberlist   func(*memberlist.Config)
}

var _ einterfaces.ClusterInterface = (*Cluster)(nil)

// New creates the cluster interface. Nothing happens until
// StartInterNodeCommunication is called.
func New(host Host) *Cluster {
	c := &Cluster{
		host:             host,
		id:               model.NewId(),
		handlers:         make(map[model.ClusterEvent]einterfaces.ClusterMessageHandler),
		nodes:            make(map[string]*peer),
		senders:          make(map[string]*peerSender),
		membersCh:        make(chan struct{}, 1),
		tcpTimeout:       defaultTCPTimeout,
		discoveryPing:    defaultDiscoveryPing,
		joinInterval:     defaultJoinInterval,
		joinIntervalLone: defaultJoinIntervalLone,
		timeouts:         defaultRPCTimeouts(),
	}
	c.fragments = newReassembler(c)
	c.seq = newSequencer(c)
	c.rpc = newRPCManager(c)
	return c
}

func (c *Cluster) logger() mlog.LoggerIFace {
	return c.host.Log()
}

// StartInterNodeCommunication joins the cluster if clustering is enabled.
func (c *Cluster) StartInterNodeCommunication() {
	c.lifecycleMu.Lock()
	defer c.lifecycleMu.Unlock()

	if c.running.Load() {
		return
	}

	cfg := c.host.Config()
	if !*cfg.ClusterSettings.Enable {
		c.logger().Debug("Cluster: high availability mode is disabled, not starting inter-node communication")
		return
	}

	if err := c.start(cfg); err != nil {
		c.logger().Error("Cluster: failed to start inter-node communication", mlog.Err(err))
	}
}

// resolveAddresses returns the address to bind to, the IP address advertised
// to the other nodes and the hostname stored in the discovery table.
func resolveAddresses(cs *model.ClusterSettings) (bindAddr, advertiseIP, discoveryHost string) {
	bindAddr = *cs.BindAddress
	if bindAddr == "" {
		bindAddr = "0.0.0.0"
	}

	advertiseIP = *cs.AdvertiseAddress
	if advertiseIP == "" {
		if ip := net.ParseIP(*cs.BindAddress); ip != nil && !ip.IsUnspecified() {
			advertiseIP = ip.String()
		} else {
			advertiseIP = model.GetServerIPAddress(*cs.NetworkInterface)
		}
	}

	switch {
	case *cs.OverrideHostname != "":
		discoveryHost = *cs.OverrideHostname
	case *cs.UseIPAddress && advertiseIP != "":
		discoveryHost = advertiseIP
	default:
		if hn, err := os.Hostname(); err == nil && hn != "" {
			discoveryHost = hn
		} else {
			discoveryHost = advertiseIP
		}
	}
	return bindAddr, advertiseIP, discoveryHost
}

// displayHostname is the name shown in the System Console and used to label
// logs and support packet files.
func displayHostname(cs *model.ClusterSettings, fallback string) string {
	if *cs.OverrideHostname != "" {
		return *cs.OverrideHostname
	}
	if hn, err := os.Hostname(); err == nil && hn != "" {
		return hn
	}
	return fallback
}

func (c *Cluster) start(cfg *model.Config) error {
	cs := &cfg.ClusterSettings
	if *cs.ClusterName == "" {
		c.logger().Warn("Cluster: ClusterSettings.ClusterName is empty, all the nodes of the cluster must use the same name")
	}

	bindAddr, advertiseIP, discoveryHost := resolveAddresses(cs)
	port := *cs.GossipPort

	mlConf := memberlist.DefaultLANConfig()
	mlConf.Name = c.id
	mlConf.BindAddr = bindAddr
	mlConf.BindPort = port
	mlConf.AdvertisePort = port
	if advertiseIP != "" {
		mlConf.AdvertiseAddr = advertiseIP
	}
	mlConf.Label = clusterLabel(*cs.ClusterName)
	mlConf.TCPTimeout = c.tcpTimeout
	mlConf.EnableCompression = *cs.EnableGossipCompression
	mlConf.Delegate = &delegate{c: c}
	mlConf.Events = &eventDelegate{c: c}
	mlConf.Logger = log.New(&memberlistLogWriter{c: c}, "", 0)
	mlConf.LogOutput = nil

	if *cs.EnableGossipEncryption {
		key, err := c.loadEncryptionKey()
		if err != nil {
			return fmt.Errorf("failed to load the gossip encryption key: %w", err)
		}
		mlConf.SecretKey = key
		mlConf.GossipVerifyIncoming = true
		mlConf.GossipVerifyOutgoing = true
	}

	if c.tuneMemberlist != nil {
		c.tuneMemberlist(mlConf)
	}

	hostname := displayHostname(cs, advertiseIP)
	c.metaMu.Lock()
	c.meta = nodeMeta{
		ID:         c.id,
		Hostname:   hostname,
		IPAddress:  advertiseIP,
		Version:    model.CurrentVersion,
		Schema:     c.host.SchemaVersion(),
		ConfigHash: configHash(cfg),
		StartAt:    model.GetMillis(),
	}
	c.metaMu.Unlock()

	c.stopCh = make(chan struct{})
	c.clusterName = *cs.ClusterName
	c.dispatcher = newDispatcher(c)
	c.running.Store(true)

	list, err := memberlist.Create(mlConf)
	if err != nil {
		c.running.Store(false)
		c.dispatcher.stop()
		close(c.stopCh)
		return fmt.Errorf("failed to create the gossip listener on %s: %w", net.JoinHostPort(bindAddr, strconv.Itoa(port)), err)
	}
	c.nodesMu.Lock()
	c.list = list
	c.nodesMu.Unlock()

	localNode := list.LocalNode()
	c.advertise = net.JoinHostPort(localNode.Addr.String(), strconv.Itoa(int(localNode.Port)))
	if advertiseIP == "" {
		// memberlist picked the address itself.
		c.metaMu.Lock()
		c.meta.IPAddress = localNode.Addr.String()
		c.metaMu.Unlock()
		if discoveryHost == "" {
			discoveryHost = localNode.Addr.String()
		}
	}

	c.discovery = &model.ClusterDiscovery{
		Type:        model.CDSTypeApp,
		ClusterName: *cs.ClusterName,
		Hostname:    discoveryHost,
		GossipPort:  int32(port),
	}

	c.cfgListener = c.host.AddConfigListener(func(_, newCfg *model.Config) {
		c.refreshConfigHash(newCfg)
	})

	c.logger().Info("Cluster: inter-node communication started",
		mlog.String("node_id", c.id),
		mlog.String("cluster_name", *cs.ClusterName),
		mlog.String("hostname", hostname),
		mlog.String("advertise_address", c.advertise),
		mlog.Bool("encryption", *cs.EnableGossipEncryption),
		mlog.Bool("compression", *cs.EnableGossipCompression),
	)

	c.registerDiscovery()
	c.joinFromDiscovery(true)

	c.wg.Add(4)
	go c.discoveryLoop()
	go c.joinLoop()
	go c.membershipLoop()
	go c.maintenanceLoop()

	c.updateLeader()
	return nil
}

// StopInterNodeCommunication leaves the cluster.
func (c *Cluster) StopInterNodeCommunication() {
	c.lifecycleMu.Lock()
	defer c.lifecycleMu.Unlock()

	if !c.running.Load() {
		return
	}
	c.running.Store(false)
	close(c.stopCh)

	if c.cfgListener != "" {
		c.host.RemoveConfigListener(c.cfgListener)
		c.cfgListener = ""
	}

	if err := c.list.Leave(leaveTimeout); err != nil {
		c.logger().Warn("Cluster: failed to gracefully leave the cluster", mlog.Err(err))
	}
	if err := c.list.Shutdown(); err != nil {
		c.logger().Warn("Cluster: failed to shut down the gossip listener", mlog.Err(err))
	}

	c.nodesMu.Lock()
	for name, s := range c.senders {
		s.close()
		delete(c.senders, name)
	}
	c.nodes = make(map[string]*peer)
	c.nodesMu.Unlock()

	c.rpc.failAll(errNotRunning)
	c.dispatcher.stop()
	c.wg.Wait()
	c.fragments.reset()
	c.seq.reset()

	c.unregisterDiscovery()

	c.leaderMu.Lock()
	c.leaderID = ""
	c.isLeader.Store(false)
	c.leaderMu.Unlock()

	c.logger().Info("Cluster: inter-node communication stopped", mlog.String("node_id", c.id))
}

// Shutdown releases all the resources held by the cluster interface. It is
// safe to call it after StopInterNodeCommunication, and any later cluster
// send is a no-op.
func (c *Cluster) Shutdown() {
	c.StopInterNodeCommunication()
}

func (c *Cluster) RegisterClusterMessageHandler(event model.ClusterEvent, crm einterfaces.ClusterMessageHandler) {
	c.handlersMu.Lock()
	defer c.handlersMu.Unlock()
	c.handlers[event] = crm
}

func (c *Cluster) handler(event model.ClusterEvent) einterfaces.ClusterMessageHandler {
	c.handlersMu.RLock()
	defer c.handlersMu.RUnlock()
	return c.handlers[event]
}

func (c *Cluster) GetClusterId() string {
	return c.id
}

func (c *Cluster) IsLeader() bool {
	return c.isLeader.Load()
}

func (c *Cluster) HealthScore() int {
	c.nodesMu.RLock()
	list := c.list
	c.nodesMu.RUnlock()
	if !c.running.Load() || list == nil {
		return 0
	}
	return list.GetHealthScore()
}

func (c *Cluster) GetMyClusterInfo() *model.ClusterInfo {
	if c.running.Load() {
		c.metaMu.RLock()
		defer c.metaMu.RUnlock()
		return c.meta.clusterInfo()
	}

	cfg := c.host.Config()
	_, ip, _ := resolveAddresses(&cfg.ClusterSettings)
	return &model.ClusterInfo{
		Id:         c.id,
		Version:    model.CurrentVersion,
		ConfigHash: configHash(cfg),
		IPAddress:  ip,
		Hostname:   displayHostname(&cfg.ClusterSettings, ip),
	}
}

// GetClusterInfos returns the information about every live node of the
// cluster, this node included.
func (c *Cluster) GetClusterInfos() ([]*model.ClusterInfo, error) {
	if !c.running.Load() {
		return []*model.ClusterInfo{}, nil
	}

	infos := []*model.ClusterInfo{c.GetMyClusterInfo()}
	for _, p := range c.peers() {
		if !p.metaOK {
			infos = append(infos, &model.ClusterInfo{
				Id:        p.node.Name,
				IPAddress: p.node.Addr.String(),
			})
			continue
		}
		infos = append(infos, p.meta.clusterInfo())
	}
	return infos, nil
}

// peers returns a snapshot of the other live nodes.
func (c *Cluster) peers() []*peer {
	c.nodesMu.RLock()
	defer c.nodesMu.RUnlock()
	out := make([]*peer, 0, len(c.nodes))
	for name, p := range c.nodes {
		if name == c.id {
			continue
		}
		out = append(out, p)
	}
	return out
}

func (c *Cluster) peer(name string) (*peer, bool) {
	c.nodesMu.RLock()
	defer c.nodesMu.RUnlock()
	p, ok := c.nodes[name]
	return p, ok
}

// --- membership ------------------------------------------------------------

func (c *Cluster) onNodeUpsert(n *memberlist.Node) {
	if !c.running.Load() {
		return
	}
	p := &peer{node: *n}
	if len(n.Meta) > 0 {
		if err := json.Unmarshal(n.Meta, &p.meta); err == nil && p.meta.ID == n.Name {
			p.metaOK = true
		}
	}
	if !p.metaOK && n.Name != c.id {
		c.logger().Warn("Cluster: node joined with unreadable metadata", mlog.String("node_id", n.Name), mlog.String("address", n.Address()))
	}

	c.nodesMu.Lock()
	_, existed := c.nodes[n.Name]
	c.nodes[n.Name] = p
	c.nodesMu.Unlock()

	if !existed && n.Name != c.id {
		c.logger().Info("Cluster: node joined", mlog.String("node_id", n.Name), mlog.String("hostname", p.meta.Hostname), mlog.String("address", n.Address()))
	}
	c.signalMembershipChange()
}

func (c *Cluster) onNodeLeave(n *memberlist.Node) {
	c.nodesMu.Lock()
	delete(c.nodes, n.Name)
	s := c.senders[n.Name]
	delete(c.senders, n.Name)
	c.nodesMu.Unlock()

	if s != nil {
		s.close()
	}
	c.rpc.nodeLeft(n.Name)
	c.seq.forget(n.Name)

	if n.Name != c.id {
		c.logger().Info("Cluster: node left", mlog.String("node_id", n.Name), mlog.String("address", n.Address()))
	}
	c.signalMembershipChange()
}

func (c *Cluster) signalMembershipChange() {
	select {
	case c.membersCh <- struct{}{}:
	default:
	}
}

func (c *Cluster) membershipLoop() {
	defer c.wg.Done()
	for {
		select {
		case <-c.stopCh:
			return
		case <-c.membersCh:
			c.updateLeader()
		}
	}
}

// electLeader deterministically picks the leader among the live nodes: the
// node which started first, ties being broken by node id. Every node sees
// the same membership once gossip converges, so they all agree.
func (c *Cluster) electLeader() string {
	c.metaMu.RLock()
	self := c.meta
	c.metaMu.RUnlock()

	best := self
	for _, p := range c.peers() {
		if !p.metaOK {
			continue
		}
		m := p.meta
		if m.StartAt < best.StartAt || (m.StartAt == best.StartAt && m.ID < best.ID) {
			best = m
		}
	}
	return best.ID
}

func (c *Cluster) updateLeader() {
	if !c.running.Load() {
		return
	}

	c.leaderMu.Lock()
	leader := c.electLeader()
	changed := leader != c.leaderID
	c.leaderID = leader
	c.isLeader.Store(leader == c.id)
	c.leaderMu.Unlock()

	if changed {
		c.logger().Info("Cluster: leader elected", mlog.String("leader_id", leader), mlog.Bool("is_leader", leader == c.id))
		c.host.InvokeClusterLeaderChangedListeners()
	}
}

// --- metadata --------------------------------------------------------------

func (c *Cluster) nodeMeta(limit int) []byte {
	c.metaMu.RLock()
	m := c.meta
	c.metaMu.RUnlock()

	for {
		b, err := json.Marshal(m)
		if err != nil {
			return nil
		}
		if len(b) <= limit {
			return b
		}
		if len(m.Hostname) == 0 {
			return nil
		}
		// Hostname is the only unbounded field.
		cut := min(len(b)-limit, len(m.Hostname))
		m.Hostname = m.Hostname[:len(m.Hostname)-cut]
	}
}

func (c *Cluster) refreshConfigHash(cfg *model.Config) {
	h := configHash(cfg)
	c.metaMu.Lock()
	changed := c.meta.ConfigHash != h
	c.meta.ConfigHash = h
	c.metaMu.Unlock()

	if !changed || !c.running.Load() {
		return
	}
	c.nodesMu.RLock()
	list := c.list
	c.nodesMu.RUnlock()
	go func() {
		if err := list.UpdateNode(updateMetaTimeout); err != nil {
			c.logger().Debug("Cluster: failed to propagate node metadata", mlog.Err(err))
		}
	}()
}

// configHash returns a fingerprint of the configuration, ignoring the
// settings which are expected to differ between nodes.
func configHash(cfg *model.Config) string {
	if cfg == nil {
		return ""
	}
	clone := cfg.Clone()
	empty := ""
	clone.ClusterSettings.OverrideHostname = &empty
	clone.ClusterSettings.NetworkInterface = &empty
	clone.ClusterSettings.BindAddress = &empty
	clone.ClusterSettings.AdvertiseAddress = &empty
	b, err := json.Marshal(clone)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func clusterLabel(clusterName string) string {
	label := labelPrefix + clusterName
	if len(label) > memberlist.LabelMaxSize {
		sum := sha256.Sum256([]byte(clusterName))
		label = labelPrefix + hex.EncodeToString(sum[:])
	}
	return label
}

// --- memberlist delegates --------------------------------------------------

type delegate struct {
	c *Cluster
}

func (d *delegate) NodeMeta(limit int) []byte                  { return d.c.nodeMeta(limit) }
func (d *delegate) NotifyMsg(buf []byte)                       { d.c.NotifyMsg(buf) }
func (d *delegate) GetBroadcasts(overhead, limit int) [][]byte { return nil }
func (d *delegate) LocalState(join bool) []byte                { return nil }
func (d *delegate) MergeRemoteState(buf []byte, join bool)     {}

// eventDelegate callbacks are invoked by memberlist while it holds internal
// locks: they must not call back into memberlist.
type eventDelegate struct {
	c *Cluster
}

func (e *eventDelegate) NotifyJoin(n *memberlist.Node)   { e.c.onNodeUpsert(n) }
func (e *eventDelegate) NotifyUpdate(n *memberlist.Node) { e.c.onNodeUpsert(n) }
func (e *eventDelegate) NotifyLeave(n *memberlist.Node)  { e.c.onNodeLeave(n) }

// memberlistLogWriter forwards memberlist's logs to mlog.
type memberlistLogWriter struct {
	c *Cluster
}

var _ io.Writer = (*memberlistLogWriter)(nil)

func (w *memberlistLogWriter) Write(p []byte) (int, error) {
	line := string(p)
	for len(line) > 0 && (line[len(line)-1] == '\n' || line[len(line)-1] == '\r') {
		line = line[:len(line)-1]
	}
	logger := w.c.logger()
	switch {
	case hasLevel(line, "[ERR]"), hasLevel(line, "[ERROR]"):
		logger.Error("Cluster: "+trimLevel(line), mlog.String("source", "memberlist"))
	case hasLevel(line, "[WARN]"):
		logger.Warn("Cluster: "+trimLevel(line), mlog.String("source", "memberlist"))
	case hasLevel(line, "[INFO]"):
		logger.Info("Cluster: "+trimLevel(line), mlog.String("source", "memberlist"))
	default:
		logger.Debug("Cluster: "+trimLevel(line), mlog.String("source", "memberlist"))
	}
	return len(p), nil
}

func hasLevel(line, level string) bool {
	return len(line) >= len(level) && line[:len(level)] == level
}

func trimLevel(line string) string {
	if len(line) > 0 && line[0] == '[' {
		for i := 1; i < len(line); i++ {
			if line[i] == ']' {
				line = line[i+1:]
				break
			}
		}
	}
	for len(line) > 0 && line[0] == ' ' {
		line = line[1:]
	}
	return line
}
