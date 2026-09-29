// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package engine

import (
	"context"
	"net/http"
	"net/url"
	"slices"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

// Bulk indexing API, used by the indexer job.

// PostAction returns the bulk write for a post read from the database. ok is
// false when the post is never indexed.
func (e *Engine) PostAction(post *model.PostForIndexing) (BulkAction, bool) {
	return e.postAction(&post.Post, post.TeamId, post.ChannelType)
}

// ChannelAction returns the bulk write for a channel. userIDs are the members
// of private channels. ok is false for channels which are never indexed
// (direct and group messages).
func (e *Engine) ChannelAction(channel *model.Channel, userIDs []string) (BulkAction, bool) {
	if !shouldIndexChannel(channel) {
		return BulkAction{}, false
	}
	return BulkAction{Op: opIndex, Index: namesFor(e.config()).channels(), ID: channel.Id, Doc: newChannelDocument(channel, userIDs)}, true
}

// UserAction returns the bulk write for a user.
func (e *Engine) UserAction(user *model.User, teamIDs, channelIDs []string) BulkAction {
	return BulkAction{Op: opIndex, Index: namesFor(e.config()).users(), ID: user.Id, Doc: newUserDocument(user, teamIDs, channelIDs)}
}

// FileAction returns the bulk write for a file read from the database.
func (e *Engine) FileAction(file *model.FileForIndexing) BulkAction {
	index := namesFor(e.config()).files()
	if !file.ShouldIndex() {
		return BulkAction{Op: opDelete, Index: index, ID: file.Id}
	}
	info := file.FileInfo
	info.Content = file.Content
	channelID := file.ChannelId
	if channelID == "" {
		channelID = info.ChannelId
	}
	return BulkAction{Op: opIndex, Index: index, ID: file.Id, Doc: newFileDocument(&info, channelID)}
}

// BulkIndex synchronously sends the given writes. Documents rejected by the
// backend are returned as failures.
func (e *Engine) BulkIndex(ctx context.Context, actions []BulkAction) ([]BulkFailure, *model.AppError) {
	c, _, appErr := e.activeClient("Engine.BulkIndex")
	if appErr != nil {
		return nil, appErr
	}
	failures, err := e.sendBulk(ctx, c, actions)
	if err != nil {
		return nil, model.NewAppError("Engine.BulkIndex", "ent.elasticsearch.index_post.error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	if len(failures) > 0 {
		logBulkFailures(e.logger(), failures)
	}
	return failures, nil
}

// Post index maintenance.

// AggregatePostIndexes merges the daily post indexes of the days before
// cutoff into monthly indexes. Each merged daily index is replaced by an
// alias to its monthly index so that posts keep being addressable by their
// creation day. It returns the number of daily indexes merged.
func (e *Engine) AggregatePostIndexes(ctx context.Context, cutoff time.Time) (int, *model.AppError) {
	c, cfg, appErr := e.activeClient("Engine.AggregatePostIndexes")
	if appErr != nil {
		return 0, appErr
	}
	names := namesFor(cfg)
	params := e.backendParams()
	e.flushPending()

	indexes, err := c.listIndexes(ctx, names.postsPattern())
	if err != nil {
		return 0, model.NewAppError("Engine.AggregatePostIndexes", "ent.elasticsearch.aggregator_worker.get_indexes.error", params, "", http.StatusInternalServerError).Wrap(err)
	}

	cutoffDay := time.Date(cutoff.UTC().Year(), cutoff.UTC().Month(), cutoff.UTC().Day(), 0, 0, 0, 0, time.UTC)
	groups := map[string][]string{}
	for _, name := range indexes {
		start, end, aggregated, ok := names.parsePostIndex(name)
		if !ok || aggregated || end.After(cutoffDay) {
			continue
		}
		target := names.monthlyPostIndex(start)
		groups[target] = append(groups[target], name)
	}

	targets := make([]string, 0, len(groups))
	for target := range groups {
		targets = append(targets, target)
	}
	slices.Sort(targets)

	merged := 0
	for _, target := range targets {
		dailies := groups[target]
		if appErr := e.mergePostIndexes(ctx, c, target, dailies); appErr != nil {
			return merged, appErr
		}
		merged += len(dailies)
		e.logger().Info("Aggregated daily post indexes", mlog.String("target", target), mlog.Int("count", len(dailies)))
	}
	return merged, nil
}

func (e *Engine) mergePostIndexes(ctx context.Context, c *client, target string, dailies []string) *model.AppError {
	params := e.backendParams()
	if err := e.ensureIndex(ctx, c, target); err != nil {
		return model.NewAppError("Engine.mergePostIndexes", "ent.elasticsearch.aggregator_worker.create_index_job.error", params, "failed to create "+target, http.StatusInternalServerError).Wrap(err)
	}

	setWriteBlock := func(blocked bool) error {
		body := map[string]any{"index": map[string]any{"blocks": map[string]any{"write": blocked}}}
		for _, daily := range dailies {
			if _, err := c.do(ctx, http.MethodPut, "/"+daily+"/_settings", nil, body); err != nil {
				return err
			}
		}
		return nil
	}

	// Stop live writes to the daily indexes while they are copied. A write
	// hitting a blocked index fails and is logged by the search layer.
	if err := setWriteBlock(true); err != nil {
		return model.NewAppError("Engine.mergePostIndexes", "ent.elasticsearch.aggregator_worker.create_index_job.error", params, "", http.StatusInternalServerError).Wrap(err)
	}

	reindex := map[string]any{
		"conflicts": "proceed",
		"source":    map[string]any{"index": dailies},
		"dest":      map[string]any{"index": target, "op_type": "index"},
	}
	if _, err := c.runTask(ctx, http.MethodPost, "/_reindex", url.Values{"refresh": {"true"}}, reindex); err != nil {
		if unblockErr := setWriteBlock(false); unblockErr != nil {
			e.logger().Warn("Failed to unblock writes on daily post indexes", mlog.Err(unblockErr))
		}
		return model.NewAppError("Engine.mergePostIndexes", "ent.elasticsearch.aggregator_worker.index_job_failed.error", params, "", http.StatusInternalServerError).Wrap(err)
	}

	// Atomically delete the daily indexes and replace them by aliases.
	actions := make([]any, 0, 2*len(dailies))
	for _, daily := range dailies {
		actions = append(actions, map[string]any{"remove_index": map[string]any{"index": daily}})
	}
	for _, daily := range dailies {
		actions = append(actions, map[string]any{"add": map[string]any{"index": target, "alias": daily}})
	}
	if _, err := c.do(ctx, http.MethodPost, "/_aliases", nil, map[string]any{"actions": actions}); err != nil {
		if unblockErr := setWriteBlock(false); unblockErr != nil {
			e.logger().Warn("Failed to unblock writes on daily post indexes", mlog.Err(unblockErr))
		}
		return model.NewAppError("Engine.mergePostIndexes", "ent.elasticsearch.aggregator_worker.delete_indexes.error", params, "", http.StatusInternalServerError).Wrap(err)
	}
	return nil
}

// DataRetentionDeleteIndexes deletes the post indexes only holding posts
// created before cutoff.
func (e *Engine) DataRetentionDeleteIndexes(rctx request.CTX, cutoff time.Time) *model.AppError {
	c, cfg, appErr := e.activeClient("Engine.DataRetentionDeleteIndexes")
	if appErr != nil {
		return appErr
	}
	names := namesFor(cfg)
	params := e.backendParams()
	e.flushPending()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	indexes, err := c.listIndexes(ctx, names.postsPattern())
	if err != nil {
		return model.NewAppError("Engine.DataRetentionDeleteIndexes", "ent.elasticsearch.data_retention_delete_indexes.get_indexes.error", params, "", http.StatusInternalServerError).Wrap(err)
	}
	var toDelete []string
	for _, name := range indexes {
		if _, end, _, ok := names.parsePostIndex(name); ok && !end.After(cutoff) {
			toDelete = append(toDelete, name)
		}
	}
	if len(toDelete) == 0 {
		return nil
	}
	if err := c.deleteIndexes(ctx, toDelete); err != nil {
		return model.NewAppError("Engine.DataRetentionDeleteIndexes", "ent.elasticsearch.data_retention_delete_indexes.delete_index.error", params, "", http.StatusInternalServerError).Wrap(err)
	}
	e.forgetIndexes()
	e.logger().Info("Deleted post indexes older than the retention period", mlog.Int("count", len(toDelete)))
	return nil
}
