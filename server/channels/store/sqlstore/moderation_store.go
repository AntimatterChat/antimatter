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

// SqlModerationStore keeps what a team's moderators use (Antimatter): timeouts, and post counts.
type SqlModerationStore struct {
	*SqlStore
}

func newSqlModerationStore(sqlStore *SqlStore) store.ModerationStore {
	return &SqlModerationStore{SqlStore: sqlStore}
}

// GetTimeouts gives a team's timeouts that still hold at a time.
func (s *SqlModerationStore) GetTimeouts(rctx request.CTX, teamID string, now int64) ([]*model.MemberTimeout, error) {
	query := s.getQueryBuilder().
		Select("TeamId", "UserId", "Until", "Reason", "CreatedBy", "CreateAt").
		From("MemberTimeouts").
		Where(sq.And{sq.Eq{"TeamId": teamID}, sq.Gt{"Until": now}}).
		OrderBy("UserId")
	timeouts := []*model.MemberTimeout{}
	if err := s.DBXFromContext(rctx.Context()).SelectBuilder(&timeouts, query); err != nil {
		return nil, errors.Wrapf(err, "failed to get timeouts for team=%s", teamID)
	}
	return timeouts, nil
}

// SaveTimeout times someone out, in place of a timeout they had.
func (s *SqlModerationStore) SaveTimeout(rctx request.CTX, timeout *model.MemberTimeout) (*model.MemberTimeout, error) {
	timeout.CreateAt = model.GetMillis()
	builder := s.getQueryBuilder().
		Insert("MemberTimeouts").
		Columns("TeamId", "UserId", "Until", "Reason", "CreatedBy", "CreateAt").
		Values(timeout.TeamId, timeout.UserId, timeout.Until, timeout.Reason, timeout.CreatedBy, timeout.CreateAt).
		SuffixExpr(sq.Expr("ON CONFLICT (TeamId, UserId) DO UPDATE SET Until = EXCLUDED.Until, Reason = EXCLUDED.Reason, CreatedBy = EXCLUDED.CreatedBy, CreateAt = EXCLUDED.CreateAt"))
	if _, err := s.GetMaster().ExecBuilder(builder); err != nil {
		return nil, errors.Wrapf(err, "failed to save timeout for team=%s user=%s", timeout.TeamId, timeout.UserId)
	}
	return timeout, nil
}

func (s *SqlModerationStore) DeleteTimeout(rctx request.CTX, teamID, userID string) error {
	if _, err := s.GetMaster().ExecBuilder(s.getQueryBuilder().Delete("MemberTimeouts").Where(sq.Eq{"TeamId": teamID, "UserId": userID})); err != nil {
		return errors.Wrapf(err, "failed to delete timeout for team=%s user=%s", teamID, userID)
	}
	return nil
}

func (s *SqlModerationStore) DeleteTimeoutsForTeam(rctx request.CTX, teamID string) error {
	if _, err := s.GetMaster().ExecBuilder(s.getQueryBuilder().Delete("MemberTimeouts").Where(sq.Eq{"TeamId": teamID})); err != nil {
		return errors.Wrapf(err, "failed to delete timeouts for team=%s", teamID)
	}
	return nil
}

// CountPosts counts someone's messages in a team's channels since a time, their system messages left out.
func (s *SqlModerationStore) CountPosts(rctx request.CTX, teamID, userID string, since int64) (int64, error) {
	query := s.getQueryBuilder().
		Select("COUNT(*)").
		From("Posts p").
		Join("Channels c ON c.Id = p.ChannelId").
		Where(sq.And{
			sq.Eq{"p.UserId": userID, "c.TeamId": teamID, "p.DeleteAt": 0},
			sq.GtOrEq{"p.CreateAt": since},
			sq.NotLike{"p.Type": model.PostSystemMessagePrefix + "%"},
		})
	var count int64
	if err := s.DBXFromContext(rctx.Context()).GetBuilder(&count, query); err != nil {
		return 0, errors.Wrapf(err, "failed to count posts for team=%s user=%s", teamID, userID)
	}
	return count, nil
}
