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
)

// Antimatter: communities made through the API have everyone in all their channels, unless the server says otherwise.
func TestCreateTeamAllChannelsMembership(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)

	team := th.CreateTeam(t)
	assert.False(t, team.AllChannelsMembership, "the tests' server turns it off")

	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.TeamSettings.AllChannelsMembershipForNewTeams = true })
	team = th.CreateTeam(t)
	assert.True(t, team.AllChannelsMembership)

	// Members can't leave its channels, nor be removed from them.
	channel, _, err := th.Client.CreateChannel(context.Background(), &model.Channel{TeamId: team.Id, Name: GenerateTestChannelName(), DisplayName: "Everyone", Type: model.ChannelTypeOpen})
	require.NoError(t, err)
	th.LinkUserToTeam(t, th.BasicUser2, team)
	_, appErr := th.App.GetChannelMember(th.Context, channel.Id, th.BasicUser2.Id)
	require.Nil(t, appErr, "joining the community joined its channels")

	client2 := th.CreateClient()
	_, _, err = client2.Login(context.Background(), th.BasicUser2.Email, th.BasicUser2.Password)
	require.NoError(t, err)
	resp, err := client2.RemoveUserFromChannel(context.Background(), channel.Id, th.BasicUser2.Id)
	require.Error(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp, err = th.SystemAdminClient.RemoveUserFromChannel(context.Background(), channel.Id, th.BasicUser2.Id)
	require.Error(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	// Its admins turn it off.
	patched, _, err := th.Client.PatchTeam(context.Background(), team.Id, &model.TeamPatch{AllChannelsMembership: model.NewPointer(false)})
	require.NoError(t, err)
	assert.False(t, patched.AllChannelsMembership)
	_, err = client2.RemoveUserFromChannel(context.Background(), channel.Id, th.BasicUser2.Id)
	require.NoError(t, err)
}
