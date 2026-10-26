// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package localcachelayer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store/storetest"
	"github.com/mattermost/mattermost/server/v8/channels/store/storetest/mocks"
)

func TestReadReceiptStore(t *testing.T) {
	StoreTestWithSqlStore(t, storetest.TestReadReceiptStore)
}

// A single user's receipt, read while the post's readers aren't cached, never stands for all of them.
func TestReadReceiptLayer_GetByPost_AfterColdSingleUserGet(t *testing.T) {
	logger := mlog.CreateConsoleTestLogger(t)
	mockStore := getMockStore(t)
	receipts := mockStore.ReadReceipt().(*mocks.ReadReceiptStore)
	a := &model.ReadReceipt{PostID: "post", UserID: "a", ExpireAt: 10}
	b := &model.ReadReceipt{PostID: "post", UserID: "b", ExpireAt: 20}
	receipts.On("Get", mock.Anything, "post", "a").Return(a, nil)
	receipts.On("GetByPost", mock.Anything, "post").Return([]*model.ReadReceipt{a, b}, nil)

	cachedStore, err := NewLocalCacheLayer(mockStore, nil, nil, getMockCacheProvider(), logger)
	require.NoError(t, err)
	rctx := request.TestContext(t)

	got, err := cachedStore.ReadReceipt().Get(rctx, "post", "a")
	require.NoError(t, err)
	assert.Equal(t, "a", got.UserID)

	all, err := cachedStore.ReadReceipt().GetByPost(rctx, "post")
	require.NoError(t, err)
	var readers []string
	for _, r := range all {
		readers = append(readers, r.UserID)
	}
	assert.ElementsMatch(t, []string{"a", "b"}, readers, "b's receipt isn't hidden by a's")
	receipts.AssertNumberOfCalls(t, "GetByPost", 1)

	// Now the readers are cached by GetByPost itself: the next call is served from the cache.
	all, err = cachedStore.ReadReceipt().GetByPost(rctx, "post")
	require.NoError(t, err)
	assert.Len(t, all, 2)
	receipts.AssertNumberOfCalls(t, "GetByPost", 1)
}
