// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package app

import (
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

type countingAIConnectionStore struct {
	store.AIConnectionStore
	gets atomic.Int32
}

func (s *countingAIConnectionStore) Get(rctx request.CTX, id string) (*model.AIConnection, error) {
	s.gets.Add(1)
	return s.AIConnectionStore.Get(rctx, id)
}

type countingAIConnectionsStore struct {
	store.Store
	connections *countingAIConnectionStore
}

func (s *countingAIConnectionsStore) AIConnection() store.AIConnectionStore {
	return s.connections
}

// The legacy agents list reads each connection the agents use once, not once per agent.
func TestGetAgents_BatchesConnectionLookups(t *testing.T) {
	th, _, c := setupAgentTest(t, "")
	for range 4 {
		createTestAgent(t, th, model.AIAgentScopeServer, c.Id, nil)
	}

	// The agents' directory is loaded first, as on a running server.
	_, appErr := th.App.GetAgents(th.Context, th.BasicUser.Id)
	require.Nil(t, appErr)

	original := th.App.Srv().Store()
	counting := &countingAIConnectionStore{AIConnectionStore: original.AIConnection()}
	th.App.Srv().SetStore(&countingAIConnectionsStore{Store: original, connections: counting})
	t.Cleanup(func() { th.App.Srv().SetStore(original) })

	agents, appErr := th.App.GetAgents(th.Context, th.BasicUser.Id)
	require.Nil(t, appErr)
	require.Len(t, agents, 4)
	for _, agent := range agents {
		assert.Equal(t, model.AIProviderOpenAICompatible, agent.ServiceType)
	}
	assert.Equal(t, int32(1), counting.gets.Load(), "one read of the connection the four agents share")
}
