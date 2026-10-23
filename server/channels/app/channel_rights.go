// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"net/http"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

// Antimatter: who may archive a channel, and who may put its messages on top.
//
// Upstream, every channel member holds delete_public/private_channel (channel_user), so any member could archive any
// channel they were in. Antimatter's defaults (permissions migration MigrationKeyAntimatterChannelArchiveRights) give
// it to the community's admins (team_admin) instead, and the kind of community adds:
//
//   - a guild (TeamKindGuild): nobody else; its moderators, when a role grants them "Manage channels";
//   - a workspace (TeamKindWorkspace): also the channel's admins, that is whoever created it and those they made
//     admins of it.
//
// What a system admin grants through the permission schemes (channel_user, channel_admin, a team override scheme) or
// a community admin through a role still applies as it is.

// SessionCanArchiveChannel tells whether a session may archive a channel of a team, or why not. Direct and group
// messages aren't archived; the default channels' extra rule (the community's admins only) is the API's.
func (a *App) SessionCanArchiveChannel(rctx request.CTX, session model.Session, channel *model.Channel) *model.AppError {
	perm := model.PermissionDeletePublicChannel
	if channel.Type == model.ChannelTypePrivate {
		perm = model.PermissionDeletePrivateChannel
	}
	if ok, _ := a.SessionHasPermissionToChannel(rctx, session, channel.Id, perm); ok {
		return nil
	}
	if channel.TeamId != "" && a.isWorkspaceTeam(channel.TeamId) {
		if ok, _ := a.SessionHasPermissionToChannel(rctx, session, channel.Id, model.PermissionManageChannelRoles); ok {
			return nil
		}
	}
	return model.MakePermissionError(&session, []*model.Permission{perm})
}

// isWorkspaceTeam tells whether a team is a workspace; one that can't be read counts as a guild, the kind that allows
// less.
func (a *App) isWorkspaceTeam(teamID string) bool {
	team, appErr := a.GetTeam(teamID)
	return appErr == nil && team.IsWorkspace()
}

// SessionCanChangeOnTop tells whether a session may put a channel's messages on top, or take them off: in a direct or
// group message, its members, as before; in a team's channel, its admins (manage_channel_roles), the community's
// admins and its moderators (who manage messages: delete_others_posts). Members don't, in any kind of community. They
// must also be able to post there, so that the channel's moderation (read-only channels, overrides), timeouts and
// membership apply. The archived channels are refused by checkCanPutOnTop.
func (a *App) SessionCanChangeOnTop(rctx request.CTX, session model.Session, channel *model.Channel) *model.AppError {
	if ok, _ := a.SessionHasPermissionToChannel(rctx, session, channel.Id, model.PermissionCreatePost); !ok {
		return model.MakePermissionError(&session, []*model.Permission{model.PermissionCreatePost})
	}
	if channel.IsGroupOrDirect() {
		return nil
	}
	for _, p := range []*model.Permission{model.PermissionManageChannelRoles, model.PermissionDeleteOthersPosts} {
		if ok, _ := a.SessionHasPermissionToChannel(rctx, session, channel.Id, p); ok {
			return nil
		}
	}
	if a.SessionHasPermissionToTeam(session, channel.TeamId, model.PermissionManageTeam) {
		return nil
	}
	return model.NewAppError("SessionCanChangeOnTop", "app.on_top_post.moderators_only.app_error", nil, "", http.StatusForbidden)
}
