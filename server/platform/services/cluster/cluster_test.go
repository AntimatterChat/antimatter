// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package cluster

import (
	"bytes"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

const waitFor = 10 * time.Second
const tick = 20 * time.Millisecond

// recorder collects the cluster messages received by a node.
type recorder struct {
	mu   sync.Mutex
	msgs []*model.ClusterMessage
}

func (r *recorder) handle(msg *model.ClusterMessage) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.msgs = append(r.msgs, msg)
}

func (r *recorder) all() []*model.ClusterMessage {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]*model.ClusterMessage(nil), r.msgs...)
}

func (r *recorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.msgs)
}

func (r *recorder) reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.msgs = nil
}

const testEvent = model.ClusterEvent("test_event")

// startCluster starts n nodes one after the other and waits for them to see
// each other.
func startCluster(t *testing.T, n int, encrypt bool) ([]*Cluster, []*fakeHost, []*recorder) {
	db := newFakeDB()
	var nodes []*Cluster
	var hosts []*fakeHost
	var recs []*recorder
	for range n {
		rec := &recorder{}
		host := newFakeHostForNode(t, db, encrypt)
		c := newTestCluster(t, host, func(c *Cluster) {
			c.RegisterClusterMessageHandler(testEvent, rec.handle)
		})
		nodes = append(nodes, c)
		hosts = append(hosts, host)
		recs = append(recs, rec)
		// Make sure start times differ, so the leader is deterministic.
		time.Sleep(5 * time.Millisecond)
	}
	for _, c := range nodes {
		require.Eventually(t, func() bool { return len(c.peers()) == n-1 }, waitFor, tick, "nodes did not discover each other")
	}
	return nodes, hosts, recs
}

func TestClusterMembershipAndLeader(t *testing.T) {
	nodes, hosts, _ := startCluster(t, 3, true)

	for _, c := range nodes {
		require.Eventually(t, func() bool { return c.electLeader() == nodes[0].id }, waitFor, tick)
	}
	require.Eventually(t, func() bool { return nodes[0].IsLeader() }, waitFor, tick)
	assert.False(t, nodes[1].IsLeader())
	assert.False(t, nodes[2].IsLeader())
	assert.GreaterOrEqual(t, hosts[0].leaderChanges.Load(), int32(1))

	infos, err := nodes[1].GetClusterInfos()
	require.NoError(t, err)
	require.Len(t, infos, 3)
	ids := map[string]bool{}
	for _, info := range infos {
		ids[info.Id] = true
		assert.Equal(t, model.CurrentVersion, info.Version)
		assert.Equal(t, "42", info.SchemaVersion)
		assert.NotEmpty(t, info.ConfigHash)
		assert.NotEmpty(t, info.IPAddress)
		assert.NotEmpty(t, info.Hostname)
	}
	for _, c := range nodes {
		assert.True(t, ids[c.GetClusterId()])
	}
	assert.Equal(t, nodes[2].id, nodes[2].GetMyClusterInfo().Id)

	// The leader leaves: the next oldest node takes over.
	nodes[0].StopInterNodeCommunication()
	assert.False(t, nodes[0].IsLeader())
	require.Eventually(t, func() bool { return nodes[1].IsLeader() }, waitFor, tick)
	require.Eventually(t, func() bool { return len(nodes[2].peers()) == 1 }, waitFor, tick)
	assert.False(t, nodes[2].IsLeader())

	// It comes back: it is now the youngest node and doesn't take over.
	nodes[0].StartInterNodeCommunication()
	require.Eventually(t, func() bool { return len(nodes[0].peers()) == 2 }, waitFor, tick)
	require.Eventually(t, func() bool { return len(nodes[1].peers()) == 2 }, waitFor, tick)
	assert.True(t, nodes[1].IsLeader())
	assert.False(t, nodes[0].IsLeader())
}

func TestClusterMessages(t *testing.T) {
	nodes, _, recs := startCluster(t, 3, true)

	t.Run("best effort", func(t *testing.T) {
		nodes[0].SendClusterMessage(&model.ClusterMessage{Event: testEvent, SendType: model.ClusterSendBestEffort, Data: []byte("hello")})
		for _, r := range recs[1:] {
			require.Eventually(t, func() bool { return r.count() == 1 }, waitFor, tick)
			assert.Equal(t, []byte("hello"), r.all()[0].Data)
		}
		assert.Zero(t, recs[0].count(), "messages are not looped back")
	})

	t.Run("large best effort falls back to reliable", func(t *testing.T) {
		for _, r := range recs {
			r.reset()
		}
		data := bytes.Repeat([]byte("x"), 100*1024)
		nodes[1].SendClusterMessage(&model.ClusterMessage{Event: testEvent, Data: data, Props: map[string]string{"k": "v"}})
		for _, i := range []int{0, 2} {
			require.Eventually(t, func() bool { return recs[i].count() == 1 }, waitFor, tick)
			assert.Equal(t, data, recs[i].all()[0].Data)
			assert.Equal(t, "v", recs[i].all()[0].Props["k"])
		}
	})

	t.Run("reliable with fragments, waiting for delivery", func(t *testing.T) {
		for _, r := range recs {
			r.reset()
		}
		data := bytes.Repeat([]byte("0123456789"), 1024*1024) // 10MB: 3 fragments
		nodes[2].SendClusterMessage(&model.ClusterMessage{Event: testEvent, SendType: model.ClusterSendReliable, WaitForAllToSend: true, Data: data})
		for _, i := range []int{0, 1} {
			require.Eventually(t, func() bool { return recs[i].count() == 1 }, waitFor, tick)
			assert.Equal(t, data, recs[i].all()[0].Data)
		}
	})

	t.Run("reliable messages keep their order", func(t *testing.T) {
		for _, r := range recs {
			r.reset()
		}
		const n = 2000
		for i := range n {
			nodes[0].SendClusterMessage(&model.ClusterMessage{Event: testEvent, SendType: model.ClusterSendReliable, Data: []byte(strconv.Itoa(i))})
		}
		for _, r := range recs[1:] {
			require.Eventually(t, func() bool { return r.count() == n }, waitFor, tick)
			for i, msg := range r.all() {
				require.Equal(t, strconv.Itoa(i), string(msg.Data))
			}
		}
	})

	t.Run("to a single node", func(t *testing.T) {
		for _, r := range recs {
			r.reset()
		}
		require.NoError(t, nodes[0].SendClusterMessageToNode(nodes[2].id, &model.ClusterMessage{Event: testEvent, SendType: model.ClusterSendReliable, Data: []byte("r")}))
		require.NoError(t, nodes[0].SendClusterMessageToNode(nodes[2].id, &model.ClusterMessage{Event: testEvent, Data: []byte("b")}))
		require.Eventually(t, func() bool { return recs[2].count() == 2 }, waitFor, tick)
		time.Sleep(100 * time.Millisecond)
		assert.Zero(t, recs[1].count())

		assert.Error(t, nodes[0].SendClusterMessageToNode(model.NewId(), &model.ClusterMessage{Event: testEvent}))
		assert.Error(t, nodes[0].SendClusterMessageToNode(nodes[0].id, &model.ClusterMessage{Event: testEvent}))
	})
}

func TestClusterWithoutEncryption(t *testing.T) {
	nodes, _, recs := startCluster(t, 2, false)
	nodes[0].SendClusterMessage(&model.ClusterMessage{Event: testEvent, SendType: model.ClusterSendReliable, Data: []byte("plain")})
	require.Eventually(t, func() bool { return recs[1].count() == 1 }, waitFor, tick)
}

func TestClusterDisabled(t *testing.T) {
	ip := nextIP()
	host := newFakeHost(t, newFakeDB(), ip, freePort(t, ip), false)
	*host.cfg.ClusterSettings.Enable = false
	c := New(host)
	c.StartInterNodeCommunication()
	defer c.Shutdown()

	assert.False(t, c.running.Load())
	assert.False(t, c.IsLeader())
	assert.Zero(t, c.HealthScore())
	c.SendClusterMessage(&model.ClusterMessage{Event: testEvent, SendType: model.ClusterSendReliable, WaitForAllToSend: true})
	assert.Error(t, c.SendClusterMessageToNode(model.NewId(), &model.ClusterMessage{Event: testEvent}))

	info := c.GetMyClusterInfo()
	require.NotNil(t, info)
	assert.Equal(t, c.GetClusterId(), info.Id)
	infos, err := c.GetClusterInfos()
	require.NoError(t, err)
	assert.Empty(t, infos)

	count, appErr := c.WebConnCountForUser("u")
	require.Nil(t, appErr)
	assert.Zero(t, count)
	queues, err := c.GetWSQueues("u", "c", 1)
	require.NoError(t, err)
	assert.Empty(t, queues)
	statuses, appErr := c.GetPluginStatuses()
	require.Nil(t, appErr)
	assert.Empty(t, statuses)
	logs, appErr := c.GetLogs(nil, 0, 10)
	require.Nil(t, appErr)
	assert.Empty(t, logs)
	require.Nil(t, c.ConfigChanged(host.cfg, host.cfg, true))
	c.StopInterNodeCommunication()
}
