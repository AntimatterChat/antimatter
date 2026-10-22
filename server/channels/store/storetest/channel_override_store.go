// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package storetest

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

func TestChannelOverrideStore(t *testing.T, rctx request.CTX, ss store.Store) {
	t.Run("SaveReplacesAndDeletes", func(t *testing.T) { testChannelOverrideSaveReplacesAndDeletes(t, rctx, ss) })
	t.Run("CategoryOverrides", func(t *testing.T) { testCategoryOverrides(t, rctx, ss) })
	t.Run("PermanentDeleteForUser", func(t *testing.T) { testChannelOverridePermanentDeleteForUser(t, rctx, ss) })
}

func testChannelOverridePermanentDeleteForUser(t *testing.T, rctx request.CTX, ss store.Store) {
	channelID, teamID, userID, otherID := model.NewId(), model.NewId(), model.NewId(), model.NewId()
	for _, o := range []*model.ChannelOverride{
		{ChannelId: channelID, TargetType: model.ChannelOverrideTargetUser, TargetId: userID, Deny: []string{"create_post"}},
		{ChannelId: channelID, TargetType: model.ChannelOverrideTargetUser, TargetId: otherID, Deny: []string{"create_post"}},
		// A role that happens to have the same ID isn't theirs.
		{ChannelId: channelID, TargetType: model.ChannelOverrideTargetRole, TargetId: userID, Deny: []string{"add_reaction"}},
	} {
		_, err := ss.ChannelOverride().Save(rctx, o)
		require.NoError(t, err)
	}
	for _, o := range []*model.CategoryOverride{
		{TeamId: teamID, Category: "community", TargetType: model.ChannelOverrideTargetUser, TargetId: userID, Deny: []string{"upload_file"}},
		{TeamId: teamID, Category: "community", TargetType: model.ChannelOverrideTargetUser, TargetId: otherID, Deny: []string{"upload_file"}},
	} {
		_, err := ss.ChannelOverride().SaveForCategory(rctx, o)
		require.NoError(t, err)
	}

	require.NoError(t, ss.ChannelOverride().PermanentDeleteForUser(rctx, userID))

	channel, err := ss.ChannelOverride().GetForChannel(rctx, channelID)
	require.NoError(t, err)
	targets := []string{}
	for _, o := range channel {
		targets = append(targets, o.TargetType+":"+o.TargetId)
	}
	assert.ElementsMatch(t, []string{model.ChannelOverrideTargetUser + ":" + otherID, model.ChannelOverrideTargetRole + ":" + userID}, targets)

	categories, err := ss.ChannelOverride().GetForCategories(rctx, teamID)
	require.NoError(t, err)
	require.Len(t, categories, 1)
	assert.Equal(t, otherID, categories[0].TargetId)
}

func testChannelOverrideSaveReplacesAndDeletes(t *testing.T, rctx request.CTX, ss store.Store) {
	channelID, roleID, userID := model.NewId(), model.NewId(), model.NewId()

	_, err := ss.ChannelOverride().Save(rctx, &model.ChannelOverride{ChannelId: channelID, TargetType: model.ChannelOverrideTargetRole, TargetId: roleID, Deny: []string{"create_post"}})
	require.NoError(t, err)
	_, err = ss.ChannelOverride().Save(rctx, &model.ChannelOverride{ChannelId: channelID, TargetType: model.ChannelOverrideTargetUser, TargetId: userID, Allow: []string{"create_post", "upload_file"}})
	require.NoError(t, err)

	// The same target again replaces its override.
	_, err = ss.ChannelOverride().Save(rctx, &model.ChannelOverride{ChannelId: channelID, TargetType: model.ChannelOverrideTargetRole, TargetId: roleID, Allow: []string{"add_reaction"}})
	require.NoError(t, err)

	got, err := ss.ChannelOverride().GetForChannel(rctx, channelID)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, model.ChannelOverrideTargetRole, got[0].TargetType)
	assert.Equal(t, []string{"add_reaction"}, got[0].Allow)
	assert.Empty(t, got[0].Deny)
	assert.Equal(t, []string{"create_post", "upload_file"}, got[1].Allow)

	require.NoError(t, ss.ChannelOverride().Delete(rctx, channelID, model.ChannelOverrideTargetUser, userID))
	got, err = ss.ChannelOverride().GetForChannel(rctx, channelID)
	require.NoError(t, err)
	require.Len(t, got, 1)

	other, err := ss.ChannelOverride().GetForChannel(rctx, model.NewId())
	require.NoError(t, err)
	assert.Empty(t, other)
}

func testCategoryOverrides(t *testing.T, rctx request.CTX, ss store.Store) {
	teamID, otherTeamID, roleID, userID := model.NewId(), model.NewId(), model.NewId(), model.NewId()
	save := func(teamID, category, targetType, targetID string, deny ...string) {
		t.Helper()
		_, err := ss.ChannelOverride().SaveForCategory(rctx, &model.CategoryOverride{TeamId: teamID, Category: category, TargetType: targetType, TargetId: targetID, Deny: deny})
		require.NoError(t, err)
	}
	categories := func(teamID string) map[string][]string {
		t.Helper()
		got, err := ss.ChannelOverride().GetForCategories(rctx, teamID)
		require.NoError(t, err)
		m := map[string][]string{}
		for _, o := range got {
			m[o.Category] = append(m[o.Category], o.TargetId+":"+strings.Join(o.Deny, ","))
		}
		return m
	}

	save(teamID, "community", model.ChannelOverrideTargetRole, roleID, "create_post")
	save(teamID, "community", model.ChannelOverrideTargetUser, userID, "upload_file")
	save(teamID, "rules", model.ChannelOverrideTargetRole, roleID, "add_reaction")
	save(otherTeamID, "community", model.ChannelOverrideTargetRole, roleID, "create_post")

	// The same target again replaces its override.
	save(teamID, "community", model.ChannelOverrideTargetRole, roleID, "use_channel_mentions")
	assert.Equal(t, map[string][]string{"community": {roleID + ":use_channel_mentions", userID + ":upload_file"}, "rules": {roleID + ":add_reaction"}}, categories(teamID))

	require.NoError(t, ss.ChannelOverride().DeleteForCategory(rctx, teamID, "community", model.ChannelOverrideTargetUser, userID))
	assert.Len(t, categories(teamID)["community"], 1)

	// A rename takes the overrides along, in place of any the new name had.
	require.NoError(t, ss.ChannelOverride().RenameCategory(rctx, teamID, "community", "rules"))
	assert.Equal(t, map[string][]string{"rules": {roleID + ":use_channel_mentions"}}, categories(teamID))

	require.NoError(t, ss.ChannelOverride().DeleteCategory(rctx, teamID, "rules"))
	assert.Empty(t, categories(teamID))
	assert.Len(t, categories(otherTeamID), 1, "other teams keep theirs")

	require.NoError(t, ss.ChannelOverride().DeleteCategoriesForTeam(rctx, otherTeamID))
	assert.Empty(t, categories(otherTeamID))
}
