// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package storetest

import (
	"sync"
	"sync/atomic"
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
	t.Run("PermanentDeleteTimeoutsByUser", func(t *testing.T) { testModerationPermanentDeleteTimeoutsByUser(t, rctx, ss) })
	t.Run("GetPostIdsInRange", func(t *testing.T) { testModerationGetPostIdsInRange(t, rctx, ss) })
	t.Run("Bans", func(t *testing.T) { testModerationBans(t, rctx, ss) })
	t.Run("InviteLinks", func(t *testing.T) { testModerationInviteLinks(t, rctx, ss) })
}

func testModerationPermanentDeleteTimeoutsByUser(t *testing.T, rctx request.CTX, ss store.Store) {
	teamID, otherTeamID, ada, bob := model.NewId(), model.NewId(), model.NewId(), model.NewId()
	until := model.GetMillis() + time.Hour.Milliseconds()
	for _, timeout := range []*model.MemberTimeout{
		{TeamId: teamID, UserId: ada, Until: until},
		{TeamId: otherTeamID, UserId: ada, Until: until},
		{TeamId: teamID, UserId: bob, Until: until},
	} {
		_, err := ss.Moderation().SaveTimeout(rctx, timeout)
		require.NoError(t, err)
	}

	require.NoError(t, ss.Moderation().PermanentDeleteTimeoutsByUser(rctx, ada))
	require.NoError(t, ss.Moderation().PermanentDeleteTimeoutsByUser(rctx, ada), "deleting again is fine")

	for _, team := range []string{teamID, otherTeamID} {
		timeouts, err := ss.Moderation().GetTimeouts(rctx, team, model.GetMillis())
		require.NoError(t, err)
		for _, timeout := range timeouts {
			assert.NotEqual(t, ada, timeout.UserId, "their timeouts are gone in every team")
		}
	}
	timeouts, err := ss.Moderation().GetTimeouts(rctx, teamID, model.GetMillis())
	require.NoError(t, err)
	require.Len(t, timeouts, 1, "others keep theirs")
	assert.Equal(t, bob, timeouts[0].UserId)
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
		_, saveErr := ss.Post().Save(rctx, &model.Post{ChannelId: channelID, UserId: userID, Message: "hi", CreateAt: at, Type: postType})
		require.NoError(t, saveErr)
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

func testModerationGetPostIdsInRange(t *testing.T, rctx request.CTX, ss store.Store) {
	team, err := ss.Team().Save(&model.Team{DisplayName: "Bans", Name: NewTestID(), Email: MakeEmail(), Type: model.TeamOpen})
	require.NoError(t, err)
	channel, err := ss.Channel().Save(rctx, &model.Channel{TeamId: team.Id, DisplayName: "Chat", Name: NewTestID(), Type: model.ChannelTypeOpen}, -1)
	require.NoError(t, err)
	other, err := ss.Channel().Save(rctx, &model.Channel{TeamId: model.NewId(), DisplayName: "Elsewhere", Name: NewTestID(), Type: model.ChannelTypeOpen}, -1)
	require.NoError(t, err)
	userID := model.NewId()
	now := model.GetMillis()
	hour := time.Hour.Milliseconds()
	post := func(channelID, userID string, at int64, postType string) string {
		t.Helper()
		saved, saveErr := ss.Post().Save(rctx, &model.Post{ChannelId: channelID, UserId: userID, Message: "hi", CreateAt: at, Type: postType})
		require.NoError(t, saveErr)
		return saved.Id
	}
	older := post(channel.Id, userID, now-3*hour, "")
	recent := post(channel.Id, userID, now-hour, "")
	post(channel.Id, userID, now-48*hour, "")
	post(channel.Id, userID, now-hour, model.PostTypeJoinChannel)
	post(channel.Id, model.NewId(), now-hour, "")
	post(other.Id, userID, now-hour, "")
	deleted := post(channel.Id, userID, now-hour, "")
	require.NoError(t, ss.Post().Delete(rctx, deleted, now, userID))
	post(channel.Id, userID, now+hour, "")

	ids, err := ss.Moderation().GetPostIdsInRange(rctx, team.Id, userID, now-24*hour, now, 10)
	require.NoError(t, err)
	assert.Equal(t, []string{older, recent}, ids, "their messages of the range in this team, oldest first; not system ones, others', other teams', deleted ones nor later ones")

	ids, err = ss.Moderation().GetPostIdsInRange(rctx, team.Id, userID, now-24*hour, now, 1)
	require.NoError(t, err)
	assert.Equal(t, []string{older}, ids, "up to the limit")
}

func testModerationBans(t *testing.T, rctx request.CTX, ss store.Store) {
	teamID, otherTeamID, ada, bob, carl := model.NewId(), model.NewId(), model.NewId(), model.NewId(), model.NewId()
	now := model.GetMillis()
	hour := time.Hour.Milliseconds()
	save := func(teamID, userID string, expireAt int64, reason string) {
		t.Helper()
		_, err := ss.Moderation().SaveBan(rctx, &model.MemberBan{TeamId: teamID, UserId: userID, Reason: reason, CreatedBy: model.NewId(), ExpireAt: expireAt})
		require.NoError(t, err)
	}
	active := func(teamID string, at int64) []string {
		t.Helper()
		bans, err := ss.Moderation().GetBans(rctx, teamID, at)
		require.NoError(t, err)
		ids := []string{}
		for _, ban := range bans {
			ids = append(ids, ban.UserId+":"+ban.Reason)
		}
		return ids
	}

	save(teamID, ada, 0, "spam")
	save(teamID, bob, now+hour, "")
	save(otherTeamID, ada, 0, "elsewhere")

	assert.ElementsMatch(t, []string{ada + ":spam", bob + ":"}, active(teamID, now))
	assert.ElementsMatch(t, []string{ada + ":spam"}, active(teamID, now+hour), "an ended ban no longer holds")

	ban, err := ss.Moderation().GetBan(rctx, teamID, ada, now)
	require.NoError(t, err)
	assert.Equal(t, "spam", ban.Reason)
	assert.NotZero(t, ban.CreateAt)
	_, err = ss.Moderation().GetBan(rctx, teamID, bob, now+hour)
	var nfErr *store.ErrNotFound
	require.ErrorAs(t, err, &nfErr, "an ended ban isn't found")
	_, err = ss.Moderation().GetBan(rctx, teamID, carl, now)
	require.ErrorAs(t, err, &nfErr)

	save(teamID, bob, 0, "again")
	assert.ElementsMatch(t, []string{ada + ":spam", bob + ":again"}, active(teamID, now+hour), "a new ban replaces the old")

	require.NoError(t, ss.Moderation().DeleteBan(rctx, teamID, ada))
	assert.ElementsMatch(t, []string{bob + ":again"}, active(teamID, now))
	assert.Len(t, active(otherTeamID, now), 1, "a ban is lifted in its team only")

	save(teamID, ada, 0, "")
	require.NoError(t, ss.Moderation().PermanentDeleteBansByUser(rctx, ada))
	assert.ElementsMatch(t, []string{bob + ":again"}, active(teamID, now), "their bans are gone in every team")
	assert.Empty(t, active(otherTeamID, now))

	require.NoError(t, ss.Moderation().DeleteBansForTeam(rctx, teamID))
	assert.Empty(t, active(teamID, now))
}

func testModerationInviteLinks(t *testing.T, rctx request.CTX, ss store.Store) {
	teamID, otherTeamID, ada, bob := model.NewId(), model.NewId(), model.NewId(), model.NewId()
	now := model.GetMillis()
	hour := time.Hour.Milliseconds()
	save := func(teamID, creatorID string, at, expireAt int64, maxUses int) *model.TeamInviteLink {
		t.Helper()
		link, err := ss.Moderation().SaveInviteLink(rctx, &model.TeamInviteLink{Id: model.NewId(), TeamId: teamID, CreatorId: creatorID, CreateAt: at, ExpireAt: expireAt, MaxUses: maxUses})
		require.NoError(t, err)
		return link
	}
	uses := func(id string) int {
		t.Helper()
		link, err := ss.Moderation().GetInviteLink(rctx, id)
		require.NoError(t, err)
		return link.Uses
	}

	first := save(teamID, ada, now-hour, 0, 2)
	second := save(teamID, bob, now, now+hour, 0)
	elsewhere := save(otherTeamID, ada, now, 0, 0)

	links, err := ss.Moderation().GetInviteLinks(rctx, teamID)
	require.NoError(t, err)
	require.Len(t, links, 2)
	assert.Equal(t, second.Id, links[0].Id, "the latest first")
	assert.Equal(t, first.Id, links[1].Id)
	count, err := ss.Moderation().CountInviteLinks(rctx, teamID)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)

	t.Run("a link is used up to its maximum, and a use given back is usable again", func(t *testing.T) {
		for range 2 {
			ok, useErr := ss.Moderation().UseInviteLink(rctx, first.Id, now)
			require.NoError(t, useErr)
			assert.True(t, ok)
		}
		ok, useErr := ss.Moderation().UseInviteLink(rctx, first.Id, now)
		require.NoError(t, useErr)
		assert.False(t, ok, "used up")
		assert.Equal(t, 2, uses(first.Id))

		require.NoError(t, ss.Moderation().ReleaseInviteLinkUse(rctx, first.Id))
		assert.Equal(t, 1, uses(first.Id))
		ok, useErr = ss.Moderation().UseInviteLink(rctx, first.Id, now)
		require.NoError(t, useErr)
		assert.True(t, ok)
	})

	t.Run("a use is never given back below none", func(t *testing.T) {
		require.NoError(t, ss.Moderation().ReleaseInviteLinkUse(rctx, elsewhere.Id))
		assert.Equal(t, 0, uses(elsewhere.Id))
	})

	t.Run("an ended link isn't used", func(t *testing.T) {
		ok, useErr := ss.Moderation().UseInviteLink(rctx, second.Id, now+hour)
		require.NoError(t, useErr)
		assert.False(t, ok)
		ok, useErr = ss.Moderation().UseInviteLink(rctx, second.Id, now)
		require.NoError(t, useErr)
		assert.True(t, ok, "before its end, without a limit")
	})

	t.Run("the last use goes to one person only", func(t *testing.T) {
		last := save(teamID, ada, now, 0, 1)
		var wg sync.WaitGroup
		var taken atomic.Int32
		for range 8 {
			wg.Go(func() {
				if ok, useErr := ss.Moderation().UseInviteLink(rctx, last.Id, now); useErr == nil && ok {
					taken.Add(1)
				}
			})
		}
		wg.Wait()
		assert.Equal(t, int32(1), taken.Load())
		assert.Equal(t, 1, uses(last.Id))
	})

	t.Run("a deleted link is gone", func(t *testing.T) {
		require.NoError(t, ss.Moderation().DeleteInviteLink(rctx, second.Id))
		_, getErr := ss.Moderation().GetInviteLink(rctx, second.Id)
		var nfErr *store.ErrNotFound
		require.ErrorAs(t, getErr, &nfErr)
		ok, useErr := ss.Moderation().UseInviteLink(rctx, second.Id, now)
		require.NoError(t, useErr)
		assert.False(t, ok, "a revoked link isn't used")
	})

	t.Run("someone's links go with them, in every team; a team's with it", func(t *testing.T) {
		require.NoError(t, ss.Moderation().PermanentDeleteInviteLinksByCreator(rctx, ada))
		for _, team := range []string{teamID, otherTeamID} {
			links, listErr := ss.Moderation().GetInviteLinks(rctx, team)
			require.NoError(t, listErr)
			for _, link := range links {
				assert.NotEqual(t, ada, link.CreatorId)
			}
		}
		save(teamID, bob, now, 0, 0)
		require.NoError(t, ss.Moderation().DeleteInviteLinksForTeam(rctx, teamID))
		count, countErr := ss.Moderation().CountInviteLinks(rctx, teamID)
		require.NoError(t, countErr)
		assert.Zero(t, count)
	})
}
