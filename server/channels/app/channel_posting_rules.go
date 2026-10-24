// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"net/http"
	"slices"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

// Antimatter: who starts conversations in a channel (root posts; opening topics in a forum). create_post covers root
// posts and replies alike, so "everyone replies, only moderators post" needs a right of its own: create_root_post,
// which no role holds and only overrides give or take away. Before overrides, someone has it when the channel's
// root_posts setting is members, or when they moderate the channel and it is moderators. They also need create_post
// itself (overrides and timeouts applied), so a timed-out moderator doesn't start one either.

// IsChannelModerator tells whether someone moderates a team channel: system admins, the community's admins
// (manage_team), the channel's admins (manage_channel_roles there) and the community's moderators (delete_others_posts
// there, through their roles or an override). They're the people who may put messages on top (SessionCanChangeOnTop)
// and move anyone's thread (IsThreadModerator).
func (a *App) IsChannelModerator(rctx request.CTX, userID string, channel *model.Channel) bool {
	if a.HasPermissionTo(rctx, userID, model.PermissionManageSystem) {
		return true
	}
	if channel.TeamId != "" && a.HasPermissionToTeam(rctx, userID, channel.TeamId, model.PermissionManageTeam) {
		return true
	}
	for _, p := range []*model.Permission{model.PermissionManageChannelRoles, model.PermissionDeleteOthersPosts} {
		if has, _ := a.HasPermissionToChannel(rctx, userID, channel.Id, p); has {
			return true
		}
	}
	return false
}

// SessionIsChannelModerator is IsChannelModerator for a session.
func (a *App) SessionIsChannelModerator(rctx request.CTX, session model.Session, channel *model.Channel) bool {
	if a.SessionHasPermissionTo(session, model.PermissionManageSystem) {
		return true
	}
	if channel.TeamId != "" && a.SessionHasPermissionToTeam(session, channel.TeamId, model.PermissionManageTeam) {
		return true
	}
	for _, p := range []*model.Permission{model.PermissionManageChannelRoles, model.PermissionDeleteOthersPosts} {
		if has, _ := a.SessionHasPermissionToChannel(rctx, session, channel.Id, p); has {
			return true
		}
	}
	return false
}

// hasRootPostOverride tells whether one of the overrides allows or denies create_root_post.
func hasRootPostOverride(overrides []*model.ChannelOverride) bool {
	return slices.ContainsFunc(overrides, func(o *model.ChannelOverride) bool {
		return slices.Contains(o.Allow, model.PermissionCreateRootPost.Id) || slices.Contains(o.Deny, model.PermissionCreateRootPost.Id)
	})
}

// CanStartConversations tells whether someone may start a conversation (a root post) in a channel, or why not (403).
// Direct and group messages, and team channels where members start conversations and no override is about it, answer
// at once: whoever may post there may start one, as before channel types.
func (a *App) CanStartConversations(rctx request.CTX, userID string, channel *model.Channel) *model.AppError {
	if channel.IsGroupOrDirect() || channel.TeamId == "" {
		return nil
	}
	info := a.GetChannelTypeInfo(channel.Id)
	overrides := a.overridesFor(rctx, channel, model.PermissionCreateRootPost.Id)
	if info.RootPosts == model.AMRootPostsMembers && !hasRootPostOverride(overrides) {
		return nil
	}

	baseline := info.RootPosts == model.AMRootPostsMembers || a.IsChannelModerator(rctx, userID, channel)
	roles := func() (string, bool) { return a.userTeamRoles(rctx, userID, channel.TeamId) }
	has := a.moderated(rctx, channel, userID, roles, model.PermissionCreateRootPost.Id, baseline)
	if has {
		has, _ = a.HasPermissionToChannel(rctx, userID, channel.Id, model.PermissionCreatePost)
	}
	if has {
		return nil
	}
	if !baseline && info.RootPosts == model.AMRootPostsModerators {
		return model.NewAppError("CanStartConversations", "app.post.root_posts.moderators_only.app_error", nil, "", http.StatusForbidden)
	}
	return model.NewAppError("CanStartConversations", "app.post.root_posts.denied.app_error", nil, "", http.StatusForbidden)
}

// checkChannelPostingRules is what a channel's type asks of a post, in CreatePost: rule 2, a root post needs
// create_root_post (judged on its author: an incoming webhook's maker, the one who ran a command, a bot). System
// messages, apps' posts (trusted, as on the rest of the plugin API) and the copies of a moved or copied thread (the
// one moving it is checked instead, in MoveThreadToChannel and MoveThread) skip it; imports don't go through here.
func (a *App) checkChannelPostingRules(rctx request.CTX, post *model.Post, channel *model.Channel, flags model.CreatePostFlags) *model.AppError {
	if post.RootId != "" || post.IsSystemMessage() || flags.FromPlugin || flags.RelocatedPost {
		return nil
	}
	return a.CanStartConversations(rctx, post.UserId, channel)
}
