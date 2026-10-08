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

func TestChannelOverrideStore(t *testing.T, rctx request.CTX, ss store.Store) {
	t.Run("SaveReplacesAndDeletes", func(t *testing.T) { testChannelOverrideSaveReplacesAndDeletes(t, rctx, ss) })
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
