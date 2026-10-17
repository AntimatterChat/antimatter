// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/i18n"
)

// The default channel became general (General); it was town-square (Town Square). These tests cover new communities,
// the upgrade's renaming, and lookups by the old name.

// asBeforeTheUpgrade gives a community's default channel its old name, town-square, and displayName, as communities
// had before the upgrade.
func asBeforeTheUpgrade(t *testing.T, th *TestHelper, team *model.Team, displayName string) *model.Channel {
	t.Helper()
	channel, err := th.Store.Channel().GetByName(team.Id, model.DefaultChannelName, false)
	require.NoError(t, err)
	channel.Name = model.LegacyDefaultChannelName
	channel.DisplayName = displayName
	channel, err = th.Store.Channel().Update(th.Context, channel)
	require.NoError(t, err)
	th.Store.Channel().ClearCaches()
	return channel
}

func channelNamed(t *testing.T, th *TestHelper, teamID, name string) *model.Channel {
	t.Helper()
	channel, err := th.Store.Channel().GetByNameIncludeDeleted(teamID, name, false)
	require.NoError(t, err)
	return channel
}

func TestNewCommunitiesGetGeneral(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)

	team := th.CreateTeam(t)
	general, appErr := th.App.GetChannelByName(th.Context, model.DefaultChannelName, team.Id, false)
	require.Nil(t, appErr)
	assert.Equal(t, "general", general.Name)
	assert.Equal(t, i18n.T("api.channel.create_default_channels.general"), general.DisplayName)
	assert.Equal(t, "General", i18n.GetUserTranslations("en")("api.channel.create_default_channels.general"))
	assert.Equal(t, model.ChannelTypeOpen, general.Type)

	_, err := th.Store.Channel().GetByNameIncludeDeleted(team.Id, model.LegacyDefaultChannelName, false)
	require.Error(t, err, "new communities have no town-square")

	defaultChannel := th.App.GetDefaultChannelForTeam(th.Context, team)
	require.NotNil(t, defaultChannel)
	assert.Equal(t, general.Id, defaultChannel.Id, "general is the default channel")
	assert.True(t, th.App.IsDefaultChannel(th.Context, general))

	t.Run("town-square in the experimental default channels stands for general", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) {
			cfg.TeamSettings.ExperimentalDefaultChannels = []string{model.LegacyDefaultChannelName, "foo"}
		})
		defer th.App.UpdateConfig(func(cfg *model.Config) { cfg.TeamSettings.ExperimentalDefaultChannels = []string{} })
		assert.ElementsMatch(t, []string{model.DefaultChannelName, "foo"}, th.App.DefaultChannelNames(th.Context))
	})
}

func TestGeneralChannelMigration(t *testing.T) {
	th := Setup(t).InitBasic(t)

	// Communities as they were before the upgrade.
	plain := th.CreateTeam(t)
	plainChannel := asBeforeTheUpgrade(t, th, plain, "Town Square")

	translated := th.CreateTeam(t)
	frenchOldName := i18n.GetUserTranslations("fr")("api.channel.create_default_channels.town_square")
	require.NotEqual(t, "Town Square", frenchOldName, "the French catalog has its own old name")
	translatedChannel := asBeforeTheUpgrade(t, th, translated, frenchOldName)

	renamedByAdmins := th.CreateTeam(t)
	renamedChannel := asBeforeTheUpgrade(t, th, renamedByAdmins, "Lobby")

	withOwnGeneral := th.CreateTeam(t)
	keptChannel := asBeforeTheUpgrade(t, th, withOwnGeneral, "Town Square")
	ownGeneral := th.CreateChannel(t, withOwnGeneral, func(c *model.Channel) { c.Name = model.DefaultChannelName; c.DisplayName = "General chat" })

	withOwnGeneralAndChoice := th.CreateTeam(t)
	asBeforeTheUpgrade(t, th, withOwnGeneralAndChoice, "Town Square")
	th.CreateChannel(t, withOwnGeneralAndChoice, func(c *model.Channel) { c.Name = model.DefaultChannelName })
	lobby := th.CreateChannel(t, withOwnGeneralAndChoice)
	withOwnGeneralAndChoice.DefaultChannelId = lobby.Id
	_, err := th.Store.Team().Update(withOwnGeneralAndChoice)
	require.NoError(t, err)

	archived := th.CreateTeam(t)
	archivedChannel := asBeforeTheUpgrade(t, th, archived, "Town Square")
	require.Nil(t, th.App.DeleteChannel(th.Context, archivedChannel, th.SystemAdminUser.Id))

	_, err = th.Store.System().PermanentDeleteByName(generalChannelMigrationKey)
	require.NoError(t, err)
	require.NoError(t, th.Server.doGeneralChannelMigration())

	t.Run("renames town-square to general, keeping its ID", func(t *testing.T) {
		got := channelNamed(t, th, plain.Id, model.DefaultChannelName)
		assert.Equal(t, plainChannel.Id, got.Id)
		assert.Equal(t, "General", got.DisplayName)
		_, err := th.Store.Channel().GetByNameIncludeDeleted(plain.Id, model.LegacyDefaultChannelName, false)
		assert.Error(t, err, "no town-square is left")

		defaultChannel := th.App.GetDefaultChannelForTeam(th.Context, plain)
		require.NotNil(t, defaultChannel)
		assert.Equal(t, plainChannel.Id, defaultChannel.Id, "it is still the default channel")
	})

	t.Run("renames a translated default name too", func(t *testing.T) {
		got := channelNamed(t, th, translated.Id, model.DefaultChannelName)
		assert.Equal(t, translatedChannel.Id, got.Id)
		assert.Equal(t, i18n.GetUserTranslations("fr")("api.channel.create_default_channels.general"), got.DisplayName)
	})

	t.Run("keeps a display name the admins chose", func(t *testing.T) {
		got := channelNamed(t, th, renamedByAdmins.Id, model.DefaultChannelName)
		assert.Equal(t, renamedChannel.Id, got.Id)
		assert.Equal(t, "Lobby", got.DisplayName)
	})

	t.Run("keeps town-square when the community has a general channel of its own", func(t *testing.T) {
		kept := channelNamed(t, th, withOwnGeneral.Id, model.LegacyDefaultChannelName)
		assert.Equal(t, keptChannel.Id, kept.Id)
		assert.Equal(t, "Town Square", kept.DisplayName, "left as it is")
		own := channelNamed(t, th, withOwnGeneral.Id, model.DefaultChannelName)
		assert.Equal(t, ownGeneral.Id, own.Id)
		assert.Equal(t, "General chat", own.DisplayName)

		team, appErr := th.App.GetTeam(withOwnGeneral.Id)
		require.Nil(t, appErr)
		assert.Equal(t, keptChannel.Id, team.DefaultChannelId, "town-square stays the default channel")
		defaultChannel := th.App.GetDefaultChannelForTeam(th.Context, team)
		require.NotNil(t, defaultChannel)
		assert.Equal(t, keptChannel.Id, defaultChannel.Id)

		byOldName, appErr := th.App.GetChannelByName(th.Context, model.LegacyDefaultChannelName, team.Id, false)
		require.Nil(t, appErr)
		assert.Equal(t, keptChannel.Id, byOldName.Id, "its links still lead to it")
	})

	t.Run("keeps the default channel a community chose", func(t *testing.T) {
		team, appErr := th.App.GetTeam(withOwnGeneralAndChoice.Id)
		require.Nil(t, appErr)
		assert.Equal(t, lobby.Id, team.DefaultChannelId)
		channelNamed(t, th, team.Id, model.LegacyDefaultChannelName)
	})

	t.Run("keeps an archived town-square", func(t *testing.T) {
		got := channelNamed(t, th, archived.Id, model.LegacyDefaultChannelName)
		assert.Equal(t, archivedChannel.Id, got.Id)
		assert.NotZero(t, got.DeleteAt)
	})

	t.Run("is recorded, and runs once", func(t *testing.T) {
		row, err := th.Store.System().GetByName(generalChannelMigrationKey)
		require.NoError(t, err)
		assert.Equal(t, "true", row.Value)

		later := th.CreateTeam(t)
		laterChannel := asBeforeTheUpgrade(t, th, later, "Town Square")
		require.NoError(t, th.Server.doGeneralChannelMigration())
		got := channelNamed(t, th, later.Id, model.LegacyDefaultChannelName)
		assert.Equal(t, laterChannel.Id, got.Id, "a second start renames nothing")
	})
}

func TestGeneralChannelDisplayName(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t)

	for _, locale := range []string{"en", "fr", "de", "ja"} {
		tr := i18n.GetUserTranslations(locale)
		old := tr("api.channel.create_default_channels.town_square")
		got, ok := th.Server.generalChannelDisplayName(old)
		assert.True(t, ok, locale)
		assert.Equal(t, tr("api.channel.create_default_channels.general"), got, locale)
	}

	for _, chosen := range []string{"", "Lobby", "town square", "Town Square HQ"} {
		_, ok := th.Server.generalChannelDisplayName(chosen)
		assert.False(t, ok, chosen)
	}
}

func TestLegacyDefaultChannelLookups(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)

	team := th.CreateTeam(t)
	general, appErr := th.App.GetChannelByName(th.Context, model.DefaultChannelName, team.Id, false)
	require.Nil(t, appErr)

	t.Run("town-square gives the default channel", func(t *testing.T) {
		got, appErr := th.App.GetChannelByName(th.Context, model.LegacyDefaultChannelName, team.Id, false)
		require.Nil(t, appErr)
		assert.Equal(t, general.Id, got.Id)

		got, appErr = th.App.GetChannelByName(th.Context, model.LegacyDefaultChannelName, team.Id, true)
		require.Nil(t, appErr)
		assert.Equal(t, general.Id, got.Id)

		got, appErr = th.App.GetChannelByNameForTeamName(th.Context, model.LegacyDefaultChannelName, team.Name, false)
		require.Nil(t, appErr)
		assert.Equal(t, general.Id, got.Id)
	})

	t.Run("apps asking for town-square get it too", func(t *testing.T) {
		api := th.SetupPluginAPI()
		got, appErr := api.GetChannelByName(team.Id, model.LegacyDefaultChannelName, false)
		require.Nil(t, appErr)
		assert.Equal(t, general.Id, got.Id)

		got, appErr = api.GetChannelByNameForTeamName(team.Name, model.LegacyDefaultChannelName, false)
		require.Nil(t, appErr)
		assert.Equal(t, general.Id, got.Id)
	})

	t.Run("other names that aren't there are still not found", func(t *testing.T) {
		_, appErr := th.App.GetChannelByName(th.Context, "no-such-channel", team.Id, false)
		require.NotNil(t, appErr)
		assert.Equal(t, http.StatusNotFound, appErr.StatusCode)

		_, appErr = th.App.GetChannelByName(th.Context, model.LegacyDefaultChannelName, "", false)
		require.NotNil(t, appErr, "outside a community, there is no default channel")
	})

	t.Run("town-square gives the default channel the community chose", func(t *testing.T) {
		chosenTeam := th.CreateTeam(t)
		lobby := th.CreateChannel(t, chosenTeam)
		chosenTeam.DefaultChannelId = lobby.Id
		_, err := th.Store.Team().Update(chosenTeam)
		require.NoError(t, err)
		th.Store.Team().ClearCaches()

		got, appErr := th.App.GetChannelByName(th.Context, model.LegacyDefaultChannelName, chosenTeam.Id, false)
		require.Nil(t, appErr)
		assert.Equal(t, lobby.Id, got.Id)
	})

	t.Run("not found when the community has no default channel", func(t *testing.T) {
		noDefault := th.CreateTeam(t)
		channel, appErr := th.App.GetChannelByName(th.Context, model.DefaultChannelName, noDefault.Id, false)
		require.Nil(t, appErr)
		require.Nil(t, th.App.DeleteChannel(th.Context, channel, th.SystemAdminUser.Id))

		_, appErr = th.App.GetChannelByName(th.Context, model.LegacyDefaultChannelName, noDefault.Id, false)
		require.NotNil(t, appErr)
		assert.Equal(t, http.StatusNotFound, appErr.StatusCode)
	})

	t.Run("a community not renamed yet still has its default channel", func(t *testing.T) {
		old := th.CreateTeam(t)
		oldChannel := asBeforeTheUpgrade(t, th, old, "Town Square")
		defaultChannel := th.App.GetDefaultChannelForTeam(th.Context, old)
		require.NotNil(t, defaultChannel)
		assert.Equal(t, oldChannel.Id, defaultChannel.Id)
	})

	t.Run("incoming webhooks posting to town-square post in the default channel", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableIncomingWebhooks = true })
		basicGeneral, appErr := th.App.GetChannelByName(th.Context, model.DefaultChannelName, th.BasicTeam.Id, false)
		require.Nil(t, appErr)
		hook, appErr := th.App.CreateIncomingWebhookForChannel(th.BasicUser.Id, th.BasicChannel, &model.IncomingWebhook{ChannelId: th.BasicChannel.Id})
		require.Nil(t, appErr)
		defer func() {
			require.Nil(t, th.App.DeleteIncomingWebhook(hook.Id))
		}()

		text := "to the old default channel " + model.NewId()
		require.Nil(t, th.App.HandleIncomingWebhook(th.Context, hook.Id, &model.IncomingWebhookRequest{Text: text, ChannelName: model.LegacyDefaultChannelName}))
		list, appErr := th.App.GetPosts(th.Context, basicGeneral.Id, 0, 10)
		require.Nil(t, appErr)
		found := false
		for _, post := range list.Posts {
			if post.Message == text {
				found = true
			}
		}
		assert.True(t, found, "the post is in general")
	})
}
