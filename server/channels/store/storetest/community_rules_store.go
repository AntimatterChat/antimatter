// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package storetest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

func TestCommunityRulesStore(t *testing.T, rctx request.CTX, ss store.Store) {
	t.Run("SaveAndGet", func(t *testing.T) { testCommunityRulesSaveAndGet(t, rctx, ss) })
	t.Run("Acceptances", func(t *testing.T) { testCommunityRulesAcceptances(t, rctx, ss) })
	t.Run("CountAcceptances", func(t *testing.T) { testCommunityRulesCountAcceptances(t, rctx, ss) })
	t.Run("Delete", func(t *testing.T) { testCommunityRulesDelete(t, rctx, ss) })
}

func testCommunityRulesSaveAndGet(t *testing.T, rctx request.CTX, ss store.Store) {
	teamID, adminID := model.NewId(), model.NewId()

	_, err := ss.CommunityRules().Get(rctx, teamID)
	var nfErr *store.ErrNotFound
	require.ErrorAs(t, err, &nfErr, "a community that never had rules has none")

	saved, err := ss.CommunityRules().Save(rctx, &model.CommunityRules{TeamId: teamID, Enabled: true, Text: "Be kind.", UpdatedBy: adminID}, false)
	require.NoError(t, err)
	assert.Equal(t, int64(1), saved.Version, "the first rules are version 1, asked again or not")
	assert.NotZero(t, saved.UpdateAt)

	got, err := ss.CommunityRules().Get(rctx, teamID)
	require.NoError(t, err)
	assert.True(t, got.Enabled)
	assert.Equal(t, "Be kind.", got.Text)
	assert.Equal(t, adminID, got.UpdatedBy)
	assert.Equal(t, int64(1), got.Version)

	saved, err = ss.CommunityRules().Save(rctx, &model.CommunityRules{TeamId: teamID, Enabled: true, Text: "Be kind!", UpdatedBy: adminID}, false)
	require.NoError(t, err)
	assert.Equal(t, int64(1), saved.Version, "a fix that doesn't ask again keeps the version")

	saved, err = ss.CommunityRules().Save(rctx, &model.CommunityRules{TeamId: teamID, Enabled: true, Text: "Be kind. No spam.", UpdatedBy: adminID}, true)
	require.NoError(t, err)
	assert.Equal(t, int64(2), saved.Version, "asking again goes to the next version")

	saved, err = ss.CommunityRules().Save(rctx, &model.CommunityRules{TeamId: teamID, Enabled: false, Text: "Be kind. No spam.", UpdatedBy: adminID}, false)
	require.NoError(t, err)
	assert.Equal(t, int64(2), saved.Version, "turning them off keeps the version")
	got, err = ss.CommunityRules().Get(rctx, teamID)
	require.NoError(t, err)
	assert.False(t, got.Enabled)
	assert.Equal(t, "Be kind. No spam.", got.Text, "the text stays for when they're turned on again")
}

func testCommunityRulesAcceptances(t *testing.T, rctx request.CTX, ss store.Store) {
	teamID, otherTeamID, ada, bob := model.NewId(), model.NewId(), model.NewId(), model.NewId()

	version, err := ss.CommunityRules().GetAcceptedVersion(rctx, teamID, ada)
	require.NoError(t, err)
	assert.Zero(t, version, "nobody agreed yet")

	require.NoError(t, ss.CommunityRules().Accept(rctx, &model.CommunityRulesAcceptance{TeamId: teamID, UserId: ada, Version: 1, AcceptedAt: 100}))
	require.NoError(t, ss.CommunityRules().Accept(rctx, &model.CommunityRulesAcceptance{TeamId: teamID, UserId: ada, Version: 1, AcceptedAt: 200}), "agreeing again isn't an error")
	require.NoError(t, ss.CommunityRules().Accept(rctx, &model.CommunityRulesAcceptance{TeamId: teamID, UserId: ada, Version: 3, AcceptedAt: 300}))
	require.NoError(t, ss.CommunityRules().Accept(rctx, &model.CommunityRulesAcceptance{TeamId: otherTeamID, UserId: ada, Version: 5}))
	require.NoError(t, ss.CommunityRules().Accept(rctx, &model.CommunityRulesAcceptance{TeamId: teamID, UserId: bob, Version: 2}))

	version, err = ss.CommunityRules().GetAcceptedVersion(rctx, teamID, ada)
	require.NoError(t, err)
	assert.Equal(t, int64(3), version, "the latest version agreed to, this community's only")
	version, err = ss.CommunityRules().GetAcceptedVersion(rctx, teamID, bob)
	require.NoError(t, err)
	assert.Equal(t, int64(2), version)

	acceptances, err := ss.CommunityRules().GetAcceptancesForUser(rctx, ada)
	require.NoError(t, err)
	require.Len(t, acceptances, 3, "every agreement is kept, each version once")
	for _, a := range acceptances {
		if a.TeamId == teamID && a.Version == 1 {
			assert.Equal(t, int64(100), a.AcceptedAt, "agreeing again keeps the first time")
		}
		if a.TeamId == otherTeamID {
			assert.NotZero(t, a.AcceptedAt, "the time is filled in when not given")
		}
	}
}

func testCommunityRulesCountAcceptances(t *testing.T, rctx request.CTX, ss store.Store) {
	team, err := ss.Team().Save(&model.Team{DisplayName: "Rules", Name: NewTestID(), Email: MakeEmail(), Type: model.TeamOpen})
	require.NoError(t, err)
	member := func(deactivated, left bool) string {
		t.Helper()
		user, saveErr := ss.User().Save(rctx, &model.User{Email: MakeEmail(), Username: "u" + NewTestID()})
		require.NoError(t, saveErr)
		if deactivated {
			user.DeleteAt = model.GetMillis()
			_, saveErr = ss.User().Update(rctx, user, true)
			require.NoError(t, saveErr)
		}
		tm := &model.TeamMember{TeamId: team.Id, UserId: user.Id}
		if left {
			tm.DeleteAt = model.GetMillis()
		}
		_, saveErr = ss.Team().SaveMember(rctx, tm, -1)
		require.NoError(t, saveErr)
		return user.Id
	}
	ada, bob, cat := member(false, false), member(false, false), member(false, false)
	gone := member(true, false)
	left := member(false, true)
	botID := member(false, false)
	_, err = ss.Bot().Save(&model.Bot{UserId: botID, Username: "b" + NewTestID(), OwnerId: ada})
	require.NoError(t, err)

	for _, id := range []string{ada, bob, gone, left, botID} {
		require.NoError(t, ss.CommunityRules().Accept(rctx, &model.CommunityRulesAcceptance{TeamId: team.Id, UserId: id, Version: 2}))
	}
	require.NoError(t, ss.CommunityRules().Accept(rctx, &model.CommunityRulesAcceptance{TeamId: team.Id, UserId: cat, Version: 1}))

	stats, err := ss.CommunityRules().CountAcceptances(rctx, team.Id, 2)
	require.NoError(t, err)
	assert.Equal(t, int64(3), stats.MemberCount, "active people only: no bots, deactivated accounts or people who left")
	assert.Equal(t, int64(2), stats.AcceptedCount, "those of them who agreed to this version")
	assert.Equal(t, int64(2), stats.Version)
	assert.Equal(t, team.Id, stats.TeamId)

	stats, err = ss.CommunityRules().CountAcceptances(rctx, team.Id, 3)
	require.NoError(t, err)
	assert.Equal(t, int64(3), stats.MemberCount)
	assert.Zero(t, stats.AcceptedCount, "nobody agreed to a new version yet")
}

func testCommunityRulesDelete(t *testing.T, rctx request.CTX, ss store.Store) {
	teamID, otherTeamID, ada, bob := model.NewId(), model.NewId(), model.NewId(), model.NewId()
	for _, id := range []string{teamID, otherTeamID} {
		_, err := ss.CommunityRules().Save(rctx, &model.CommunityRules{TeamId: id, Enabled: true, Text: "Rules"}, false)
		require.NoError(t, err)
		require.NoError(t, ss.CommunityRules().Accept(rctx, &model.CommunityRulesAcceptance{TeamId: id, UserId: ada, Version: 1}))
		require.NoError(t, ss.CommunityRules().Accept(rctx, &model.CommunityRulesAcceptance{TeamId: id, UserId: bob, Version: 1}))
	}

	require.NoError(t, ss.CommunityRules().DeleteForTeam(rctx, teamID))
	_, err := ss.CommunityRules().Get(rctx, teamID)
	var nfErr *store.ErrNotFound
	require.ErrorAs(t, err, &nfErr)
	version, err := ss.CommunityRules().GetAcceptedVersion(rctx, teamID, ada)
	require.NoError(t, err)
	assert.Zero(t, version, "the community's agreements go with it")
	version, err = ss.CommunityRules().GetAcceptedVersion(rctx, otherTeamID, ada)
	require.NoError(t, err)
	assert.Equal(t, int64(1), version, "other communities keep theirs")

	require.NoError(t, ss.CommunityRules().PermanentDeleteByUser(rctx, bob))
	acceptances, err := ss.CommunityRules().GetAcceptancesForUser(rctx, bob)
	require.NoError(t, err)
	assert.Empty(t, acceptances, "a deleted account's agreements go with it")
	version, err = ss.CommunityRules().GetAcceptedVersion(rctx, otherTeamID, ada)
	require.NoError(t, err)
	assert.Equal(t, int64(1), version, "others keep theirs")
}
