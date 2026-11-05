// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package app

import (
	"net/http"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

// What a community's permanent deletion (PermanentDeleteTeam) removes of what Antimatter keeps for it, besides its
// channels and its members:
//   - its server categories, timeouts, bans, invite links and category overrides (DeleteServerCategoriesForTeam);
//   - its rules, with who agreed to them (deleteCommunityRulesForTeam);
//   - its agents, MCP servers (with everyone's sign-ins) and model connections (with their usage), and with them
//     every secret they keep (deleteCommunityAIForTeam);
//   - its roles and teams, with their members (deleteCommunityGroupsForTeam);
//   - its attributes and their values (deleteCommunityAttributesForTeam);
//   - its settings, in the Systems table, and its own columns, with the team row (Team().PermanentDelete).

// deleteCommunityAIForTeam deletes a community's agents, MCP servers and model connections, deleted ones too, with
// their sealed secrets, everyone's sign-ins to its MCP servers and the connections' usage. The agents' accounts are
// deactivated, as when an agent is deleted: their messages elsewhere (direct messages) stay, shown as an agent's.
func (a *App) deleteCommunityAIForTeam(rctx request.CTX, teamID string) *model.AppError {
	failed := func(err error) *model.AppError {
		return model.NewAppError("deleteCommunityAIForTeam", "app.team.permanent_delete.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	scopes := []string{teamID}

	agents, err := a.Srv().Store().AIAgent().GetForScopes(rctx, scopes)
	if err != nil {
		return failed(err)
	}
	for _, agent := range agents {
		if _, appErr := a.UpdateBotActive(rctx, agent.UserId, false); appErr != nil {
			rctx.Logger().Warn("Couldn't deactivate the account of a deleted community's agent", mlog.String("user_id", agent.UserId), mlog.String("team_id", teamID), mlog.Err(appErr))
		}
		if err = a.Srv().Store().AgentActivity().Delete(rctx, agent.UserId); err != nil {
			rctx.Logger().Warn("Couldn't forget a deleted community's agent's activity", mlog.String("user_id", agent.UserId), mlog.String("team_id", teamID), mlog.Err(err))
		}
		a.setAIAgentStatus(agent.UserId, model.StatusOffline)
	}
	if err = a.Srv().Store().AIAgent().PermanentDeleteForScope(rctx, teamID); err != nil {
		return failed(err)
	}
	a.invalidateAIAgents()

	servers, err := a.Srv().Store().AIMCPServer().GetForScopes(rctx, scopes)
	if err != nil {
		return failed(err)
	}
	if err := a.Srv().Store().AIMCPServer().PermanentDeleteForScope(rctx, teamID); err != nil {
		return failed(err)
	}
	for _, server := range servers {
		a.forgetAIMCPTools(server.Id, "")
	}
	if len(servers) > 0 {
		a.publishAIToolsChanged("")
	}

	// Its agents being gone, nothing else uses its connections: a community's are for its own features only.
	if err := a.Srv().Store().AIConnection().PermanentDeleteForScope(rctx, teamID); err != nil {
		return failed(err)
	}
	return nil
}

// deleteCommunityGroupsForTeam deletes a community's roles and teams, archived ones too, with their members.
func (a *App) deleteCommunityGroupsForTeam(teamID string) *model.AppError {
	if err := a.Srv().Store().Group().PermanentDeleteForTeam(teamID); err != nil {
		return model.NewAppError("deleteCommunityGroupsForTeam", "app.team.permanent_delete.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	return nil
}

// deleteCommunityAttributesForTeam deletes a community's attributes and their values, as the property fields and
// values are deleted everywhere (marked deleted). It doesn't stop the community's deletion: what fails is logged.
func (a *App) deleteCommunityAttributesForTeam(rctx request.CTX, teamID string) {
	groupID, appErr := a.communityAttributesGroupID(rctx)
	if appErr != nil {
		rctx.Logger().Warn("Couldn't delete a deleted community's attributes", mlog.String("team_id", teamID), mlog.Err(appErr))
		return
	}
	fields, appErr := a.GetCommunityAttributes(rctx, teamID)
	if appErr != nil {
		rctx.Logger().Warn("Couldn't delete a deleted community's attributes", mlog.String("team_id", teamID), mlog.Err(appErr))
		return
	}
	for _, field := range fields {
		if appErr := a.DeletePropertyValuesForField(rctx, groupID, field.ID); appErr != nil {
			rctx.Logger().Warn("Couldn't delete the values of a deleted community's attribute", mlog.String("team_id", teamID), mlog.String("field_id", field.ID), mlog.Err(appErr))
			continue
		}
		if appErr := a.DeletePropertyField(rctx, groupID, field.ID, true, ""); appErr != nil {
			rctx.Logger().Warn("Couldn't delete a deleted community's attribute", mlog.String("team_id", teamID), mlog.String("field_id", field.ID), mlog.Err(appErr))
		}
	}
}
