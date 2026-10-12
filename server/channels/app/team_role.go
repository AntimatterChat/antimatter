// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"net/http"
	"slices"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

// Antimatter roles grant permissions through Mattermost's own roles: each role (a group of one team) has a Mattermost
// role named after it, which its members hold in that team as an explicit team role. Team and channel permission
// checks then see it as they see any team role.

// GetTeamRole gives the Mattermost role an Antimatter role grants, made the first time it's needed.
func (a *App) GetTeamRole(rctx request.CTX, group *model.Group) (*model.Role, *model.AppError) {
	name := model.TeamRoleName(group.Id)
	role, appErr := a.GetRoleByName(rctx, name)
	if appErr == nil {
		return role, nil
	}
	if appErr.StatusCode != http.StatusNotFound {
		return nil, appErr
	}
	displayName := group.DisplayName
	if len(displayName) > model.RoleDisplayNameMaxLength {
		displayName = displayName[:model.RoleDisplayNameMaxLength]
	}
	return a.CreateRole(&model.Role{
		Name:        name,
		DisplayName: displayName,
		Description: "Antimatter role of team " + group.TeamId,
		Permissions: []string{},
	})
}

// SetTeamRolePermissions sets what an Antimatter role grants. Only permissions a role may hold are accepted, and
// someone who isn't a team admin can only hand out permissions they have themselves in the team.
func (a *App) SetTeamRolePermissions(rctx request.CTX, session model.Session, group *model.Group, permissions []string) (*model.Role, *model.AppError) {
	if !group.IsRole() {
		return nil, model.NewAppError("SetTeamRolePermissions", "app.team_role.not_a_role.app_error", nil, "", http.StatusBadRequest)
	}
	if group.DeleteAt != 0 {
		return nil, model.NewAppError("SetTeamRolePermissions", "app.team_role.deleted.app_error", nil, "", http.StatusBadRequest)
	}
	slices.Sort(permissions)
	permissions = slices.Compact(permissions)
	if !model.AreTeamRolePermissions(permissions) {
		return nil, model.NewAppError("SetTeamRolePermissions", "app.team_role.permissions.app_error", nil, "", http.StatusBadRequest)
	}
	if !a.SessionHasPermissionToTeam(session, group.TeamId, model.PermissionManageTeam) {
		for _, p := range permissions {
			if !a.SessionHasPermissionToTeam(session, group.TeamId, &model.Permission{Id: p}) {
				return nil, model.NewAppError("SetTeamRolePermissions", "app.team_role.permission_not_held.app_error", map[string]any{"Permission": p}, "", http.StatusForbidden)
			}
		}
	}

	role, appErr := a.GetTeamRole(rctx, group)
	if appErr != nil {
		return nil, appErr
	}
	role, appErr = a.PatchRole(role, &model.RolePatch{Permissions: &permissions})
	if appErr != nil {
		return nil, appErr
	}

	// Its members hold it (again): roles made before they granted anything, or drifted, catch up here.
	a.syncTeamRoleForAllMembers(rctx, group, true)
	return role, nil
}

// CheckTeamRoleMembers refuses people who aren't in the team of a role or an Antimatter team (both are for their
// team's members), and anyone joining a deleted one.
func (a *App) CheckTeamRoleMembers(rctx request.CTX, group *model.Group, userIDs []string) *model.AppError {
	if !group.BelongsToTeam() {
		return nil
	}
	deletedErr, notInTeamErr := "app.team_role.deleted.app_error", "app.team_role.not_in_team.app_error"
	if group.IsTeam {
		deletedErr, notInTeamErr = "app.group_team.deleted.app_error", "app.group_team.not_in_team.app_error"
	}
	if group.DeleteAt != 0 && len(userIDs) > 0 {
		return model.NewAppError("CheckTeamRoleMembers", deletedErr, nil, "", http.StatusBadRequest)
	}
	for _, userID := range userIDs {
		member, err := a.Srv().Store().Team().GetMember(rctx, group.TeamId, userID)
		if err != nil || member.DeleteAt != 0 {
			return model.NewAppError("CheckTeamRoleMembers", notInTeamErr, nil, "user_id="+userID, http.StatusBadRequest)
		}
	}
	return nil
}

// syncTeamRole grants or takes back an Antimatter role's Mattermost role in its team, for the people given.
func (a *App) syncTeamRole(rctx request.CTX, group *model.Group, userIDs []string, grant bool) {
	if !group.IsRole() || len(userIDs) == 0 {
		return
	}
	role, appErr := a.GetTeamRole(rctx, group)
	if appErr != nil {
		rctx.Logger().Warn("Failed to get the role of a team role group", mlog.String("group_id", group.Id), mlog.Err(appErr))
		return
	}
	for _, userID := range userIDs {
		member, err := a.Srv().Store().Team().GetMember(rctx, group.TeamId, userID)
		if err != nil || member.DeleteAt != 0 {
			continue
		}
		roles := strings.Fields(member.Roles)
		has := slices.Contains(roles, role.Name)
		switch {
		case grant && !has:
			roles = append(roles, role.Name)
		case !grant && has:
			roles = slices.DeleteFunc(roles, func(r string) bool { return r == role.Name })
		default:
			continue
		}
		if _, appErr := a.UpdateTeamMemberRoles(rctx, group.TeamId, userID, strings.Join(roles, " ")); appErr != nil {
			rctx.Logger().Warn("Failed to update a member's team roles for a team role", mlog.String("group_id", group.Id), mlog.String("user_id", userID), mlog.Err(appErr))
		}
	}

	// The channels only the role sees follow too.
	a.syncRoleGatedChannels(rctx, group, userIDs, grant)
}

// syncTeamRoleForAllMembers grants or takes back a role from all its members, when it's restored or deleted.
func (a *App) syncTeamRoleForAllMembers(rctx request.CTX, group *model.Group, grant bool) {
	if !group.IsRole() {
		return
	}
	members, err := a.Srv().Store().Group().GetMemberUsers(group.Id)
	if err != nil {
		rctx.Logger().Warn("Failed to get the members of a team role group", mlog.String("group_id", group.Id), mlog.Err(err))
		return
	}
	userIDs := make([]string, 0, len(members))
	for _, u := range members {
		userIDs = append(userIDs, u.Id)
	}
	a.syncTeamRole(rctx, group, userIDs, grant)
}

// highestTeamRolePosition is the place of someone's highest role in a team (0 = highest); -1 without any. Roles anyone
// can take don't rank: taking one mustn't lift someone above the roles they manage.
func (a *App) highestTeamRolePosition(userID, teamID string) int {
	groups, err := a.Srv().Store().Group().GetGroups(0, 1000, model.GroupSearchOpts{FilterHasMember: userID, Source: model.GroupSourceCustom}, nil)
	if err != nil {
		return -1
	}
	highest := -1
	for _, g := range groups {
		if g.TeamId == teamID && g.IsRole() && g.DeleteAt == 0 && !g.SelfAssignable && (highest == -1 || g.Position < highest) {
			highest = g.Position
		}
	}
	return highest
}

// LowestTeamRolePositionFor is the highest place someone who isn't a team admin may give a role: just below their own
// highest role, or below all the team's roles when they have none. -1 means anywhere (team admins).
func (a *App) LowestTeamRolePositionFor(session model.Session, teamID string) int {
	if a.SessionHasPermissionToTeam(session, teamID, model.PermissionManageTeam) {
		return -1
	}
	if highest := a.highestTeamRolePosition(session.UserId, teamID); highest != -1 {
		return highest + 1
	}
	return a.nextTeamRolePosition(teamID)
}

// nextTeamRolePosition is the place below all of a team's roles.
func (a *App) nextTeamRolePosition(teamID string) int {
	groups, err := a.Srv().Store().Group().GetGroups(0, 1000, model.GroupSearchOpts{Source: model.GroupSourceCustom}, nil)
	if err != nil {
		return model.GroupPositionMax
	}
	next := 0
	for _, g := range groups {
		if g.TeamId == teamID && g.IsRole() && g.DeleteAt == 0 && g.Position >= next {
			next = min(g.Position+1, model.GroupPositionMax)
		}
	}
	return next
}

// sessionCanManageTeamRole tells whether someone may manage an Antimatter role: its team's admins, or those who may
// manage the team's roles, for roles below their own highest one (Discord's hierarchy). An Antimatter team of the
// team has no place among its roles: those who may manage the roles manage all its teams.
func (a *App) sessionCanManageTeamRole(session model.Session, group *model.Group) bool {
	if a.SessionHasPermissionToTeam(session, group.TeamId, model.PermissionManageTeam) {
		return true
	}
	if !a.SessionHasPermissionToTeam(session, group.TeamId, model.PermissionManageTeamRoles) {
		return false
	}
	if group.IsTeam {
		return true
	}
	highest := a.highestTeamRolePosition(session.UserId, group.TeamId)
	return highest != -1 && group.Position > highest
}

// SessionCanSelfAssignRole tells whether someone is taking or leaving, for themselves only, a role open to all of its
// team, as a member of that team.
func (a *App) SessionCanSelfAssignRole(rctx request.CTX, session model.Session, group *model.Group, userIDs []string) bool {
	if !group.IsRole() || !group.SelfAssignable || group.DeleteAt != 0 || len(userIDs) != 1 || userIDs[0] != session.UserId {
		return false
	}
	member, err := a.Srv().Store().Team().GetMember(rctx, group.TeamId, session.UserId)
	return err == nil && member.DeleteAt == 0
}

// SessionHasSchemePermissionToTeam is SessionHasPermissionToTeam without what Antimatter roles grant. Mattermost's
// own endpoints setting team members' roles use it: a role handing out "manage team roles" is about Antimatter roles,
// under their hierarchy, and mustn't let its holders make themselves team admins.
func (a *App) SessionHasSchemePermissionToTeam(session model.Session, teamID string, permission *model.Permission) bool {
	if teamID == "" {
		return false
	}
	if session.IsUnrestricted() {
		return true
	}
	if member := session.GetTeamByTeamId(teamID); member != nil {
		roles := slices.DeleteFunc(member.GetRoles(), model.IsTeamRoleName)
		if a.RolesGrantPermission(roles, permission.Id) {
			return true
		}
	}
	return a.RolesGrantPermission(session.GetUserRoles(), permission.Id)
}

// KeepTeamRoles gives the team roles to set on someone through Mattermost's own endpoint: those asked for, but the
// Antimatter roles they hold as they are. Those come and go with the roles' membership only.
func (a *App) KeepTeamRoles(rctx request.CTX, teamID, userID, roles string) (string, *model.AppError) {
	member, err := a.Srv().Store().Team().GetMember(rctx, teamID, userID)
	if err != nil {
		return "", model.NewAppError("KeepTeamRoles", "app.team.get_member.missing.app_error", nil, "", http.StatusNotFound).Wrap(err)
	}
	kept := slices.DeleteFunc(strings.Fields(roles), model.IsTeamRoleName)
	for _, r := range strings.Fields(member.ExplicitRoles) {
		if model.IsTeamRoleName(r) {
			kept = append(kept, r)
		}
	}
	return strings.Join(kept, " "), nil
}

// LeaveTeamRoles takes someone out of a team's roles and Antimatter teams when they leave the team: coming back doesn't
// bring them back.
func (a *App) LeaveTeamRoles(rctx request.CTX, teamID, userID string) {
	groups, err := a.Srv().Store().Group().GetGroups(0, 1000, model.GroupSearchOpts{FilterHasMember: userID, Source: model.GroupSourceCustom}, nil)
	if err != nil {
		rctx.Logger().Warn("Failed to get the roles of someone leaving a team", mlog.String("team_id", teamID), mlog.String("user_id", userID), mlog.Err(err))
		return
	}
	for _, g := range groups {
		if g.TeamId != teamID {
			continue
		}
		if _, appErr := a.DeleteGroupMember(g.Id, userID); appErr != nil {
			rctx.Logger().Warn("Failed to take someone leaving a team out of its role", mlog.String("group_id", g.Id), mlog.String("user_id", userID), mlog.Err(appErr))
		}
	}
}
