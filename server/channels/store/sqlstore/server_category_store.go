// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package sqlstore

import (
	sq "github.com/mattermost/squirrel"
	"github.com/pkg/errors"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

// SqlServerCategoryStore keeps the sidebar categories admins set up for everyone in a team (Antimatter).
type SqlServerCategoryStore struct {
	*SqlStore

	selectQuery sq.SelectBuilder
}

func newSqlServerCategoryStore(sqlStore *SqlStore) store.ServerCategoryStore {
	s := &SqlServerCategoryStore{SqlStore: sqlStore}

	s.selectQuery = s.getQueryBuilder().
		Select("TeamId", "Name", "SortOrder", "Locked", "CreateAt", "UpdateAt").
		From("ServerCategories")

	return s
}

func (s *SqlServerCategoryStore) GetForTeam(rctx request.CTX, teamID string) ([]*model.ServerCategory, error) {
	query := s.selectQuery.Where(sq.Eq{"TeamId": teamID}).OrderBy("SortOrder", "Name")

	categories := []*model.ServerCategory{}
	if err := s.DBXFromContext(rctx.Context()).SelectBuilder(&categories, query); err != nil {
		return nil, errors.Wrapf(err, "failed to get server categories for team=%s", teamID)
	}

	return categories, nil
}

// ReplaceForTeam stores a team's categories in the order given, in place of those it had. Categories keep the time
// they were created when their name stays.
func (s *SqlServerCategoryStore) ReplaceForTeam(rctx request.CTX, teamID string, categories []*model.ServerCategory) (_ []*model.ServerCategory, err error) {
	tx, err := s.GetMaster().Begin()
	if err != nil {
		return nil, errors.Wrap(err, "failed to start transaction")
	}
	defer finalizeTransactionX(tx, &err)

	existing := []*model.ServerCategory{}
	if err = tx.SelectBuilder(&existing, s.selectQuery.Where(sq.Eq{"TeamId": teamID})); err != nil {
		return nil, errors.Wrapf(err, "failed to get server categories for team=%s", teamID)
	}
	created := make(map[string]int64, len(existing))
	for _, c := range existing {
		created[c.Name] = c.CreateAt
	}

	if _, err = tx.ExecBuilder(s.getQueryBuilder().Delete("ServerCategories").Where(sq.Eq{"TeamId": teamID})); err != nil {
		return nil, errors.Wrapf(err, "failed to delete server categories for team=%s", teamID)
	}

	now := model.GetMillis()
	result := make([]*model.ServerCategory, 0, len(categories))
	if len(categories) > 0 {
		insert := s.getQueryBuilder().
			Insert("ServerCategories").
			Columns("TeamId", "Name", "SortOrder", "Locked", "CreateAt", "UpdateAt")
		for i, c := range categories {
			createAt := now
			if at, ok := created[c.Name]; ok {
				createAt = at
			}
			stored := &model.ServerCategory{TeamId: teamID, Name: c.Name, SortOrder: i, Locked: c.Locked, CreateAt: createAt, UpdateAt: now}
			insert = insert.Values(stored.TeamId, stored.Name, stored.SortOrder, stored.Locked, stored.CreateAt, stored.UpdateAt)
			result = append(result, stored)
		}
		if _, err = tx.ExecBuilder(insert); err != nil {
			return nil, errors.Wrapf(err, "failed to save server categories for team=%s", teamID)
		}
	}

	if err = tx.Commit(); err != nil {
		return nil, errors.Wrap(err, "failed to commit server categories")
	}

	return result, nil
}
