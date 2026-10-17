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
const antimatterThreadTitleProp = "am_thread_title"

// CanMoveThread says whether a user may move the thread of root, in the
// channel source, elsewhere: the thread's author, the channel's admins, the
// community's admins, those with "Manage messages" in the channel (delete
// others' messages, as moving deletes the originals) and system admins.
func (a *App) CanMoveThread(rctx request.CTX, userID string, root *model.Post, source *model.Channel) bool {
	if root.UserId == userID {
		return true
	}
	if a.HasPermissionTo(rctx, userID, model.PermissionManageSystem) {
		return true
	}
	if source.TeamId != "" && a.HasPermissionToTeam(rctx, userID, source.TeamId, model.PermissionManageTeam) {
		return true
	}
	if member, appErr := a.GetChannelMember(rctx, source.Id, userID); appErr == nil && member.SchemeAdmin {
		return true
	}
	has, _ := a.HasPermissionToChannel(rctx, userID, source.Id, model.PermissionDeleteOthersPosts)
	return has
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

	if !a.CanMoveThread(rctx, user.Id, root, source) {
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
	if limit := *a.Config().WranglerSettings.MoveThreadMaxCount; limit > 0 && int64(wpl.NumPosts()) > limit {
		return nil, moveThreadError("api.post.move_thread.too_long.app_error", map[string]any{"Count": wpl.NumPosts(), "Max": limit}, http.StatusBadRequest)
	}

	team, appErr := a.GetTeam(target.TeamId)
	if appErr != nil {
		return nil, appErr
	}

	rctx.Logger().Info("Moving a thread", mlog.String("user_id", user.Id), mlog.String("root_id", root.Id), mlog.String("from_channel_id", source.Id), mlog.String("to_channel_id", target.Id), mlog.Int("messages", wpl.NumPosts()), mlog.Bool("flatten", flatten))

	newRoot, _, appErr := a.copyPostList(rctx, wpl, target, copyPostListOptions{keepTimes: true, silent: true, flatten: flatten})
	if appErr != nil {
		return nil, appErr
	}

	T, err := i18n.GetTranslationsBySystemLocale()
	if err != nil {
		return nil, model.NewAppError("MoveThreadToChannel", "app.post.move_thread_command.error", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	// The note in the target: under the moved thread, or after the messages.
	note := &model.Post{
		UserId:    user.Id,
		Type:      model.PostTypeWrangler,
		ChannelId: target.Id,
		Message:   T("app.post.move_thread.from_another_channel"),
		Props:     model.StringInterface{"TranslationID": "app.post.move_thread.from_another_channel"},
	}
	if !flatten {
		note.RootId = newRoot.Id
	}
	if _, _, appErr = a.CreatePost(rctx, note, target, model.CreatePostFlags{}); appErr != nil {
		return nil, appErr
	}

	// Deleting the first message deletes its replies.
	if _, appErr = a.DeletePost(rctx, root.Id, user.Id); appErr != nil {
		return nil, appErr
	}

	// The pointer in the old place, as upstream's move leaves it.
	link := makePostLink(*a.Config().ServiceSettings.SiteURL, team.Name, newRoot.Id)
	translationID := "app.post.move_thread_command.channel.multiple_messages"
	message := T(translationID, model.StringInterface{"NumMessages": wpl.NumPosts(), "Link": link})
	if wpl.NumPosts() == 1 {
		translationID = "app.post.move_thread_command.channel.one_message"
		message = T(translationID, model.StringInterface{"Link": link})
	}
	if _, _, appErr = a.CreatePost(rctx, &model.Post{
		UserId:    user.Id,
		Type:      model.PostTypeWrangler,
		ChannelId: source.Id,
		Message:   message,
		Props: model.StringInterface{
			"TranslationID":        translationID,
			"MovedThreadPermalink": link,
			"NumMessages":          wpl.NumPosts(),
		},
	}, source, model.CreatePostFlags{}); appErr != nil {
		return nil, appErr
	}

	return newRoot, nil
}
