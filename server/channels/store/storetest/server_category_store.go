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

func TestServerCategoryStore(t *testing.T, rctx request.CTX, ss store.Store) {
	t.Run("ReplaceAndGet", func(t *testing.T) { testServerCategoryReplaceAndGet(t, rctx, ss) })
	t.Run("ReplaceKeepsCreateAt", func(t *testing.T) { testServerCategoryReplaceKeepsCreateAt(t, rctx, ss) })
	t.Run("TeamsAreSeparate", func(t *testing.T) { testServerCategoryTeamsAreSeparate(t, rctx, ss) })
}

func testServerCategoryReplaceAndGet(t *testing.T, rctx request.CTX, ss store.Store) {
	teamID := model.NewId()

	got, err := ss.ServerCategory().GetForTeam(rctx, teamID)
	require.NoError(t, err)
	assert.Empty(t, got)

	_, err = ss.ServerCategory().ReplaceForTeam(rctx, teamID, []*model.ServerCategory{{Name: "Rules", Locked: true}, {Name: "Community"}})
	require.NoError(t, err)

	got, err = ss.ServerCategory().GetForTeam(rctx, teamID)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "Rules", got[0].Name)
	assert.True(t, got[0].Locked)
	assert.Equal(t, 0, got[0].SortOrder)
	assert.Equal(t, "Community", got[1].Name)
	assert.False(t, got[1].Locked)
	assert.Equal(t, 1, got[1].SortOrder)

	_, err = ss.ServerCategory().ReplaceForTeam(rctx, teamID, nil)
	require.NoError(t, err)
	got, err = ss.ServerCategory().GetForTeam(rctx, teamID)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func testServerCategoryReplaceKeepsCreateAt(t *testing.T, rctx request.CTX, ss store.Store) {
	teamID := model.NewId()

	first, err := ss.ServerCategory().ReplaceForTeam(rctx, teamID, []*model.ServerCategory{{Name: "Projects"}})
	require.NoError(t, err)

	second, err := ss.ServerCategory().ReplaceForTeam(rctx, teamID, []*model.ServerCategory{{Name: "Teams"}, {Name: "Projects", Locked: true}})
	require.NoError(t, err)
	require.Len(t, second, 2)
	assert.Equal(t, first[0].CreateAt, second[1].CreateAt)
	assert.Equal(t, 1, second[1].SortOrder)
	assert.True(t, second[1].Locked)
}

func testServerCategoryTeamsAreSeparate(t *testing.T, rctx request.CTX, ss store.Store) {
	teamA, teamB := model.NewId(), model.NewId()

	_, err := ss.ServerCategory().ReplaceForTeam(rctx, teamA, []*model.ServerCategory{{Name: "A"}})
	require.NoError(t, err)
	_, err = ss.ServerCategory().ReplaceForTeam(rctx, teamB, []*model.ServerCategory{{Name: "B"}})
	require.NoError(t, err)

	got, err := ss.ServerCategory().GetForTeam(rctx, teamA)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "A", got[0].Name)
}
