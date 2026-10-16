// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

// TestUserBlockPrivacy: whom someone blocks or ignores is theirs alone. Nobody else reads it or sets it for them.
func TestUserBlockPrivacy(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	ctx := context.Background()
	blocker, blocked := th.BasicUser, th.BasicUser2

	block := model.Preference{UserId: blocker.Id, Category: model.PreferenceCategoryUserBlock, Name: blocked.Id, Value: model.UserBlockBlock}
	_, err := th.Client.UpdatePreferences(ctx, blocker.Id, model.Preferences{block})
	require.NoError(t, err)

	blockedClient := th.CreateClient()
	_, _, err = blockedClient.Login(ctx, blocked.Email, blocked.Password)
	require.NoError(t, err)

	t.Run("the person blocked can't read it", func(t *testing.T) {
		_, resp, err := blockedClient.GetPreferences(ctx, blocker.Id)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
		_, resp, err = blockedClient.GetPreferencesByCategory(ctx, blocker.Id, model.PreferenceCategoryUserBlock)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
		_, resp, err = blockedClient.GetPreferenceByCategoryAndName(ctx, blocker.Id, model.PreferenceCategoryUserBlock, blocked.Id)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("nobody lifts it or sets one for someone else", func(t *testing.T) {
		resp, err := blockedClient.UpdatePreferences(ctx, blocker.Id, model.Preferences{{
			UserId: blocker.Id, Category: model.PreferenceCategoryUserBlock, Name: blocked.Id, Value: model.UserBlockIgnore,
		}})
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
		resp, err = blockedClient.DeletePreferences(ctx, blocker.Id, model.Preferences{block})
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)

		prefs, appErr := th.App.GetPreferenceByCategoryAndNameForUser(th.Context, blocker.Id, model.PreferenceCategoryUserBlock, blocked.Id)
		require.Nil(t, appErr)
		assert.Equal(t, model.UserBlockBlock, prefs.Value)
	})

	t.Run("nobody blocks themselves, and only known levels are accepted", func(t *testing.T) {
		resp, err := th.Client.UpdatePreferences(ctx, blocker.Id, model.Preferences{{
			UserId: blocker.Id, Category: model.PreferenceCategoryUserBlock, Name: blocker.Id, Value: model.UserBlockBlock,
		}})
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
		resp, err = th.Client.UpdatePreferences(ctx, blocker.Id, model.Preferences{{
			UserId: blocker.Id, Category: model.PreferenceCategoryUserBlock, Name: blocked.Id, Value: "ban",
		}})
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})
}
