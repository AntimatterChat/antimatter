// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package sqlstore

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
	"github.com/mattermost/mattermost/server/v8/channels/store/storetest"
)

func TestRenameFavoritesCategories(t *testing.T) {
	StoreTestWithSqlStore(t, testRenameFavoritesCategories)
}

func testRenameFavoritesCategories(t *testing.T, rctx request.CTX, ss store.Store, s storetest.SqlStore) {
	teamID := model.NewId()
	oldDefault, renamedByHand, other := model.NewId(), model.NewId(), model.NewId()
	t.Cleanup(func() {
		_, err := s.GetMaster().Exec("DELETE FROM SidebarCategories WHERE TeamId = ?", teamID)
		require.NoError(t, err)
	})

	favoritesID := func(userID string) string {
		return fmt.Sprintf("%s_%s_%s", model.SidebarCategoryFavorites, userID, teamID)
	}
	name := func(id string) string {
		category, err := ss.Channel().GetSidebarCategory(id)
		require.NoError(t, err)
		return category.DisplayName
	}
	setName := func(id, displayName string) {
		_, err := s.GetMaster().Exec("UPDATE SidebarCategories SET DisplayName = ? WHERE Id = ?", displayName, id)
		require.NoError(t, err)
	}

	for _, userID := range []string{oldDefault, renamedByHand, other} {
		_, err := ss.Channel().CreateInitialSidebarCategories(rctx, userID, teamID)
		require.NoError(t, err)
	}

	// New users' Favorites category is spelt the British way.
	assert.Equal(t, "Favourites", name(favoritesID(other)))

	// Before the upgrade, it was called Favorites. A category of another name, and a custom category called
	// Favorites, aren't the old default.
	setName(favoritesID(oldDefault), "Favorites")
	setName(favoritesID(renamedByHand), "Starred")
	custom, err := ss.Channel().CreateSidebarCategory(oldDefault, teamID, &model.SidebarCategoryWithChannels{
		SidebarCategory: model.SidebarCategory{DisplayName: "Favorites"},
	})
	require.NoError(t, err)

	t.Run("one batch at a time, in Id order", func(t *testing.T) {
		id := favoritesID(oldDefault)
		// A prefix of the Id sorts just before it.
		lastID, renamed, err := ss.Channel().RenameFavoritesCategories("Favorites", "Favourites", id[:len(id)-1], 1)
		require.NoError(t, err)
		assert.Equal(t, id, lastID)
		assert.EqualValues(t, 1, renamed)
		assert.Equal(t, "Favourites", name(id))

		// Already renamed: nothing to do.
		lastID, renamed, err = ss.Channel().RenameFavoritesCategories("Favorites", "Favourites", id[:len(id)-1], 1)
		require.NoError(t, err)
		assert.Equal(t, id, lastID)
		assert.EqualValues(t, 0, renamed)
	})

	t.Run("every batch", func(t *testing.T) {
		setName(favoritesID(oldDefault), "Favorites")
		afterID := ""
		for {
			lastID, _, err := ss.Channel().RenameFavoritesCategories("Favorites", "Favourites", afterID, 1000)
			require.NoError(t, err)
			if lastID == "" {
				break
			}
			require.NotEqual(t, afterID, lastID)
			afterID = lastID
		}

		assert.Equal(t, "Favourites", name(favoritesID(oldDefault)))
		assert.Equal(t, "Starred", name(favoritesID(renamedByHand)), "a name other than the old default stays")
		assert.Equal(t, "Favourites", name(favoritesID(other)))
		assert.Equal(t, "Favorites", name(custom.Id), "a custom category keeps the name its owner gave it")
	})

	t.Run("after the last one", func(t *testing.T) {
		// Longer than any Id, all of z: it sorts after every one, whatever the collation.
		lastID, renamed, err := ss.Channel().RenameFavoritesCategories("Favorites", "Favourites", strings.Repeat("z", 80), 10)
		require.NoError(t, err)
		assert.Empty(t, lastID)
		assert.Zero(t, renamed)
	})

	t.Run("needs a batch size", func(t *testing.T) {
		_, _, err := ss.Channel().RenameFavoritesCategories("Favorites", "Favourites", "", 0)
		require.Error(t, err)
	})
}
