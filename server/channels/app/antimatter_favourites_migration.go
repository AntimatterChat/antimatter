// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"fmt"

	"github.com/mattermost/mattermost/server/public/shared/mlog"
)

const (
	favouritesCategoryMigrationKey = "AntimatterFavouritesCategoryMigrationComplete"

	// favoritesCategoryOldName is the name the server gave each Favorites sidebar category before Antimatter's English
	// interface said "Favourites". The store gives new ones favouritesCategoryName.
	favoritesCategoryOldName = "Favorites"
	favouritesCategoryName   = "Favourites"
)

// favouritesCategoryMigrationBatchSize is how many Favorites categories doFavouritesCategoryMigration looks at at once.
var favouritesCategoryMigrationBatchSize = 1000

// doFavouritesCategoryMigration runs once, on the first start of the version whose English interface says
// "Favourites": it renames the Favorites sidebar categories still called "Favorites" to "Favourites". The classic web
// app and the mobile app show their own, translated, label for that category, but Fusion shows the stored name. A
// Favorites category can't be renamed through the API, so a name other than the old default came from elsewhere (an
// import, say) and is left alone, as are custom categories called Favorites.
//
// It works one batch at a time, so that it never updates the whole table at once. Running it again before it's marked
// done is harmless: the categories it renamed aren't called Favorites any more.
func (s *Server) doFavouritesCategoryMigration() error {
	if done, err := s.migrationDone(favouritesCategoryMigrationKey); done || err != nil {
		return err
	}

	var renamed int64
	afterID := ""
	for {
		lastID, n, err := s.Store().Channel().RenameFavoritesCategories(favoritesCategoryOldName, favouritesCategoryName, afterID, favouritesCategoryMigrationBatchSize)
		if err != nil {
			return fmt.Errorf("failed to rename the Favorites categories: %w", err)
		}
		renamed += n
		if lastID == "" {
			break
		}
		afterID = lastID
	}
	if renamed > 0 {
		mlog.Info("Renamed the Favorites sidebar categories to Favourites", mlog.Int("categories", renamed))
	}

	return s.markMigrationDone(favouritesCategoryMigrationKey)
}
