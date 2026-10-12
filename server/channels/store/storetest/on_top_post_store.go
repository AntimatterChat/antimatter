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

func TestOnTopPostStore(t *testing.T, rctx request.CTX, ss store.Store) {
	t.Run("PutOnTopAndRemove", func(t *testing.T) { testOnTopPostPutAndRemove(t, rctx, ss) })
	t.Run("Expiry", func(t *testing.T) { testOnTopPostExpiry(t, rctx, ss) })
	t.Run("DeletedAndMovedPosts", func(t *testing.T) { testOnTopPostDeletedAndMoved(t, rctx, ss) })
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

	_, err := ss.OnTopPost().Save(rctx, &model.OnTopPost{PostId: first.Id, ChannelId: channelID, CreatedBy: by, CreateAt: now - 2000})
	require.NoError(t, err)
	_, err = ss.OnTopPost().Save(rctx, &model.OnTopPost{PostId: second.Id, ChannelId: channelID, CreatedBy: by, CreateAt: now - 1000, ExpiresAt: now + time.Hour.Milliseconds()})
	require.NoError(t, err)
	_, err = ss.OnTopPost().Save(rctx, &model.OnTopPost{PostId: elsewhere.Id, ChannelId: otherChannelID, CreatedBy: by, CreateAt: now})
	require.NoError(t, err)

	assert.Equal(t, []string{second.Id, first.Id}, onTopPostIDs(t, rctx, ss, channelID, now), "the latest put on top comes first")

	// Putting a message on top again moves it first, with its new expiry and who did it.
	other := model.NewId()
	_, err = ss.OnTopPost().Save(rctx, &model.OnTopPost{PostId: first.Id, ChannelId: channelID, CreatedBy: other, CreateAt: now, ExpiresAt: now + 2*time.Hour.Milliseconds()})
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
	assert.Equal(t, []string{elsewhere.Id}, onTopPostIDs(t, rctx, ss, otherChannelID, now), "other channels keep theirs")
}

func testOnTopPostExpiry(t *testing.T, rctx request.CTX, ss store.Store) {
	channelID := model.NewId()
	now := model.GetMillis()
	hour := time.Hour.Milliseconds()
	forAnHour, forever := onTopTestPost(t, rctx, ss, channelID), onTopTestPost(t, rctx, ss, channelID)
	_, err := ss.OnTopPost().Save(rctx, &model.OnTopPost{PostId: forAnHour.Id, ChannelId: channelID, CreateAt: now, ExpiresAt: now + hour})
	require.NoError(t, err)
	_, err = ss.OnTopPost().Save(rctx, &model.OnTopPost{PostId: forever.Id, ChannelId: channelID, CreateAt: now - 1})
	require.NoError(t, err)

	assert.Equal(t, []string{forAnHour.Id, forever.Id}, onTopPostIDs(t, rctx, ss, channelID, now))
	assert.Equal(t, []string{forever.Id}, onTopPostIDs(t, rctx, ss, channelID, now+hour), "an expired message is no longer on top")

	count, err := ss.OnTopPost().DeleteExpired(rctx, channelID, now)
	require.NoError(t, err)
	assert.Zero(t, count)
	count, err = ss.OnTopPost().DeleteExpired(rctx, channelID, now+hour)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count, "only the expired one goes, not the one until removed")
	_, err = ss.OnTopPost().Get(rctx, forAnHour.Id)
	require.Error(t, err)
	_, err = ss.OnTopPost().Get(rctx, forever.Id)
	require.NoError(t, err)
}

func testOnTopPostDeletedAndMoved(t *testing.T, rctx request.CTX, ss store.Store) {
	channelID := model.NewId()
	now := model.GetMillis()
	deleted, moved, kept := onTopTestPost(t, rctx, ss, channelID), onTopTestPost(t, rctx, ss, channelID), onTopTestPost(t, rctx, ss, channelID)
	for _, post := range []*model.Post{deleted, moved, kept} {
		_, err := ss.OnTopPost().Save(rctx, &model.OnTopPost{PostId: post.Id, ChannelId: channelID, CreateAt: now})
		require.NoError(t, err)
	}

	require.NoError(t, ss.Post().Delete(rctx, deleted.Id, model.GetMillis(), model.NewId()))
	moved.ChannelId = model.NewId()
	_, err := ss.Post().Overwrite(rctx, moved)
	require.NoError(t, err)

	assert.Equal(t, []string{kept.Id}, onTopPostIDs(t, rctx, ss, channelID, now), "deleted messages, and those moved elsewhere, leave the top")
}
