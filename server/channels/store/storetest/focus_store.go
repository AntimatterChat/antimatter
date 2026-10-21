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

func TestFocusStore(t *testing.T, rctx request.CTX, ss store.Store) {
	t.Run("SaveGetAndEnd", func(t *testing.T) { testFocusSaveGetAndEnd(t, rctx, ss) })
	t.Run("GetExpired", func(t *testing.T) { testFocusGetExpired(t, rctx, ss) })
	t.Run("GetMissedChannels", func(t *testing.T) { testFocusGetMissedChannels(t, rctx, ss) })
	t.Run("GetMissedThreads", func(t *testing.T) { testFocusGetMissedThreads(t, rctx, ss) })
	t.Run("GetFirstPostsAfter", func(t *testing.T) { testFocusGetFirstPostsAfter(t, rctx, ss) })
}

func testFocusSaveGetAndEnd(t *testing.T, rctx request.CTX, ss store.Store) {
	userID := model.NewId()

	_, err := ss.Focus().Get(rctx, userID)
	var nfErr *store.ErrNotFound
	require.ErrorAs(t, err, &nfErr, "someone who never focused has no focus")

	focus := &model.UserFocus{UserId: userID, StartAt: 1000, EndAt: 2000, Reason: "Working on the new release", PrevStatus: model.StatusAway, PrevManual: true, PrevDNDEndTime: 5, PrevCustomStatus: `{"emoji":"calendar"}`, FocusCustomStatus: `{"emoji":"dart"}`}
	require.NoError(t, ss.Focus().Save(rctx, focus))

	got, err := ss.Focus().Get(rctx, userID)
	require.NoError(t, err)
	assert.Equal(t, focus, got, "everything kept comes back, the previous status included")
	assert.True(t, got.IsActive())

	ended, err := ss.Focus().MarkEnded(rctx, userID, 999, 1500)
	require.NoError(t, err)
	assert.False(t, ended, "another focus (started at another time) isn't ended")

	ended, err = ss.Focus().MarkEnded(rctx, userID, 1000, 1500)
	require.NoError(t, err)
	assert.True(t, ended)
	ended, err = ss.Focus().MarkEnded(rctx, userID, 1000, 1600)
	require.NoError(t, err)
	assert.False(t, ended, "a focus ends once: the second caller doesn't restore anything")

	got, err = ss.Focus().Get(rctx, userID)
	require.NoError(t, err)
	assert.Equal(t, int64(1500), got.EndedAt)
	assert.False(t, got.IsActive())

	// A new focus replaces the last one.
	require.NoError(t, ss.Focus().Save(rctx, &model.UserFocus{UserId: userID, StartAt: 3000, EndAt: 4000}))
	got, err = ss.Focus().Get(rctx, userID)
	require.NoError(t, err)
	assert.Equal(t, int64(3000), got.StartAt)
	assert.Zero(t, got.EndedAt)
	assert.Empty(t, got.PrevCustomStatus)

	require.NoError(t, ss.Focus().PermanentDeleteByUser(rctx, userID))
	_, err = ss.Focus().Get(rctx, userID)
	require.ErrorAs(t, err, &nfErr)
}

func testFocusGetExpired(t *testing.T, rctx request.CTX, ss store.Store) {
	now := model.GetMillis()
	due, later, ended := model.NewId(), model.NewId(), model.NewId()
	require.NoError(t, ss.Focus().Save(rctx, &model.UserFocus{UserId: due, StartAt: now - 2000, EndAt: now - 1000}))
	require.NoError(t, ss.Focus().Save(rctx, &model.UserFocus{UserId: later, StartAt: now - 2000, EndAt: now + 60000}))
	require.NoError(t, ss.Focus().Save(rctx, &model.UserFocus{UserId: ended, StartAt: now - 2000, EndAt: now - 1000, EndedAt: now - 1500}))
	defer func() {
		for _, id := range []string{due, later, ended} {
			_ = ss.Focus().PermanentDeleteByUser(rctx, id)
		}
	}()

	expired, err := ss.Focus().GetExpired(rctx, now, 100)
	require.NoError(t, err)
	ids := []string{}
	for _, f := range expired {
		ids = append(ids, f.UserId)
	}
	assert.Contains(t, ids, due)
	assert.NotContains(t, ids, later, "a focus still running isn't expired")
	assert.NotContains(t, ids, ended, "a focus that ended isn't ended again")
}

// focusFixture is a user, someone else, and channels they're members of (or not), with posts at given times.
type focusFixture struct {
	rctx   request.CTX
	ss     store.Store
	t      *testing.T
	me     string
	other  string
	teamID string
}

func (f *focusFixture) channel(channelType model.ChannelType, lastViewedAt int64, member bool, notifyProps model.StringMap) *model.Channel {
	name := "focus-" + model.NewId()
	teamID := f.teamID
	if channelType == model.ChannelTypeDirect || channelType == model.ChannelTypeGroup {
		teamID = ""
	}
	channel, err := f.ss.Channel().Save(f.rctx, &model.Channel{TeamId: teamID, DisplayName: name, Name: name, Type: channelType}, -1)
	require.NoError(f.t, err)
	if notifyProps == nil {
		notifyProps = model.GetDefaultChannelNotifyProps()
	}
	if member {
		_, err = f.ss.Channel().SaveMember(f.rctx, &model.ChannelMember{ChannelId: channel.Id, UserId: f.me, NotifyProps: notifyProps, LastViewedAt: lastViewedAt})
		require.NoError(f.t, err)
	}
	_, err = f.ss.Channel().SaveMember(f.rctx, &model.ChannelMember{ChannelId: channel.Id, UserId: f.other, NotifyProps: model.GetDefaultChannelNotifyProps()})
	require.NoError(f.t, err)
	return channel
}

func (f *focusFixture) post(channelID, userID, rootID string, at int64) *model.Post {
	post, err := f.ss.Post().Save(f.rctx, &model.Post{ChannelId: channelID, UserId: userID, RootId: rootID, Message: "hello", CreateAt: at})
	require.NoError(f.t, err)
	return post
}

func newFocusFixture(t *testing.T, rctx request.CTX, ss store.Store) *focusFixture {
	return &focusFixture{rctx: rctx, ss: ss, t: t, me: model.NewId(), other: model.NewId(), teamID: model.NewId()}
}

func testFocusGetMissedChannels(t *testing.T, rctx request.CTX, ss store.Store) {
	f := newFocusFixture(t, rctx, ss)
	since := model.GetMillis() - 3600000
	until := since + 1800000

	busy := f.channel(model.ChannelTypeOpen, since-1000, true, nil)
	f.post(busy.Id, f.other, "", since-500) // before the focus: not in the summary
	f.post(busy.Id, f.other, "", since+1000)
	f.post(busy.Id, f.me, "", since+2000) // the user's own message isn't counted
	f.post(busy.Id, f.other, "", since+3000)
	f.post(busy.Id, f.other, "", until+1000) // after the end: not in the summary

	readDuring := f.channel(model.ChannelTypeOpen, since+5000, true, nil)
	f.post(readDuring.Id, f.other, "", since+4000) // read during the focus: not missed
	f.post(readDuring.Id, f.other, "", since+6000)

	notMine := f.channel(model.ChannelTypeOpen, 0, false, nil)
	f.post(notMine.Id, f.other, "", since+1000)

	archived := f.channel(model.ChannelTypePrivate, since-1000, true, nil)
	f.post(archived.Id, f.other, "", since+1000)
	require.NoError(t, ss.Channel().Delete(archived.Id, model.GetMillis()))

	quiet := f.channel(model.ChannelTypeOpen, since-1000, true, nil)
	f.post(quiet.Id, f.other, "", since-2000)

	threaded := f.channel(model.ChannelTypeOpen, since-1000, true, nil)
	root := f.post(threaded.Id, f.other, "", since-2000)
	f.post(threaded.Id, f.other, root.Id, since+1000)

	channels, err := ss.Focus().GetMissedChannels(rctx, f.me, since, until, false, 100)
	require.NoError(t, err)
	byID := map[string]*model.FocusSummaryChannel{}
	for _, c := range channels {
		byID[c.ChannelId] = c
	}

	require.Contains(t, byID, busy.Id)
	assert.Equal(t, int64(2), byID[busy.Id].Count, "only others' messages in the window")
	assert.Equal(t, since+3000, byID[busy.Id].LastPostAt)
	assert.Equal(t, f.other, byID[busy.Id].LastSenderId)
	assert.Equal(t, f.teamID, byID[busy.Id].TeamId)
	assert.Equal(t, model.ChannelTypeOpen, byID[busy.Id].Type)
	assert.Equal(t, "all", byID[busy.Id].MarkUnread)

	require.Contains(t, byID, readDuring.Id)
	assert.Equal(t, int64(1), byID[readDuring.Id].Count, "what was read during the focus isn't missed")

	assert.NotContains(t, byID, notMine.Id, "only the user's channels")
	assert.NotContains(t, byID, archived.Id, "no archived channels")
	assert.NotContains(t, byID, quiet.Id, "no channel without messages in the window")

	require.Contains(t, byID, threaded.Id)
	assert.Equal(t, int64(1), byID[threaded.Id].Count, "replies count without collapsed threads")

	rootOnly, err := ss.Focus().GetMissedChannels(rctx, f.me, since, until, true, 100)
	require.NoError(t, err)
	for _, c := range rootOnly {
		assert.NotEqual(t, threaded.Id, c.ChannelId, "with collapsed threads, replies are the threads' and not the channel's")
	}

	limited, err := ss.Focus().GetMissedChannels(rctx, f.me, since, until, false, 1)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(limited), 1, "bounded")
}

func testFocusGetMissedThreads(t *testing.T, rctx request.CTX, ss store.Store) {
	f := newFocusFixture(t, rctx, ss)
	since := model.GetMillis() - 3600000
	until := since + 1800000

	channel := f.channel(model.ChannelTypeOpen, since-1000, true, nil)
	followed := f.post(channel.Id, f.me, "", since-5000)
	f.post(channel.Id, f.other, followed.Id, since+1000)
	f.post(channel.Id, f.other, followed.Id, since+2000)
	f.post(channel.Id, f.me, followed.Id, since+2500)
	_, err := ss.Thread().MaintainMembership(f.me, followed.Id, store.ThreadMembershipOpts{Following: true, UpdateFollowing: true})
	require.NoError(t, err)
	require.NoError(t, ss.Thread().MarkAsRead(f.me, followed.Id, since-1000))

	unfollowed := f.post(channel.Id, f.other, "", since-5000)
	f.post(channel.Id, f.other, unfollowed.Id, since+1000)
	_, err = ss.Thread().MaintainMembership(f.me, unfollowed.Id, store.ThreadMembershipOpts{Following: false, UpdateFollowing: true})
	require.NoError(t, err)
	require.NoError(t, ss.Thread().MarkAsRead(f.me, unfollowed.Id, since-1000))

	// A thread followed in a channel the user left isn't theirs to read any more.
	left := f.channel(model.ChannelTypeOpen, since-1000, true, nil)
	leftRoot := f.post(left.Id, f.me, "", since-5000)
	f.post(left.Id, f.other, leftRoot.Id, since+1000)
	_, err = ss.Thread().MaintainMembership(f.me, leftRoot.Id, store.ThreadMembershipOpts{Following: true, UpdateFollowing: true})
	require.NoError(t, err)
	require.NoError(t, ss.Thread().MarkAsRead(f.me, leftRoot.Id, since-1000))
	require.NoError(t, ss.Channel().RemoveMember(rctx, left.Id, f.me))

	threads, err := ss.Focus().GetMissedThreads(rctx, f.me, since, until, 50)
	require.NoError(t, err)
	byID := map[string]*model.FocusSummaryThread{}
	for _, th := range threads {
		byID[th.PostId] = th
	}
	require.Contains(t, byID, followed.Id)
	assert.Equal(t, int64(2), byID[followed.Id].Count, "others' replies in the window")
	assert.Equal(t, f.other, byID[followed.Id].LastSenderId)
	assert.Equal(t, channel.Id, byID[followed.Id].ChannelId)
	assert.NotContains(t, byID, unfollowed.Id, "only followed threads")
	assert.NotContains(t, byID, leftRoot.Id, "only threads in the user's channels")
}

func testFocusGetFirstPostsAfter(t *testing.T, rctx request.CTX, ss store.Store) {
	f := newFocusFixture(t, rctx, ss)
	at := model.GetMillis() - 3600000
	channel := f.channel(model.ChannelTypeOpen, 0, true, nil)
	f.post(channel.Id, f.other, "", at-1000)
	first := f.post(channel.Id, f.other, "", at+1000)
	f.post(channel.Id, f.other, first.Id, at+500)
	f.post(channel.Id, f.other, "", at+2000)
	empty := f.channel(model.ChannelTypeOpen, 0, true, nil)

	firsts, err := ss.Focus().GetFirstPostsAfter(rctx, []string{channel.Id, empty.Id}, at, true)
	require.NoError(t, err)
	assert.Equal(t, first.Id, firsts[channel.Id], "the first root message after the time")
	assert.NotContains(t, firsts, empty.Id)

	firsts, err = ss.Focus().GetFirstPostsAfter(rctx, []string{channel.Id}, at, false)
	require.NoError(t, err)
	assert.NotEqual(t, first.Id, firsts[channel.Id], "a reply can come first without collapsed threads")

	firsts, err = ss.Focus().GetFirstPostsAfter(rctx, nil, at, false)
	require.NoError(t, err)
	assert.Empty(t, firsts)
}
