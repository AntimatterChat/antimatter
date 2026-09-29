// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package cluster

import (
	"bytes"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/memberlist"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
)

const (
	// Best-effort messages are sent over UDP only when they fit comfortably
	// in a single datagram; larger ones fall back to the reliable transport.
	maxBestEffortSize = 1200

	// Payloads larger than this are split into fragments, keeping every
	// memberlist stream (which is limited to 20MB when encrypted) small.
	fragmentSize = 4 * 1024 * 1024
	maxFragments = 256

	// Reliable sends are batched per peer up to this many bytes / items.
	batchMaxBytes = 1024 * 1024
	batchMaxItems = 128

	senderQueueSize   = 10000
	sendAttempts      = 3
	waitForAllTimeout = 30 * time.Second
	enqueueTimeout    = 10 * time.Second
	fragmentTTL       = 2 * time.Minute
)

var errQueueFull = errors.New("send queue is full")

type outItem struct {
	payload []byte
	done    chan error // nil when nobody waits for the result
}

// peerSender delivers reliable payloads to one peer, in order, batching
// payloads queued while a previous send is in flight.
type peerSender struct {
	c     *Cluster
	name  string
	queue chan *outItem
	quit  chan struct{}
	once  sync.Once

	// Only used by the run goroutine.
	epoch uint64
	seq   uint64
}

func (s *peerSender) close() {
	s.once.Do(func() { close(s.quit) })
}

func (s *peerSender) run() {
	defer s.c.wg.Done()
	for {
		select {
		case <-s.quit:
			s.failPending(errNodeGone)
			return
		case <-s.c.stopCh:
			s.failPending(errNotRunning)
			return
		case it := <-s.queue:
			items := []*outItem{it}
			size := len(it.payload)
		drain:
			for size < batchMaxBytes && len(items) < batchMaxItems {
				select {
				case next := <-s.queue:
					items = append(items, next)
					size += len(next.payload)
				default:
					break drain
				}
			}
			err := s.send(items)
			for _, it := range items {
				if it.done != nil {
					it.done <- err
				}
			}
		}
	}
}

func (s *peerSender) failPending(err error) {
	for {
		select {
		case it := <-s.queue:
			if it.done != nil {
				it.done <- err
			}
		default:
			return
		}
	}
}

func (s *peerSender) send(items []*outItem) error {
	var payload []byte
	if len(items) == 1 {
		payload = items[0].payload
	} else {
		payloads := make([][]byte, len(items))
		for i, it := range items {
			payloads[i] = it.payload
		}
		payload = encodeBatch(payloads)
	}
	payload = encodeSequenced(s.c.id, s.epoch, s.seq, payload)

	var err error
	for attempt := range sendAttempts {
		if attempt > 0 {
			select {
			case <-s.quit:
				return errNodeGone
			case <-s.c.stopCh:
				return errNotRunning
			case <-time.After(time.Duration(attempt*attempt) * 250 * time.Millisecond):
			}
		}
		p, ok := s.c.peer(s.name)
		if !ok {
			return errNodeGone
		}
		list := s.c.memberlist()
		if list == nil {
			return errNotRunning
		}
		node := p.node
		if err = list.SendReliable(&node, payload); err == nil {
			s.seq++
			return nil
		}
		s.c.logger().Debug("Cluster: reliable send failed", mlog.String("node_id", s.name), mlog.Int("attempt", attempt+1), mlog.Err(err))
	}
	s.c.logger().Warn("Cluster: failed to send to node", mlog.String("node_id", s.name), mlog.Int("messages", len(items)), mlog.Err(err))
	// The peer may or may not have received the payload: start a new epoch
	// so that it doesn't wait for this sequence number.
	s.epoch = s.c.epochs.Add(1)
	s.seq = 0
	return err
}

func (c *Cluster) memberlist() *memberlist.Memberlist {
	c.nodesMu.RLock()
	defer c.nodesMu.RUnlock()
	return c.list
}

func (c *Cluster) sender(name string) (*peerSender, error) {
	c.nodesMu.Lock()
	defer c.nodesMu.Unlock()
	// Checked under the lock: StopInterNodeCommunication clears the running
	// flag before closing the senders under the same lock, so no sender can
	// be created once the stop is under way.
	if !c.running.Load() {
		return nil, errNotRunning
	}
	if s, ok := c.senders[name]; ok {
		return s, nil
	}
	if _, ok := c.nodes[name]; !ok || name == c.id {
		return nil, errNodeGone
	}
	s := &peerSender{
		c:     c,
		name:  name,
		queue: make(chan *outItem, senderQueueSize),
		quit:  make(chan struct{}),
		epoch: c.epochs.Add(1),
	}
	c.senders[name] = s
	c.wg.Add(1)
	go s.run()
	return s, nil
}

// enqueueReliable queues payloads (already split into fragments if needed)
// for the given peer. When block is false, a full queue drops the payloads.
// The returned channels receive the delivery result of each payload.
func (c *Cluster) enqueueReliable(name string, payloads [][]byte, wait, block bool) ([]chan error, error) {
	s, err := c.sender(name)
	if err != nil {
		return nil, err
	}
	var dones []chan error
	var timer <-chan time.Time
	for _, p := range payloads {
		it := &outItem{payload: p}
		if wait {
			it.done = make(chan error, 1)
			dones = append(dones, it.done)
		}
		if !block {
			select {
			case s.queue <- it:
				continue
			default:
				return dones, errQueueFull
			}
		}
		if timer == nil {
			timer = time.After(enqueueTimeout)
		}
		select {
		case s.queue <- it:
		case <-s.quit:
			return dones, errNodeGone
		case <-c.stopCh:
			return dones, errNotRunning
		case <-timer:
			return dones, errQueueFull
		}
	}
	return dones, nil
}

func splitPayload(payload []byte) [][]byte {
	if len(payload) <= fragmentSize {
		return [][]byte{payload}
	}
	return splitIntoFragments(payload, fragmentSize)
}

// sendReliableTo delivers payload to a single node and waits for the result.
func (c *Cluster) sendReliableTo(name string, payload []byte, timeout time.Duration) error {
	dones, err := c.enqueueReliable(name, splitPayload(payload), true, true)
	if err != nil {
		return err
	}
	return waitAll(dones, timeout)
}

func waitAll(dones []chan error, timeout time.Duration) error {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	var firstErr error
	for _, d := range dones {
		select {
		case err := <-d:
			if err != nil && firstErr == nil {
				firstErr = err
			}
		case <-timer.C:
			return fmt.Errorf("timed out after %s waiting for delivery", timeout)
		}
	}
	return firstErr
}

// SendClusterMessage sends msg to every other node of the cluster.
func (c *Cluster) SendClusterMessage(msg *model.ClusterMessage) {
	if msg == nil || !c.running.Load() {
		return
	}
	peers := c.peers()
	if len(peers) == 0 {
		return
	}

	if metrics := c.host.Metrics(); metrics != nil {
		metrics.IncrementClusterEventType(msg.Event)
	}

	if msg.SendType != model.ClusterSendReliable {
		payload := encodeClusterMessage(msg, flagBestEffort)
		if len(payload) <= maxBestEffortSize {
			c.sendBestEffort(peers, payload, msg)
			return
		}
		if metrics := c.host.Metrics(); metrics != nil {
			metrics.ObserveClusterReliableFallbackLength(msg.Event, len(payload))
		}
		// Too large for a datagram: use the reliable transport, but keep the
		// best-effort semantics (never block, drop when overloaded).
		for _, p := range peers {
			if _, err := c.enqueueReliable(p.node.Name, splitPayload(payload), false, msg.WaitForAllToSend); err != nil {
				c.logger().Debug("Cluster: dropped best-effort message", append(msg.LogFields(), mlog.String("node_id", p.node.Name), mlog.Err(err))...)
			}
		}
		return
	}

	payloads := splitPayload(encodeClusterMessage(msg, 0))
	var dones []chan error
	for _, p := range peers {
		d, err := c.enqueueReliable(p.node.Name, payloads, msg.WaitForAllToSend, msg.WaitForAllToSend || len(payloads) > 1)
		if err != nil {
			c.logger().Error("Cluster: failed to queue reliable message", append(msg.LogFields(), mlog.String("node_id", p.node.Name), mlog.Err(err))...)
		}
		dones = append(dones, d...)
	}
	if msg.WaitForAllToSend {
		if err := waitAll(dones, waitForAllTimeout); err != nil {
			c.logger().Error("Cluster: failed to deliver message to all nodes", append(msg.LogFields(), mlog.Err(err))...)
		}
	}
}

func (c *Cluster) sendBestEffort(peers []*peer, payload []byte, msg *model.ClusterMessage) {
	list := c.memberlist()
	if list == nil {
		return
	}
	for _, p := range peers {
		node := p.node
		if err := list.SendBestEffort(&node, payload); err != nil {
			c.logger().Debug("Cluster: best-effort send failed", append(msg.LogFields(), mlog.String("node_id", node.Name), mlog.Err(err))...)
		}
	}
}

// SendClusterMessageToNode sends msg to a single node, identified by its
// cluster id, and reports delivery failures.
func (c *Cluster) SendClusterMessageToNode(nodeID string, msg *model.ClusterMessage) error {
	if msg == nil {
		return errors.New("nil cluster message")
	}
	if !c.running.Load() {
		return errNotRunning
	}
	p, ok := c.peer(nodeID)
	if !ok || nodeID == c.id {
		return fmt.Errorf("%w: %s", errNodeGone, nodeID)
	}

	if metrics := c.host.Metrics(); metrics != nil {
		metrics.IncrementClusterEventType(msg.Event)
	}

	if msg.SendType != model.ClusterSendReliable {
		payload := encodeClusterMessage(msg, flagBestEffort)
		if len(payload) <= maxBestEffortSize {
			list := c.memberlist()
			if list == nil {
				return errNotRunning
			}
			node := p.node
			return list.SendBestEffort(&node, payload)
		}
		if metrics := c.host.Metrics(); metrics != nil {
			metrics.ObserveClusterReliableFallbackLength(msg.Event, len(payload))
		}
		return c.sendReliableTo(nodeID, payload, waitForAllTimeout)
	}
	return c.sendReliableTo(nodeID, encodeClusterMessage(msg, 0), waitForAllTimeout)
}

// NotifyMsg is called by memberlist for every user message received.
func (c *Cluster) NotifyMsg(buf []byte) {
	if !c.running.Load() || len(buf) == 0 {
		return
	}
	// memberlist may reuse the buffer, and messages are handled
	// asynchronously.
	c.handlePayload(bytes.Clone(buf), 0)
}

func (c *Cluster) handlePayload(buf []byte, depth int) {
	kind, flags, body, err := parseEnvelope(buf)
	if err != nil {
		c.logger().Warn("Cluster: discarding malformed message", mlog.Int("size", len(buf)))
		return
	}

	switch kind {
	case kindMessage:
		msg, err := decodeClusterMessage(body)
		if err != nil {
			c.logger().Warn("Cluster: discarding malformed cluster message", mlog.Err(err))
			return
		}
		c.dispatcher.dispatch(msg, flags&flagBestEffort != 0)
	case kindSequenced:
		if depth > 0 {
			c.logger().Warn("Cluster: discarding nested sequenced message")
			return
		}
		c.seq.handle(body)
	case kindBatch:
		if depth > 1 {
			c.logger().Warn("Cluster: discarding nested batch")
			return
		}
		items, err := decodeBatch(body)
		if err != nil {
			c.logger().Warn("Cluster: discarding malformed batch", mlog.Err(err))
			return
		}
		for _, it := range items {
			c.handlePayload(it, depth+1)
		}
	case kindFragment:
		f, err := decodeFragment(body)
		if err != nil {
			c.logger().Warn("Cluster: discarding malformed fragment", mlog.Err(err))
			return
		}
		if full := c.fragments.add(f); full != nil {
			c.handlePayload(full, depth+1)
		}
	case kindRequest:
		c.rpc.handleRequest(body)
	case kindResponse:
		c.rpc.handleResponse(body)
	default:
		c.logger().Warn("Cluster: discarding message of unknown kind", mlog.Int("kind", int(kind)))
	}
}

// --- fragment reassembly ---------------------------------------------------

type partial struct {
	parts    [][]byte
	received int
	size     int
	updated  time.Time
}

type reassembler struct {
	c       *Cluster
	mu      sync.Mutex
	pending map[string]*partial
}

func newReassembler(c *Cluster) *reassembler {
	return &reassembler{c: c, pending: make(map[string]*partial)}
}

// add stores a fragment and returns the reassembled payload once all the
// fragments have been received.
func (r *reassembler) add(f *fragment) []byte {
	r.mu.Lock()
	defer r.mu.Unlock()

	p, ok := r.pending[f.id]
	if !ok {
		p = &partial{parts: make([][]byte, f.total)}
		r.pending[f.id] = p
	}
	if len(p.parts) != f.total {
		delete(r.pending, f.id)
		return nil
	}
	p.updated = time.Now()
	if p.parts[f.index] == nil {
		p.parts[f.index] = f.data
		p.received++
		p.size += len(f.data)
	}
	if p.received < f.total {
		return nil
	}
	delete(r.pending, f.id)
	full := make([]byte, 0, p.size)
	for _, part := range p.parts {
		full = append(full, part...)
	}
	return full
}

func (r *reassembler) gc(now time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, p := range r.pending {
		if now.Sub(p.updated) > fragmentTTL {
			delete(r.pending, id)
		}
	}
}

func (r *reassembler) reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pending = make(map[string]*partial)
}

// maintenanceLoop drops stale partial payloads and unblocks message streams
// waiting for a sequence number that never arrived.
func (c *Cluster) maintenanceLoop() {
	defer c.wg.Done()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-c.stopCh:
			return
		case now := <-ticker.C:
			c.fragments.gc(now)
			c.seq.skipGaps(now)
		}
	}
}
