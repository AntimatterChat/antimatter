// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package storetest

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

func TestModerationStore(t *testing.T, rctx request.CTX, ss store.Store) {
	t.Run("Timeouts", func(t *testing.T) { testModerationTimeouts(t, rctx, ss) })
	t.Run("CountPosts", func(t *testing.T) { testModerationCountPosts(t, rctx, ss) })
}

func testModerationTimeouts(t *testing.T, rctx request.CTX, ss store.Store) {
	teamID, otherTeamID, ada, bob := model.NewId(), model.NewId(), model.NewId(), model.NewId()
	now := model.GetMillis()
	hour := time.Hour.Milliseconds()
	save := func(teamID, userID string, until int64, reason string) {
		t.Helper()
		_, err := ss.Moderation().SaveTimeout(rctx, &model.MemberTimeout{TeamId: teamID, UserId: userID, Until: until, Reason: reason, CreatedBy: model.NewId()})
		require.NoError(t, err)
	}
	active := func(teamID string, at int64) []string {
		t.Helper()
		timeouts, err := ss.Moderation().GetTimeouts(rctx, teamID, at)
		require.NoError(t, err)
		ids := []string{}
		for _, timeout := range timeouts {
			ids = append(ids, timeout.UserId+":"+timeout.Reason)
		}
		return ids
	}

	save(teamID, ada, now+hour, "spam")
	save(teamID, bob, now+2*hour, "")
	save(otherTeamID, ada, now+hour, "elsewhere")

	assert.ElementsMatch(t, []string{ada + ":spam", bob + ":"}, active(teamID, now))
	assert.ElementsMatch(t, []string{bob + ":"}, active(teamID, now+hour), "an expired timeout no longer holds")

	save(teamID, ada, now+3*hour, "again")
	assert.ElementsMatch(t, []string{ada + ":again", bob + ":"}, active(teamID, now+hour), "a new timeout replaces the old")

	require.NoError(t, ss.Moderation().DeleteTimeout(rctx, teamID, ada))
	assert.ElementsMatch(t, []string{bob + ":"}, active(teamID, now))

	require.NoError(t, ss.Moderation().DeleteTimeoutsForTeam(rctx, teamID))
	assert.Empty(t, active(teamID, now))
	assert.Len(t, active(otherTeamID, now), 1, "other teams keep theirs")
}

func testModerationCountPosts(t *testing.T, rctx request.CTX, ss store.Store) {
	team, err := ss.Team().Save(&model.Team{DisplayName: "Mods", Name: NewTestID(), Email: MakeEmail(), Type: model.TeamOpen})
	require.NoError(t, err)
	channel, err := ss.Channel().Save(rctx, &model.Channel{TeamId: team.Id, DisplayName: "Chat", Name: NewTestID(), Type: model.ChannelTypeOpen}, -1)
	require.NoError(t, err)
	other, err := ss.Channel().Save(rctx, &model.Channel{TeamId: model.NewId(), DisplayName: "Elsewhere", Name: NewTestID(), Type: model.ChannelTypeOpen}, -1)
	require.NoError(t, err)
	userID := model.NewId()
	now := model.GetMillis()
	post := func(channelID string, at int64, postType string) {
		t.Helper()
		_, err := ss.Post().Save(rctx, &model.Post{ChannelId: channelID, UserId: userID, Message: "hi", CreateAt: at, Type: postType})
		require.NoError(t, err)
	}
	post(channel.Id, now-time.Hour.Milliseconds(), "")
	post(channel.Id, now-2*time.Hour.Milliseconds(), "")
	post(channel.Id, now-3*24*time.Hour.Milliseconds(), "")
	post(channel.Id, now-time.Hour.Milliseconds(), model.PostTypeJoinChannel)
	post(other.Id, now-time.Hour.Milliseconds(), "")

	day, err := ss.Moderation().CountPosts(rctx, team.Id, userID, now-24*time.Hour.Milliseconds())
	require.NoError(t, err)
	assert.Equal(t, int64(2), day, "this team's messages of the day, system messages left out")
	week, err := ss.Moderation().CountPosts(rctx, team.Id, userID, now-7*24*time.Hour.Milliseconds())
	require.NoError(t, err)
	assert.Equal(t, int64(3), week)
}
