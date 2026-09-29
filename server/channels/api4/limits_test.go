// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

func TestGetServerLimits(t *testing.T) {
	mainHelper.Parallel(t)

	t.Run("admin users get the active user count and no limits", func(t *testing.T) {
		th := Setup(t).InitBasic(t)

		serverLimits, resp, err := th.SystemAdminClient.GetServerLimits(context.Background())
		require.NoError(t, err)
		CheckOKStatus(t, resp)

		require.Greater(t, serverLimits.ActiveUserCount, int64(0))
		require.Equal(t, int64(0), serverLimits.MaxUsersLimit)
		require.Equal(t, int64(0), serverLimits.MaxUsersHardLimit)
		require.Equal(t, int64(0), serverLimits.SingleChannelGuestLimit)
		require.Equal(t, int64(0), serverLimits.PostHistoryLimit)
		require.Equal(t, int64(0), serverLimits.LastAccessiblePostTime)
	})

	t.Run("non-admin users get no user count data and no limits", func(t *testing.T) {
		th := Setup(t).InitBasic(t)

		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.GuestAccountsSettings.Enable = true })

		serverLimits, resp, err := th.Client.GetServerLimits(context.Background())
		require.NoError(t, err)
		CheckOKStatus(t, resp)

		require.Equal(t, int64(0), serverLimits.ActiveUserCount)
		require.Equal(t, int64(0), serverLimits.MaxUsersLimit)
		require.Equal(t, int64(0), serverLimits.MaxUsersHardLimit)
		require.Equal(t, int64(0), serverLimits.SingleChannelGuestCount)
		require.Equal(t, int64(0), serverLimits.SingleChannelGuestLimit)
		require.Equal(t, int64(0), serverLimits.PostHistoryLimit)
		require.Equal(t, int64(0), serverLimits.LastAccessiblePostTime)
	})
}
