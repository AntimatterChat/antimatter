// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"net/http"

	"github.com/mattermost/mattermost/server/public/model"
)

// Antimatter forums, rule 1: a forum takes only topics. A root post there needs a title (props.am_thread_title, 1 to
// 100 runes), from everyone and every way of posting, apps, moved threads and imports included; replies need none
// and lose any. Editing a topic without a title keeps the old one.

// isForumChannel tells whether a channel is a team forum, from memory.
func (a *App) isForumChannel(channel *model.Channel) bool {
	return channel != nil && !channel.IsGroupOrDirect() && channel.TeamId != "" && a.GetChannelTypeInfo(channel.Id).IsForum()
}

// checkForumStructure is rule 1 when a post is made: in a forum a root post is a topic, with a title (kept trimmed),
// and a reply has none. System messages skip it.
func (a *App) checkForumStructure(post *model.Post, channel *model.Channel) *model.AppError {
	if post.IsSystemMessage() || !a.isForumChannel(channel) {
		return nil
	}
	if post.RootId != "" {
		if post.GetProp(model.PostPropsThreadTitle) != nil {
			post.DelProp(model.PostPropsThreadTitle)
		}
		return nil
	}
	title := post.TopicTitle()
	if appErr := model.ValidateForumTopicTitle(title); appErr != nil {
		return appErr
	}
	post.AddProp(model.PostPropsThreadTitle, title)
	return nil
}

// applyForumTopicEdit is rule 1 when a post is edited (UpdatePost, and so PatchPost and the plugin API): a topic
// edited without a title (or an empty one) keeps its own, so a client that doesn't know titles can't erase it; a new
// title renames it. A reply in a forum keeps having none.
func (a *App) applyForumTopicEdit(newPost, oldPost *model.Post) *model.AppError {
	if oldPost.IsSystemMessage() || !a.GetChannelTypeInfo(oldPost.ChannelId).IsForum() {
		return nil
	}
	if oldPost.RootId != "" {
		if newPost.GetProp(model.PostPropsThreadTitle) != nil {
			newPost.DelProp(model.PostPropsThreadTitle)
		}
		return nil
	}
	title := newPost.TopicTitle()
	if title == "" {
		if old := oldPost.GetProp(model.PostPropsThreadTitle); old != nil {
			newPost.AddProp(model.PostPropsThreadTitle, old)
		} else if newPost.GetProp(model.PostPropsThreadTitle) != nil {
			newPost.DelProp(model.PostPropsThreadTitle)
		}
		return nil
	}
	if appErr := model.ValidateForumTopicTitle(title); appErr != nil {
		return appErr
	}
	newPost.AddProp(model.PostPropsThreadTitle, title)
	return nil
}

// forumTitleForMove gives the title a thread moved into a forum gets: the one asked for, else the thread's own. A
// move into a forum can't flatten the thread (its replies would become topics without titles).
func forumTitleForMove(root *model.Post, title string, flatten bool) (string, *model.AppError) {
	if flatten {
		return "", model.NewAppError("MoveThreadToChannel", "app.post.move_thread.forum_flatten.app_error", nil, "", http.StatusBadRequest)
	}
	if title == "" {
		title = root.TopicTitle()
	}
	if title == "" {
		return "", model.NewAppError("MoveThreadToChannel", "app.post.move_thread.forum_title_required.app_error", nil, "", http.StatusBadRequest)
	}
	if appErr := model.ValidateForumTopicTitle(title); appErr != nil {
		return "", appErr
	}
	return title, nil
}
