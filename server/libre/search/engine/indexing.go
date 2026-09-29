// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

const (
	byQueryTimeout       = 2 * time.Hour
	taskPollInitial      = 100 * time.Millisecond
	taskPollMax          = 2 * time.Second
	backfillRequestsRate = "2000"
)

func ignoreUnavailable() url.Values {
	return url.Values{"ignore_unavailable": {"true"}, "allow_no_indices": {"true"}}
}

// write sends a single live write, either right away or through the batching
// processor, depending on LiveIndexingBatchSize.
func (e *Engine) write(where, errorID string, action BulkAction) *model.AppError {
	st := e.state.Load()
	if st == nil {
		return model.NewAppError(where, "ent.elasticsearch.not_started.error", e.backendParams(), "", http.StatusInternalServerError)
	}
	if model.SafeDereference(e.settings().LiveIndexingBatchSize) > 1 {
		st.processor.add(action)
		return nil
	}

	// Keep the ordering with writes buffered before a batch size change.
	st.processor.flush()

	ctx, cancel := e.requestContext()
	defer cancel()
	failures, err := e.sendBulk(ctx, st.client, []BulkAction{action})
	if err != nil {
		return model.NewAppError(where, errorID, nil, "", http.StatusInternalServerError).Wrap(err)
	}
	if len(failures) > 0 {
		return model.NewAppError(where, errorID, nil, failures[0].Reason, http.StatusInternalServerError)
	}
	return nil
}

// flushPending makes sure buffered writes are sent before a by-query
// operation that could otherwise be overtaken by them.
func (e *Engine) flushPending() {
	if p := e.processor(); p != nil {
		p.flush()
	}
}

type taskResult struct {
	Total    int64             `json:"total"`
	Deleted  int64             `json:"deleted"`
	Updated  int64             `json:"updated"`
	Created  int64             `json:"created"`
	Failures []json.RawMessage `json:"failures"`
}

// runTask starts a long running operation (_delete_by_query,
// _update_by_query, _reindex) as a task and waits for its completion.
func (c *client) runTask(ctx context.Context, method, path string, query url.Values, body any) (*taskResult, error) {
	if query == nil {
		query = url.Values{}
	}
	query.Set("wait_for_completion", "false")

	var started struct {
		Task string `json:"task"`
	}
	if err := c.doJSON(ctx, method, path, query, body, &started); err != nil {
		return nil, err
	}
	if started.Task == "" {
		return nil, errors.New("the search backend did not return a task id")
	}

	wait := taskPollInitial
	for {
		var status struct {
			Completed bool            `json:"completed"`
			Response  *taskResult     `json:"response"`
			Error     json.RawMessage `json:"error"`
		}
		if err := c.doJSON(ctx, http.MethodGet, "/_tasks/"+started.Task, nil, nil, &status); err != nil {
			return nil, err
		}
		if status.Completed {
			if len(status.Error) > 0 && string(status.Error) != "null" {
				return nil, fmt.Errorf("task %s failed: %s", started.Task, string(status.Error))
			}
			if status.Response != nil && len(status.Response.Failures) > 0 {
				return status.Response, fmt.Errorf("task %s completed with %d failures: %s", started.Task, len(status.Response.Failures), string(status.Response.Failures[0]))
			}
			if status.Response == nil {
				status.Response = &taskResult{}
			}
			return status.Response, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
		}
		wait = min(wait*2, taskPollMax)
	}
}

func byQueryParams() url.Values {
	q := ignoreUnavailable()
	q.Set("conflicts", "proceed")
	q.Set("refresh", "true")
	return q
}

// deleteByQuery deletes the documents matching query from the index pattern.
func (e *Engine) deleteByQuery(where, errorID, target string, query map[string]any, params url.Values) *model.AppError {
	c, _, appErr := e.activeClient(where)
	if appErr != nil {
		return appErr
	}
	e.flushPending()
	ctx, cancel := context.WithTimeout(context.Background(), byQueryTimeout)
	defer cancel()
	q := byQueryParams()
	for k, v := range params {
		q[k] = v
	}
	if _, err := c.runTask(ctx, http.MethodPost, "/"+target+"/_delete_by_query", q, map[string]any{"query": query}); err != nil {
		return model.NewAppError(where, errorID, e.backendParams(), "", http.StatusInternalServerError).Wrap(err)
	}
	return nil
}

func termQuery(field string, value any) map[string]any {
	return map[string]any{"term": map[string]any{field: value}}
}

func termsQuery(field string, values []string) map[string]any {
	return map[string]any{"terms": map[string]any{field: values}}
}

// IndexPost adds or replaces a post in the index of its creation day.
func (e *Engine) IndexPost(post *model.Post, teamId string, channelType string) *model.AppError {
	if post == nil {
		return nil
	}
	action, ok := e.postAction(post, teamId, channelType)
	if !ok {
		return nil
	}
	return e.write("Engine.IndexPost", "ent.elasticsearch.index_post.error", action)
}

// postAction returns the write keeping the index in sync with the post. ok is
// false when the post is never indexed and nothing has to be done.
func (e *Engine) postAction(post *model.Post, teamID, channelType string) (BulkAction, bool) {
	if !isSearchablePostType(post.Type) {
		return BulkAction{}, false
	}
	index := namesFor(e.config()).postIndexForTime(post.CreateAt)
	if post.DeleteAt != 0 {
		return BulkAction{Op: opDelete, Index: index, ID: post.Id}, true
	}
	return BulkAction{Op: opIndex, Index: index, ID: post.Id, Doc: newPostDocument(post, teamID, channelType)}, true
}

func (e *Engine) DeletePost(post *model.Post) *model.AppError {
	if post == nil {
		return nil
	}
	if post.CreateAt == 0 {
		return e.deleteByQuery("Engine.DeletePost", "ent.elasticsearch.delete_post.error", namesFor(e.config()).postsPattern(),
			map[string]any{"ids": map[string]any{"values": []string{post.Id}}}, nil)
	}
	index := namesFor(e.config()).postIndexForTime(post.CreateAt)
	return e.write("Engine.DeletePost", "ent.elasticsearch.delete_post.error", BulkAction{Op: opDelete, Index: index, ID: post.Id})
}

func (e *Engine) DeleteChannelPosts(rctx request.CTX, channelID string) *model.AppError {
	return e.deleteByQuery("Engine.DeleteChannelPosts", "ent.elasticsearch.delete_channel_posts.error",
		namesFor(e.config()).postsPattern(), termQuery("channel_id", channelID), nil)
}

func (e *Engine) DeleteUserPosts(rctx request.CTX, userID string) *model.AppError {
	return e.deleteByQuery("Engine.DeleteUserPosts", "ent.elasticsearch.delete_user_posts.error",
		namesFor(e.config()).postsPattern(), termQuery("user_id", userID), nil)
}

// updatePostsChannelType sets the channel type of the posts of the given
// channels that don't have it yet.
func (e *Engine) updatePostsChannelType(where, errorID string, channelIDs []string, channelType string, throttle bool) *model.AppError {
	if len(channelIDs) == 0 {
		return nil
	}
	c, cfg, appErr := e.activeClient(where)
	if appErr != nil {
		return appErr
	}
	e.flushPending()

	body := map[string]any{
		"query": map[string]any{"bool": map[string]any{
			"filter":   []any{termsQuery("channel_id", channelIDs)},
			"must_not": []any{termQuery("channel_type", channelType)},
		}},
		"script": map[string]any{
			"lang":   "painless",
			"source": "ctx._source.channel_type = params.channel_type",
			"params": map[string]any{"channel_type": channelType},
		},
	}
	q := byQueryParams()
	if throttle {
		q.Set("requests_per_second", backfillRequestsRate)
	}
	ctx, cancel := context.WithTimeout(context.Background(), byQueryTimeout)
	defer cancel()
	if _, err := c.runTask(ctx, http.MethodPost, "/"+namesFor(cfg).postsPattern()+"/_update_by_query", q, body); err != nil {
		return model.NewAppError(where, errorID, e.backendParams(), "", http.StatusInternalServerError).Wrap(err)
	}
	return nil
}

func (e *Engine) UpdatePostsChannelTypeByChannelId(rctx request.CTX, channelID string, channelType string) *model.AppError {
	return e.updatePostsChannelType("Engine.UpdatePostsChannelTypeByChannelId", "ent.elasticsearch.update_posts_channel_type.error", []string{channelID}, channelType, false)
}

func (e *Engine) BackfillPostsChannelType(rctx request.CTX, channelIDs []string, channelType string) *model.AppError {
	const chunk = 1000
	for i := 0; i < len(channelIDs); i += chunk {
		part := channelIDs[i:min(i+chunk, len(channelIDs))]
		if appErr := e.updatePostsChannelType("Engine.BackfillPostsChannelType", "ent.elasticsearch.backfill_posts_channel_type.error", part, channelType, true); appErr != nil {
			return appErr
		}
	}
	return nil
}

// IndexChannel indexes a public or private channel. userIDs holds the
// members of private channels. The team members are not stored: the teams of
// a user are resolved from the user index at query time.
func (e *Engine) IndexChannel(rctx request.CTX, channel *model.Channel, userIDs, teamMemberIDs []string) *model.AppError {
	if channel == nil {
		return nil
	}
	if !shouldIndexChannel(channel) {
		// Direct and group messages don't take part in channel autocomplete.
		return nil
	}
	return e.write("Engine.IndexChannel", "ent.elasticsearch.index_channel.error", BulkAction{
		Op: opIndex, Index: namesFor(e.config()).channels(), ID: channel.Id, Doc: newChannelDocument(channel, userIDs),
	})
}

// SyncBulkIndexChannels indexes channels synchronously, in one bulk request.
func (e *Engine) SyncBulkIndexChannels(rctx request.CTX, channels []*model.Channel, getUserIDsForChannel func(channel *model.Channel) ([]string, error), teamMemberIDs []string) *model.AppError {
	c, cfg, appErr := e.activeClient("Engine.SyncBulkIndexChannels")
	if appErr != nil {
		return appErr
	}
	index := namesFor(cfg).channels()
	actions := make([]BulkAction, 0, len(channels))
	for _, channel := range channels {
		if channel == nil {
			continue
		}
		if !shouldIndexChannel(channel) {
			continue
		}
		var userIDs []string
		if channel.Type == model.ChannelTypePrivate && getUserIDsForChannel != nil {
			ids, err := getUserIDsForChannel(channel)
			if err != nil {
				return model.NewAppError("Engine.SyncBulkIndexChannels", "ent.elasticsearch.getAllChannelMembers.error", nil, "", http.StatusInternalServerError).Wrap(err)
			}
			userIDs = ids
		}
		actions = append(actions, BulkAction{Op: opIndex, Index: index, ID: channel.Id, Doc: newChannelDocument(channel, userIDs)})
	}

	e.flushPending()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	failures, err := e.sendBulk(ctx, c, actions)
	if err != nil {
		return model.NewAppError("Engine.SyncBulkIndexChannels", "ent.elasticsearch.index_channels_batch.error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	if len(failures) > 0 {
		return model.NewAppError("Engine.SyncBulkIndexChannels", "ent.elasticsearch.index_channel.error", nil,
			strconv.Itoa(len(failures))+" channel(s) rejected: "+failures[0].Reason, http.StatusInternalServerError)
	}
	return nil
}

func (e *Engine) DeleteChannel(channel *model.Channel) *model.AppError {
	if channel == nil {
		return nil
	}
	return e.write("Engine.DeleteChannel", "ent.elasticsearch.delete_channel.error", BulkAction{Op: opDelete, Index: namesFor(e.config()).channels(), ID: channel.Id})
}

func (e *Engine) IndexUser(rctx request.CTX, user *model.User, teamsIds, channelsIds []string) *model.AppError {
	if user == nil {
		return nil
	}
	return e.write("Engine.IndexUser", "ent.elasticsearch.index_user.error", BulkAction{
		Op: opIndex, Index: namesFor(e.config()).users(), ID: user.Id, Doc: newUserDocument(user, teamsIds, channelsIds),
	})
}

func (e *Engine) DeleteUser(user *model.User) *model.AppError {
	if user == nil {
		return nil
	}
	return e.write("Engine.DeleteUser", "ent.elasticsearch.delete_user.error", BulkAction{Op: opDelete, Index: namesFor(e.config()).users(), ID: user.Id})
}

func (e *Engine) IndexFile(file *model.FileInfo, channelId string) *model.AppError {
	if file == nil {
		return nil
	}
	index := namesFor(e.config()).files()
	if !ShouldIndexFile(file) {
		return e.write("Engine.IndexFile", "ent.elasticsearch.index_file.error", BulkAction{Op: opDelete, Index: index, ID: file.Id})
	}
	return e.write("Engine.IndexFile", "ent.elasticsearch.index_file.error", BulkAction{
		Op: opIndex, Index: index, ID: file.Id, Doc: newFileDocument(file, channelId),
	})
}

func (e *Engine) DeleteFile(fileID string) *model.AppError {
	return e.write("Engine.DeleteFile", "ent.elasticsearch.delete_file.error", BulkAction{Op: opDelete, Index: namesFor(e.config()).files(), ID: fileID})
}

func (e *Engine) DeletePostFiles(rctx request.CTX, postID string) *model.AppError {
	return e.deleteByQuery("Engine.DeletePostFiles", "ent.elasticsearch.delete_post_files.error",
		namesFor(e.config()).files(), termQuery("post_id", postID), nil)
}

func (e *Engine) DeleteUserFiles(rctx request.CTX, userID string) *model.AppError {
	return e.deleteByQuery("Engine.DeleteUserFiles", "ent.elasticsearch.delete_user_files.error",
		namesFor(e.config()).files(), termQuery("user_id", userID), nil)
}

// DeleteFilesBatch removes up to limit files created before endTime, like
// the corresponding database batch deletion.
func (e *Engine) DeleteFilesBatch(rctx request.CTX, endTime, limit int64) *model.AppError {
	var params url.Values
	if limit > 0 {
		params = url.Values{"max_docs": {strconv.FormatInt(limit, 10)}}
	}
	return e.deleteByQuery("Engine.DeleteFilesBatch", "ent.elasticsearch.delete_files_batch.error",
		namesFor(e.config()).files(), map[string]any{"range": map[string]any{"create_at": map[string]any{"lt": endTime}}}, params)
}

// logBulkFailures logs a sample of rejected documents.
func logBulkFailures(logger mlog.LoggerIFace, failures []BulkFailure) {
	for i, f := range failures {
		if i >= 10 {
			logger.Warn("More documents were rejected by the search backend", mlog.Int("count", len(failures)-i))
			return
		}
		logger.Warn("Document rejected by the search backend", mlog.String("index", f.Index), mlog.String("id", f.ID), mlog.Int("status", f.Status), mlog.String("reason", f.Reason))
	}
}
