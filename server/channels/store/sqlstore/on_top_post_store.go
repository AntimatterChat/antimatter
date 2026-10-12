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

// GetForChannel gives the messages on top of a channel at a time, the latest put there first. Those deleted since are
// left out. Messages never change channel in place (moving a thread copies it and deletes the original), so the post's
// channel is the one it was put on top of.
func (s *SqlOnTopPostStore) GetForChannel(rctx request.CTX, channelID string, now int64) ([]*model.OnTopPost, error) {
	query := s.getQueryBuilder().
		Select(onTopPostColumns...).
		From("OnTopPosts o").
		Join("Posts p ON p.Id = o.PostId").
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

// Save puts a message on top of its channel; putting it there again sets who did it, when, and until when. In one
// transaction, holding a lock on the channel's messages on top, it first forgets those of the channel that are stale
// (expired at onTop.CreateAt, deleted, or whose message is gone), then refuses with an ErrLimitExceeded when the
// channel already has maxPerChannel others on top, so that concurrent puts can't go over the limit.
func (s *SqlOnTopPostStore) Save(rctx request.CTX, onTop *model.OnTopPost, maxPerChannel int) (_ *model.OnTopPost, err error) {
	if onTop.CreateAt == 0 {
		onTop.CreateAt = model.GetMillis()
	}
	tx, err := s.GetMaster().Begin()
	if err != nil {
		return nil, errors.Wrap(err, "failed to start transaction")
	}
	defer finalizeTransactionX(tx, &err)

	if _, err = tx.Exec("SELECT pg_advisory_xact_lock(hashtext($1))", "ontopposts:"+onTop.ChannelId); err != nil {
		return nil, errors.Wrapf(err, "failed to lock the messages on top of channel=%s", onTop.ChannelId)
	}
	if _, err = tx.ExecBuilder(s.deleteStaleQuery(onTop.ChannelId, onTop.CreateAt)); err != nil {
		return nil, errors.Wrapf(err, "failed to forget the stale messages on top of channel=%s", onTop.ChannelId)
	}
	var others int
	countQuery := s.getQueryBuilder().
		Select("COUNT(*)").
		From("OnTopPosts").
		Where(sq.And{sq.Eq{"ChannelId": onTop.ChannelId}, sq.NotEq{"PostId": onTop.PostId}})
	if err = tx.GetBuilder(&others, countQuery); err != nil {
		return nil, errors.Wrapf(err, "failed to count the messages on top of channel=%s", onTop.ChannelId)
	}
	if others >= maxPerChannel {
		return nil, store.NewErrLimitExceeded("OnTopPosts", others, "channel_id="+onTop.ChannelId)
	}

	builder := s.getQueryBuilder().
		Insert("OnTopPosts").
		Columns("PostId", "ChannelId", "CreatedBy", "CreateAt", "ExpiresAt").
		Values(onTop.PostId, onTop.ChannelId, onTop.CreatedBy, onTop.CreateAt, onTop.ExpiresAt).
		SuffixExpr(sq.Expr("ON CONFLICT (PostId) DO UPDATE SET ChannelId = EXCLUDED.ChannelId, CreatedBy = EXCLUDED.CreatedBy, CreateAt = EXCLUDED.CreateAt, ExpiresAt = EXCLUDED.ExpiresAt"))
	if _, err = tx.ExecBuilder(builder); err != nil {
		return nil, errors.Wrapf(err, "failed to put post=%s on top", onTop.PostId)
	}
	if err = tx.Commit(); err != nil {
		return nil, errors.Wrap(err, "failed to commit transaction")
	}
	return onTop, nil
}

// deleteStaleQuery forgets a channel's messages on top whose time there ended, or whose message was deleted (a reply
// with its thread, by data retention...) or is gone (deleted for good).
func (s *SqlOnTopPostStore) deleteStaleQuery(channelID string, now int64) sq.DeleteBuilder {
	return s.getQueryBuilder().
		Delete("OnTopPosts").
		Where(sq.And{
			sq.Eq{"ChannelId": channelID},
			sq.Or{
				sq.And{sq.Gt{"ExpiresAt": 0}, sq.LtOrEq{"ExpiresAt": now}},
				sq.Expr("NOT EXISTS (SELECT 1 FROM Posts p WHERE p.Id = OnTopPosts.PostId AND p.DeleteAt = 0)"),
			},
		})
}

// Delete takes a message off the top of its channel; ErrNotFound when it wasn't there.
func (s *SqlOnTopPostStore) Delete(rctx request.CTX, postID string) error {
	result, err := s.GetMaster().ExecBuilder(s.getQueryBuilder().Delete("OnTopPosts").Where(sq.Eq{"PostId": postID}))
	if err != nil {
		return errors.Wrapf(err, "failed to remove post=%s from the top", postID)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return errors.Wrap(err, "failed to count the messages removed from the top")
	}
	if count == 0 {
		return store.NewErrNotFound("OnTopPost", postID)
	}
	return nil
}

// DeleteForChannel forgets all the messages on top of a channel, as it's deleted for good.
func (s *SqlOnTopPostStore) DeleteForChannel(rctx request.CTX, channelID string) error {
	if _, err := s.GetMaster().ExecBuilder(s.getQueryBuilder().Delete("OnTopPosts").Where(sq.Eq{"ChannelId": channelID})); err != nil {
		return errors.Wrapf(err, "failed to remove the messages on top of channel=%s", channelID)
	}
	return nil
}
