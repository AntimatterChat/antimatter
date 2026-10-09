// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package sqlstore

import (
	"slices"
	"strings"

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

// lockTeam serialises the changes of a team's categories, within the transaction.
func (s *SqlServerCategoryStore) lockTeam(tx *sqlxTxWrapper, teamID string) error {
	_, err := tx.Exec("SELECT pg_advisory_xact_lock(hashtext($1))", "servercategories:"+teamID)
	return errors.Wrapf(err, "failed to lock server categories for team=%s", teamID)
}

// ReplaceForTeam stores a team's categories in the order given, in place of those it had, and gives back those it
// had. Categories keep the time they were created when their name stays. A category left out must be named in
// removed (ignoring case): otherwise the list was made from an older one, and nothing changes (ErrConflict).
func (s *SqlServerCategoryStore) ReplaceForTeam(rctx request.CTX, teamID string, categories []*model.ServerCategory, removed []string) (_ []*model.ServerCategory, _ []*model.ServerCategory, err error) {
	tx, err := s.GetMaster().Begin()
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to start transaction")
	}
	defer finalizeTransactionX(tx, &err)

	if err = s.lockTeam(tx, teamID); err != nil {
		return nil, nil, err
	}

	existing := []*model.ServerCategory{}
	if err = tx.SelectBuilder(&existing, s.selectQuery.Where(sq.Eq{"TeamId": teamID}).OrderBy("SortOrder", "Name")); err != nil {
		return nil, nil, errors.Wrapf(err, "failed to get server categories for team=%s", teamID)
	}
	created := make(map[string]int64, len(existing))
	for _, c := range existing {
		created[c.Name] = c.CreateAt
		if model.FindServerCategory(categories, c.Name) == nil && !slices.ContainsFunc(removed, func(r string) bool { return strings.EqualFold(strings.TrimSpace(r), c.Name) }) {
			return nil, nil, store.NewErrConflict("ServerCategories", errors.New("the list left out a category it didn't remove"), "name="+c.Name)
		}
	}

	if _, err = tx.ExecBuilder(s.getQueryBuilder().Delete("ServerCategories").Where(sq.Eq{"TeamId": teamID})); err != nil {
		return nil, nil, errors.Wrapf(err, "failed to delete server categories for team=%s", teamID)
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
			return nil, nil, errors.Wrapf(err, "failed to save server categories for team=%s", teamID)
		}
	}

	if err = tx.Commit(); err != nil {
		return nil, nil, errors.Wrap(err, "failed to commit server categories")
	}

	return existing, result, nil
}

// AddForTeam adds a category, open, at the end of a team's list unless it has one by that name (ignoring case); it
// gives back the category, and whether it was added.
func (s *SqlServerCategoryStore) AddForTeam(rctx request.CTX, teamID string, name string) (_ *model.ServerCategory, _ bool, err error) {
	tx, err := s.GetMaster().Begin()
	if err != nil {
		return nil, false, errors.Wrap(err, "failed to start transaction")
	}
	defer finalizeTransactionX(tx, &err)

	if err = s.lockTeam(tx, teamID); err != nil {
		return nil, false, err
	}

	existing := []*model.ServerCategory{}
	if err = tx.SelectBuilder(&existing, s.selectQuery.Where(sq.Eq{"TeamId": teamID})); err != nil {
		return nil, false, errors.Wrapf(err, "failed to get server categories for team=%s", teamID)
	}
	if c := model.FindServerCategory(existing, name); c != nil {
		return c, false, nil
	}
	sortOrder := 0
	for _, c := range existing {
		sortOrder = max(sortOrder, c.SortOrder+1)
	}
	now := model.GetMillis()
	category := &model.ServerCategory{TeamId: teamID, Name: name, SortOrder: sortOrder, CreateAt: now, UpdateAt: now}
	insert := s.getQueryBuilder().
		Insert("ServerCategories").
		Columns("TeamId", "Name", "SortOrder", "Locked", "CreateAt", "UpdateAt").
		Values(category.TeamId, category.Name, category.SortOrder, category.Locked, category.CreateAt, category.UpdateAt)
	if _, err = tx.ExecBuilder(insert); err != nil {
		return nil, false, errors.Wrapf(err, "failed to add a server category for team=%s", teamID)
	}

	if err = tx.Commit(); err != nil {
		return nil, false, errors.Wrap(err, "failed to commit server categories")
	}
	return category, true, nil
}

// DeleteForTeam removes a team's categories, as the team goes.
func (s *SqlServerCategoryStore) DeleteForTeam(rctx request.CTX, teamID string) error {
	if _, err := s.GetMaster().ExecBuilder(s.getQueryBuilder().Delete("ServerCategories").Where(sq.Eq{"TeamId": teamID})); err != nil {
		return errors.Wrapf(err, "failed to delete server categories for team=%s", teamID)
	}
	return nil
}
