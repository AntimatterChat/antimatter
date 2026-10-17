// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package sqlstore

import (
	sq "github.com/mattermost/squirrel"
	"github.com/pkg/errors"

	"github.com/mattermost/mattermost/server/public/model"
)

// RenameFavoritesCategories renames the Favorites sidebar categories still called from to to, one batch at a time: it
// looks at up to limit Favorites categories in Id order, after afterID, and renames those of them called from. It
// returns the Id of the last category it looked at, to pass as afterID for the next batch ("" once there are none
// left), and how many it renamed. A category whose name isn't from is left alone.
func (s SqlChannelStore) RenameFavoritesCategories(from, to, afterID string, limit int) (string, int64, error) {
	if limit <= 0 {
		return "", 0, errors.New("RenameFavoritesCategories: limit must be positive")
	}

	var ids []string
	query := s.getQueryBuilder().
		Select("Id").
		From("SidebarCategories").
		Where(sq.Eq{"Type": model.SidebarCategoryFavorites}).
		Where(sq.Gt{"Id": afterID}).
		OrderBy("Id ASC").
		Limit(uint64(limit))
	if err := s.GetMaster().SelectBuilder(&ids, query); err != nil {
		return "", 0, errors.Wrap(err, "failed to get Favorites sidebar categories")
	}
	if len(ids) == 0 {
		return "", 0, nil
	}

	result, err := s.GetMaster().ExecBuilder(s.getQueryBuilder().
		Update("SidebarCategories").
		Set("DisplayName", to).
		Where(sq.Eq{
			"Id":          ids,
			"Type":        model.SidebarCategoryFavorites,
			"DisplayName": from,
		}))
	if err != nil {
		return "", 0, errors.Wrap(err, "failed to rename Favorites sidebar categories")
	}
	renamed, err := result.RowsAffected()
	if err != nil {
		return "", 0, errors.Wrap(err, "failed to count renamed Favorites sidebar categories")
	}
	return ids[len(ids)-1], renamed, nil
}
