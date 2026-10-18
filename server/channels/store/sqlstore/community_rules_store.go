// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package sqlstore

import (
	"database/sql"

	sq "github.com/mattermost/squirrel"
	"github.com/pkg/errors"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

// SqlCommunityRulesStore keeps each community's rules (Antimatter), and who agreed to which version of them.
type SqlCommunityRulesStore struct {
	*SqlStore
}

func newSqlCommunityRulesStore(sqlStore *SqlStore) store.CommunityRulesStore {
	return &SqlCommunityRulesStore{SqlStore: sqlStore}
}

var communityRulesColumns = []string{"TeamId", "Enabled", "Text", "Version", "UpdatedBy", "UpdateAt"}

func (s *SqlCommunityRulesStore) Get(rctx request.CTX, teamID string) (*model.CommunityRules, error) {
	query := s.getQueryBuilder().
		Select(communityRulesColumns...).
		From("CommunityRules").
		Where(sq.Eq{"TeamId": teamID})
	var rules model.CommunityRules
	if err := s.DBXFromContext(rctx.Context()).GetBuilder(&rules, query); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, store.NewErrNotFound("CommunityRules", teamID)
		}
		return nil, errors.Wrapf(err, "failed to get the rules of team=%s", teamID)
	}
	return &rules, nil
}

// Save works out the version in the database, so two admins saving at once can't both ask for the same new one.
func (s *SqlCommunityRulesStore) Save(rctx request.CTX, rules *model.CommunityRules, askAgain bool) (*model.CommunityRules, error) {
	rules.UpdateAt = model.GetMillis()
	increment := 0
	if askAgain {
		increment = 1
	}
	builder := s.getQueryBuilder().
		Insert("CommunityRules").
		Columns(communityRulesColumns...).
		Values(rules.TeamId, rules.Enabled, rules.Text, 1, rules.UpdatedBy, rules.UpdateAt).
		SuffixExpr(sq.Expr("ON CONFLICT (TeamId) DO UPDATE SET Enabled = EXCLUDED.Enabled, Text = EXCLUDED.Text, UpdatedBy = EXCLUDED.UpdatedBy, UpdateAt = EXCLUDED.UpdateAt, Version = CommunityRules.Version + ? RETURNING Version", increment))
	var version int64
	if err := s.GetMaster().GetBuilder(&version, builder); err != nil {
		return nil, errors.Wrapf(err, "failed to save the rules of team=%s", rules.TeamId)
	}
	rules.Version = version
	return rules, nil
}

func (s *SqlCommunityRulesStore) Accept(rctx request.CTX, acceptance *model.CommunityRulesAcceptance) error {
	if acceptance.AcceptedAt == 0 {
		acceptance.AcceptedAt = model.GetMillis()
	}
	builder := s.getQueryBuilder().
		Insert("CommunityRulesAcceptances").
		Columns("TeamId", "UserId", "Version", "AcceptedAt").
		Values(acceptance.TeamId, acceptance.UserId, acceptance.Version, acceptance.AcceptedAt).
		Suffix("ON CONFLICT (TeamId, UserId, Version) DO NOTHING")
	if _, err := s.GetMaster().ExecBuilder(builder); err != nil {
		return errors.Wrapf(err, "failed to save the agreement of user=%s to the rules of team=%s", acceptance.UserId, acceptance.TeamId)
	}
	return nil
}

func (s *SqlCommunityRulesStore) GetAcceptedVersion(rctx request.CTX, teamID, userID string) (int64, error) {
	query := s.getQueryBuilder().
		Select("COALESCE(MAX(Version), 0)").
		From("CommunityRulesAcceptances").
		Where(sq.Eq{"TeamId": teamID, "UserId": userID})
	var version int64
	if err := s.DBXFromContext(rctx.Context()).GetBuilder(&version, query); err != nil {
		return 0, errors.Wrapf(err, "failed to get the version of the rules of team=%s user=%s agreed to", teamID, userID)
	}
	return version, nil
}

func (s *SqlCommunityRulesStore) GetAcceptancesForUser(rctx request.CTX, userID string) ([]*model.CommunityRulesAcceptance, error) {
	query := s.getQueryBuilder().
		Select("TeamId", "UserId", "Version", "AcceptedAt").
		From("CommunityRulesAcceptances").
		Where(sq.Eq{"UserId": userID}).
		OrderBy("AcceptedAt DESC", "TeamId", "Version DESC")
	acceptances := []*model.CommunityRulesAcceptance{}
	if err := s.DBXFromContext(rctx.Context()).SelectBuilder(&acceptances, query); err != nil {
		return nil, errors.Wrapf(err, "failed to get the agreements of user=%s", userID)
	}
	return acceptances, nil
}

func (s *SqlCommunityRulesStore) CountAcceptances(rctx request.CTX, teamID string, version int64) (*model.CommunityRulesStats, error) {
	query := s.getQueryBuilder().
		Select("COUNT(*) AS Members", "COUNT(a.UserId) AS Accepted").
		From("TeamMembers tm").
		Join("Users u ON u.Id = tm.UserId").
		LeftJoin("Bots b ON b.UserId = tm.UserId").
		LeftJoin("CommunityRulesAcceptances a ON a.TeamId = tm.TeamId AND a.UserId = tm.UserId AND a.Version = ?", version).
		Where(sq.Eq{"tm.TeamId": teamID, "tm.DeleteAt": 0, "u.DeleteAt": 0, "b.UserId": nil})
	var counts struct {
		Members  int64
		Accepted int64
	}
	if err := s.DBXFromContext(rctx.Context()).GetBuilder(&counts, query); err != nil {
		return nil, errors.Wrapf(err, "failed to count the agreements to the rules of team=%s", teamID)
	}
	return &model.CommunityRulesStats{TeamId: teamID, Version: version, AcceptedCount: counts.Accepted, MemberCount: counts.Members}, nil
}

func (s *SqlCommunityRulesStore) DeleteForTeam(rctx request.CTX, teamID string) error {
	if _, err := s.GetMaster().ExecBuilder(s.getQueryBuilder().Delete("CommunityRulesAcceptances").Where(sq.Eq{"TeamId": teamID})); err != nil {
		return errors.Wrapf(err, "failed to delete the agreements to the rules of team=%s", teamID)
	}
	if _, err := s.GetMaster().ExecBuilder(s.getQueryBuilder().Delete("CommunityRules").Where(sq.Eq{"TeamId": teamID})); err != nil {
		return errors.Wrapf(err, "failed to delete the rules of team=%s", teamID)
	}
	return nil
}

func (s *SqlCommunityRulesStore) PermanentDeleteByUser(rctx request.CTX, userID string) error {
	if _, err := s.GetMaster().ExecBuilder(s.getQueryBuilder().Delete("CommunityRulesAcceptances").Where(sq.Eq{"UserId": userID})); err != nil {
		return errors.Wrapf(err, "failed to delete the agreements of user=%s", userID)
	}
	return nil
}
