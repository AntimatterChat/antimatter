// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/mattermost/mattermost/server/public/model"
)

// Antimatter forums: the topic list of a forum, for whoever may read it, and a topic's accepted answer (Q&A) and
// status (Ideas), for the asker and the forum's moderators (see app.PatchForumTopic).

// forumTopicsQueryMaxRunes caps the topic list's q.
const forumTopicsQueryMaxRunes = 200

func (api *API) InitForumTopics() {
	api.BaseRoutes.Channel.Handle("/topics", api.APISessionRequired(getForumTopics)).Methods(http.MethodGet)
	api.BaseRoutes.Post.Handle("/topic", api.APISessionRequired(patchForumTopic)).Methods(http.MethodPatch)
}

// forumTopicsOptions reads the topic list's query: sort, answered, status, mine, following, q, before, per_page and
// replies. It sets c.Err on a bad one.
func forumTopicsOptions(c *Context, r *http.Request) model.ForumTopicsOptions {
	query := r.URL.Query()
	opts := model.ForumTopicsOptions{UserId: c.AppContext.Session().UserId, PerPage: model.ForumTopicsPerPageDefault}

	switch sort := query.Get("sort"); sort {
	case "", model.ForumTopicSortActivity, model.ForumTopicSortNewest, model.ForumTopicSortVotes, model.ForumTopicSortUnanswered, model.ForumTopicSortStatus:
		opts.Sort = sort
	default:
		c.SetInvalidURLParam("sort")
		return opts
	}

	if v := query.Get("answered"); v != "" {
		answered, err := strconv.ParseBool(v)
		if err != nil {
			c.SetInvalidURLParam("answered")
			return opts
		}
		opts.Answered = &answered
	}
	if v := query.Get("status"); v != "" {
		if !model.IsValidTopicStatus(v) {
			c.SetInvalidURLParam("status")
			return opts
		}
		opts.Status = v
	}
	for name, field := range map[string]*bool{"mine": &opts.Mine, "following": &opts.Following} {
		if v := query.Get(name); v != "" {
			b, err := strconv.ParseBool(v)
			if err != nil {
				c.SetInvalidURLParam(name)
				return opts
			}
			*field = b
		}
	}

	opts.Query = strings.TrimSpace(query.Get("q"))
	if utf8.RuneCountInString(opts.Query) > forumTopicsQueryMaxRunes {
		c.SetInvalidURLParam("q")
		return opts
	}

	cursor, err := model.ParseForumTopicCursor(query.Get("before"))
	if err != nil {
		c.SetInvalidURLParam("before")
		return opts
	}
	opts.Before = cursor

	if v := query.Get("per_page"); v != "" {
		perPage, err := strconv.Atoi(v)
		if err != nil || perPage < 1 {
			c.SetInvalidURLParam("per_page")
			return opts
		}
		opts.PerPage = min(perPage, model.ForumTopicsPerPageMax)
	}
	if v := query.Get("replies"); v != "" {
		replies, err := strconv.Atoi(v)
		if err != nil || replies < 0 || replies > model.ForumTopicRepliesMax {
			c.SetInvalidURLParam("replies")
			return opts
		}
		opts.Replies = replies
	}
	return opts
}

// getForumTopics lists a forum's topics, a page at a time, for whoever may read the channel.
func getForumTopics(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireChannelId()
	if c.Err != nil {
		return
	}
	channel, appErr := c.App.GetChannel(c.AppContext, c.Params.ChannelId)
	if appErr != nil {
		c.Err = appErr
		return
	}
	if ok, _ := c.App.SessionHasPermissionToReadChannel(c.AppContext, *c.AppContext.Session(), channel); !ok {
		c.SetPermissionError(model.PermissionReadChannelContent)
		return
	}

	opts := forumTopicsOptions(c, r)
	if c.Err != nil {
		return
	}
	list, appErr := c.App.GetForumTopics(c.AppContext, channel, opts)
	if appErr != nil {
		c.Err = appErr
		return
	}
	writeJSON(c, w, list)
}

// patchForumTopic accepts a Q&A topic's answer or sets an Ideas topic's status, and answers with the topic's first
// post.
func patchForumTopic(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequirePostId()
	if c.Err != nil {
		return
	}

	var patch model.ForumTopicPatch
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		c.SetInvalidParamWithErr("topic", err)
		return
	}
	if patch.AcceptedAnswerId == nil && patch.Status == nil {
		c.SetInvalidParam("topic")
		return
	}
	if patch.AcceptedAnswerId != nil && *patch.AcceptedAnswerId != "" && !model.IsValidId(*patch.AcceptedAnswerId) {
		c.SetInvalidParam("accepted_answer_id")
		return
	}

	auditRec := c.MakeAuditRecord(model.AuditEventPatchForumTopic, model.AuditStatusFail)
	defer c.LogAuditRec(auditRec)
	model.AddEventParameterToAuditRec(auditRec, "post_id", c.Params.PostId)
	if patch.AcceptedAnswerId != nil {
		model.AddEventParameterToAuditRec(auditRec, "accepted_answer_id", *patch.AcceptedAnswerId)
	}
	if patch.Status != nil {
		model.AddEventParameterToAuditRec(auditRec, "status", *patch.Status)
	}

	// Who can't read the topic learns nothing from the answer either.
	if ok, _ := c.App.SessionHasPermissionToReadPost(c.AppContext, *c.AppContext.Session(), c.Params.PostId); !ok {
		c.SetPermissionError(model.PermissionReadChannelContent)
		return
	}

	updated, appErr := c.App.PatchForumTopic(c.AppContext, *c.AppContext.Session(), c.Params.PostId, &patch)
	if appErr != nil {
		c.Err = appErr
		return
	}

	auditRec.Success()
	auditRec.AddEventObjectType("post")
	writeJSON(c, w, updated)
}
