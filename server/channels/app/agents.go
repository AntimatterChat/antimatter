// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"net/http"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

// Antimatter: GET /api/v4/agents, /agents/status and /llmservices keep their shapes for the classic web app, the
// mobile apps and older clients, now served from the server's agents and model connections rather than the AI chat
// app's bridge. The end-to-end tests' stand-in model may set what they answer (ServiceSettings.EnableTesting only).

// GetAgentsStatus tells whether a person has an agent to talk to, with the reason (a translation ID) when not.
func (a *App) GetAgentsStatus(rctx request.CTX, userID string) (bool, string) {
	if m := a.aiE2EModel(); m != nil {
		if available, reason, ok := m.statusOverride(); ok {
			return available, reason
		}
	}
	agents, _, appErr := a.GetAIAgentsForUser(rctx, userID)
	if appErr != nil || len(agents) == 0 {
		// The reason is a translation ID, which the clients show.
		return false, model.NewAppError("GetAgentsStatus", "app.agents.status.no_agents", nil, "", http.StatusOK).Id
	}
	return true, ""
}

// GetAgents lists the built-in agents a person may talk to, as the AI chat app's bridge listed its bots: the
// connection they use as their service, never anything about its key.
func (a *App) GetAgents(rctx request.CTX, userID string) ([]model.BridgeAgentInfo, *model.AppError) {
	if m := a.aiE2EModel(); m != nil {
		if agents, ok := m.agentsOverride(); ok {
			return agents, nil
		}
	}
	agents, users, appErr := a.GetAIAgentsForUser(rctx, userID)
	if appErr != nil {
		return nil, appErr
	}
	infos := make([]model.BridgeAgentInfo, 0, len(agents))
	for i, agent := range agents {
		info := model.BridgeAgentInfo{
			ID:          agent.UserId,
			DisplayName: users[i].GetDisplayName(model.ShowFullName),
			Username:    users[i].Username,
			ServiceID:   agent.ConnectionId,
			IsDefault:   i == 0,
		}
		if c, err := a.Srv().Store().AIConnection().Get(rctx, agent.ConnectionId); err == nil {
			info.ServiceType = c.Provider
		}
		infos = append(infos, info)
	}
	return infos, nil
}

// GetUsersForAgents gives the accounts of the agents a person may talk to (the users autocomplete's agents).
func (a *App) GetUsersForAgents(rctx request.CTX, userID string) ([]*model.User, *model.AppError) {
	if m := a.aiE2EModel(); m != nil {
		if agents, ok := m.agentsOverride(); ok {
			users := make([]*model.User, 0, len(agents))
			for _, agent := range agents {
				if user, err := a.Srv().Store().User().GetByUsername(agent.Username); err == nil {
					users = append(users, user)
				}
			}
			return users, nil
		}
	}
	_, users, appErr := a.GetAIAgentsForUser(rctx, userID)
	if appErr != nil {
		return nil, appErr
	}
	return users, nil
}

// GetLLMServices lists the model connections a person may use, as the bridge listed its services.
func (a *App) GetLLMServices(rctx request.CTX, userID string) ([]model.BridgeServiceInfo, *model.AppError) {
	if m := a.aiE2EModel(); m != nil {
		if services, ok := m.servicesOverride(); ok {
			return services, nil
		}
	}
	connections, appErr := a.GetAIConnectionsForUser(rctx, userID, "", "")
	if appErr != nil {
		return nil, appErr
	}
	services := make([]model.BridgeServiceInfo, 0, len(connections))
	for _, c := range connections {
		services = append(services, model.BridgeServiceInfo{ID: c.Id, Name: c.Name, Type: c.Provider})
	}
	return services, nil
}
