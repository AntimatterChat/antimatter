// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"errors"
	"net/http"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/i18n"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

// Antimatter forums: the forum data. The topic list (votes are 👍 reactions, the author's own left out); the accepted
// answer (Q&A) and the status (Ideas), props of a topic's first post that only PatchForumTopic writes (posting,
// editing and apps can't set them, see model.PostPropsServerOwned); the answer taken back when its reply is deleted;
// and voters following the ideas they vote for.

// GetForumTopics gives a page of a forum's topics, their posts ready for the person asking (opts.UserId). The Q&A
// sort and filter need a Q&A forum, the status ones an Ideas forum; the accepted answer and the status are given in
// those modes only.
func (a *App) GetForumTopics(rctx request.CTX, channel *model.Channel, opts model.ForumTopicsOptions) (*model.ForumTopicList, *model.AppError) {
	if !a.isForumChannel(channel) {
		return nil, model.NewAppError("GetForumTopics", "app.topic.not_a_forum.app_error", nil, "", http.StatusBadRequest)
	}
	info := a.GetChannelTypeInfo(channel.Id)
	qa := info.Mode == model.AMChannelModeQA
	ideas := info.Mode == model.AMChannelModeIdeas
	if (!qa && (opts.Sort == model.ForumTopicSortUnanswered || opts.Answered != nil)) ||
		(!ideas && (opts.Sort == model.ForumTopicSortStatus || opts.Status != "")) {
		return nil, model.NewAppError("GetForumTopics", "app.topic.mode.app_error", nil, "", http.StatusBadRequest)
	}

	opts.ChannelId = channel.Id
	list, err := a.Srv().Store().Post().GetForumTopics(rctx, opts)
	if err != nil {
		return nil, model.NewAppError("GetForumTopics", "app.topic.list.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	posts := model.NewPostList()
	for _, topic := range list.Topics {
		if !qa {
			topic.AcceptedAnswerId = ""
		}
		if !ideas {
			topic.Status = ""
		}
		posts.AddPost(topic.Post)
		posts.AddOrder(topic.Post.Id)
		for _, reply := range topic.Replies {
			posts.AddPost(reply)
		}
	}
	prepared := a.PreparePostListForClient(rctx, posts, nil)
	prepared, _, appErr := a.SanitizePostListMetadataForUser(rctx, prepared, opts.UserId)
	if appErr != nil {
		return nil, appErr
	}
	for _, topic := range list.Topics {
		if post, ok := prepared.Posts[topic.Post.Id]; ok {
			topic.Post = post
		}
		for i, reply := range topic.Replies {
			if post, ok := prepared.Posts[reply.Id]; ok {
				topic.Replies[i] = post
			}
		}
	}
	return list, nil
}

// getTopicRoot gives a forum topic's first post, from the master, or why it isn't one (400).
func (a *App) getTopicRoot(rctx request.CTX, postID string) (*model.Post, *model.Channel, *model.AppError) {
	root, err := a.Srv().Store().Post().GetSingle(RequestContextWithMaster(rctx), postID, false)
	if err != nil {
		var nfErr *store.ErrNotFound
		if errors.As(err, &nfErr) {
			return nil, nil, model.NewAppError("PatchForumTopic", "app.post.get.app_error", nil, "", http.StatusNotFound).Wrap(err)
		}
		return nil, nil, model.NewAppError("PatchForumTopic", "app.post.get.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	channel, appErr := a.GetChannel(rctx, root.ChannelId)
	if appErr != nil {
		return nil, nil, appErr
	}
	if root.RootId != "" || root.IsSystemMessage() || !a.isForumChannel(channel) {
		return nil, nil, model.NewAppError("PatchForumTopic", "app.topic.not_a_topic.app_error", nil, "", http.StatusBadRequest)
	}
	return root, channel, nil
}

// PatchForumTopic accepts a reply as a Q&A topic's answer ("" takes it back), or sets an Ideas topic's status, for a
// session that may read the forum (the caller checks it). The topic's author, while they may still post there, and
// the forum's moderators accept; only moderators set a status. Each change leaves a system reply in the topic, which
// tells its followers. It gives the topic's first post as it is now.
func (a *App) PatchForumTopic(rctx request.CTX, session model.Session, postID string, patch *model.ForumTopicPatch) (*model.Post, *model.AppError) {
	root, channel, appErr := a.getTopicRoot(rctx, postID)
	if appErr != nil {
		return nil, appErr
	}
	if channel.DeleteAt != 0 {
		return nil, model.NewAppError("PatchForumTopic", "api.post.update_post.can_not_update_post_in_deleted.error", nil, "", http.StatusBadRequest)
	}
	info := a.GetChannelTypeInfo(channel.Id)

	var answer *model.Post
	answerChanged := false
	if patch.AcceptedAnswerId != nil {
		if info.Mode != model.AMChannelModeQA {
			return nil, model.NewAppError("PatchForumTopic", "app.topic.mode.app_error", nil, "", http.StatusBadRequest)
		}
		if !a.sessionMayAcceptAnswer(rctx, session, root, channel) {
			return nil, model.NewAppError("PatchForumTopic", "app.topic.accept.not_allowed.app_error", nil, "", http.StatusForbidden)
		}
		if id := *patch.AcceptedAnswerId; id != "" {
			reply, err := a.Srv().Store().Post().GetSingle(RequestContextWithMaster(rctx), id, false)
			if err != nil || reply.RootId != root.Id || reply.IsSystemMessage() {
				return nil, model.NewAppError("PatchForumTopic", "app.topic.accept.not_a_reply.app_error", nil, "", http.StatusBadRequest)
			}
			answer = reply
		}
		current, _ := root.GetProp(model.PostPropsAcceptedAnswer).(string)
		answerChanged = current != *patch.AcceptedAnswerId
	}

	statusChanged := false
	if patch.Status != nil {
		if info.Mode != model.AMChannelModeIdeas {
			return nil, model.NewAppError("PatchForumTopic", "app.topic.mode.app_error", nil, "", http.StatusBadRequest)
		}
		if !a.SessionIsChannelModerator(rctx, session, channel) {
			return nil, model.NewAppError("PatchForumTopic", "app.topic.status.not_allowed.app_error", nil, "", http.StatusForbidden)
		}
		if !model.IsValidTopicStatus(*patch.Status) {
			return nil, model.NewAppError("PatchForumTopic", "app.topic.status.invalid.app_error", nil, "", http.StatusBadRequest)
		}
		current, _ := root.GetProp(model.PostPropsTopicStatus).(string)
		if current == "" {
			current = model.TopicStatusOpen
		}
		statusChanged = current != *patch.Status
	}

	if !answerChanged && !statusChanged {
		return a.PreparePostForClientWithEmbedsAndImages(rctx, root, &model.PreparePostForClientOpts{IncludePriority: true}), nil
	}

	now := model.GetMillis()
	updated, appErr := a.updateTopicProps(rctx, root, func(post *model.Post) {
		if answerChanged {
			if answer != nil {
				post.AddProp(model.PostPropsAcceptedAnswer, answer.Id)
			} else {
				post.DelProp(model.PostPropsAcceptedAnswer)
			}
		}
		if statusChanged {
			post.AddProp(model.PostPropsTopicStatus, *patch.Status)
			post.AddProp(model.PostPropsTopicStatusBy, session.UserId)
			post.AddProp(model.PostPropsTopicStatusAt, now)
		}
	})
	if appErr != nil {
		return nil, appErr
	}

	actor, appErr := a.GetUser(rctx, session.UserId)
	if appErr != nil {
		return nil, appErr
	}
	if answerChanged {
		a.postTopicAnswerReply(rctx, channel, root, actor, answer)
	}
	if statusChanged {
		a.postTopicStatusReply(rctx, channel, root, actor, *patch.Status)
	}
	return updated, nil
}

// sessionMayAcceptAnswer tells whether a session may accept (or take back) a Q&A topic's answer: the topic's author,
// while they may still post there (overrides and timeouts applied), or the forum's moderators.
func (a *App) sessionMayAcceptAnswer(rctx request.CTX, session model.Session, root *model.Post, channel *model.Channel) bool {
	if session.UserId == root.UserId {
		if ok, _ := a.SessionHasPermissionToChannel(rctx, session, channel.Id, model.PermissionCreatePost); ok {
			return true
		}
	}
	return a.SessionIsChannelModerator(rctx, session, channel)
}

// updateTopicProps changes a topic's server-owned props without editing it: UpdateAt changes, EditAt doesn't (no
// "Edited" mark), there's no edit history, and post_edited goes out.
func (a *App) updateTopicProps(rctx request.CTX, root *model.Post, apply func(*model.Post)) (*model.Post, *model.AppError) {
	post := root.Clone()
	apply(post)
	saved, err := a.Srv().Store().Post().Overwrite(rctx, post)
	if err != nil {
		var appErr *model.AppError
		if errors.As(err, &appErr) {
			return nil, appErr
		}
		return nil, model.NewAppError("updateTopicProps", "app.topic.update.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	a.invalidateCacheForChannelPosts(saved.ChannelId)

	rpost := a.PreparePostForClientWithEmbedsAndImages(rctx, saved, &model.PreparePostForClientOpts{IsEditPost: true, IncludePriority: true})
	rpost.IsFollowing = nil
	message := model.NewWebSocketEvent(model.WebsocketEventPostEdited, "", rpost.ChannelId, "", nil, "")
	a.markPostDeliveryForBroadcast(rctx, message, rpost)
	if appErr := a.publishWebsocketEventForPost(rctx, rpost, message); appErr != nil {
		return nil, appErr
	}
	return rpost, nil
}

// postTopicAnswerReply leaves the system reply that says who accepted which reply, or took the answer back.
func (a *App) postTopicAnswerReply(rctx request.CTX, channel *model.Channel, root *model.Post, actor *model.User, answer *model.Post) {
	T, err := i18n.GetTranslationsBySystemLocale()
	if err != nil {
		rctx.Logger().Warn("Failed to get the translations for a topic's system reply", mlog.Err(err))
		return
	}
	props := model.StringInterface{"username": actor.Username}
	message := T("app.topic.answer.removed", map[string]any{"Username": actor.Username})
	if answer != nil {
		authorName := ""
		if author, appErr := a.GetUser(rctx, answer.UserId); appErr == nil {
			authorName = author.Username
		}
		props["answer_id"] = answer.Id
		props["answer_user_id"] = answer.UserId
		props["answer_username"] = authorName
		message = T("app.topic.answer.accepted", map[string]any{"Username": actor.Username, "AuthorUsername": authorName})
	}
	a.postTopicSystemReply(rctx, channel, root, actor, model.PostTypeTopicAnswer, message, props)
}

// postTopicStatusReply leaves the system reply that says who set which status.
func (a *App) postTopicStatusReply(rctx request.CTX, channel *model.Channel, root *model.Post, actor *model.User, status string) {
	T, err := i18n.GetTranslationsBySystemLocale()
	if err != nil {
		rctx.Logger().Warn("Failed to get the translations for a topic's system reply", mlog.Err(err))
		return
	}
	var name string
	switch status {
	case model.TopicStatusPlanned:
		name = T("app.topic.status.name.planned")
	case model.TopicStatusInProgress:
		name = T("app.topic.status.name.in_progress")
	case model.TopicStatusDone:
		name = T("app.topic.status.name.done")
	case model.TopicStatusDeclined:
		name = T("app.topic.status.name.declined")
	default:
		name = T("app.topic.status.name.open")
	}
	message := T("app.topic.status.changed", map[string]any{"Username": actor.Username, "Status": name})
	a.postTopicSystemReply(rctx, channel, root, actor, model.PostTypeTopicStatus, message, model.StringInterface{"username": actor.Username, "status": status})
}

func (a *App) postTopicSystemReply(rctx request.CTX, channel *model.Channel, root *model.Post, actor *model.User, postType, message string, props model.StringInterface) {
	post := &model.Post{UserId: actor.Id, ChannelId: channel.Id, RootId: root.Id, Type: postType, Message: message, Props: props}
	if _, _, appErr := a.CreatePost(rctx, post, channel, model.CreatePostFlags{}); appErr != nil {
		rctx.Logger().Warn("Failed to post a topic's system reply", mlog.String("post_id", root.Id), mlog.String("type", postType), mlog.Err(appErr))
	}
}

// clearAcceptedAnswerOf takes the accepted answer back from a forum topic when that reply is deleted.
func (a *App) clearAcceptedAnswerOf(rctx request.CTX, reply *model.Post) {
	if reply.RootId == "" || !a.GetChannelTypeInfo(reply.ChannelId).IsForum() {
		return
	}
	root, err := a.Srv().Store().Post().GetSingle(RequestContextWithMaster(rctx), reply.RootId, false)
	if err != nil {
		return // the topic is gone too
	}
	if accepted, _ := root.GetProp(model.PostPropsAcceptedAnswer).(string); accepted != reply.Id {
		return
	}
	if _, appErr := a.updateTopicProps(rctx, root, func(post *model.Post) { post.DelProp(model.PostPropsAcceptedAnswer) }); appErr != nil {
		rctx.Logger().Warn("Failed to take back the accepted answer of a topic", mlog.String("post_id", root.Id), mlog.Err(appErr))
	}
}

// followTopicOnVote makes whoever votes for an Ideas topic (a 👍 on its first post, not their own) follow it, so its
// status changes reach them. Taking the vote back doesn't unfollow.
func (a *App) followTopicOnVote(rctx request.CTX, reaction *model.Reaction, post *model.Post, channel *model.Channel) {
	if post.RootId != "" || post.UserId == reaction.UserId || !model.IsVoteEmoji(reaction.EmojiName) {
		return
	}
	if !*a.Config().ServiceSettings.ThreadAutoFollow || !a.isForumChannel(channel) || a.GetChannelTypeInfo(channel.Id).Mode != model.AMChannelModeIdeas {
		return
	}
	if appErr := a.UpdateThreadFollowForUser(reaction.UserId, channel.TeamId, post.Id, true); appErr != nil {
		rctx.Logger().Warn("Failed to make a voter follow an idea", mlog.String("post_id", post.Id), mlog.String("user_id", reaction.UserId), mlog.Err(appErr))
	}
}
