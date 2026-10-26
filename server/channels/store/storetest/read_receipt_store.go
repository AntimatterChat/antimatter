// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package storetest

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

func TestReadReceiptStore(t *testing.T, rctx request.CTX, ss store.Store, s SqlStore) {
	t.Run("GetReadCountForPost", func(t *testing.T) { testGetReadCountForPost(t, rctx, ss) })
	t.Run("PermanentDeleteByUser", func(t *testing.T) { testReadReceiptPermanentDeleteByUser(t, rctx, ss) })
	t.Run("SaveConcurrentDuplicateIsIdempotent", func(t *testing.T) { testReadReceiptSaveConcurrentDuplicateIsIdempotent(t, rctx, ss) })
}

func testReadReceiptPermanentDeleteByUser(t *testing.T, rctx request.CTX, ss store.Store) {
	rrStore := ss.ReadReceipt()

	deletedUserID := model.NewId()
	survivingUserID := model.NewId()
	postID1 := model.NewId()
	postID2 := model.NewId()

	for _, receipt := range []*model.ReadReceipt{
		{PostID: postID1, UserID: deletedUserID},
		{PostID: postID2, UserID: deletedUserID},
		{PostID: postID1, UserID: survivingUserID},
	} {
		_, err := rrStore.Save(rctx, receipt)
		require.NoError(t, err)
	}

	// Read everything back first so that any caching layer is populated, and a stale entry
	// would survive the delete below.
	for _, postID := range []string{postID1, postID2} {
		receipts, err := rrStore.GetByPost(rctx, postID)
		require.NoError(t, err)
		require.NotEmpty(t, receipts)
	}
	_, err := rrStore.Get(rctx, postID1, deletedUserID)
	require.NoError(t, err)

	require.NoError(t, rrStore.PermanentDeleteByUser(rctx, deletedUserID))

	_, err = rrStore.Get(rctx, postID1, deletedUserID)
	require.Error(t, err)
	_, err = rrStore.Get(rctx, postID2, deletedUserID)
	require.Error(t, err)

	surviving, err := rrStore.Get(rctx, postID1, survivingUserID)
	require.NoError(t, err)
	require.Equal(t, survivingUserID, surviving.UserID)

	receipts, err := rrStore.GetByPost(rctx, postID1)
	require.NoError(t, err)
	require.Len(t, receipts, 1)
	require.Equal(t, survivingUserID, receipts[0].UserID)

	receipts, err = rrStore.GetByPost(rctx, postID2)
	require.NoError(t, err)
	require.Empty(t, receipts)
}

func testGetReadCountForPost(t *testing.T, rctx request.CTX, ss store.Store) {
	rrStore := ss.ReadReceipt()

	receipt1 := &model.ReadReceipt{
		PostID:   "post1",
		UserID:   "user1",
		ExpireAt: 0,
	}
	receipt2 := &model.ReadReceipt{
		PostID:   "post1",
		UserID:   "user2",
		ExpireAt: 0,
	}
	receipt3 := &model.ReadReceipt{
		PostID:   "post2",
		UserID:   "user3",
		ExpireAt: 0,
	}

	_, err := rrStore.Save(rctx, receipt1)
	if err != nil {
		t.Fatalf("failed to save read receipt 1: %v", err)
	}
	_, err = rrStore.Save(rctx, receipt2)
	if err != nil {
		t.Fatalf("failed to save read receipt 2: %v", err)
	}
	_, err = rrStore.Save(rctx, receipt3)
	if err != nil {
		t.Fatalf("failed to save read receipt 3: %v", err)
	}

	count, err := rrStore.GetReadCountForPost(rctx, "post1")
	if err != nil {
		t.Fatalf("failed to get read count for post1: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected read count for post1 to be 2, got %d", count)
	}

	count, err = rrStore.GetReadCountForPost(rctx, "post2")
	if err != nil {
		t.Fatalf("failed to get read count for post2: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected read count for post2 to be 1, got %d", count)
	}

	count, err = rrStore.GetReadCountForPost(rctx, "post3")
	if err != nil {
		t.Fatalf("failed to get read count for post3: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected read count for post3 to be 0, got %d", count)
	}
}

// Two reveals of the same post by the same user at once (two tabs, a double tap) both succeed: the second gets the
// receipt the first saved.
func testReadReceiptSaveConcurrentDuplicateIsIdempotent(t *testing.T, rctx request.CTX, ss store.Store) {
	postID, userID := model.NewId(), model.NewId()
	const reveals = 4
	var wg sync.WaitGroup
	saved := make([]*model.ReadReceipt, reveals)
	asked := make([]*model.ReadReceipt, reveals)
	errs := make([]error, reveals)
	for i := range reveals {
		asked[i] = &model.ReadReceipt{PostID: postID, UserID: userID, ExpireAt: model.GetMillis() + int64(1000*(i+1))}
		wg.Go(func() { saved[i], errs[i] = ss.ReadReceipt().Save(rctx, asked[i]) })
	}
	wg.Wait()

	first := 0
	stored, err := ss.ReadReceipt().Get(rctx, postID, userID)
	require.NoError(t, err)
	for i := range reveals {
		require.NoError(t, errs[i], "reveal %d", i)
		assert.Equal(t, stored.ExpireAt, saved[i].ExpireAt, "every reveal gets the stored receipt")
		if saved[i] == asked[i] {
			first++
		}
	}
	assert.Equal(t, 1, first, "one reveal saved it")
}
