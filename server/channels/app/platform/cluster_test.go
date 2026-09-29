// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package platform

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/v8/channels/testlib"
)

func TestIsLeader(t *testing.T) {
	mainHelper.Parallel(t)

	t.Run("cluster enabled and cluster interface returns cluster leader", func(t *testing.T) {
		fakeCluster := &testlib.FakeClusterInterface{}
		th := SetupWithCluster(t, fakeCluster)
		th.Service.UpdateConfig(func(cfg *model.Config) {
			*cfg.ClusterSettings.Enable = true
		})

		// This is the only case where IsLeader returns false because it's the only case where we actually call
		// FakeClusterInterface.IsLeader() which returns false
		assert.False(t, th.Service.IsLeader())
	})

	t.Run("cluster settings disabled returns true", func(t *testing.T) {
		fakeCluster := &testlib.FakeClusterInterface{}
		th := SetupWithCluster(t, fakeCluster)
		th.Service.UpdateConfig(func(cfg *model.Config) {
			*cfg.ClusterSettings.Enable = false
		})

		assert.True(t, th.Service.IsLeader())
	})
}
