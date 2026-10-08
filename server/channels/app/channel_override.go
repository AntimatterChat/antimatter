// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

// Antimatter channel overrides, as Discord's permission overwrites: a role of the channel's team, or one person, may be
// allowed or denied some permissions in a channel. Channel permission checks first answer from roles as Mattermost
// does, then the channel's overrides change that answer (model.ApplyChannelOverrides). Team and system admins aren't
// affected. The @everyone override is the channel's moderation (its scheme), already in the first answer.

// channelOverridesTTL bounds how long a node trusts its cached overrides, for changes made through another node.
const channelOverridesTTL = time.Minute

type cachedChannelOverrides struct {
	overrides []*model.ChannelOverride
	loadedAt  time.Time
}

var (
	overridablePermissions     map[string]bool
	overridablePermissionsOnce sync.Once
)

func isOverridablePermission(permissionID string) bool {
	overridablePermissionsOnce.Do(func() {
		overridablePermissions = map[string]bool{}
		for _, p := range model.ChannelOverridePermissions() {
			overridablePermissions[p] = true
		}
	})
	return overridablePermissions[permissionID]
}

// GetChannelOverrides gives a channel's overrides, from the cache when fresh.
func (a *App) GetChannelOverrides(rctx request.CTX, channelID string) ([]*model.ChannelOverride, *model.AppError) {
	if cached, ok := a.Channels().channelOverrides.Load(channelID); ok {
		entry := cached.(cachedChannelOverrides)
		if time.Since(entry.loadedAt) < channelOverridesTTL {
			return entry.overrides, nil
		}
	}
	overrides, err := a.Srv().Store().ChannelOverride().GetForChannel(rctx, channelID)
	if err != nil {
		return nil, model.NewAppError("GetChannelOverrides", "app.channel_override.get.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	a.Channels().channelOverrides.Store(channelID, cachedChannelOverrides{overrides: overrides, loadedAt: time.Now()})
	return overrides, nil
}

// SaveChannelOverride sets the override of a role or person in a channel; an empty one is removed. A role must be
// one of the channel's team.
func (a *App) SaveChannelOverride(rctx request.CTX, channel *model.Channel, override *model.ChannelOverride) (*model.ChannelOverride, *model.AppError) {
	override.ChannelId = channel.Id
	if appErr := override.IsValid(); appErr != nil {
		return nil, appErr
	}
	if override.TargetType == model.ChannelOverrideTargetRole {
		group, err := a.Srv().Store().Group().Get(override.TargetId)
		if err != nil || !group.IsRole() || group.TeamId != channel.TeamId {
			return nil, model.NewAppError("SaveChannelOverride", "app.channel_override.role.app_error", nil, "", http.StatusBadRequest)
		}
	}

	var err error
	if override.IsEmpty() {
		err = a.Srv().Store().ChannelOverride().Delete(rctx, channel.Id, override.TargetType, override.TargetId)
	} else {
		override, err = a.Srv().Store().ChannelOverride().Save(rctx, override)
	}
	if err != nil {
		return nil, model.NewAppError("SaveChannelOverride", "app.channel_override.save.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	a.Channels().channelOverrides.Delete(channel.Id)
	a.publishChannelOverridesUpdated(channel)
	return override, nil
}

func (a *App) publishChannelOverridesUpdated(channel *model.Channel) {
	message := model.NewWebSocketEvent(model.WebsocketEventChannelOverridesUpdated, "", channel.Id, "", nil, "")
	message.Add("channel_id", channel.Id)
	a.Publish(message)
}

// applyChannelOverrides changes a permission check's answer for someone in a channel by the channel's overrides.
// teamRoles are the person's roles in the channel's team; teamAdmin, whether overrides pass them by.
func (a *App) applyChannelOverrides(rctx request.CTX, channel *model.Channel, userID, teamRoles string, teamAdmin bool, permissionID string, has bool) bool {
	if teamAdmin || channel == nil || channel.TeamId == "" || !isOverridablePermission(permissionID) {
		return has
	}
	overrides, appErr := a.GetChannelOverrides(rctx, channel.Id)
	if appErr != nil {
		rctx.Logger().Warn("Failed to get channel overrides", mlog.String("channel_id", channel.Id), mlog.Err(appErr))
		return has
	}
	if len(overrides) == 0 {
		return has
	}
	return model.ApplyChannelOverrides(has, permissionID, model.TeamRoleGroupIDs(teamRoles), userID, overrides)
}

// sessionTeamRoles gives what overrides need of someone in a team: their team roles, and whether they're an admin
// there (or of the system), whom overrides pass by.
func (a *App) sessionTeamRoles(session model.Session, teamID string) (string, bool) {
	admin := a.SessionHasPermissionToTeam(session, teamID, model.PermissionManageTeam)
	if member := session.GetTeamByTeamId(teamID); member != nil {
		return member.Roles, admin
	}
	return "", admin
}

func (a *App) userTeamRoles(rctx request.CTX, userID, teamID string) (string, bool) {
	admin := a.HasPermissionToTeam(rctx, userID, teamID, model.PermissionManageTeam)
	if member, err := a.Srv().Store().Team().GetMember(rctx, teamID, userID); err == nil {
		return member.Roles, admin
	}
	return "", admin
}

func (a *App) SessionHasPermissionToChannel(rctx request.CTX, session model.Session, channelID string, permission *model.Permission) (hasPermission bool, isMember bool) {
	has, isMember := a.sessionHasPermissionToChannelByRoles(rctx, session, channelID, permission)
	if session.IsUnrestricted() || !isOverridablePermission(permission.Id) {
		return has, isMember
	}
	channel, appErr := a.GetChannel(rctx, channelID)
	if appErr != nil {
		return has, isMember
	}
	roles, admin := a.sessionTeamRoles(session, channel.TeamId)
	return a.applyChannelOverrides(rctx, channel, session.UserId, roles, admin, permission.Id, has), isMember
}

func (a *App) SessionHasPermissionToChannels(rctx request.CTX, session model.Session, channelIDs []string, permission *model.Permission) bool {
	if !isOverridablePermission(permission.Id) {
		return a.sessionHasPermissionToChannelsByRoles(rctx, session, channelIDs, permission)
	}
	return !slices.ContainsFunc(channelIDs, func(id string) bool {
		has, _ := a.SessionHasPermissionToChannel(rctx, session, id, permission)
		return !has
	})
}

func (a *App) SessionHasPermissionToChannelByPost(session model.Session, postID string, permission *model.Permission) bool {
	has := a.sessionHasPermissionToChannelByPostByRoles(session, postID, permission)
	if session.IsUnrestricted() || !isOverridablePermission(permission.Id) {
		return has
	}
	channel, err := a.Srv().Store().Channel().GetForPost(postID)
	if err != nil {
		return has
	}
	roles, admin := a.sessionTeamRoles(session, channel.TeamId)
	return a.applyChannelOverrides(request.EmptyContext(a.Log()), channel, session.UserId, roles, admin, permission.Id, has)
}

func (a *App) HasPermissionToChannel(rctx request.CTX, askingUserId string, channelID string, permission *model.Permission) (hasPermission bool, isMember bool) {
	has, isMember := a.hasPermissionToChannelByRoles(rctx, askingUserId, channelID, permission)
	if askingUserId == "" || !isOverridablePermission(permission.Id) {
		return has, isMember
	}
	channel, appErr := a.GetChannel(rctx, channelID)
	if appErr != nil {
		return has, isMember
	}
	roles, admin := a.userTeamRoles(rctx, askingUserId, channel.TeamId)
	return a.applyChannelOverrides(rctx, channel, askingUserId, roles, admin, permission.Id, has), isMember
}

func (a *App) HasPermissionToChannelByPost(rctx request.CTX, askingUserId string, postID string, permission *model.Permission) bool {
	has := a.hasPermissionToChannelByPostByRoles(rctx, askingUserId, postID, permission)
	if !isOverridablePermission(permission.Id) {
		return has
	}
	channel, err := a.Srv().Store().Channel().GetForPost(postID)
	if err != nil {
		return has
	}
	roles, admin := a.userTeamRoles(rctx, askingUserId, channel.TeamId)
	return a.applyChannelOverrides(rctx, channel, askingUserId, roles, admin, permission.Id, has)
}

// Role-gated channels: "only these roles can see it" is a private channel constrained to the roles' groups, as
// Mattermost's group-synced channels. Its members are those of the roles, and follow them as people join and leave.

// GetChannelRoleAccess gives the roles a channel is for; none when it isn't role-gated.
func (a *App) GetChannelRoleAccess(channel *model.Channel) ([]string, *model.AppError) {
	groups, err := a.Srv().Store().Group().GetGroupsByChannel(channel.Id, model.GroupSearchOpts{})
	if err != nil {
		return nil, model.NewAppError("GetChannelRoleAccess", "app.channel_role_access.get.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	ids := []string{}
	for _, g := range groups {
		if g.IsRole() && g.TeamId == channel.TeamId {
			ids = append(ids, g.Id)
		}
	}
	return ids, nil
}

// SetChannelRoleAccess makes a private channel for the given roles of its team only, or for anyone invited again (no
// roles). People of the roles join it; others leave.
func (a *App) SetChannelRoleAccess(rctx request.CTX, channel *model.Channel, groupIDs []string) *model.AppError {
	if channel.Type != model.ChannelTypePrivate {
		return model.NewAppError("SetChannelRoleAccess", "app.channel_role_access.private_only.app_error", nil, "", http.StatusBadRequest)
	}
	slices.Sort(groupIDs)
	groupIDs = slices.Compact(groupIDs)
	roles := make([]*model.Group, 0, len(groupIDs))
	for _, id := range groupIDs {
		group, err := a.Srv().Store().Group().Get(id)
		if err != nil || !group.IsRole() || group.TeamId != channel.TeamId || group.DeleteAt != 0 {
			return model.NewAppError("SetChannelRoleAccess", "app.channel_override.role.app_error", nil, "", http.StatusBadRequest)
		}
		roles = append(roles, group)
	}

	current, appErr := a.GetChannelRoleAccess(channel)
	if appErr != nil {
		return appErr
	}
	for _, id := range current {
		if !slices.Contains(groupIDs, id) {
			if _, appErr := a.DeleteGroupSyncable(id, channel.Id, model.GroupSyncableTypeChannel); appErr != nil {
				return appErr
			}
		}
	}
	for _, id := range groupIDs {
		if !slices.Contains(current, id) {
			if _, appErr := a.UpsertGroupSyncable(model.NewGroupChannel(id, channel.Id, true)); appErr != nil {
				return appErr
			}
		}
	}

	constrained := len(groupIDs) > 0
	if channel.IsGroupConstrained() != constrained {
		channel.GroupConstrained = model.NewPointer(constrained)
		if _, appErr := a.UpdateChannel(rctx, channel); appErr != nil {
			return appErr
		}
	}

	for _, role := range roles {
		a.addRoleMembersToChannel(rctx, role, channel)
	}
	if constrained {
		if err := a.DeleteGroupConstrainedChannelMemberships(rctx, &channel.Id); err != nil {
			rctx.Logger().Warn("Failed to remove people outside the roles of a role-gated channel", mlog.String("channel_id", channel.Id), mlog.Err(err))
		}
	}
	return nil
}

func (a *App) addRoleMembersToChannel(rctx request.CTX, role *model.Group, channel *model.Channel, userIDs ...string) {
	if len(userIDs) == 0 {
		users, err := a.Srv().Store().Group().GetMemberUsers(role.Id)
		if err != nil {
			rctx.Logger().Warn("Failed to get the members of a role", mlog.String("group_id", role.Id), mlog.Err(err))
			return
		}
		for _, u := range users {
			userIDs = append(userIDs, u.Id)
		}
	}
	for _, userID := range userIDs {
		if _, appErr := a.AddChannelMember(rctx, userID, channel, ChannelMemberOpts{}); appErr != nil {
			rctx.Logger().Warn("Failed to add a role's member to a role-gated channel", mlog.String("channel_id", channel.Id), mlog.String("user_id", userID), mlog.Err(appErr))
		}
	}
}

// syncRoleGatedChannels brings the channels gated by a role in step with someone joining or leaving it.
func (a *App) syncRoleGatedChannels(rctx request.CTX, role *model.Group, userIDs []string, joined bool) {
	syncables, err := a.Srv().Store().Group().GetAllGroupSyncablesByGroupId(role.Id, model.GroupSyncableTypeChannel)
	if err != nil {
		return
	}
	for _, s := range syncables {
		channel, appErr := a.GetChannel(rctx, s.SyncableId)
		if appErr != nil || !channel.IsGroupConstrained() {
			continue
		}
		if joined {
			a.addRoleMembersToChannel(rctx, role, channel, userIDs...)
		} else if err := a.DeleteGroupConstrainedChannelMemberships(rctx, &channel.Id); err != nil {
			rctx.Logger().Warn("Failed to remove people who left a role from its channels", mlog.String("channel_id", channel.Id), mlog.Err(err))
		}
	}
}
