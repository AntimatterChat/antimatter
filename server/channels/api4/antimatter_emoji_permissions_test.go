// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/v8/channels/utils"
)

// TestCustomEmojiPermissionsByRole pins who may add and delete custom emoji with the default roles, which the Fusion
// emoji picker mirrors to show its "+" and "Custom emoji" buttons and its delete buttons
// (webapp/fusion/src/fusion/selectors/custom_emoji.ts): members add emoji and delete their own, admins delete
// anyone's, guests do neither, and with custom emoji off nobody does, admins included.
func TestCustomEmojiPermissionsByRole(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	ctx := context.Background()

	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.EnableCustomEmoji = true
		*cfg.GuestAccountsSettings.Enable = true
	})

	member := th.Client
	other := th.CreateClient()
	th.LoginBasic2WithClient(t, other)
	admin := th.SystemAdminClient
	_, guest := th.CreateGuestAndClient(t)

	add := func(t *testing.T, client *model.Client4, creatorID string) (*model.Emoji, *model.Response, error) {
		t.Helper()
		return client.CreateEmoji(ctx, &model.Emoji{CreatorId: creatorID, Name: model.NewId()}, utils.CreateTestGif(t, 10, 10), "image.gif")
	}
	mine := func(t *testing.T, client *model.Client4, user *model.User) *model.Emoji {
		t.Helper()
		emoji, _, err := add(t, client, user.Id)
		require.NoError(t, err)
		return emoji
	}

	t.Run("members add emoji and delete their own", func(t *testing.T) {
		emoji := mine(t, member, th.BasicUser)
		_, err := member.DeleteEmoji(ctx, emoji.Id)
		require.NoError(t, err)
	})

	t.Run("members don't delete others' emoji", func(t *testing.T) {
		emoji := mine(t, other, th.BasicUser2)
		resp, err := member.DeleteEmoji(ctx, emoji.Id)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("admins delete anyone's emoji", func(t *testing.T) {
		emoji := mine(t, member, th.BasicUser)
		_, err := admin.DeleteEmoji(ctx, emoji.Id)
		require.NoError(t, err)
	})

	t.Run("guests neither add nor delete emoji", func(t *testing.T) {
		guestUser, _, err := guest.GetMe(ctx, "")
		require.NoError(t, err)
		_, resp, err := add(t, guest, guestUser.Id)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)

		emoji := mine(t, member, th.BasicUser)
		resp, err = guest.DeleteEmoji(ctx, emoji.Id)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("an emoji is added in its creator's name only", func(t *testing.T) {
		_, resp, err := add(t, member, th.BasicUser2.Id)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("with custom emoji off, nobody adds or deletes, admins included", func(t *testing.T) {
		emoji := mine(t, member, th.BasicUser)
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableCustomEmoji = false })
		defer th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableCustomEmoji = true })

		for name, client := range map[string]*model.Client4{"member": member, "admin": admin} {
			me, _, err := client.GetMe(ctx, "")
			require.NoError(t, err)
			_, resp, err := add(t, client, me.Id)
			require.Error(t, err, name)
			CheckNotImplementedStatus(t, resp)

			resp, err = client.DeleteEmoji(ctx, emoji.Id)
			require.Error(t, err, name)
			assert.Equal(t, http.StatusForbidden, resp.StatusCode, name)
		}
	})
}
