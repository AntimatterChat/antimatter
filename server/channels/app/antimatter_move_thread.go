// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"net/http"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/i18n"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

// Antimatter's thread moving. Upstream's move-thread API answers only while the
// MoveThreadsEnabled feature flag is on, and upstream retired that flag (the
// server refuses to start with it): moving threads went to their Wrangler
// plugin. Antimatter moves threads itself, between the channels of one
// community, with its own rules on who may move what.

// antimatterThreadTitleProp is where Fusion keeps a topic's name (a named
// thread), on the thread's first message.
const antimatterThreadTitleProp = model.PostPropsThreadTitle

// IsThreadModerator says whether a user moderates the threads of the channel
// source: its channel moderators (IsChannelModerator: its channel admins, the
// community's admins and moderators, who manage messages, as moving deletes
// the originals, and system admins). They may move any of its threads to
// another channel of the community.
func (a *App) IsThreadModerator(rctx request.CTX, userID string, source *model.Channel) bool {
	return a.IsChannelModerator(rctx, userID, source)
}

// CanMoveThread says whether a user may move the thread (posts, its first
// message first) from source to target. Moderators may (IsThreadModerator).
// The thread's author without moderation rights may only when every message of
// the thread is theirs, or when both channels are public: otherwise they could
// expose others' replies from a private channel, or hide them in a private
// channel of their own. It returns nil when allowed.
func (a *App) CanMoveThread(rctx request.CTX, userID string, posts []*model.Post, source, target *model.Channel) *model.AppError {
	if len(posts) == 0 {
		return moveThreadError("api.post.move_thread.unsupported.app_error", nil, http.StatusBadRequest)
	}
	if a.IsThreadModerator(rctx, userID, source) {
		return nil
	}
	root := posts[0]
	for _, post := range posts {
		if post.RootId == "" {
			root = post
			break
		}
	}
	if root.UserId != userID {
		return moveThreadError("api.post.move_thread.no_permission", nil, http.StatusForbidden)
	}
	if source.Type == model.ChannelTypeOpen && target.Type == model.ChannelTypeOpen {
		return nil
	}
	for _, post := range posts {
		if post.UserId != userID {
			return moveThreadError("api.post.move_thread.private_needs_moderator.app_error", nil, http.StatusForbidden)
		}
	}
	return nil
}

func moveThreadError(id string, params map[string]any, status int) *model.AppError {
	return model.NewAppError("MoveThreadToChannel", id, params, "", status)
}

// MoveThreadToChannel moves the thread of postID (its first message and the
// replies) to targetChannelID, in the same community, for the user. The
// messages keep their authors, times, reactions and files; their copies
// notify nobody. With flatten, the replies become messages of the target
// channel instead of a thread (Fusion's "Make it a channel"). The old place
// gets a pointer to the new one, and the target a note that the messages came
// from elsewhere. It returns the moved first message.
func (a *App) MoveThreadToChannel(rctx request.CTX, postID, targetChannelID string, flatten bool, user *model.User) (*model.Post, *model.AppError) {
	return a.MoveThreadToChannelWithTitle(rctx, postID, targetChannelID, flatten, "", user)
}

// MoveThreadToChannelWithTitle is MoveThreadToChannel with the title the thread gets as a topic when the target is a
// forum (forums take only topics): title, else the thread's own; without either the move is refused, as is
// flattening it into a forum. Elsewhere the title is ignored.
func (a *App) MoveThreadToChannelWithTitle(rctx request.CTX, postID, targetChannelID string, flatten bool, title string, user *model.User) (*model.Post, *model.AppError) {
	post, appErr := a.GetSinglePost(rctx, postID, false)
	if appErr != nil {
		return nil, appErr
	}
	root := post
	if post.RootId != "" {
		if root, appErr = a.GetSinglePost(rctx, post.RootId, false); appErr != nil {
			return nil, appErr
		}
	}
	if root.IsSystemMessage() || root.Type == model.PostTypeBurnOnRead {
		return nil, moveThreadError("api.post.move_thread.unsupported.app_error", nil, http.StatusBadRequest)
	}

	source, appErr := a.GetChannel(rctx, root.ChannelId)
	if appErr != nil {
		return nil, appErr
	}
	target, appErr := a.GetChannel(rctx, targetChannelID)
	if appErr != nil {
		return nil, appErr
	}

	// Only authors and moderators get further; CanMoveThread decides once the
	// thread and the target are known.
	if root.UserId != user.Id && !a.IsThreadModerator(rctx, user.Id, source) {
		return nil, moveThreadError("api.post.move_thread.no_permission", nil, http.StatusForbidden)
	}
	if source.DeleteAt != 0 || target.DeleteAt != 0 {
		return nil, moveThreadError("api.post.move_thread.archived.app_error", nil, http.StatusBadRequest)
	}
	if source.TeamId == "" || target.TeamId != source.TeamId || source.IsGroupOrDirect() || target.IsGroupOrDirect() {
		return nil, moveThreadError("api.post.move_thread.same_community.app_error", nil, http.StatusBadRequest)
	}
	if target.Id == source.Id {
		return nil, moveThreadError("api.post.move_thread.same_channel.app_error", nil, http.StatusBadRequest)
	}
	if canPost, _ := a.HasPermissionToChannel(rctx, user.Id, target.Id, model.PermissionCreatePost); !canPost {
		return nil, moveThreadError("api.post.move_thread.cannot_post.app_error", nil, http.StatusForbidden)
	}
	// The moved thread starts a conversation there (its replies too, flattened): the target's rule on who starts
	// them holds the one moving it (its copies skip it).
	if appErr = a.CanStartConversations(rctx, user.Id, target); appErr != nil {
		return nil, appErr
	}
	// The copies aren't held back by the community's rules (their authors aren't speaking anew); the one moving is.
	if appErr = a.checkCommunityRules(rctx, user, target); appErr != nil {
		return nil, appErr
	}
	// Antimatter: a forum takes only topics, so the thread comes in as one, with a title.
	forumTitle := ""
	if a.isForumChannel(target) {
		if forumTitle, appErr = forumTitleForMove(root, title, flatten); appErr != nil {
			return nil, appErr
		}
	}

	thread, appErr := a.GetPostThread(rctx, root.Id, model.GetPostsOptions{}, user.Id)
	if appErr != nil {
		return nil, appErr
	}
	wpl := thread.BuildWranglerPostList()
	if wpl.NumPosts() == 0 {
		return nil, moveThreadError("api.post.move_thread.unsupported.app_error", nil, http.StatusBadRequest)
	}
	for _, p := range wpl.Posts {
		if p.Type == model.PostTypeBurnOnRead {
			return nil, moveThreadError("api.post.move_thread.unsupported.app_error", nil, http.StatusBadRequest)
		}
	}
	if appErr = a.CanMoveThread(rctx, user.Id, wpl.Posts, source, target); appErr != nil {
		return nil, appErr
	}
	if limit := *a.Config().WranglerSettings.MoveThreadMaxCount; limit > 0 && int64(wpl.NumPosts()) > limit {
		return nil, moveThreadError("api.post.move_thread.too_long.app_error", map[string]any{"Count": wpl.NumPosts(), "Max": limit}, http.StatusBadRequest)
	}

	team, appErr := a.GetTeam(target.TeamId)
	if appErr != nil {
		return nil, appErr
	}

	rctx.Logger().Info("Moving a thread", mlog.String("user_id", user.Id), mlog.String("root_id", root.Id), mlog.String("from_channel_id", source.Id), mlog.String("to_channel_id", target.Id), mlog.Int("messages", wpl.NumPosts()), mlog.Bool("flatten", flatten))

	if forumTitle != "" {
		wpl.Posts[0].AddProp(model.PostPropsThreadTitle, forumTitle)
	}
	newRoot, _, appErr := a.copyPostList(rctx, wpl, target, copyPostListOptions{keepTimes: true, silent: true, flatten: flatten})
	if appErr != nil {
		return nil, appErr
	}

	T, err := i18n.GetTranslationsBySystemLocale()
	if err != nil {
		return nil, model.NewAppError("MoveThreadToChannel", "app.post.move_thread_command.error", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	// Deleting the first message deletes its replies.
	if _, appErr = a.DeletePost(rctx, root.Id, user.Id); appErr != nil {
		return nil, appErr
	}

	siteURL := *a.Config().ServiceSettings.SiteURL

	// The pointer in the old place: "A thread with 13 messages has been moved to #target", the name a link to the
	// moved thread. A private target is "another channel" (its name stays out of the message and the props); the
	// link is kept, and refuses who can't see it.
	pointerMessage, pointerProps := movedThreadPointer(T, wpl.NumPosts(), target, team.Name, newRoot.Id, makePostLink(siteURL, team.Name, newRoot.Id))
	pointer, _, appErr := a.CreatePost(rctx, &model.Post{
		UserId:    user.Id,
		Type:      model.PostTypeWrangler,
		ChannelId: source.Id,
		Message:   pointerMessage,
		Props:     pointerProps,
	}, source, model.CreatePostFlags{})
	if appErr != nil {
		return nil, appErr
	}

	// The note in the target, under the moved thread or after the messages: "This thread was moved from #source",
	// the name a link to the pointer left there.
	noteMessage, noteProps := movedThreadNote(T, source, team.Name, pointer.Id, makePostLink(siteURL, team.Name, pointer.Id))
	note := &model.Post{
		UserId:    user.Id,
		Type:      model.PostTypeWrangler,
		ChannelId: target.Id,
		Message:   noteMessage,
		Props:     noteProps,
	}
	if !flatten {
		note.RootId = newRoot.Id
	}
	if _, _, appErr = a.CreatePost(rctx, note, target, model.CreatePostFlags{}); appErr != nil {
		return nil, appErr
	}

	return newRoot, nil
}

// The props of Antimatter's pointer and note posts, beside upstream's TranslationID, MovedThreadPermalink and
// NumMessages, so clients write the channel's name as a link (#name) instead of a raw address. A private channel's
// name and display name are left out: readers who aren't in it see "another channel".
const (
	PostPropsMovedThreadChannelID          = "MovedThreadChannelId"
	PostPropsMovedThreadChannelName        = "MovedThreadChannelName"
	PostPropsMovedThreadChannelDisplayName = "MovedThreadChannelDisplayName"
	PostPropsMovedThreadTeamName           = "MovedThreadTeamName"
	PostPropsMovedThreadPostID             = "MovedThreadPostId"
	PostPropsMovedFromChannelID            = "MovedFromChannelId"
	PostPropsMovedFromChannelName          = "MovedFromChannelName"
	PostPropsMovedFromChannelDisplayName   = "MovedFromChannelDisplayName"
	PostPropsMovedFromTeamName             = "MovedFromTeamName"
	PostPropsMovedFromPostID               = "MovedFromPostId"
	PostPropsMovedFromPermalink            = "MovedFromPermalink"
)

// movedThreadPointer gives the message and props of the pointer left where a thread was.
func movedThreadPointer(T i18n.TranslateFunc, count int, target *model.Channel, teamName, newRootID, link string) (string, model.StringInterface) {
	public := target.Type == model.ChannelTypeOpen
	translationID := "app.post.move_thread.moved_to"
	if !public {
		translationID = "app.post.move_thread.moved_to_hidden"
	}
	if count == 1 {
		translationID += ".one"
	} else {
		translationID += ".multiple"
	}
	message := T(translationID, map[string]any{"NumMessages": count, "ChannelName": target.Name, "Link": link})

	props := model.StringInterface{
		"TranslationID":               translationID,
		"MovedThreadPermalink":        link,
		"NumMessages":                 count,
		PostPropsMovedThreadChannelID: target.Id,
		PostPropsMovedThreadTeamName:  teamName,
		PostPropsMovedThreadPostID:    newRootID,
	}
	if public {
		props[PostPropsMovedThreadChannelName] = target.Name
		props[PostPropsMovedThreadChannelDisplayName] = target.DisplayName
	}
	return message, props
}

// movedThreadNote gives the message and props of the note that says where a moved thread came from.
func movedThreadNote(T i18n.TranslateFunc, source *model.Channel, teamName, pointerID, link string) (string, model.StringInterface) {
	public := source.Type == model.ChannelTypeOpen
	translationID := "app.post.move_thread.moved_from"
	if !public {
		translationID = "app.post.move_thread.moved_from_hidden"
	}
	message := T(translationID, map[string]any{"ChannelName": source.Name, "Link": link})

	props := model.StringInterface{
		"TranslationID":             translationID,
		PostPropsMovedFromChannelID: source.Id,
		PostPropsMovedFromTeamName:  teamName,
		PostPropsMovedFromPostID:    pointerID,
		PostPropsMovedFromPermalink: link,
	}
	if public {
		props[PostPropsMovedFromChannelName] = source.Name
		props[PostPropsMovedFromChannelDisplayName] = source.DisplayName
	}
	return message, props
}
