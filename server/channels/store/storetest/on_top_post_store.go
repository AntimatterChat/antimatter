// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package storetest

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

func TestOnTopPostStore(t *testing.T, rctx request.CTX, ss store.Store) {
	t.Run("PutOnTopAndRemove", func(t *testing.T) { testOnTopPostPutAndRemove(t, rctx, ss) })
	t.Run("Expiry", func(t *testing.T) { testOnTopPostExpiry(t, rctx, ss) })
	t.Run("DeletedPosts", func(t *testing.T) { testOnTopPostDeleted(t, rctx, ss) })
	t.Run("Limit", func(t *testing.T) { testOnTopPostLimit(t, rctx, ss) })
	t.Run("DeleteForChannel", func(t *testing.T) { testOnTopPostDeleteForChannel(t, rctx, ss) })
	t.Run("PermanentDeleteByUser", func(t *testing.T) { testOnTopPostPermanentDeleteByUser(t, rctx, ss) })
}

// A message permanently deleted with its author's messages is no longer kept on top.
func testOnTopPostPermanentDeleteByUser(t *testing.T, rctx request.CTX, ss store.Store) {
	channelID := model.NewId()
	now := model.GetMillis()
	gone := onTopTestPost(t, rctx, ss, channelID)
	kept := onTopTestPost(t, rctx, ss, channelID)
	for _, post := range []*model.Post{gone, kept} {
		_, err := ss.OnTopPost().Save(rctx, &model.OnTopPost{PostId: post.Id, ChannelId: channelID, CreateAt: now}, model.OnTopPostsMaxPerChannel)
		require.NoError(t, err)
	}

	require.NoError(t, ss.Post().PermanentDeleteByUser(rctx, gone.UserId))

	var nfErr *store.ErrNotFound
	_, err := ss.OnTopPost().Get(rctx, gone.Id)
	assert.ErrorAs(t, err, &nfErr)
	_, err = ss.OnTopPost().Get(rctx, kept.Id)
	require.NoError(t, err)
}

func onTopTestPost(t *testing.T, rctx request.CTX, ss store.Store, channelID string) *model.Post {
	t.Helper()
	post, err := ss.Post().Save(rctx, &model.Post{ChannelId: channelID, UserId: model.NewId(), Message: "on top"})
	require.NoError(t, err)
	return post
}

func onTopPostIDs(t *testing.T, rctx request.CTX, ss store.Store, channelID string, now int64) []string {
	t.Helper()
	onTop, err := ss.OnTopPost().GetForChannel(rctx, channelID, now)
	require.NoError(t, err)
	ids := []string{}
	for _, o := range onTop {
		ids = append(ids, o.PostId)
	}
	return ids
}

func testOnTopPostPutAndRemove(t *testing.T, rctx request.CTX, ss store.Store) {
	channelID, otherChannelID, by := model.NewId(), model.NewId(), model.NewId()
	now := model.GetMillis()
	first, second, elsewhere := onTopTestPost(t, rctx, ss, channelID), onTopTestPost(t, rctx, ss, channelID), onTopTestPost(t, rctx, ss, otherChannelID)

	_, err := ss.OnTopPost().Save(rctx, &model.OnTopPost{PostId: first.Id, ChannelId: channelID, CreatedBy: by, CreateAt: now - 2000}, model.OnTopPostsMaxPerChannel)
	require.NoError(t, err)
	_, err = ss.OnTopPost().Save(rctx, &model.OnTopPost{PostId: second.Id, ChannelId: channelID, CreatedBy: by, CreateAt: now - 1000, ExpiresAt: now + time.Hour.Milliseconds()}, model.OnTopPostsMaxPerChannel)
	require.NoError(t, err)
	_, err = ss.OnTopPost().Save(rctx, &model.OnTopPost{PostId: elsewhere.Id, ChannelId: otherChannelID, CreatedBy: by, CreateAt: now}, model.OnTopPostsMaxPerChannel)
	require.NoError(t, err)

	assert.Equal(t, []string{second.Id, first.Id}, onTopPostIDs(t, rctx, ss, channelID, now), "the latest put on top comes first")

	// Putting a message on top again moves it first, with its new expiry and who did it.
	other := model.NewId()
	_, err = ss.OnTopPost().Save(rctx, &model.OnTopPost{PostId: first.Id, ChannelId: channelID, CreatedBy: other, CreateAt: now, ExpiresAt: now + 2*time.Hour.Milliseconds()}, model.OnTopPostsMaxPerChannel)
	require.NoError(t, err)
	assert.Equal(t, []string{first.Id, second.Id}, onTopPostIDs(t, rctx, ss, channelID, now))
	got, err := ss.OnTopPost().Get(rctx, first.Id)
	require.NoError(t, err)
	assert.Equal(t, other, got.CreatedBy)
	assert.Equal(t, now+2*time.Hour.Milliseconds(), got.ExpiresAt)

	require.NoError(t, ss.OnTopPost().Delete(rctx, first.Id))
	assert.Equal(t, []string{second.Id}, onTopPostIDs(t, rctx, ss, channelID, now))
	_, err = ss.OnTopPost().Get(rctx, first.Id)
	var nfErr *store.ErrNotFound
	assert.ErrorAs(t, err, &nfErr)
	assert.ErrorAs(t, ss.OnTopPost().Delete(rctx, first.Id), &nfErr, "it's no longer there")
	assert.Equal(t, []string{elsewhere.Id}, onTopPostIDs(t, rctx, ss, otherChannelID, now), "other channels keep theirs")
}

func testOnTopPostExpiry(t *testing.T, rctx request.CTX, ss store.Store) {
	channelID := model.NewId()
	now := model.GetMillis()
	hour := time.Hour.Milliseconds()
	forAnHour, forever := onTopTestPost(t, rctx, ss, channelID), onTopTestPost(t, rctx, ss, channelID)
	_, err := ss.OnTopPost().Save(rctx, &model.OnTopPost{PostId: forAnHour.Id, ChannelId: channelID, CreateAt: now, ExpiresAt: now + hour}, model.OnTopPostsMaxPerChannel)
	require.NoError(t, err)
	_, err = ss.OnTopPost().Save(rctx, &model.OnTopPost{PostId: forever.Id, ChannelId: channelID, CreateAt: now - 1}, model.OnTopPostsMaxPerChannel)
	require.NoError(t, err)

	assert.Equal(t, []string{forAnHour.Id, forever.Id}, onTopPostIDs(t, rctx, ss, channelID, now))
	assert.Equal(t, []string{forever.Id}, onTopPostIDs(t, rctx, ss, channelID, now+hour), "an expired message is no longer on top")

	// The next message put on top, once the hour is over, forgets the expired one, not the one until removed.
	next := onTopTestPost(t, rctx, ss, channelID)
	_, err = ss.OnTopPost().Save(rctx, &model.OnTopPost{PostId: next.Id, ChannelId: channelID, CreateAt: now + hour - 1}, model.OnTopPostsMaxPerChannel)
	require.NoError(t, err)
	_, err = ss.OnTopPost().Get(rctx, forAnHour.Id)
	require.NoError(t, err, "not expired yet")
	_, err = ss.OnTopPost().Save(rctx, &model.OnTopPost{PostId: next.Id, ChannelId: channelID, CreateAt: now + hour}, model.OnTopPostsMaxPerChannel)
	require.NoError(t, err)
	_, err = ss.OnTopPost().Get(rctx, forAnHour.Id)
	require.Error(t, err)
	_, err = ss.OnTopPost().Get(rctx, forever.Id)
	require.NoError(t, err)
}

func testOnTopPostDeleted(t *testing.T, rctx request.CTX, ss store.Store) {
	channelID := model.NewId()
	now := model.GetMillis()
	deleted, gone, kept := onTopTestPost(t, rctx, ss, channelID), onTopTestPost(t, rctx, ss, channelID), onTopTestPost(t, rctx, ss, channelID)
	for _, post := range []*model.Post{deleted, gone, kept} {
		_, err := ss.OnTopPost().Save(rctx, &model.OnTopPost{PostId: post.Id, ChannelId: channelID, CreateAt: now}, model.OnTopPostsMaxPerChannel)
		require.NoError(t, err)
	}

	require.NoError(t, ss.Post().Delete(rctx, deleted.Id, model.GetMillis(), model.NewId()))
	require.NoError(t, ss.Post().PermanentDelete(rctx, gone.Id))
	assert.Equal(t, []string{kept.Id}, onTopPostIDs(t, rctx, ss, channelID, now), "deleted messages leave the top")

	// The next message put on top forgets them.
	next := onTopTestPost(t, rctx, ss, channelID)
	_, err := ss.OnTopPost().Save(rctx, &model.OnTopPost{PostId: next.Id, ChannelId: channelID, CreateAt: now + 1}, model.OnTopPostsMaxPerChannel)
	require.NoError(t, err)
	for _, post := range []*model.Post{deleted, gone} {
		_, err = ss.OnTopPost().Get(rctx, post.Id)
		require.Error(t, err)
	}
	assert.Equal(t, []string{next.Id, kept.Id}, onTopPostIDs(t, rctx, ss, channelID, now+1))
}

func testOnTopPostLimit(t *testing.T, rctx request.CTX, ss store.Store) {
	channelID := model.NewId()
	now := model.GetMillis()
	const limit = 3
	posts := []*model.Post{}
	for i := range limit {
		post := onTopTestPost(t, rctx, ss, channelID)
		posts = append(posts, post)
		_, err := ss.OnTopPost().Save(rctx, &model.OnTopPost{PostId: post.Id, ChannelId: channelID, CreateAt: now + int64(i)}, limit)
		require.NoError(t, err)
	}

	_, err := ss.OnTopPost().Save(rctx, &model.OnTopPost{PostId: onTopTestPost(t, rctx, ss, channelID).Id, ChannelId: channelID, CreateAt: now + limit}, limit)
	var limitErr *store.ErrLimitExceeded
	require.ErrorAs(t, err, &limitErr, "the channel is full")
	_, err = ss.OnTopPost().Save(rctx, &model.OnTopPost{PostId: posts[0].Id, ChannelId: channelID, CreateAt: now + limit}, limit)
	require.NoError(t, err, "one already there can get a new time")

	// Concurrent puts into the last free place: only one gets it.
	require.NoError(t, ss.OnTopPost().Delete(rctx, posts[1].Id))
	const racers = 6
	candidates := make([]*model.Post, racers)
	for i := range candidates {
		candidates[i] = onTopTestPost(t, rctx, ss, channelID)
	}
	var wg sync.WaitGroup
	errs := make([]error, racers)
	for i, post := range candidates {
		wg.Go(func() {
			_, errs[i] = ss.OnTopPost().Save(rctx, &model.OnTopPost{PostId: post.Id, ChannelId: channelID, CreateAt: now + limit + 1}, limit)
		})
	}
	wg.Wait()
	saved := 0
	for _, err := range errs {
		if err == nil {
			saved++
		} else {
			require.ErrorAs(t, err, &limitErr)
		}
	}
	assert.Equal(t, 1, saved)
	assert.Len(t, onTopPostIDs(t, rctx, ss, channelID, now), limit)
}

func testOnTopPostDeleteForChannel(t *testing.T, rctx request.CTX, ss store.Store) {
	channelID, otherChannelID := model.NewId(), model.NewId()
	now := model.GetMillis()
	post, other := onTopTestPost(t, rctx, ss, channelID), onTopTestPost(t, rctx, ss, otherChannelID)
	_, err := ss.OnTopPost().Save(rctx, &model.OnTopPost{PostId: post.Id, ChannelId: channelID, CreateAt: now}, model.OnTopPostsMaxPerChannel)
	require.NoError(t, err)
	_, err = ss.OnTopPost().Save(rctx, &model.OnTopPost{PostId: other.Id, ChannelId: otherChannelID, CreateAt: now}, model.OnTopPostsMaxPerChannel)
	require.NoError(t, err)

	require.NoError(t, ss.OnTopPost().DeleteForChannel(rctx, channelID))
	_, err = ss.OnTopPost().Get(rctx, post.Id)
	require.Error(t, err)
	_, err = ss.OnTopPost().Get(rctx, other.Id)
	require.NoError(t, err, "other channels keep theirs")
}
