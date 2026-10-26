// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/v8/channels/app/imports"
)

// town-square, the default channel's Mattermost name, finds General in channel mentions and in imports of a
// Mattermost export, as it does in the other lookups (TestLegacyDefaultChannelLookups).
func TestLegacyDefaultChannelMentionsAndImports(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)

	team := th.CreateTeam(t)
	general, appErr := th.App.GetChannelByName(th.Context, model.DefaultChannelName, team.Id, false)
	require.Nil(t, appErr)

	t.Run("GetChannelsByNames gives General under the name town-square", func(t *testing.T) {
		channels, appErr := th.App.GetChannelsByNames(th.Context, []string{model.LegacyDefaultChannelName, model.DefaultChannelName}, team.Id)
		require.Nil(t, appErr)
		byName := map[string]*model.Channel{}
		for _, channel := range channels {
			byName[channel.Name] = channel
		}
		require.Contains(t, byName, model.LegacyDefaultChannelName)
		assert.Equal(t, general.Id, byName[model.LegacyDefaultChannelName].Id)
		assert.Equal(t, general.DisplayName, byName[model.LegacyDefaultChannelName].DisplayName)
		require.Contains(t, byName, model.DefaultChannelName)

		// The copy doesn't rename the cached channel.
		again, appErr := th.App.GetChannelByName(th.Context, model.DefaultChannelName, team.Id, false)
		require.Nil(t, appErr)
		assert.Equal(t, model.DefaultChannelName, again.Name)

		// Outside a community there is no default channel.
		channels, appErr = th.App.GetChannelsByNames(th.Context, []string{model.LegacyDefaultChannelName}, "")
		require.Nil(t, appErr)
		for _, channel := range channels {
			assert.NotEqual(t, general.Id, channel.Id)
		}
	})

	t.Run("a ~town-square mention links to General", func(t *testing.T) {
		// BasicUser is a member of BasicTeam.
		basicGeneral, appErr := th.App.GetChannelByName(th.Context, model.DefaultChannelName, th.BasicTeam.Id, false)
		require.Nil(t, appErr)
		post := &model.Post{
			UserId:    th.BasicUser.Id,
			ChannelId: th.BasicChannel.Id,
			Message:   "see ~town-square",
		}
		require.Nil(t, th.App.FillInPostProps(th.Context, post, th.BasicChannel))
		mentions, ok := post.GetProp(model.PostPropsChannelMentions).(map[string]any)
		require.True(t, ok, "channel_mentions is set")
		require.Contains(t, mentions, model.LegacyDefaultChannelName)
		mention := mentions[model.LegacyDefaultChannelName].(map[string]any)
		assert.Equal(t, basicGeneral.Id, mention["id"])
		assert.Equal(t, basicGeneral.DisplayName, mention["display_name"])
	})

	t.Run("importing a Mattermost export's town-square updates General", func(t *testing.T) {
		before, nErr := th.App.Srv().Store().Channel().GetTeamChannels(team.Id)
		require.NoError(t, nErr)

		open := model.ChannelTypeOpen
		appErr := th.App.importChannel(th.Context, &imports.ChannelImportData{
			Team:        &team.Name,
			Name:        new(model.LegacyDefaultChannelName),
			DisplayName: new("Town Square"),
			Type:        &open,
			Purpose:     new("Imported purpose"),
		}, false)
		require.Nil(t, appErr)

		after, nErr := th.App.Srv().Store().Channel().GetTeamChannels(team.Id)
		require.NoError(t, nErr)
		assert.Len(t, after, len(before), "no second default channel")
		_, nErr = th.App.Srv().Store().Channel().GetByNameIncludeDeleted(team.Id, model.LegacyDefaultChannelName, false)
		require.Error(t, nErr, "no channel named town-square was made")

		updated, appErr := th.App.GetChannel(th.Context, general.Id)
		require.Nil(t, appErr)
		assert.Equal(t, model.DefaultChannelName, updated.Name)
		assert.Equal(t, general.DisplayName, updated.DisplayName, "General keeps its name")
		assert.Equal(t, "Imported purpose", updated.Purpose)
	})

	t.Run("imported posts and memberships of town-square go to General", func(t *testing.T) {
		th.LinkUserToTeam(t, th.BasicUser2, team)

		appErr := th.App.importUserChannels(th.Context, th.BasicUser2, team, &[]imports.UserChannelImportData{{
			Name:  new(model.LegacyDefaultChannelName),
			Roles: new(model.ChannelUserRoleId + " " + model.ChannelAdminRoleId),
		}}, false)
		require.Nil(t, appErr)
		member, appErr := th.App.GetChannelMember(th.Context, general.Id, th.BasicUser2.Id)
		require.Nil(t, appErr)
		assert.True(t, member.SchemeAdmin)

		message := "imported into town-square " + model.NewId()
		errLine, appErr := th.App.importMultiplePostLines(th.Context, []imports.LineImportWorkerData{{
			LineImportData: imports.LineImportData{
				Post: &imports.PostImportData{
					Team:     &team.Name,
					Channel:  new(model.LegacyDefaultChannelName),
					User:     &th.BasicUser2.Username,
					Message:  &message,
					CreateAt: new(model.GetMillis()),
				},
			},
			LineNumber: 1,
		}}, false, true, false, &imports.ImportReport{})
		require.Nil(t, appErr)
		require.Equal(t, 0, errLine)

		list, appErr := th.App.GetPosts(th.Context, general.Id, 0, 20)
		require.Nil(t, appErr)
		found := false
		for _, post := range list.Posts {
			if post.Message == message {
				found = true
			}
		}
		assert.True(t, found, "the imported post is in General")
	})
}
