// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package sqlstore

import (
	"github.com/pkg/errors"

	sq "github.com/mattermost/squirrel"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

// Antimatter forums: the topic list. A topic is a live, non-system root post of the channel; its thread row (when it
// has replies) gives the reply count, the last reply's time and the participants. Votes are the 👍 reactions of
// everyone but the topic's author, one per person whatever the emoji's name. Topics on top of the channel come first,
// then the sort's keys, all descending, with the id last, so a cursor of (on top, key 1, key 2, id) pages through
// them (keyset pagination). No index of its own: the channel's root posts are read through the existing ones.

// forumTopicRow is one topic as the list's query gives it, before its post is read.
type forumTopicRow struct {
	Id           string
	ReplyCount   int64
	LastReplyAt  int64
	Participants model.StringArray
	Votes        int64
	OnTop        int
	Key1         int64
	Key2         int64
}

// forumTopicSortKeys gives the columns of the inner query (alias x) that sort a list, by its sort: the first and
// second keys (columns, never constants: ORDER BY 0 would be a position).
func forumTopicSortKeys(sort string) (string, string) {
	switch sort {
	case model.ForumTopicSortNewest:
		return "x.CreateAt", "x.Activity"
	case model.ForumTopicSortVotes:
		return "x.Votes", "x.Activity"
	case model.ForumTopicSortUnanswered:
		return "x.Unanswered", "x.Activity"
	case model.ForumTopicSortStatus:
		return "x.StatusRank", "x.Activity"
	default:
		return "x.Activity", "x.CreateAt"
	}
}

func (s *SqlPostStore) GetForumTopics(rctx request.CTX, opts model.ForumTopicsOptions) (*model.ForumTopicList, error) {
	perPage := opts.PerPage
	if perPage <= 0 {
		perPage = model.ForumTopicsPerPageDefault
	}
	if perPage > model.ForumTopicsPerPageMax {
		perPage = model.ForumTopicsPerPageMax
	}

	voteArgs := make([]any, len(model.VoteEmojiNames))
	for i, name := range model.VoteEmojiNames {
		voteArgs[i] = name
	}
	votes := sq.Expr("(SELECT COUNT(DISTINCT r.UserId) FROM Reactions r WHERE r.PostId = p.Id AND r.UserId <> p.UserId AND COALESCE(r.DeleteAt, 0) = 0 AND r.EmojiName IN ("+sq.Placeholders(len(voteArgs))+"))", voteArgs...)
	onTop := sq.Expr("(CASE WHEN EXISTS (SELECT 1 FROM OnTopPosts o WHERE o.PostId = p.Id AND o.ChannelId = p.ChannelId AND (o.ExpiresAt = 0 OR o.ExpiresAt > ?)) THEN 1 ELSE 0 END)", model.GetMillis())

	inner := sq.Select(
		"p.Id",
		"p.CreateAt",
		"COALESCE(t.ReplyCount, 0) AS ReplyCount",
		"COALESCE(t.LastReplyAt, 0) AS LastReplyAt",
		"COALESCE(t.Participants, '[]') AS Participants",
		"GREATEST(p.CreateAt, COALESCE(t.LastReplyAt, 0)) AS Activity",
		"(CASE WHEN COALESCE(p.Props->>'"+model.PostPropsAcceptedAnswer+"', '') = '' THEN 1 ELSE 0 END) AS Unanswered",
		"(CASE COALESCE(NULLIF(p.Props->>'"+model.PostPropsTopicStatus+"', ''), 'open')"+
			" WHEN 'in_progress' THEN 5 WHEN 'planned' THEN 4 WHEN 'open' THEN 3 WHEN 'done' THEN 2 WHEN 'declined' THEN 1"+
			" ELSE 3 END) AS StatusRank",
	).
		Column(sq.Alias(votes, "Votes")).
		Column(sq.Alias(onTop, "OnTop")).
		From("Posts p").
		LeftJoin("Threads t ON t.PostId = p.Id").
		Where(sq.Eq{"p.ChannelId": opts.ChannelId, "p.RootId": "", "p.DeleteAt": 0}).
		Where("p.Type NOT LIKE 'system_%'")

	if opts.Answered != nil {
		if *opts.Answered {
			inner = inner.Where("COALESCE(p.Props->>'" + model.PostPropsAcceptedAnswer + "', '') <> ''")
		} else {
			inner = inner.Where("COALESCE(p.Props->>'" + model.PostPropsAcceptedAnswer + "', '') = ''")
		}
	}
	if opts.Status != "" {
		inner = inner.Where(sq.Expr("COALESCE(NULLIF(p.Props->>'"+model.PostPropsTopicStatus+"', ''), 'open') = ?", opts.Status))
	}
	if opts.Mine {
		inner = inner.Where(sq.Eq{"p.UserId": opts.UserId})
	}
	if opts.Following {
		inner = inner.Where(sq.Expr("EXISTS (SELECT 1 FROM ThreadMemberships tm WHERE tm.PostId = p.Id AND tm.UserId = ? AND tm.Following = true)", opts.UserId))
	}
	if opts.Query != "" {
		like := "%" + sanitizeSearchTerm(opts.Query, "\\") + "%"
		inner = inner.Where(sq.Expr("(COALESCE(p.Props->>'"+model.PostPropsThreadTitle+"', '') ILIKE ? OR p.Message ILIKE ?)", like, like))
	}

	key1, key2 := forumTopicSortKeys(opts.Sort)
	query := s.getQueryBuilder().
		Select("x.Id", "x.ReplyCount", "x.LastReplyAt", "x.Participants", "x.Votes", "x.OnTop", key1+" AS Key1", key2+" AS Key2").
		FromSelect(inner, "x").
		OrderBy("x.OnTop DESC", key1+" DESC", key2+" DESC", "x.Id DESC").
		Limit(uint64(perPage + 1))
	if c := opts.Before; c != nil {
		query = query.Where(sq.Expr("(x.OnTop, "+key1+", "+key2+", x.Id) < (?, ?, ?, ?)", c.OnTop, c.Key1, c.Key2, c.Id))
	}

	rows := []forumTopicRow{}
	if err := s.DBXFromContext(rctx.Context()).SelectBuilder(&rows, query); err != nil {
		return nil, errors.Wrapf(err, "failed to get the topics of channel %s", opts.ChannelId)
	}

	list := &model.ForumTopicList{Topics: []*model.ForumTopic{}}
	if len(rows) > perPage {
		rows = rows[:perPage]
		last := rows[perPage-1]
		list.NextCursor = (&model.ForumTopicCursor{OnTop: last.OnTop, Key1: last.Key1, Key2: last.Key2, Id: last.Id}).String()
	}
	if len(rows) == 0 {
		return list, nil
	}

	ids := make([]string, len(rows))
	for i, row := range rows {
		ids[i] = row.Id
	}
	posts := []*model.Post{}
	if err := s.DBXFromContext(rctx.Context()).SelectBuilder(&posts, s.postsQuery.Where(sq.Eq{"Posts.Id": ids})); err != nil {
		return nil, errors.Wrapf(err, "failed to get the first posts of the topics of channel %s", opts.ChannelId)
	}
	byID := make(map[string]*model.Post, len(posts))
	for _, post := range posts {
		byID[post.Id] = post
	}

	var replies map[string][]*model.Post
	if opts.Replies > 0 {
		var err error
		if replies, err = s.latestForumReplies(rctx, ids, min(opts.Replies, model.ForumTopicRepliesMax)); err != nil {
			return nil, err
		}
	}

	for _, row := range rows {
		post, ok := byID[row.Id]
		if !ok {
			continue // deleted between the two reads
		}
		participants := []string(row.Participants)
		if len(participants) > model.ForumTopicParticipantsMax {
			participants = participants[:model.ForumTopicParticipantsMax]
		}
		if participants == nil {
			participants = []string{}
		}
		post.ReplyCount = row.ReplyCount
		accepted, _ := post.GetProp(model.PostPropsAcceptedAnswer).(string)
		status, _ := post.GetProp(model.PostPropsTopicStatus).(string)
		if status == "" {
			status = model.TopicStatusOpen
		}
		list.Topics = append(list.Topics, &model.ForumTopic{
			Post:             post,
			ReplyCount:       row.ReplyCount,
			LastReplyAt:      row.LastReplyAt,
			ParticipantIds:   participants,
			Votes:            row.Votes,
			AcceptedAnswerId: accepted,
			Status:           status,
			OnTop:            row.OnTop == 1,
			Replies:          replies[row.Id],
		})
	}
	return list, nil
}

// latestForumReplies gives the latest n live, non-system replies of each root, oldest first.
func (s *SqlPostStore) latestForumReplies(rctx request.CTX, rootIDs []string, n int) (map[string][]*model.Post, error) {
	ranked := sq.Select("Id", "ROW_NUMBER() OVER (PARTITION BY RootId ORDER BY CreateAt DESC, Id DESC) AS rn").
		From("Posts").
		Where(sq.Eq{"RootId": rootIDs, "DeleteAt": 0}).
		Where("Type NOT LIKE 'system_%'")
	latest := sq.Select("r.Id").FromSelect(ranked, "r").Where(sq.LtOrEq{"r.rn": n})

	posts := []*model.Post{}
	query := s.postsQuery.Where(sq.Expr("Posts.Id IN (?)", latest)).OrderBy("Posts.CreateAt ASC", "Posts.Id ASC")
	if err := s.DBXFromContext(rctx.Context()).SelectBuilder(&posts, query); err != nil {
		return nil, errors.Wrap(err, "failed to get the latest replies of topics")
	}
	replies := make(map[string][]*model.Post, len(rootIDs))
	for _, post := range posts {
		replies[post.RootId] = append(replies[post.RootId], post)
	}
	return replies, nil
}
