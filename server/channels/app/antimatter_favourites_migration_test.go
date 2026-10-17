// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

func TestFavouritesCategoryMigration(t *testing.T) {
	th := Setup(t).InitBasic(t)

	favoritesCategory := func(userID string) *model.SidebarCategoryWithChannels {
		t.Helper()
		categories, err := th.Store.Channel().GetSidebarCategoriesForTeamForUser(userID, th.BasicTeam.Id)
		require.NoError(t, err)
		for _, category := range categories.Categories {
			if category.Type == model.SidebarCategoryFavorites {
				return category
			}
		}
		require.FailNow(t, "no Favorites category", "user %s", userID)
		return nil
	}
	name := func(id string) string {
		t.Helper()
		category, err := th.Store.Channel().GetSidebarCategory(id)
		require.NoError(t, err)
		return category.DisplayName
	}
	setName := func(id, displayName string) {
		t.Helper()
		_, err := th.SQLStore.GetMaster().Exec("UPDATE SidebarCategories SET DisplayName = ? WHERE Id = ?", displayName, id)
		require.NoError(t, err)
	}

	// New members get a Favourites category: the store and the migration agree on the name.
	oldDefault := favoritesCategory(th.BasicUser.Id)
	assert.Equal(t, favouritesCategoryName, oldDefault.DisplayName)
	assert.Equal(t, "Favourites", favouritesCategoryName)

	// Before the upgrade, it was called Favorites.
	setName(oldDefault.Id, "Favorites")
	otherName := favoritesCategory(th.BasicUser2.Id)
	setName(otherName.Id, "Starred")
	custom, err := th.Store.Channel().CreateSidebarCategory(th.BasicUser.Id, th.BasicTeam.Id, &model.SidebarCategoryWithChannels{
		SidebarCategory: model.SidebarCategory{DisplayName: "Favorites"},
	})
	require.NoError(t, err)

	_, err = th.Store.System().PermanentDeleteByName(favouritesCategoryMigrationKey)
	require.NoError(t, err)
	require.NoError(t, th.Server.doFavouritesCategoryMigration())

	assert.Equal(t, "Favourites", name(oldDefault.Id), "the old default name becomes Favourites")
	assert.Equal(t, "Starred", name(otherName.Id), "another name stays")
	assert.Equal(t, "Favorites", name(custom.Id), "a custom category keeps the name its owner gave it")
	assert.Equal(t, model.SidebarCategoryCustom, custom.Type)

	_, err = th.Store.System().GetByName(favouritesCategoryMigrationKey)
	require.NoError(t, err, "the migration is recorded as done")

	t.Run("runs once", func(t *testing.T) {
		setName(oldDefault.Id, "Favorites")
		require.NoError(t, th.Server.doFavouritesCategoryMigration())
		assert.Equal(t, "Favorites", name(oldDefault.Id))
	})

	t.Run("in batches", func(t *testing.T) {
		batchSize := favouritesCategoryMigrationBatchSize
		favouritesCategoryMigrationBatchSize = 2
		t.Cleanup(func() { favouritesCategoryMigrationBatchSize = batchSize })

		user := th.CreateUser(t)
		th.LinkUserToTeam(t, user, th.BasicTeam)
		another := favoritesCategory(user.Id)
		setName(oldDefault.Id, "Favorites")
		setName(another.Id, "Favorites")

		_, err := th.Store.System().PermanentDeleteByName(favouritesCategoryMigrationKey)
		require.NoError(t, err)
		require.NoError(t, th.Server.doFavouritesCategoryMigration())
		assert.Equal(t, "Favourites", name(oldDefault.Id))
		assert.Equal(t, "Favourites", name(another.Id))
	})
}
