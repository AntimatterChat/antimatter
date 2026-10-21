// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package sqlstore

import (
	"database/sql"
	"fmt"

	sq "github.com/mattermost/squirrel"
	"github.com/pkg/errors"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

// SqlFocusStore keeps everyone's focus mode (Antimatter), and finds what they missed while focusing.
type SqlFocusStore struct {
	*SqlStore
}

func newSqlFocusStore(sqlStore *SqlStore) store.FocusStore {
	return &SqlFocusStore{SqlStore: sqlStore}
}

var focusColumns = []string{"UserId", "StartAt", "EndAt", "Reason", "EndedAt", "PrevStatus", "PrevManual", "PrevDNDEndTime", "PrevCustomStatus", "FocusCustomStatus"}

func (s *SqlFocusStore) Save(rctx request.CTX, focus *model.UserFocus) error {
	builder := s.getQueryBuilder().
		Insert("UserFocus").
		Columns(focusColumns...).
		Values(focus.UserId, focus.StartAt, focus.EndAt, focus.Reason, focus.EndedAt, focus.PrevStatus, focus.PrevManual, focus.PrevDNDEndTime, focus.PrevCustomStatus, focus.FocusCustomStatus).
		Suffix("ON CONFLICT (UserId) DO UPDATE SET StartAt = EXCLUDED.StartAt, EndAt = EXCLUDED.EndAt, Reason = EXCLUDED.Reason, EndedAt = EXCLUDED.EndedAt, PrevStatus = EXCLUDED.PrevStatus, PrevManual = EXCLUDED.PrevManual, PrevDNDEndTime = EXCLUDED.PrevDNDEndTime, PrevCustomStatus = EXCLUDED.PrevCustomStatus, FocusCustomStatus = EXCLUDED.FocusCustomStatus")
	if _, err := s.GetMaster().ExecBuilder(builder); err != nil {
		return errors.Wrapf(err, "failed to save the focus of user=%s", focus.UserId)
	}
	return nil
}

func (s *SqlFocusStore) Get(rctx request.CTX, userID string) (*model.UserFocus, error) {
	query := s.getQueryBuilder().
		Select(focusColumns...).
		From("UserFocus").
		Where(sq.Eq{"UserId": userID})
	var focus model.UserFocus
	if err := s.DBXFromContext(rctx.Context()).GetBuilder(&focus, query); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, store.NewErrNotFound("UserFocus", userID)
		}
		return nil, errors.Wrapf(err, "failed to get the focus of user=%s", userID)
	}
	return &focus, nil
}

func (s *SqlFocusStore) MarkEnded(rctx request.CTX, userID string, startAt, endedAt int64) (bool, error) {
	builder := s.getQueryBuilder().
		Update("UserFocus").
		Set("EndedAt", endedAt).
		Where(sq.Eq{"UserId": userID, "StartAt": startAt, "EndedAt": 0})
	result, err := s.GetMaster().ExecBuilder(builder)
	if err != nil {
		return false, errors.Wrapf(err, "failed to end the focus of user=%s", userID)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, errors.Wrapf(err, "failed to end the focus of user=%s", userID)
	}
	return rows == 1, nil
}

func (s *SqlFocusStore) GetExpired(rctx request.CTX, before int64, limit int) ([]*model.UserFocus, error) {
	query := s.getQueryBuilder().
		Select(focusColumns...).
		From("UserFocus").
		Where(sq.And{sq.Eq{"EndedAt": 0}, sq.LtOrEq{"EndAt": before}}).
		OrderBy("EndAt").
		Limit(uint64(limit))
	focuses := []*model.UserFocus{}
	if err := s.GetMaster().SelectBuilder(&focuses, query); err != nil {
		return nil, errors.Wrap(err, "failed to find the focuses that ended")
	}
	return focuses, nil
}

// The messages counted: from others, not deleted, no system messages; at most FocusSummaryMaxCount per channel or
// thread, so a busy channel costs no more than that.
const focusMissedPosts = `
	FROM Posts p
	WHERE p.DeleteAt = 0 AND p.CreateAt > GREATEST(?, %[1]s) AND p.CreateAt <= ?
		AND p.UserId <> ? AND p.Type NOT LIKE 'system_%%'`

func (s *SqlFocusStore) GetMissedChannels(rctx request.CTX, userID string, since, until int64, rootOnly bool, limit int) ([]*model.FocusSummaryChannel, error) {
	mentions := "cm.MentionCount"
	root := ""
	if rootOnly {
		mentions = "cm.MentionCountRoot"
		root = " AND p.RootId = ''"
	}
	posts := fmt.Sprintf(focusMissedPosts, "m.LastViewedAt") + " AND p.ChannelId = m.ChannelId" + root

	// The member's channels with something newer than both the start and their last read, the most recent first;
	// then, for each, the messages of the window after that last read (an index range per channel, bounded).
	query := `
	WITH m AS (
		SELECT c.Id AS ChannelId, c.TeamId, c.Type, c.Name, c.DisplayName, c.LastPostAt AS ChannelLastPostAt,
			cm.LastViewedAt, ` + mentions + ` AS MentionCount,
			COALESCE(cm.NotifyProps->>'mark_unread', 'all') AS MarkUnread
		FROM ChannelMembers cm
		JOIN Channels c ON c.Id = cm.ChannelId
		WHERE cm.UserId = ? AND c.DeleteAt = 0 AND c.LastPostAt > ? AND c.LastPostAt > cm.LastViewedAt
		ORDER BY c.LastPostAt DESC
		LIMIT ?
	)
	SELECT m.ChannelId, m.TeamId, m.Type, m.Name, m.DisplayName, m.ChannelLastPostAt, m.LastViewedAt, m.MentionCount,
		m.MarkUnread, n.Count, l.LastPostAt, l.LastSenderId
	FROM m
	CROSS JOIN LATERAL (SELECT COUNT(*) AS Count FROM (SELECT 1 ` + posts + ` LIMIT ?) x) n
	CROSS JOIN LATERAL (SELECT p.CreateAt AS LastPostAt, p.UserId AS LastSenderId ` + posts + ` ORDER BY p.CreateAt DESC LIMIT 1) l
	WHERE n.Count > 0
	ORDER BY l.LastPostAt DESC`

	args := []any{userID, since, limit, since, until, userID, model.FocusSummaryMaxCount, since, until, userID}
	channels := []*model.FocusSummaryChannel{}
	if err := s.DBXFromContext(rctx.Context()).Select(&channels, query, args...); err != nil {
		return nil, errors.Wrapf(err, "failed to find the channels user=%s missed", userID)
	}
	return channels, nil
}

func (s *SqlFocusStore) GetMissedThreads(rctx request.CTX, userID string, since, until int64, limit int) ([]*model.FocusSummaryThread, error) {
	posts := fmt.Sprintf(focusMissedPosts, "t.LastViewedAt") + " AND p.RootId = t.PostId"

	query := `
	WITH t AS (
		SELECT th.PostId, th.ChannelId, c.TeamId, tm.UnreadMentions AS MentionCount, tm.LastViewed AS LastViewedAt
		FROM ThreadMemberships tm
		JOIN Threads th ON th.PostId = tm.PostId
		JOIN Channels c ON c.Id = th.ChannelId
		JOIN ChannelMembers cm ON cm.ChannelId = th.ChannelId AND cm.UserId = tm.UserId
		WHERE tm.UserId = ? AND tm.Following = true AND COALESCE(th.ThreadDeleteAt, 0) = 0 AND c.DeleteAt = 0
			AND th.LastReplyAt > ? AND th.LastReplyAt > tm.LastViewed
		ORDER BY th.LastReplyAt DESC
		LIMIT ?
	)
	SELECT t.PostId, t.ChannelId, t.TeamId, t.MentionCount, t.LastViewedAt, n.Count, l.LastReplyAt, l.LastSenderId
	FROM t
	CROSS JOIN LATERAL (SELECT COUNT(*) AS Count FROM (SELECT 1 ` + posts + ` LIMIT ?) x) n
	CROSS JOIN LATERAL (SELECT p.CreateAt AS LastReplyAt, p.UserId AS LastSenderId ` + posts + ` ORDER BY p.CreateAt DESC LIMIT 1) l
	WHERE n.Count > 0
	ORDER BY l.LastReplyAt DESC`

	args := []any{userID, since, limit, since, until, userID, model.FocusSummaryMaxCount, since, until, userID}
	threads := []*model.FocusSummaryThread{}
	if err := s.DBXFromContext(rctx.Context()).Select(&threads, query, args...); err != nil {
		return nil, errors.Wrapf(err, "failed to find the threads user=%s missed", userID)
	}
	return threads, nil
}

func (s *SqlFocusStore) GetFirstPostsAfter(rctx request.CTX, channelIDs []string, after int64, rootOnly bool) (map[string]string, error) {
	firsts := map[string]string{}
	if len(channelIDs) == 0 {
		return firsts, nil
	}
	query := s.getQueryBuilder().
		Select("DISTINCT ON (ChannelId) ChannelId", "Id").
		From("Posts").
		Where(sq.Eq{"ChannelId": channelIDs, "DeleteAt": 0}).
		Where(sq.Gt{"CreateAt": after}).
		OrderBy("ChannelId", "CreateAt", "Id")
	if rootOnly {
		query = query.Where(sq.Eq{"RootId": ""})
	}
	rows := []struct {
		ChannelId string
		Id        string
	}{}
	if err := s.DBXFromContext(rctx.Context()).SelectBuilder(&rows, query); err != nil {
		return nil, errors.Wrap(err, "failed to find the first messages after a time")
	}
	for _, row := range rows {
		firsts[row.ChannelId] = row.Id
	}
	return firsts, nil
}

func (s *SqlFocusStore) PermanentDeleteByUser(rctx request.CTX, userID string) error {
	builder := s.getQueryBuilder().Delete("UserFocus").Where(sq.Eq{"UserId": userID})
	if _, err := s.GetMaster().ExecBuilder(builder); err != nil {
		return errors.Wrapf(err, "failed to delete the focus of user=%s", userID)
	}
	return nil
}
