// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package app

import (
	"net/http"
	"sync"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/i18n"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

// Antimatter: a community's default role, as Discord's @everyone: a role everyone in the community has (but guests and
// bots, as for the channels everyone is in), below all its other roles, which can't be deleted nor be given or taken
// away. It's what members with no other role look like (its colour, its icon, its group in the member list) and may do
// (its permissions); the other roles add to it. It's made the first time the community's roles are managed
// (EnsureTeamDefaultRole), with nothing set, so a community that never uses roles never has one; the community keeps
// its id (Team.DefaultRoleId), which only the server sets.

// teamDefaultRoleFills are the communities whose default role is being given to everyone in the background.
var teamDefaultRoleFills sync.Map

// IsTeamDefaultRole tells whether a group is its community's default role.
func (a *App) IsTeamDefaultRole(group *model.Group) bool {
	if group == nil || !group.IsRole() {
		return false
	}
	team, appErr := a.GetTeam(group.TeamId)
	return appErr == nil && team.DefaultRoleId == group.Id
}

// teamDefaultRoleID is the id of a community's default role, "" without one.
func (a *App) teamDefaultRoleID(teamID string) string {
	team, appErr := a.GetTeam(teamID)
	if appErr != nil {
		return ""
	}
	return team.DefaultRoleId
}

// EnsureTeamDefaultRole gives a community's default role, made (and given to everyone in it, in the background) the
// first time. Two asking at once get the same role.
func (a *App) EnsureTeamDefaultRole(rctx request.CTX, teamID string) (*model.Group, *model.AppError) {
	team, appErr := a.GetTeam(teamID)
	if appErr != nil {
		return nil, appErr
	}
	if team.DefaultRoleId != "" {
		if group, err := a.Srv().Store().Group().Get(team.DefaultRoleId); err == nil && group.DeleteAt == 0 && group.TeamId == teamID {
			return group, nil
		}
	}

	// Its name is never mentioned (it isn't referenceable): it only has to be free.
	name := "default-" + teamID
	if team.DefaultRoleId != "" {
		name += "-" + model.NewId()[:6]
	}
	displayName := i18n.T("app.team_role.default_role.name")
	if displayName == "" || displayName == "app.team_role.default_role.name" {
		displayName = "Members"
	}
	group, appErr := a.CreateGroup(&model.Group{
		Name:        &name,
		DisplayName: displayName,
		Source:      model.GroupSourceCustom,
		TeamId:      teamID,
		Position:    model.GroupPositionMax,
	})
	if appErr != nil {
		// Made meanwhile by someone else, under the same name.
		if existing, err := a.Srv().Store().Group().GetByName(name, model.GroupSearchOpts{}); err == nil && existing.DeleteAt == 0 {
			group = existing
		} else {
			return nil, appErr
		}
	}

	row := &model.System{Name: model.TeamDefaultRoleSystemKeyPrefix + teamID, Value: group.Id}
	var err error
	if team.DefaultRoleId == "" {
		row, err = a.Srv().Store().System().InsertIfExists(row)
	} else {
		// The one it had is gone (deleted before deleting it was refused): this one replaces it.
		err = a.Srv().Store().System().SaveOrUpdate(row)
	}
	if err != nil {
		return nil, model.NewAppError("EnsureTeamDefaultRole", "app.team_role.default_role.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	if row.Value != group.Id {
		// Someone else's came first: theirs it is.
		if _, delErr := a.Srv().Store().Group().Delete(group.Id); delErr != nil {
			rctx.Logger().Warn("Failed to delete a second default role", mlog.String("group_id", group.Id), mlog.Err(delErr))
		}
		if group, err = a.Srv().Store().Group().Get(row.Value); err != nil {
			return nil, model.NewAppError("EnsureTeamDefaultRole", "app.team_role.default_role.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
		}
	}

	a.Srv().Store().Team().ClearCaches()
	if updated, appErr := a.GetTeam(teamID); appErr == nil {
		if appErr := a.sendTeamEvent(updated, model.WebsocketEventUpdateTeam); appErr != nil {
			rctx.Logger().Warn("Failed to tell of a community's default role", mlog.String("team_id", teamID), mlog.Err(appErr))
		}
	}
	a.fillTeamDefaultRoleInBackground(rctx, teamID, group)
	return group, nil
}

// fillTeamDefaultRoleInBackground gives a community's default role to everyone in it who should have it.
func (a *App) fillTeamDefaultRoleInBackground(rctx request.CTX, teamID string, group *model.Group) {
	if _, running := teamDefaultRoleFills.LoadOrStore(teamID, true); running {
		return
	}
	logger := rctx.Logger()
	a.Srv().Go(func() {
		defer teamDefaultRoleFills.Delete(teamID)
		a.fillTeamDefaultRole(request.EmptyContext(logger), teamID, group)
	})
}

// fillTeamDefaultRole gives a community's default role to everyone in it but guests and bots, in batches.
func (a *App) fillTeamDefaultRole(rctx request.CTX, teamID string, group *model.Group) {
	for page := 0; ; page++ {
		users, err := a.Srv().Store().User().GetProfiles(&model.UserGetOptions{InTeamId: teamID, Active: true, Page: page, PerPage: communityFillBatchSize})
		if err != nil {
			rctx.Logger().Warn("Failed to get the members of a community for its default role", mlog.String("team_id", teamID), mlog.Err(err))
			return
		}
		userIDs := make([]string, 0, len(users))
		for _, user := range users {
			if isCommunityWideUser(user) {
				userIDs = append(userIDs, user.Id)
			}
		}
		if len(userIDs) > 0 {
			if _, appErr := a.UpsertGroupMembers(group.Id, userIDs); appErr != nil {
				rctx.Logger().Warn("Failed to give a community's default role to its members", mlog.String("team_id", teamID), mlog.Err(appErr))
			}
		}
		if len(users) < communityFillBatchSize {
			return
		}
	}
}

// joinTeamDefaultRole gives someone joining a community its default role, when it has one.
func (a *App) joinTeamDefaultRole(rctx request.CTX, team *model.Team, user *model.User) {
	if team == nil || team.DefaultRoleId == "" || !isCommunityWideUser(user) {
		return
	}
	if _, appErr := a.UpsertGroupMembers(team.DefaultRoleId, []string{user.Id}); appErr != nil {
		rctx.Logger().Warn("Failed to give someone joining a community its default role", mlog.String("team_id", team.Id), mlog.String("user_id", user.Id), mlog.Err(appErr))
	}
}

// CheckTeamDefaultRoleChange refuses what can't be done to a community's default role through the API: deleting it,
// giving it or taking it away (everyone has it), opening it to all, or moving it from below the other roles.
// Nil for any other group.
func (a *App) CheckTeamDefaultRoleChange(group *model.Group, patch *model.GroupPatch, deleting, members bool) *model.AppError {
	if !a.IsTeamDefaultRole(group) {
		return nil
	}
	switch {
	case deleting:
		return model.NewAppError("CheckTeamDefaultRoleChange", "app.team_role.default_role.delete.app_error", nil, "", http.StatusBadRequest)
	case members:
		return model.NewAppError("CheckTeamDefaultRoleChange", "app.team_role.default_role.members.app_error", nil, "", http.StatusBadRequest)
	case patch != nil && ((patch.SelfAssignable != nil && *patch.SelfAssignable) || (patch.Position != nil && *patch.Position != group.Position) || patch.AllowReference != nil || patch.Name != nil):
		return model.NewAppError("CheckTeamDefaultRoleChange", "app.team_role.default_role.patch.app_error", nil, "", http.StatusBadRequest)
	}
	return nil
}
