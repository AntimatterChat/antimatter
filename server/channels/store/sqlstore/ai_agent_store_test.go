// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package sqlstore

import (
	"testing"

	"github.com/mattermost/mattermost/server/v8/channels/store/storetest"
)

func TestAIAgentStore(t *testing.T) {
	StoreTest(t, storetest.TestAIAgentStore)
}

func TestAgentActivityStore(t *testing.T) {
	StoreTest(t, storetest.TestAgentActivityStore)
}

func TestAIAgentAgentsPluginUserAgents(t *testing.T) {
	StoreTestWithSqlStore(t, storetest.TestAIAgentAgentsPluginUserAgents)
}

func TestAIMCPServerStore(t *testing.T) {
	StoreTest(t, storetest.TestAIMCPServerStore)
}
