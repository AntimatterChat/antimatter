// Copyright (c) 2026-present Antimatter contributors.
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

// SqlAgentActivityStore keeps what each agent is doing now (Antimatter): one row per agent, replaced as it changes.
type SqlAgentActivityStore struct {
	*SqlStore
}

func newSqlAgentActivityStore(sqlStore *SqlStore) store.AgentActivityStore {
	return &SqlAgentActivityStore{SqlStore: sqlStore}
}

var agentActivityColumns = []string{"UserId", "State", "Text", "ThreadPostId", "UpdateAt", "ExpiresAt"}

func (s *SqlAgentActivityStore) Save(rctx request.CTX, activity *model.AgentActivity) error {
	if !model.IsValidId(activity.UserId) || !model.IsValidAgentState(activity.State) {
		return store.NewErrInvalidInput("AgentActivity", "UserId", activity.UserId)
	}
	builder := s.getQueryBuilder().
		Insert("AgentActivity").
		Columns(agentActivityColumns...).
		Values(activity.UserId, activity.State, activity.Text, activity.ThreadPostId, activity.UpdateAt, activity.ExpiresAt).
		SuffixExpr(sq.Expr("ON CONFLICT (UserId) DO UPDATE SET State = EXCLUDED.State, Text = EXCLUDED.Text, " +
			"ThreadPostId = EXCLUDED.ThreadPostId, UpdateAt = EXCLUDED.UpdateAt, ExpiresAt = EXCLUDED.ExpiresAt"))
	if _, err := s.GetMaster().ExecBuilder(builder); err != nil {
		return errors.Wrapf(err, "failed to save the activity of the agent user_id=%s", activity.UserId)
	}
	return nil
}

func (s *SqlAgentActivityStore) Get(rctx request.CTX, userID string) (*model.AgentActivity, error) {
	var activity model.AgentActivity
	query := s.getQueryBuilder().Select(agentActivityColumns...).From("AgentActivity").Where(sq.Eq{"UserId": userID})
	if err := s.DBXFromContext(rctx.Context()).GetBuilder(&activity, query); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, store.NewErrNotFound("AgentActivity", userID)
		}
		return nil, errors.Wrapf(err, "failed to get the activity of the agent user_id=%s", userID)
	}
	return &activity, nil
}

func (s *SqlAgentActivityStore) GetMany(rctx request.CTX, userIDs []string) ([]*model.AgentActivity, error) {
	activities := []*model.AgentActivity{}
	if len(userIDs) == 0 {
		return activities, nil
	}
	query := s.getQueryBuilder().Select(agentActivityColumns...).From("AgentActivity").Where(sq.Eq{"UserId": userIDs}).OrderBy("UserId")
	if err := s.DBXFromContext(rctx.Context()).SelectBuilder(&activities, query); err != nil {
		return nil, errors.Wrap(err, "failed to get the activities of agents")
	}
	return activities, nil
}

func (s *SqlAgentActivityStore) Delete(rctx request.CTX, userID string) error {
	if _, err := s.GetMaster().ExecBuilder(s.getQueryBuilder().Delete("AgentActivity").Where(sq.Eq{"UserId": userID})); err != nil {
		return errors.Wrapf(err, "failed to delete the activity of the agent user_id=%s", userID)
	}
	return nil
}
