// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

// Antimatter: the default channels can be archived, by the community's admins.
func TestDeleteDefaultChannels(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)

	// BasicUser2 is a plain member of the community, who may archive public channels.
	th.AddPermissionToRole(t, model.PermissionDeletePublicChannel.Id, model.ChannelUserRoleId)
	client2 := th.CreateClient()
	_, _, err := client2.Login(context.Background(), th.BasicUser2.Email, th.BasicUser2.Password)
	require.NoError(t, err)

	for _, name := range []string{model.DefaultChannelName, model.WelcomeChannelName} {
		channel, appErr := th.App.GetChannelByName(th.Context, name, th.BasicTeam.Id, false)
		require.Nil(t, appErr)

		resp, deleteErr := client2.DeleteChannel(context.Background(), channel.Id)
		require.Error(t, deleteErr, name)
		CheckForbiddenStatus(t, resp)

		_, err = th.SystemAdminClient.DeleteChannel(context.Background(), channel.Id)
		require.NoError(t, err, name)
	}

	// Others they may archive as before.
	channel := th.CreatePublicChannel(t)
	th.AddUserToChannel(t, th.BasicUser2, channel)
	_, err = client2.DeleteChannel(context.Background(), channel.Id)
	require.NoError(t, err)
}
