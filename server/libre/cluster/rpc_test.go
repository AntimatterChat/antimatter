// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package cluster

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

func TestClusterRequests(t *testing.T) {
	nodes, hosts, _ := startCluster(t, 3, true)
	rctx := request.TestContext(t)

	for i, h := range hosts {
		h.wsConns = 10 * (i + 1)
		h.logLines = []string{"line from " + nodes[i].id}
		h.webConns["user1"] = i + 1
	}

	t.Run("cluster stats", func(t *testing.T) {
		stats, appErr := nodes[0].GetClusterStats(rctx)
		require.Nil(t, appErr)
		require.Len(t, stats, 2)
		total := 0
		for _, s := range stats {
			assert.NotEqual(t, nodes[0].id, s.Id)
			assert.Equal(t, 3, s.TotalMasterDbConnections)
			assert.Equal(t, 2, s.TotalReadDbConnections)
			total += s.TotalWebsocketConnections
		}
		assert.Equal(t, 50, total)
	})

	t.Run("logs", func(t *testing.T) {
		lines, appErr := nodes[0].GetLogs(rctx, 0, 100)
		require.Nil(t, appErr)
		joined := strings.Join(lines, "\n")
		assert.Contains(t, joined, "line from "+nodes[1].id)
		assert.Contains(t, joined, "line from "+nodes[2].id)
		assert.NotContains(t, joined, "line from "+nodes[0].id)
		assert.Contains(t, joined, logSeparator)

		logs, appErr := nodes[0].QueryLogs(rctx, 0, 100)
		require.Nil(t, appErr)
		// All the test nodes share the OS hostname.
		require.Len(t, logs, 2)
		var all []string
		for _, l := range logs {
			all = append(all, l...)
		}
		assert.ElementsMatch(t, []string{"line from " + nodes[1].id, "line from " + nodes[2].id}, all)
	})

	t.Run("plugin statuses", func(t *testing.T) {
		statuses, appErr := nodes[1].GetPluginStatuses()
		require.Nil(t, appErr)
		require.Len(t, statuses, 2)
		assert.Equal(t, "p1", statuses[0].PluginId)
	})

	t.Run("webconn count", func(t *testing.T) {
		count, appErr := nodes[0].WebConnCountForUser("user1")
		require.Nil(t, appErr)
		assert.Equal(t, 5, count)
		count, appErr = nodes[0].WebConnCountForUser("nobody")
		require.Nil(t, appErr)
		assert.Zero(t, count)
	})

	t.Run("websocket queues", func(t *testing.T) {
		raw, err := json.Marshal(model.NewWebSocketEvent(model.WebsocketEventPosted, "", "", "", nil, ""))
		require.NoError(t, err)
		hosts[2].wsQueues["conn1"] = &model.WSQueues{
			ActiveQ:    []model.ActiveQueueItem{{Type: model.WebSocketMsgTypeEvent, Buf: raw}},
			ReuseCount: 3,
		}
		queues, err := nodes[0].GetWSQueues("user1", "conn1", 12)
		require.NoError(t, err)
		require.Len(t, queues, 2)
		assert.Nil(t, queues[nodes[1].id])
		require.NotNil(t, queues[nodes[2].id])
		assert.Equal(t, 3, queues[nodes[2].id].ReuseCount)
		require.Len(t, queues[nodes[2].id].ActiveQ, 1)
		assert.JSONEq(t, string(raw), string(queues[nodes[2].id].ActiveQ[0].Buf))
	})

	t.Run("support packet", func(t *testing.T) {
		big := bytes.Repeat([]byte("heap"), 2*1024*1024) // 8MB: fragmented response
		hosts[1].supportFiles = []model.FileData{{Filename: "heap.prof", Body: big}}
		hosts[2].supportFiles = []model.FileData{{Filename: "config.json", Body: []byte("{}")}}

		files, err := nodes[0].GenerateSupportPacket(rctx, &model.SupportPacketOptions{IncludeLogs: true, CPUProfileDuration: new(time.Duration)})
		require.NoError(t, err)
		require.Len(t, files, 2)
		hostname := nodes[1].GetMyClusterInfo().Hostname
		require.Len(t, files[nodes[1].id], 1)
		assert.Equal(t, hostname+"/heap.prof", files[nodes[1].id][0].Filename)
		assert.Equal(t, big, files[nodes[1].id][0].Body)
		assert.Equal(t, nodes[2].GetMyClusterInfo().Hostname+"/config.json", files[nodes[2].id][0].Filename)
	})

	t.Run("config propagation", func(t *testing.T) {
		newCfg := hosts[0].cfg.Clone()
		*newCfg.ServiceSettings.SiteURL = "https://chat.example.com"
		require.Nil(t, nodes[0].ConfigChanged(hosts[0].cfg, newCfg, true))
		for _, h := range hosts[1:] {
			applied := h.appliedConfigs()
			require.Len(t, applied, 1)
			assert.Equal(t, "https://chat.example.com", *applied[0].ServiceSettings.SiteURL)
		}
		assert.Empty(t, hosts[0].appliedConfigs())

		// Not propagated when the change itself comes from another node.
		require.Nil(t, nodes[1].ConfigChanged(hosts[1].cfg, newCfg, false))
		assert.Empty(t, hosts[0].appliedConfigs())
	})
}

func TestClusterRequestFailures(t *testing.T) {
	nodes, hosts, _ := startCluster(t, 2, true)
	nodes[0].timeouts.short = 300 * time.Millisecond

	// A node which doesn't answer in time makes WebConnCountForUser fail,
	// so that the user isn't wrongly set offline.
	hosts[1].blockWebConn = make(chan struct{})
	_, appErr := nodes[0].WebConnCountForUser("user1")
	require.NotNil(t, appErr)
	assert.Equal(t, "ent.cluster.timeout.error", appErr.Id)
	close(hosts[1].blockWebConn)

	// Once the node is gone, it is simply not counted anymore.
	nodes[1].StopInterNodeCommunication()
	require.Eventually(t, func() bool { return len(nodes[0].peers()) == 0 }, waitFor, tick)
	count, appErr := nodes[0].WebConnCountForUser("user1")
	require.Nil(t, appErr)
	assert.Zero(t, count)
	stats, appErr := nodes[0].GetClusterStats(request.TestContext(t))
	require.Nil(t, appErr)
	assert.Empty(t, stats)
}

func TestEncryptionKeySharedThroughDatabase(t *testing.T) {
	db := newFakeDB()
	h1 := newFakeHostForNode(t, db, true)
	h2 := newFakeHostForNode(t, db, true)
	c1 := New(h1)
	c2 := New(h2)
	k1, err := c1.loadEncryptionKey()
	require.NoError(t, err)
	k2, err := c2.loadEncryptionKey()
	require.NoError(t, err)
	assert.Len(t, k1, 32)
	assert.Equal(t, k1, k2)
	assert.NotEmpty(t, db.system[model.SystemClusterEncryptionKey])
}
