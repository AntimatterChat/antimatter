// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package sqlstore

import (
	"fmt"

	sq "github.com/mattermost/squirrel"
	"github.com/pkg/errors"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

// Antimatter's channel export reads one channel's posts with queries of its own, narrower than the compliance
// export's: only the columns the file shows (no Props but the few values it needs), by creation time on the
// (ChannelId, DeleteAt, CreateAt) index, and only the current version of each message.

// channelExportWhere selects the channel's posts created in the range, as the channel shows them now: no system
// messages, no deleted ones, no previous versions of edited ones (kept as deleted posts with an OriginalId) and no
// burn-on-read ones (whose content isn't the post's).
func channelExportWhere(filter model.ChannelExportFilter) sq.And {
	where := sq.And{
		sq.Eq{"Posts.ChannelId": filter.ChannelId},
		sq.Eq{"Posts.DeleteAt": 0},
		sq.Eq{"Posts.OriginalId": ""},
		sq.NotLike{"Posts.Type": "system_%"},
		sq.NotEq{"Posts.Type": model.PostTypeBurnOnRead},
	}
	if filter.SinceCreateAt > 0 {
		where = append(where, sq.GtOrEq{"Posts.CreateAt": filter.SinceCreateAt})
	}
	if filter.UntilCreateAt > 0 {
		where = append(where, sq.LtOrEq{"Posts.CreateAt": filter.UntilCreateAt})
	}
	return where
}

func (s SqlComplianceStore) ChannelExportCount(rctx request.CTX, filter model.ChannelExportFilter, limit int) (int, error) {
	posts := s.getQueryBuilder().Select("1").From("Posts").Where(channelExportWhere(filter))
	if limit > 0 {
		posts = posts.Limit(uint64(limit))
	}
	builder := s.getQueryBuilder().Select("COUNT(*)").FromSelect(posts, "ExportedPosts")

	var counts []int
	if err := s.GetReplica().SelectBuilderCtx(rctx.Context(), &counts, builder); err != nil {
		return 0, errors.Wrapf(err, "failed to count the posts to export of channel %s", filter.ChannelId)
	}
	if len(counts) == 0 {
		return 0, nil
	}
	return counts[0], nil
}

func (s SqlComplianceStore) ChannelExportPosts(rctx request.CTX, filter model.ChannelExportFilter, cursor model.ChannelExportCursor, limit int) ([]*model.ChannelExportPost, error) {
	builder := s.getQueryBuilder().
		Select(
			"Posts.Id AS PostId",
			"Posts.CreateAt AS PostCreateAt",
			"COALESCE(Posts.EditAt, 0) AS PostEditAt",
			"COALESCE(Posts.RootId, '') AS PostRootId",
			"COALESCE(Posts.Type, '') AS PostType",
			"COALESCE(Posts.Message, '') AS PostMessage",
			"Posts.FileIds AS PostFileIds",
			"Posts.UserId AS UserId",
			"COALESCE(Users.Username, '') AS Username",
			"COALESCE(Users.Email, '') AS UserEmail",
			"Bots.UserId IS NOT NULL AS IsBot",
			"COALESCE(Posts.Props->>'from_webhook', '') AS FromWebhook",
			"COALESCE(Posts.Props->>'override_username', '') AS OverrideUsername",
			// The attachments only, and not too large ones: Props can hold far more.
			fmt.Sprintf(`CASE WHEN jsonb_typeof(Posts.Props->'attachments') = 'array'
				AND octet_length((Posts.Props->'attachments')::text) <= %d
				THEN (Posts.Props->'attachments')::text ELSE '' END AS Attachments`, model.ChannelExportMaxAttachmentsSize),
		).
		From("Posts").
		LeftJoin("Users ON Users.Id = Posts.UserId").
		LeftJoin("Bots ON Bots.UserId = Posts.UserId").
		Where(channelExportWhere(filter)).
		OrderBy("Posts.CreateAt", "Posts.Id").
		Limit(uint64(limit))
	if cursor.AfterId != "" {
		builder = builder.Where(sq.Expr("(Posts.CreateAt, Posts.Id) > (?, ?)", cursor.AfterCreateAt, cursor.AfterId))
	}

	posts := []*model.ChannelExportPost{}
	if err := s.GetReplica().SelectBuilderCtx(rctx.Context(), &posts, builder); err != nil {
		return nil, errors.Wrapf(err, "failed to get the posts to export of channel %s", filter.ChannelId)
	}
	return posts, nil
}
