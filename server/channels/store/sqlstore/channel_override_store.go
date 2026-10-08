// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package sqlstore

import (
	"strings"

	sq "github.com/mattermost/squirrel"
	"github.com/pkg/errors"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

// SqlChannelOverrideStore keeps the permission overrides of channels (Antimatter), for roles and people.
type SqlChannelOverrideStore struct {
	*SqlStore

	selectQuery sq.SelectBuilder
}

// channelOverrideRow is an override as stored: its permission lists space-separated.
type channelOverrideRow struct {
	ChannelId  string
	TargetType string
	TargetId   string
	Allow      string
	Deny       string
	UpdateAt   int64
}

func (r *channelOverrideRow) toModel() *model.ChannelOverride {
	return &model.ChannelOverride{
		ChannelId:  r.ChannelId,
		TargetType: r.TargetType,
		TargetId:   r.TargetId,
		Allow:      strings.Fields(r.Allow),
		Deny:       strings.Fields(r.Deny),
		UpdateAt:   r.UpdateAt,
	}
}

func newSqlChannelOverrideStore(sqlStore *SqlStore) store.ChannelOverrideStore {
	s := &SqlChannelOverrideStore{SqlStore: sqlStore}

	s.selectQuery = s.getQueryBuilder().
		Select("ChannelId", "TargetType", "TargetId", "Allow", "Deny", "UpdateAt").
		From("ChannelOverrides")

	return s
}

func (s *SqlChannelOverrideStore) GetForChannel(rctx request.CTX, channelID string) ([]*model.ChannelOverride, error) {
	rows := []*channelOverrideRow{}
	if err := s.DBXFromContext(rctx.Context()).SelectBuilder(&rows, s.selectQuery.Where(sq.Eq{"ChannelId": channelID}).OrderBy("TargetType", "TargetId")); err != nil {
		return nil, errors.Wrapf(err, "failed to get overrides for channel=%s", channelID)
	}

	overrides := make([]*model.ChannelOverride, 0, len(rows))
	for _, r := range rows {
		overrides = append(overrides, r.toModel())
	}
	return overrides, nil
}

// Save stores an override in place of the one the channel had for the same role or person.
func (s *SqlChannelOverrideStore) Save(rctx request.CTX, override *model.ChannelOverride) (*model.ChannelOverride, error) {
	override.UpdateAt = model.GetMillis()
	builder := s.getQueryBuilder().
		Insert("ChannelOverrides").
		Columns("ChannelId", "TargetType", "TargetId", "Allow", "Deny", "UpdateAt").
		Values(override.ChannelId, override.TargetType, override.TargetId, strings.Join(override.Allow, " "), strings.Join(override.Deny, " "), override.UpdateAt).
		SuffixExpr(sq.Expr("ON CONFLICT (ChannelId, TargetType, TargetId) DO UPDATE SET Allow = EXCLUDED.Allow, Deny = EXCLUDED.Deny, UpdateAt = EXCLUDED.UpdateAt"))

	if _, err := s.GetMaster().ExecBuilder(builder); err != nil {
		return nil, errors.Wrapf(err, "failed to save override for channel=%s target=%s", override.ChannelId, override.TargetId)
	}
	return override, nil
}

func (s *SqlChannelOverrideStore) Delete(rctx request.CTX, channelID, targetType, targetID string) error {
	builder := s.getQueryBuilder().
		Delete("ChannelOverrides").
		Where(sq.Eq{"ChannelId": channelID, "TargetType": targetType, "TargetId": targetID})

	if _, err := s.GetMaster().ExecBuilder(builder); err != nil {
		return errors.Wrapf(err, "failed to delete override for channel=%s target=%s", channelID, targetID)
	}
	return nil
}
