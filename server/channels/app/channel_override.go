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

// channelOverridesTTL bounds how long a node trusts its cached overrides, should a change made through another node
// not reach it (clusterEventInvalidateChannelOverrides).
const channelOverridesTTL = time.Minute

// clusterEventInvalidateChannelOverrides tells the other nodes a channel's overrides changed; its data is the channel.
const clusterEventInvalidateChannelOverrides = model.ClusterEvent("inv_channel_overrides")

type cachedChannelOverrides struct {
	overrides []*model.ChannelOverride
	loadedAt  time.Time
}

type cachedCategoryOverrides struct {
	overrides []*model.CategoryOverride
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

// GetChannelOverrides gives a channel's own overrides, as set on it.
func (a *App) GetChannelOverrides(rctx request.CTX, channelID string) ([]*model.ChannelOverride, *model.AppError) {
	overrides, err := a.Srv().Store().ChannelOverride().GetForChannel(rctx, channelID)
	if err != nil {
		return nil, model.NewAppError("GetChannelOverrides", "app.channel_override.get.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	return overrides, nil
}

// GetEffectiveChannelOverrides gives the overrides that apply in a channel: those of its server category, with the
// channel's own on top (model.EffectiveChannelOverrides). From the cache when fresh.
func (a *App) GetEffectiveChannelOverrides(rctx request.CTX, channel *model.Channel) ([]*model.ChannelOverride, *model.AppError) {
	if cached, ok := a.Channels().channelOverrides.Load(channel.Id); ok {
		entry := cached.(cachedChannelOverrides)
		if time.Since(entry.loadedAt) < channelOverridesTTL {
			return entry.overrides, nil
		}
	}
	own, appErr := a.GetChannelOverrides(rctx, channel.Id)
	if appErr != nil {
		return nil, appErr
	}
	var inherited []*model.CategoryOverride
	if a.Config().FeatureFlags.ManagedChannelCategories && channel.TeamId != "" {
		name, appErr := a.GetChannelServerCategory(rctx, channel.Id)
		if appErr != nil {
			return nil, appErr
		}
		if name != "" {
			all, appErr := a.GetCategoryOverrides(rctx, channel.TeamId)
			if appErr != nil {
				return nil, appErr
			}
			key := model.CategoryOverrideKey(name)
			for _, o := range all {
				if o.Category == key {
					inherited = append(inherited, o)
				}
			}
		}
	}
	overrides := model.EffectiveChannelOverrides(channel.Id, inherited, own)
	a.Channels().channelOverrides.Store(channel.Id, cachedChannelOverrides{overrides: overrides, loadedAt: time.Now()})
	return overrides, nil
}

// GetCategoryOverrides gives the overrides of a team's server categories, from the cache when fresh.
func (a *App) GetCategoryOverrides(rctx request.CTX, teamID string) ([]*model.CategoryOverride, *model.AppError) {
	if cached, ok := a.Channels().categoryOverrides.Load(teamID); ok {
		entry := cached.(cachedCategoryOverrides)
		if time.Since(entry.loadedAt) < channelOverridesTTL {
			return entry.overrides, nil
		}
	}
	overrides, err := a.Srv().Store().ChannelOverride().GetForCategories(rctx, teamID)
	if err != nil {
		return nil, model.NewAppError("GetCategoryOverrides", "app.channel_override.get.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	a.Channels().categoryOverrides.Store(teamID, cachedCategoryOverrides{overrides: overrides, loadedAt: time.Now()})
	return overrides, nil
}

// SaveCategoryOverride sets the override of a role or person on one of a team's server categories; an empty one is
// removed. The channels filed in the category inherit it.
func (a *App) SaveCategoryOverride(rctx request.CTX, teamID string, override *model.CategoryOverride) (*model.CategoryOverride, *model.AppError) {
	categories, appErr := a.GetServerCategories(rctx, teamID)
	if appErr != nil {
		return nil, appErr
	}
	category := model.FindServerCategory(categories, override.Category)
	if category == nil {
		return nil, model.NewAppError("SaveCategoryOverride", "app.category_override.category.app_error", nil, "", http.StatusBadRequest)
	}
	override.TeamId = teamID
	override.Category = model.CategoryOverrideKey(category.Name)
	if appErr := override.IsValid(); appErr != nil {
		return nil, appErr
	}
	if appErr := a.checkOverrideTarget(rctx, teamID, override.TargetType, override.TargetId, override.IsEmpty()); appErr != nil {
		return nil, appErr
	}

	var err error
	if override.IsEmpty() {
		err = a.Srv().Store().ChannelOverride().DeleteForCategory(rctx, teamID, override.Category, override.TargetType, override.TargetId)
	} else {
		override, err = a.Srv().Store().ChannelOverride().SaveForCategory(rctx, override)
	}
	if err != nil {
		return nil, model.NewAppError("SaveCategoryOverride", "app.channel_override.save.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	a.invalidateAllChannelOverrides()
	a.publishCategoryOverridesUpdated(teamID)
	return override, nil
}

// renameCategoryOverrides follows a team's server categories being renamed and removed: the overrides go along with
// a renamed category, and with a removed one.
func (a *App) renameCategoryOverrides(rctx request.CTX, teamID string, moves map[string]string) {
	changed := false
	for from, to := range moves {
		var err error
		if to == "" {
			err = a.Srv().Store().ChannelOverride().DeleteCategory(rctx, teamID, from)
		} else {
			err = a.Srv().Store().ChannelOverride().RenameCategory(rctx, teamID, from, model.CategoryOverrideKey(to))
		}
		if err != nil {
			rctx.Logger().Warn("Failed to move the overrides of a server category", mlog.String("team_id", teamID), mlog.String("from", from), mlog.String("to", to), mlog.Err(err))
		}
		changed = changed || from != model.CategoryOverrideKey(to)
	}
	if changed {
		a.invalidateAllChannelOverrides()
		a.publishCategoryOverridesUpdated(teamID)
	}
}

// checkOverrideTarget accepts the role or person an override is for: a live role of the team, or one of its people.
// A person who left can still have their override removed.
func (a *App) checkOverrideTarget(rctx request.CTX, teamID, targetType, targetID string, removing bool) *model.AppError {
	switch targetType {
	case model.ChannelOverrideTargetRole:
		group, err := a.Srv().Store().Group().Get(targetID)
		if err != nil || !group.IsRole() || group.TeamId != teamID || (group.DeleteAt != 0 && !removing) {
			return model.NewAppError("checkOverrideTarget", "app.channel_override.role.app_error", nil, "", http.StatusBadRequest)
		}
	case model.ChannelOverrideTargetUser:
		member, err := a.Srv().Store().Team().GetMember(rctx, teamID, targetID)
		if (err != nil || member.DeleteAt != 0) && !removing {
			return model.NewAppError("checkOverrideTarget", "app.channel_override.user.app_error", nil, "", http.StatusBadRequest)
		}
	}
	return nil
}

func (a *App) publishCategoryOverridesUpdated(teamID string) {
	message := model.NewWebSocketEvent(model.WebsocketEventCategoryOverridesUpdated, teamID, "", "", nil, "")
	message.Add("team_id", teamID)
	a.Publish(message)
}

// SaveChannelOverride sets the override of a role or person in a channel; an empty one is removed. A role must be
// one of the channel's team.
func (a *App) SaveChannelOverride(rctx request.CTX, channel *model.Channel, override *model.ChannelOverride) (*model.ChannelOverride, *model.AppError) {
	override.ChannelId = channel.Id
	if appErr := override.IsValid(); appErr != nil {
		return nil, appErr
	}
	// A role or person of the channel's team: overrides don't reach outside it.
	if appErr := a.checkOverrideTarget(rctx, channel.TeamId, override.TargetType, override.TargetId, override.IsEmpty()); appErr != nil {
		return nil, appErr
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
	a.invalidateChannelOverrides(rctx, channel.Id)
	a.publishChannelOverridesUpdated(channel)
	return override, nil
}

// invalidateChannelOverrides forgets a channel's cached overrides here and on the other nodes, and reloads them from
// the master database, which a replica may lag behind.
func (a *App) invalidateChannelOverrides(rctx request.CTX, channelID string) {
	a.Channels().channelOverrides.Delete(channelID)
	if channel, appErr := a.GetChannel(rctx, channelID); appErr == nil {
		_, _ = a.GetEffectiveChannelOverrides(RequestContextWithMaster(rctx), channel)
	}
	a.sendChannelOverridesInvalidation(channelID)
}

// invalidateAllChannelOverrides forgets every cached override here and on the other nodes, as a category's change
// reaches all its channels.
func (a *App) invalidateAllChannelOverrides() {
	a.Channels().forgetAllOverrides()
	a.sendChannelOverridesInvalidation("")
}

func (a *App) sendChannelOverridesInvalidation(channelID string) {
	if cluster := a.Srv().Platform().Cluster(); cluster != nil {
		cluster.SendClusterMessage(&model.ClusterMessage{
			Event:    clusterEventInvalidateChannelOverrides,
			SendType: model.ClusterSendReliable,
			Data:     []byte(channelID),
		})
	}
}

func (ch *Channels) forgetAllOverrides() {
	ch.channelOverrides.Clear()
	ch.categoryOverrides.Clear()
}

// clusterInvalidateChannelOverridesHandler forgets the overrides another node changed: a channel's, or all of them.
func (ch *Channels) clusterInvalidateChannelOverridesHandler(msg *model.ClusterMessage) {
	if len(msg.Data) == 0 {
		ch.forgetAllOverrides()
		return
	}
	ch.channelOverrides.Delete(string(msg.Data))
}

// ChangedOverridePermissions gives the permissions an override changes from the one in place: those it allows or denies
// anew, and those it stops allowing or denying.
func (a *App) ChangedOverridePermissions(rctx request.CTX, channelID string, override *model.ChannelOverride) ([]string, *model.AppError) {
	overrides, appErr := a.GetChannelOverrides(rctx, channelID)
	if appErr != nil {
		return nil, appErr
	}
	var current model.ChannelOverride
	for _, o := range overrides {
		if o.TargetType == override.TargetType && o.TargetId == override.TargetId {
			current = *o
		}
	}
	changed := []string{}
	for _, pair := range [][2][]string{{current.Allow, override.Allow}, {current.Deny, override.Deny}} {
		for _, p := range pair[0] {
			if !slices.Contains(pair[1], p) {
				changed = append(changed, p)
			}
		}
		for _, p := range pair[1] {
			if !slices.Contains(pair[0], p) {
				changed = append(changed, p)
			}
		}
	}
	slices.Sort(changed)
	return slices.Compact(changed), nil
}

func (a *App) publishChannelOverridesUpdated(channel *model.Channel) {
	message := model.NewWebSocketEvent(model.WebsocketEventChannelOverridesUpdated, "", channel.Id, "", nil, "")
	message.Add("channel_id", channel.Id)
	a.Publish(message)
}

// overridesFor gives the overrides that may change a permission check in a channel; none for permissions overrides
// don't touch, channels without overrides, and direct and group messages.
func (a *App) overridesFor(rctx request.CTX, channel *model.Channel, permissionID string) []*model.ChannelOverride {
	if channel == nil || channel.TeamId == "" || !isOverridablePermission(permissionID) {
		return nil
	}
	overrides, appErr := a.GetEffectiveChannelOverrides(rctx, channel)
	if appErr != nil {
		rctx.Logger().Warn("Failed to get channel overrides", mlog.String("channel_id", channel.Id), mlog.Err(appErr))
		return nil
	}
	return overrides
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

// The permission checks below answer from roles as Mattermost does, then let the channel's overrides change that
// answer. Someone's team roles are only looked up when the channel has overrides.

func (a *App) SessionHasPermissionToChannel(rctx request.CTX, session model.Session, channelID string, permission *model.Permission) (hasPermission bool, isMember bool) {
	has, isMember := a.sessionHasPermissionToChannelByRoles(rctx, session, channelID, permission)
	if session.IsUnrestricted() || !isOverridablePermission(permission.Id) {
		return has, isMember
	}
	channel, appErr := a.GetChannel(rctx, channelID)
	if appErr != nil {
		return has, isMember
	}
	overrides := a.overridesFor(rctx, channel, permission.Id)
	if len(overrides) == 0 {
		return has, isMember
	}
	roles, admin := a.sessionTeamRoles(session, channel.TeamId)
	if admin {
		return has, isMember
	}
	return model.ApplyChannelOverrides(has, permission.Id, model.TeamRoleGroupIDs(roles), session.UserId, overrides), isMember
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
	overrides := a.overridesFor(request.EmptyContext(a.Log()), channel, permission.Id)
	if len(overrides) == 0 {
		return has
	}
	roles, admin := a.sessionTeamRoles(session, channel.TeamId)
	if admin {
		return has
	}
	return model.ApplyChannelOverrides(has, permission.Id, model.TeamRoleGroupIDs(roles), session.UserId, overrides)
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
	overrides := a.overridesFor(rctx, channel, permission.Id)
	if len(overrides) == 0 {
		return has, isMember
	}
	roles, admin := a.userTeamRoles(rctx, askingUserId, channel.TeamId)
	if admin {
		return has, isMember
	}
	return model.ApplyChannelOverrides(has, permission.Id, model.TeamRoleGroupIDs(roles), askingUserId, overrides), isMember
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
	overrides := a.overridesFor(rctx, channel, permission.Id)
	if len(overrides) == 0 {
		return has
	}
	roles, admin := a.userTeamRoles(rctx, askingUserId, channel.TeamId)
	if admin {
		return has
	}
	return model.ApplyChannelOverrides(has, permission.Id, model.TeamRoleGroupIDs(roles), askingUserId, overrides)
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

// releaseRoleGatedChannels leaves the channels a role gates to their other roles, or to anyone invited again when it
// was their last, before the role is deleted: deleting a role doesn't empty its channels.
func (a *App) releaseRoleGatedChannels(rctx request.CTX, role *model.Group) {
	syncables, err := a.Srv().Store().Group().GetAllGroupSyncablesByGroupId(role.Id, model.GroupSyncableTypeChannel)
	if err != nil {
		rctx.Logger().Warn("Failed to get the channels of a role", mlog.String("group_id", role.Id), mlog.Err(err))
		return
	}
	for _, s := range syncables {
		channel, appErr := a.GetChannel(rctx, s.SyncableId)
		if appErr != nil {
			continue
		}
		current, appErr := a.GetChannelRoleAccess(channel)
		if appErr != nil {
			continue
		}
		remaining := slices.DeleteFunc(current, func(id string) bool { return id == role.Id })
		if appErr := a.SetChannelRoleAccess(rctx, channel, remaining); appErr != nil {
			rctx.Logger().Warn("Failed to release a channel of a deleted role", mlog.String("channel_id", channel.Id), mlog.String("group_id", role.Id), mlog.Err(appErr))
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
