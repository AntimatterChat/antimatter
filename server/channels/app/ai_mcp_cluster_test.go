// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/v8/channels/testlib"
)

// Editing, deleting or signing out of an MCP server drops its cached tool lists on every node, not only the one that
// handled the request.
func TestPatchAIMCPServer_InvalidatesToolCacheClusterWide(t *testing.T) {
	th := setupAITest(t)
	f := newFakeMCPServer(t)
	s := createTestMCPServer(t, th, f, nil)
	other := createTestMCPServer(t, th, f, nil)
	cluster := &testlib.FakeClusterInterface{}
	th.Server.Platform().SetCluster(cluster)

	// Another node, with lists of both servers cached.
	node := &aiMCPState{}
	fill := func() {
		node.put(aiMCPToolsKey(s.Id, ""), nil)
		node.put(aiMCPToolsKey(s.Id, th.BasicUser.Id), nil)
		node.put(aiMCPToolsKey(s.Id, th.BasicUser2.Id), nil)
		node.put(aiMCPToolsKey(other.Id, th.BasicUser.Id), nil)
	}
	deliver := func() {
		messages := cluster.SelectMessages(func(m *model.ClusterMessage) bool { return m.Event == clusterEventInvalidateAIMCPTools })
		require.NotEmpty(t, messages, "the other nodes are told")
		for _, m := range messages {
			applyAIMCPToolsInvalidation(th.TestLogger, node, m)
		}
		cluster.ClearMessages()
	}
	cached := func(serverID, userID string) bool {
		_, ok := node.get(aiMCPToolsKey(serverID, userID))
		return ok
	}

	fill()
	name := "Tracker 2"
	_, appErr := th.App.PatchAIMCPServer(th.Context, s, &model.AIMCPServerPatch{Name: &name}, th.SystemAdminUser.Id)
	require.Nil(t, appErr)
	deliver()
	assert.False(t, cached(s.Id, ""), "the edited server's lists are gone on the other node")
	assert.False(t, cached(s.Id, th.BasicUser.Id))
	assert.False(t, cached(s.Id, th.BasicUser2.Id))
	assert.True(t, cached(other.Id, th.BasicUser.Id), "another server's stay")

	fill()
	signInToTestMCPServer(t, th, s, th.BasicUser.Id, "maya-token-123456")
	require.Nil(t, th.App.DisconnectAIMCPServer(th.Context, th.BasicUser.Id, s))
	deliver()
	assert.False(t, cached(s.Id, th.BasicUser.Id), "a sign-out drops that person's list everywhere")
	assert.True(t, cached(s.Id, th.BasicUser2.Id), "and only theirs")

	fill()
	require.Nil(t, th.App.DeleteAIMCPServer(th.Context, s))
	deliver()
	assert.False(t, cached(s.Id, th.BasicUser2.Id), "a deleted server's lists are gone everywhere")
	assert.True(t, cached(other.Id, th.BasicUser.Id))
}
