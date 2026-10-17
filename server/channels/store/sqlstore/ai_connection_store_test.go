// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package sqlstore

import (
	"testing"

	"github.com/mattermost/mattermost/server/v8/channels/store/storetest"
)

func TestAIConnectionStore(t *testing.T) {
	StoreTest(t, storetest.TestAIConnectionStore)
}

func TestAIConnectionAgentsPluginConfig(t *testing.T) {
	StoreTestWithSqlStore(t, storetest.TestAIConnectionAgentsPluginConfig)
}
