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

// SqlOnTopPostStore keeps the messages put on top of their channels (Antimatter).
type SqlOnTopPostStore struct {
	*SqlStore
}

func newSqlOnTopPostStore(sqlStore *SqlStore) store.OnTopPostStore {
	return &SqlOnTopPostStore{SqlStore: sqlStore}
}

var onTopPostColumns = []string{"o.PostId", "o.ChannelId", "o.CreatedBy", "o.CreateAt", "o.ExpiresAt"}

// GetForChannel gives the messages on top of a channel at a time, the latest put there first. Those deleted since,
// or moved to another channel, are left out.
func (s *SqlOnTopPostStore) GetForChannel(rctx request.CTX, channelID string, now int64) ([]*model.OnTopPost, error) {
	query := s.getQueryBuilder().
		Select(onTopPostColumns...).
		From("OnTopPosts o").
		Join("Posts p ON p.Id = o.PostId AND p.ChannelId = o.ChannelId").
		Where(sq.And{
			sq.Eq{"o.ChannelId": channelID, "p.DeleteAt": 0},
			sq.Or{sq.Eq{"o.ExpiresAt": 0}, sq.Gt{"o.ExpiresAt": now}},
		}).
		OrderBy("o.CreateAt DESC", "o.PostId")
	onTop := []*model.OnTopPost{}
	if err := s.DBXFromContext(rctx.Context()).SelectBuilder(&onTop, query); err != nil {
		return nil, errors.Wrapf(err, "failed to get the messages on top of channel=%s", channelID)
	}
	return onTop, nil
}

// Get gives a message's place on top of its channel, expired or not.
func (s *SqlOnTopPostStore) Get(rctx request.CTX, postID string) (*model.OnTopPost, error) {
	query := s.getQueryBuilder().
		Select(onTopPostColumns...).
		From("OnTopPosts o").
		Where(sq.Eq{"o.PostId": postID})
	var onTop model.OnTopPost
	if err := s.DBXFromContext(rctx.Context()).GetBuilder(&onTop, query); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, store.NewErrNotFound("OnTopPost", postID)
		}
		return nil, errors.Wrapf(err, "failed to get the message on top post=%s", postID)
	}
	return &onTop, nil
}

// Save puts a message on top of its channel; putting it there again sets who did it, when, and until when.
func (s *SqlOnTopPostStore) Save(rctx request.CTX, onTop *model.OnTopPost) (*model.OnTopPost, error) {
	if onTop.CreateAt == 0 {
		onTop.CreateAt = model.GetMillis()
	}
	builder := s.getQueryBuilder().
		Insert("OnTopPosts").
		Columns("PostId", "ChannelId", "CreatedBy", "CreateAt", "ExpiresAt").
		Values(onTop.PostId, onTop.ChannelId, onTop.CreatedBy, onTop.CreateAt, onTop.ExpiresAt).
		SuffixExpr(sq.Expr("ON CONFLICT (PostId) DO UPDATE SET ChannelId = EXCLUDED.ChannelId, CreatedBy = EXCLUDED.CreatedBy, CreateAt = EXCLUDED.CreateAt, ExpiresAt = EXCLUDED.ExpiresAt"))
	if _, err := s.GetMaster().ExecBuilder(builder); err != nil {
		return nil, errors.Wrapf(err, "failed to put post=%s on top", onTop.PostId)
	}
	return onTop, nil
}

func (s *SqlOnTopPostStore) Delete(rctx request.CTX, postID string) error {
	if _, err := s.GetMaster().ExecBuilder(s.getQueryBuilder().Delete("OnTopPosts").Where(sq.Eq{"PostId": postID})); err != nil {
		return errors.Wrapf(err, "failed to remove post=%s from the top", postID)
	}
	return nil
}

// DeleteExpired forgets a channel's messages whose time on top ended, and tells how many there were.
func (s *SqlOnTopPostStore) DeleteExpired(rctx request.CTX, channelID string, now int64) (int64, error) {
	builder := s.getQueryBuilder().
		Delete("OnTopPosts").
		Where(sq.And{sq.Eq{"ChannelId": channelID}, sq.Gt{"ExpiresAt": 0}, sq.LtOrEq{"ExpiresAt": now}})
	result, err := s.GetMaster().ExecBuilder(builder)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to delete the expired messages on top of channel=%s", channelID)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0, errors.Wrap(err, "failed to count the expired messages on top")
	}
	return count, nil
}
