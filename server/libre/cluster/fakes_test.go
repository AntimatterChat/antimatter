// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package cluster

import (
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/memberlist"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
	"github.com/mattermost/mattermost/server/v8/einterfaces"
)

// fakeDB is shared by all the nodes of a test cluster.
type fakeDB struct {
	mu        sync.Mutex
	discovery map[string]*model.ClusterDiscovery
	system    map[string]string
}

func newFakeDB() *fakeDB {
	return &fakeDB{discovery: map[string]*model.ClusterDiscovery{}, system: map[string]string{}}
}

type fakeDiscoveryStore struct{ db *fakeDB }

func discoKey(d *model.ClusterDiscovery) string {
	return d.Type + "|" + d.ClusterName + "|" + d.Hostname
}

func (s *fakeDiscoveryStore) Save(d *model.ClusterDiscovery) error {
	d.PreSave()
	s.db.mu.Lock()
	defer s.db.mu.Unlock()
	cp := *d
	s.db.discovery[d.Id] = &cp
	return nil
}

func (s *fakeDiscoveryStore) Delete(d *model.ClusterDiscovery) (bool, error) {
	s.db.mu.Lock()
	defer s.db.mu.Unlock()
	found := false
	for id, e := range s.db.discovery {
		if discoKey(e) == discoKey(d) {
			delete(s.db.discovery, id)
			found = true
		}
	}
	return found, nil
}

func (s *fakeDiscoveryStore) Exists(d *model.ClusterDiscovery) (bool, error) {
	s.db.mu.Lock()
	defer s.db.mu.Unlock()
	for _, e := range s.db.discovery {
		if discoKey(e) == discoKey(d) {
			return true, nil
		}
	}
	return false, nil
}

func (s *fakeDiscoveryStore) GetAll(typ, name string) ([]*model.ClusterDiscovery, error) {
	s.db.mu.Lock()
	defer s.db.mu.Unlock()
	var out []*model.ClusterDiscovery
	for _, e := range s.db.discovery {
		if e.Type == typ && e.ClusterName == name {
			cp := *e
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (s *fakeDiscoveryStore) SetLastPingAt(d *model.ClusterDiscovery) error {
	s.db.mu.Lock()
	defer s.db.mu.Unlock()
	for _, e := range s.db.discovery {
		if discoKey(e) == discoKey(d) {
			e.LastPingAt = model.GetMillis()
		}
	}
	return nil
}

func (s *fakeDiscoveryStore) Cleanup() error { return nil }

// fakeSystemStore only implements what the cluster uses.
type fakeSystemStore struct {
	store.SystemStore
	db *fakeDB
}

func (s *fakeSystemStore) InsertIfExists(sys *model.System) (*model.System, error) {
	s.db.mu.Lock()
	defer s.db.mu.Unlock()
	if v, ok := s.db.system[sys.Name]; ok && v != "" {
		return &model.System{Name: sys.Name, Value: v}, nil
	}
	s.db.system[sys.Name] = sys.Value
	return sys, nil
}

type fakeHost struct {
	t      testing.TB
	db     *fakeDB
	logger *mlog.Logger

	mu        sync.Mutex
	cfg       *model.Config
	listeners map[string]func(oldCfg, newCfg *model.Config)
	applied   []*model.Config

	leaderChanges atomic.Int32
	wsConns       int
	webConns      map[string]int
	wsQueues      map[string]*model.WSQueues
	blockWebConn  chan struct{}
	supportFiles  []model.FileData
	logLines      []string
}

func newFakeHost(t testing.TB, db *fakeDB, ip string, port int, encrypt bool) *fakeHost {
	cfg := &model.Config{}
	cfg.SetDefaults()
	*cfg.ClusterSettings.Enable = true
	*cfg.ClusterSettings.ClusterName = "test"
	*cfg.ClusterSettings.BindAddress = ip
	*cfg.ClusterSettings.GossipPort = port
	*cfg.ClusterSettings.UseIPAddress = true
	*cfg.ClusterSettings.EnableGossipEncryption = encrypt
	*cfg.ClusterSettings.EnableGossipCompression = true
	return &fakeHost{
		t:         t,
		db:        db,
		logger:    mlog.CreateConsoleTestLogger(t),
		cfg:       cfg,
		listeners: map[string]func(oldCfg, newCfg *model.Config){},
		webConns:  map[string]int{},
		wsQueues:  map[string]*model.WSQueues{},
	}
}

func (h *fakeHost) Config() *model.Config {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.cfg
}
func (h *fakeHost) Log() mlog.LoggerIFace                 { return h.logger }
func (h *fakeHost) Metrics() einterfaces.MetricsInterface { return nil }
func (h *fakeHost) SchemaVersion() string                 { return "42" }
func (h *fakeHost) InvokeClusterLeaderChangedListeners()  { h.leaderChanges.Add(1) }
func (h *fakeHost) TotalWebsocketConnections() int        { return h.wsConns }
func (h *fakeHost) TotalMasterDbConnections() int         { return 3 }
func (h *fakeHost) TotalReadDbConnections() int           { return 2 }
func (h *fakeHost) SystemStore() store.SystemStore        { return &fakeSystemStore{db: h.db} }
func (h *fakeHost) ClusterDiscoveryStore() store.ClusterDiscoveryStore {
	return &fakeDiscoveryStore{db: h.db}
}

func (h *fakeHost) AddConfigListener(l func(oldCfg, newCfg *model.Config)) string {
	h.mu.Lock()
	defer h.mu.Unlock()
	id := model.NewId()
	h.listeners[id] = l
	return id
}

func (h *fakeHost) RemoveConfigListener(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.listeners, id)
}

func (h *fakeHost) ApplyRemoteConfig(cfg *model.Config) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.applied = append(h.applied, cfg)
	return nil
}

func (h *fakeHost) appliedConfigs() []*model.Config {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]*model.Config(nil), h.applied...)
}

func (h *fakeHost) GetLogsSkipSend(rctx request.CTX, page, perPage int, logFilter *model.LogFilter) ([]string, *model.AppError) {
	return h.logLines, nil
}

func (h *fakeHost) GenerateSupportPacket(rctx request.CTX, options *model.SupportPacketOptions) ([]model.FileData, error) {
	out := make([]model.FileData, len(h.supportFiles))
	copy(out, h.supportFiles)
	return out, nil
}

func (h *fakeHost) GetPluginStatuses() (model.PluginStatuses, *model.AppError) {
	return model.PluginStatuses{{PluginId: "p1", Version: "1.0.0"}}, nil
}

func (h *fakeHost) WebConnCountForUser(userID string) int {
	if h.blockWebConn != nil {
		<-h.blockWebConn
	}
	return h.webConns[userID]
}

func (h *fakeHost) GetWSQueues(userID, connectionID string, seqNum int64) (*model.WSQueues, error) {
	return h.wsQueues[connectionID], nil
}

func freePort(t testing.TB, ip string) int {
	for range 50 {
		l, err := net.Listen("tcp", net.JoinHostPort(ip, "0"))
		require.NoError(t, err)
		port := l.Addr().(*net.TCPAddr).Port
		u, err := net.ListenPacket("udp", net.JoinHostPort(ip, strconv.Itoa(port)))
		l.Close()
		if err != nil {
			continue
		}
		u.Close()
		return port
	}
	t.Fatal("no free port")
	return 0
}

var nodeCounter atomic.Int32

// nextIP gives each test node its own loopback address: the discovery
// entries of the nodes are keyed on their hostname.
func nextIP() string {
	n := nodeCounter.Add(1)
	return "127.0." + strconv.Itoa(int(n/250)) + "." + strconv.Itoa(int(n%250)+1)
}

func newFakeHostForNode(t testing.TB, db *fakeDB, encrypt bool) *fakeHost {
	ip := nextIP()
	return newFakeHost(t, db, ip, freePort(t, ip), encrypt)
}

// newTestCluster creates a started cluster node tuned for fast failure
// detection. setup runs before the node starts.
func newTestCluster(t testing.TB, host *fakeHost, setup func(c *Cluster)) *Cluster {
	c := New(host)
	c.joinIntervalLone = 200 * time.Millisecond
	c.joinInterval = time.Second
	c.discoveryPing = time.Second
	c.timeouts = rpcTimeouts{short: 2 * time.Second, normal: 5 * time.Second, logs: 5 * time.Second, supportPacket: 20 * time.Second}
	c.tuneMemberlist = func(conf *memberlist.Config) {
		conf.ProbeInterval = 200 * time.Millisecond
		conf.ProbeTimeout = 100 * time.Millisecond
		conf.GossipInterval = 50 * time.Millisecond
		conf.PushPullInterval = time.Second
		conf.SuspicionMult = 2
		conf.TCPTimeout = 5 * time.Second
	}
	if setup != nil {
		setup(c)
	}
	c.StartInterNodeCommunication()
	require.True(t, c.running.Load())
	t.Cleanup(c.Shutdown)
	return c
}
