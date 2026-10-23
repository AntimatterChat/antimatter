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

func TestMessageRequests(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	ctx := context.Background()

	// BasicUser2 asks first: BasicUser's direct message waits as a request for them.
	appErr := th.App.UpdatePreferences(th.Context, th.BasicUser2.Id, model.Preferences{{UserId: th.BasicUser2.Id, Category: model.PreferenceCategoryDMPrivacy, Name: model.PreferenceNameDMPrivacyDefault, Value: model.DMPrivacyAsk}})
	require.Nil(t, appErr)
	dm := th.CreateDmChannel(t, th.BasicUser2)

	t.Run("the sender doesn't see that their message is a request", func(t *testing.T) {
		member, _, err := th.Client.GetChannelMember(ctx, dm.Id, th.BasicUser2.Id, "")
		require.NoError(t, err)
		assert.NotContains(t, member.NotifyProps, model.ChannelMemberDMRequestProp)
	})

	t.Run("its recipient accepts it", func(t *testing.T) {
		client := th.CreateClient()
		th.LoginBasic2WithClient(t, client)

		member, _, err := client.GetChannelMember(ctx, dm.Id, th.BasicUser2.Id, "")
		require.NoError(t, err)
		assert.Equal(t, model.DMRequestPending, member.NotifyProps[model.ChannelMemberDMRequestProp])

		member, resp, err := client.UpdateMessageRequest(ctx, dm.Id, model.MessageRequestActionAccept)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		assert.Equal(t, model.DMRequestAccepted, member.NotifyProps[model.ChannelMemberDMRequestProp])

		_, resp, err = client.UpdateMessageRequest(ctx, dm.Id, "later")
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)

		_, resp, err = client.UpdateMessageRequest(ctx, th.BasicChannel.Id, model.MessageRequestActionAccept)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("only members act on it", func(t *testing.T) {
		other := th.CreateUser(t)
		client := th.CreateClient()
		_, _, err := client.Login(ctx, other.Email, other.Password)
		require.NoError(t, err)
		_, resp, err := client.UpdateMessageRequest(ctx, dm.Id, model.MessageRequestActionAccept)
		require.Error(t, err)
		assert.GreaterOrEqual(t, resp.StatusCode, 400)
	})

	t.Run("not while the admin turned it off", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.PrivacySettings.EnableUserBlocking = false })
		defer th.App.UpdateConfig(func(cfg *model.Config) { *cfg.PrivacySettings.EnableUserBlocking = true })
		_, resp, err := th.Client.UpdateMessageRequest(ctx, dm.Id, model.MessageRequestActionAccept)
		require.Error(t, err)
		CheckNotImplementedStatus(t, resp)
	})
}

// Refusing direct messages from a community's people, over the API: the sender is told as if blocked.
func TestDMPrivacyRefuse(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	ctx := context.Background()

	appErr := th.App.UpdatePreferences(th.Context, th.BasicUser2.Id, model.Preferences{{UserId: th.BasicUser2.Id, Category: model.PreferenceCategoryDMPrivacy, Name: th.BasicTeam.Id, Value: model.DMPrivacyRefuse}})
	require.Nil(t, appErr)

	_, resp, err := th.Client.CreateDirectChannel(ctx, th.BasicUser.Id, th.BasicUser2.Id)
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)
	CheckErrorID(t, err, "app.user_block.blocked.app_error")

	_, resp, err = th.Client.CreateGroupChannel(ctx, []string{th.BasicUser.Id, th.BasicUser2.Id, th.SystemAdminUser.Id})
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)

	_, _, err = th.SystemAdminClient.CreateDirectChannel(ctx, th.SystemAdminUser.Id, th.BasicUser2.Id)
	require.NoError(t, err, "system admins get through")

	// Saving an unknown choice is refused.
	_, err = th.Client.UpdatePreferences(ctx, th.BasicUser.Id, model.Preferences{{UserId: th.BasicUser.Id, Category: model.PreferenceCategoryDMPrivacy, Name: model.PreferenceNameDMPrivacyDefault, Value: "friends"}})
	require.Error(t, err)
}
