// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package storetest

import (
	"errors"
	"fmt"
	"sync"
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
	t.Run("ReplaceRefusesStaleLists", func(t *testing.T) { testServerCategoryReplaceRefusesStaleLists(t, rctx, ss) })
	t.Run("TeamsAreSeparate", func(t *testing.T) { testServerCategoryTeamsAreSeparate(t, rctx, ss) })
	t.Run("AddForTeam", func(t *testing.T) { testServerCategoryAddForTeam(t, rctx, ss) })
	t.Run("AddForTeamConcurrently", func(t *testing.T) { testServerCategoryAddForTeamConcurrently(t, rctx, ss) })
	t.Run("DeleteForTeam", func(t *testing.T) { testServerCategoryDeleteForTeam(t, rctx, ss) })
}

func testServerCategoryReplaceAndGet(t *testing.T, rctx request.CTX, ss store.Store) {
	teamID := model.NewId()

	got, err := ss.ServerCategory().GetForTeam(rctx, teamID)
	require.NoError(t, err)
	assert.Empty(t, got)

	previous, _, err := ss.ServerCategory().ReplaceForTeam(rctx, teamID, []*model.ServerCategory{{Name: "Rules", Locked: true}, {Name: "Community"}}, nil)
	require.NoError(t, err)
	assert.Empty(t, previous)

	got, err = ss.ServerCategory().GetForTeam(rctx, teamID)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "Rules", got[0].Name)
	assert.True(t, got[0].Locked)
	assert.Equal(t, 0, got[0].SortOrder)
	assert.Equal(t, "Community", got[1].Name)
	assert.False(t, got[1].Locked)
	assert.Equal(t, 1, got[1].SortOrder)

	previous, _, err = ss.ServerCategory().ReplaceForTeam(rctx, teamID, nil, []string{"rules", " Community "})
	require.NoError(t, err)
	require.Len(t, previous, 2, "gives back the list it replaced")
	got, err = ss.ServerCategory().GetForTeam(rctx, teamID)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func testServerCategoryReplaceKeepsCreateAt(t *testing.T, rctx request.CTX, ss store.Store) {
	teamID := model.NewId()

	_, first, err := ss.ServerCategory().ReplaceForTeam(rctx, teamID, []*model.ServerCategory{{Name: "Projects"}}, nil)
	require.NoError(t, err)

	_, second, err := ss.ServerCategory().ReplaceForTeam(rctx, teamID, []*model.ServerCategory{{Name: "Teams"}, {Name: "Projects", Locked: true}}, nil)
	require.NoError(t, err)
	require.Len(t, second, 2)
	assert.Equal(t, first[0].CreateAt, second[1].CreateAt)
	assert.Equal(t, 1, second[1].SortOrder)
	assert.True(t, second[1].Locked)
}

func testServerCategoryReplaceRefusesStaleLists(t *testing.T, rctx request.CTX, ss store.Store) {
	teamID := model.NewId()
	_, _, err := ss.ServerCategory().ReplaceForTeam(rctx, teamID, []*model.ServerCategory{{Name: "Rules"}, {Name: "Ops"}}, nil)
	require.NoError(t, err)

	// A list made before "Ops" was added leaves it out without removing it.
	_, _, err = ss.ServerCategory().ReplaceForTeam(rctx, teamID, []*model.ServerCategory{{Name: "Rules", Locked: true}}, nil)
	var conflict *store.ErrConflict
	require.True(t, errors.As(err, &conflict), "expected a conflict, got %v", err)

	got, err := ss.ServerCategory().GetForTeam(rctx, teamID)
	require.NoError(t, err)
	require.Len(t, got, 2, "nothing changed")
	assert.False(t, got[0].Locked)
}

func testServerCategoryTeamsAreSeparate(t *testing.T, rctx request.CTX, ss store.Store) {
	teamA, teamB := model.NewId(), model.NewId()

	_, _, err := ss.ServerCategory().ReplaceForTeam(rctx, teamA, []*model.ServerCategory{{Name: "A"}}, nil)
	require.NoError(t, err)
	_, _, err = ss.ServerCategory().ReplaceForTeam(rctx, teamB, []*model.ServerCategory{{Name: "B"}}, nil)
	require.NoError(t, err)

	got, err := ss.ServerCategory().GetForTeam(rctx, teamA)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "A", got[0].Name)
}

func testServerCategoryAddForTeam(t *testing.T, rctx request.CTX, ss store.Store) {
	teamID := model.NewId()
	_, _, err := ss.ServerCategory().ReplaceForTeam(rctx, teamID, []*model.ServerCategory{{Name: "Rules", Locked: true}}, nil)
	require.NoError(t, err)

	added, ok, err := ss.ServerCategory().AddForTeam(rctx, teamID, "Voice")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "Voice", added.Name)
	assert.False(t, added.Locked)

	known, ok, err := ss.ServerCategory().AddForTeam(rctx, teamID, "rules")
	require.NoError(t, err)
	assert.False(t, ok, "a name the list has, ignoring case, isn't added again")
	assert.Equal(t, "Rules", known.Name)

	got, err := ss.ServerCategory().GetForTeam(rctx, teamID)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "Voice", got[1].Name)
	assert.Equal(t, 1, got[1].SortOrder)
}

func testServerCategoryAddForTeamConcurrently(t *testing.T, rctx request.CTX, ss store.Store) {
	teamID := model.NewId()
	var wg sync.WaitGroup
	errs := make(chan error, 10)
	for i := range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, err := ss.ServerCategory().AddForTeam(rctx, teamID, fmt.Sprintf("C%d", i%5))
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}

	got, err := ss.ServerCategory().GetForTeam(rctx, teamID)
	require.NoError(t, err)
	require.Len(t, got, 5, "each name once")
	orders := map[int]bool{}
	for _, c := range got {
		orders[c.SortOrder] = true
	}
	assert.Len(t, orders, 5, "each in its own place")
}

func testServerCategoryDeleteForTeam(t *testing.T, rctx request.CTX, ss store.Store) {
	teamA, teamB := model.NewId(), model.NewId()
	_, _, err := ss.ServerCategory().ReplaceForTeam(rctx, teamA, []*model.ServerCategory{{Name: "A"}}, nil)
	require.NoError(t, err)
	_, _, err = ss.ServerCategory().ReplaceForTeam(rctx, teamB, []*model.ServerCategory{{Name: "B"}}, nil)
	require.NoError(t, err)

	require.NoError(t, ss.ServerCategory().DeleteForTeam(rctx, teamA))

	got, err := ss.ServerCategory().GetForTeam(rctx, teamA)
	require.NoError(t, err)
	assert.Empty(t, got)
	got, err = ss.ServerCategory().GetForTeam(rctx, teamB)
	require.NoError(t, err)
	assert.Len(t, got, 1)
}
