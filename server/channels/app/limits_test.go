// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/require"
)

func TestGetServerLimits(t *testing.T) {
	mainHelper.Parallel(t)

	t.Run("server reports no limits", func(t *testing.T) {
		th := Setup(t).InitBasic(t)

		serverLimits, appErr := th.App.GetServerLimits(true)
		require.Nil(t, appErr)

		// InitBasic creates 3 users by default
		require.Equal(t, int64(3), serverLimits.ActiveUserCount)
		require.Equal(t, int64(0), serverLimits.MaxUsersLimit)
		require.Equal(t, int64(0), serverLimits.MaxUsersHardLimit)
		require.Equal(t, int64(0), serverLimits.SingleChannelGuestLimit)
		require.Equal(t, int64(0), serverLimits.PostHistoryLimit)
		require.Equal(t, int64(0), serverLimits.LastAccessiblePostTime)
	})

	t.Run("user counts are skipped when includeUserCounts is false", func(t *testing.T) {
		th := Setup(t).InitBasic(t)

		withCounts, appErr := th.App.GetServerLimits(true)
		require.Nil(t, appErr)
		require.Equal(t, int64(3), withCounts.ActiveUserCount)

		withoutCounts, appErr := th.App.GetServerLimits(false)
		require.Nil(t, appErr)
		require.Equal(t, int64(0), withoutCounts.ActiveUserCount)
	})

	t.Run("user count should increase on creating new user and decrease on permanently deleting", func(t *testing.T) {
		th := Setup(t).InitBasic(t)

		serverLimits, appErr := th.App.GetServerLimits(true)
		require.Nil(t, appErr)
		require.Equal(t, int64(3), serverLimits.ActiveUserCount)

		// now we create a new user
		newUser := th.CreateUser(t)

		serverLimits, appErr = th.App.GetServerLimits(true)
		require.Nil(t, appErr)
		require.Equal(t, int64(4), serverLimits.ActiveUserCount)

		// now we'll delete the user
		_ = th.App.PermanentDeleteUser(th.Context, newUser)
		serverLimits, appErr = th.App.GetServerLimits(true)
		require.Nil(t, appErr)
		require.Equal(t, int64(3), serverLimits.ActiveUserCount)
	})

	t.Run("user count should increase on creating new guest user and decrease on permanently deleting", func(t *testing.T) {
		th := Setup(t).InitBasic(t)

		serverLimits, appErr := th.App.GetServerLimits(true)
		require.Nil(t, appErr)
		require.Equal(t, int64(3), serverLimits.ActiveUserCount)

		// now we create a new user
		newGuestUser := th.CreateGuest(t)

		serverLimits, appErr = th.App.GetServerLimits(true)
		require.Nil(t, appErr)
		require.Equal(t, int64(4), serverLimits.ActiveUserCount)

		// now we'll delete the user
		_ = th.App.PermanentDeleteUser(th.Context, newGuestUser)
		serverLimits, appErr = th.App.GetServerLimits(true)
		require.Nil(t, appErr)
		require.Equal(t, int64(3), serverLimits.ActiveUserCount)
	})

	t.Run("user count should increase on creating new user and decrease on soft deleting", func(t *testing.T) {
		th := Setup(t).InitBasic(t)

		serverLimits, appErr := th.App.GetServerLimits(true)
		require.Nil(t, appErr)
		require.Equal(t, int64(3), serverLimits.ActiveUserCount)

		// now we create a new user
		newUser := th.CreateUser(t)

		serverLimits, appErr = th.App.GetServerLimits(true)
		require.Nil(t, appErr)
		require.Equal(t, int64(4), serverLimits.ActiveUserCount)

		// now we'll delete the user
		_, appErr = th.App.UpdateActive(th.Context, newUser, false)
		require.Nil(t, appErr)
		serverLimits, appErr = th.App.GetServerLimits(true)
		require.Nil(t, appErr)
		require.Equal(t, int64(3), serverLimits.ActiveUserCount)
	})

	t.Run("user count should increase on creating new guest user and decrease on soft deleting", func(t *testing.T) {
		th := Setup(t).InitBasic(t)

		serverLimits, appErr := th.App.GetServerLimits(true)
		require.Nil(t, appErr)
		require.Equal(t, int64(3), serverLimits.ActiveUserCount)

		// now we create a new user
		newGuestUser := th.CreateGuest(t)

		serverLimits, appErr = th.App.GetServerLimits(true)
		require.Nil(t, appErr)
		require.Equal(t, int64(4), serverLimits.ActiveUserCount)

		// now we'll delete the user
		_, appErr = th.App.UpdateActive(th.Context, newGuestUser, false)
		require.Nil(t, appErr)
		serverLimits, appErr = th.App.GetServerLimits(true)
		require.Nil(t, appErr)
		require.Equal(t, int64(3), serverLimits.ActiveUserCount)
	})

	t.Run("user count should not change on creating or deleting bots", func(t *testing.T) {
		th := Setup(t).InitBasic(t)

		serverLimits, appErr := th.App.GetServerLimits(true)
		require.Nil(t, appErr)
		require.Equal(t, int64(3), serverLimits.ActiveUserCount)

		// now we create a new bot
		newBot := th.CreateBot(t)

		serverLimits, appErr = th.App.GetServerLimits(true)
		require.Nil(t, appErr)
		require.Equal(t, int64(3), serverLimits.ActiveUserCount)

		// now we'll delete the bot
		_ = th.App.PermanentDeleteBot(th.Context, newBot.UserId)
		serverLimits, appErr = th.App.GetServerLimits(true)
		require.Nil(t, appErr)
		require.Equal(t, int64(3), serverLimits.ActiveUserCount)
	})

}

func TestShouldTrackSingleChannelGuests(t *testing.T) {
	mainHelper.Parallel(t)

	t.Run("returns false when config GuestAccountsSettings.Enable is false", func(t *testing.T) {
		th := Setup(t)

		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.GuestAccountsSettings.Enable = false })

		require.False(t, th.App.shouldTrackSingleChannelGuests())
	})

	t.Run("returns true when guest accounts are enabled", func(t *testing.T) {
		th := Setup(t)

		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.GuestAccountsSettings.Enable = true })

		require.True(t, th.App.shouldTrackSingleChannelGuests())
	})
}
