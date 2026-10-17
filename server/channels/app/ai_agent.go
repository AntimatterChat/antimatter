// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package app

import (
	"errors"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

// Agents (Antimatter): bot accounts of kind "agent", with their settings in the AIAgents table. System admins manage
// the server-wide agents; a community's admins its own. On top of the scope, an agent someone made needs
// manage_own_agent, and someone else's manage_others_agent (community admins hold both in their community).

// clusterEventInvalidateAIAgents tells the other nodes the agents changed.
const clusterEventInvalidateAIAgents = model.ClusterEvent("inv_ai_agents")

// agentDirectoryMaxAge bounds how long a node trusts its list of agents without hearing of a change: a safety net
// for a missed cluster message.
const agentDirectoryMaxAge = 5 * time.Minute

// agentDirectory is a node's list of agents, for the runtime to tell quickly whether a post is for one.
type agentDirectory struct {
	mu         sync.RWMutex
	loadedAt   time.Time
	byUserID   map[string]*model.AIAgent
	byUsername map[string]string
}

// aiAgentDirectory gives the node's agents, loading them when needed.
func (a *App) aiAgentDirectory(rctx request.CTX) *agentDirectory {
	d := a.Srv().ai.agentDirectory()
	d.mu.RLock()
	fresh := d.byUserID != nil && time.Since(d.loadedAt) < agentDirectoryMaxAge
	d.mu.RUnlock()
	if fresh {
		return d
	}
	agents, err := a.Srv().Store().AIAgent().GetAll(rctx)
	if err != nil {
		rctx.Logger().Warn("Couldn't load the agents", mlog.Err(err))
		return d
	}
	byUserID := make(map[string]*model.AIAgent, len(agents))
	byUsername := make(map[string]string, len(agents))
	ids := make([]string, 0, len(agents))
	for _, agent := range agents {
		byUserID[agent.UserId] = agent
		ids = append(ids, agent.UserId)
	}
	if len(ids) > 0 {
		users, appErr := a.GetUsers(rctx, ids)
		if appErr != nil {
			rctx.Logger().Warn("Couldn't load the agents' accounts", mlog.Err(appErr))
			return d
		}
		for _, u := range users {
			byUsername[strings.ToLower(u.Username)] = u.Id
		}
	}
	d.mu.Lock()
	d.byUserID, d.byUsername, d.loadedAt = byUserID, byUsername, time.Now()
	d.mu.Unlock()
	return d
}

func (d *agentDirectory) get(userID string) *model.AIAgent {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if agent := d.byUserID[userID]; agent != nil {
		return agent.Clone()
	}
	return nil
}

func (d *agentDirectory) byName(username string) *model.AIAgent {
	d.mu.RLock()
	id := d.byUsername[strings.ToLower(username)]
	d.mu.RUnlock()
	if id == "" {
		return nil
	}
	return d.get(id)
}

// all gives the agents, in no particular order.
func (d *agentDirectory) all() []*model.AIAgent {
	d.mu.RLock()
	defer d.mu.RUnlock()
	out := make([]*model.AIAgent, 0, len(d.byUserID))
	for _, agent := range d.byUserID {
		out = append(out, agent.Clone())
	}
	slices.SortFunc(out, func(x, y *model.AIAgent) int { return strings.Compare(x.UserId, y.UserId) })
	return out
}

func (d *agentDirectory) empty() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.byUserID) == 0
}

func (d *agentDirectory) forget() {
	d.mu.Lock()
	d.byUserID, d.byUsername = nil, nil
	d.mu.Unlock()
}

// invalidateAIAgents makes this node and the others reload the agents.
func (a *App) invalidateAIAgents() {
	a.Srv().ai.agentDirectory().forget()
	if cluster := a.Srv().Platform().Cluster(); cluster != nil {
		cluster.SendClusterMessage(&model.ClusterMessage{Event: clusterEventInvalidateAIAgents, SendType: model.ClusterSendReliable})
	}
}

func (s *Server) clusterInvalidateAIAgentsHandler(_ *model.ClusterMessage) {
	s.ai.agentDirectory().forget()
}

// SessionCanManageAIAgentScope tells whether a session may manage agents of a scope at all: system admins the
// server's, a community's admins their community's.
func (a *App) SessionCanManageAIAgentScope(session model.Session, scope string) bool {
	if scope == model.AIAgentScopeServer {
		return a.SessionHasPermissionTo(session, model.PermissionManageSystem)
	}
	if !model.IsValidId(scope) {
		return false
	}
	return a.SessionHasPermissionToTeam(session, scope, model.PermissionManageTeam)
}

// sessionHasAIAgentPermission checks an agent permission where the agent lives: the system roles for a server
// agent, the community's roles (and the system's) for a community's.
func (a *App) sessionHasAIAgentPermission(session model.Session, scope string, permission *model.Permission) bool {
	if scope == model.AIAgentScopeServer {
		return a.SessionHasPermissionTo(session, permission)
	}
	return a.SessionHasPermissionToTeam(session, scope, permission)
}

// SessionCanCreateAIAgent tells whether a session may make an agent in a scope.
func (a *App) SessionCanCreateAIAgent(session model.Session, scope string) bool {
	return a.SessionCanManageAIAgentScope(session, scope) && a.sessionHasAIAgentPermission(session, scope, model.PermissionManageOwnAgent)
}

// SessionCanManageAIAgent tells whether a session may change, enable, disable or delete an agent.
func (a *App) SessionCanManageAIAgent(session model.Session, agent *model.AIAgent) bool {
	if !a.SessionCanManageAIAgentScope(session, agent.Scope) {
		return false
	}
	permission := model.PermissionManageOthersAgent
	if agent.CreatorId != "" && agent.CreatorId == session.UserId {
		permission = model.PermissionManageOwnAgent
	}
	return a.sessionHasAIAgentPermission(session, agent.Scope, permission)
}

// AIAgentPermissionFor is the permission a session lacks to manage an agent, for the error message.
func AIAgentPermissionFor(session model.Session, agent *model.AIAgent) *model.Permission {
	if agent.CreatorId != "" && agent.CreatorId == session.UserId {
		return model.PermissionManageOwnAgent
	}
	return model.PermissionManageOthersAgent
}

func aiAgentError(where, id string, status int) *model.AppError {
	return model.NewAppError(where, "app.ai.agent."+id+".app_error", nil, "", status)
}

// GetAIAgent gives an agent that isn't deleted.
func (a *App) GetAIAgent(rctx request.CTX, userID string) (*model.AIAgent, *model.AppError) {
	agent, err := a.Srv().Store().AIAgent().Get(rctx, userID)
	if err != nil {
		var nfErr *store.ErrNotFound
		if errors.As(err, &nfErr) {
			return nil, aiAgentError("GetAIAgent", "not_found", http.StatusNotFound).Wrap(err)
		}
		return nil, aiAgentError("GetAIAgent", "get", http.StatusInternalServerError).Wrap(err)
	}
	if agent.DeleteAt != 0 {
		return nil, aiAgentError("GetAIAgent", "not_found", http.StatusNotFound)
	}
	return agent, nil
}

// aiAgentDetails adds an agent's account and its connection's name to its settings.
func (a *App) aiAgentDetails(rctx request.CTX, agent *model.AIAgent) (*model.AIAgentDetails, *model.AppError) {
	bot, appErr := a.GetBot(rctx, agent.UserId, true)
	if appErr != nil {
		return nil, appErr
	}
	details := &model.AIAgentDetails{
		AIAgent:        agent,
		Username:       bot.Username,
		DisplayName:    bot.DisplayName,
		Description:    bot.Description,
		LastIconUpdate: bot.LastIconUpdate,
		Enabled:        bot.DeleteAt == 0,
	}
	if agent.ConnectionId != "" {
		if c, err := a.Srv().Store().AIConnection().Get(rctx, agent.ConnectionId); err == nil && c.DeleteAt == 0 {
			details.ConnectionName = c.Name
		}
	}
	return details, nil
}

// GetAIAgentDetails gives an agent as its managers see it.
func (a *App) GetAIAgentDetails(rctx request.CTX, agent *model.AIAgent) (*model.AIAgentDetails, *model.AppError) {
	return a.aiAgentDetails(rctx, agent)
}

// GetAIAgentsForManager lists the agents a session manages: those of a scope, or, with an empty scope, every one it
// may manage.
func (a *App) GetAIAgentsForManager(rctx request.CTX, session model.Session, scope string) ([]*model.AIAgentDetails, *model.AppError) {
	var scopes []string
	var agents []*model.AIAgent
	var err error
	if scope != "" {
		if !a.SessionCanManageAIAgentScope(session, scope) {
			return nil, aiAgentError("GetAIAgentsForManager", "manage_permission", http.StatusForbidden)
		}
		scopes = []string{scope}
	} else if a.SessionHasPermissionTo(session, model.PermissionManageSystem) {
		// System admins manage every community's agents too.
		agents, err = a.Srv().Store().AIAgent().GetAll(rctx)
	} else {
		if a.SessionCanManageAIAgentScope(session, model.AIAgentScopeServer) {
			scopes = append(scopes, model.AIAgentScopeServer)
		}
		members, appErr := a.GetTeamMembersForUser(rctx, session.UserId, "", false)
		if appErr != nil {
			return nil, appErr
		}
		for _, m := range members {
			if m.DeleteAt == 0 && a.SessionCanManageAIAgentScope(session, m.TeamId) {
				scopes = append(scopes, m.TeamId)
			}
		}
	}
	if agents == nil && err == nil {
		agents, err = a.Srv().Store().AIAgent().GetForScopes(rctx, scopes)
	}
	if err != nil {
		return nil, aiAgentError("GetAIAgentsForManager", "get", http.StatusInternalServerError).Wrap(err)
	}
	out := make([]*model.AIAgentDetails, 0, len(agents))
	for _, agent := range agents {
		if !a.SessionCanManageAIAgent(session, agent) {
			continue
		}
		details, appErr := a.aiAgentDetails(rctx, agent)
		if appErr != nil {
			rctx.Logger().Warn("An agent's account is missing", mlog.String("user_id", agent.UserId), mlog.Err(appErr))
			continue
		}
		out = append(out, details)
	}
	slices.SortFunc(out, func(x, y *model.AIAgentDetails) int {
		return strings.Compare(strings.ToLower(x.DisplayName+x.Username), strings.ToLower(y.DisplayName+y.Username))
	})
	return out, nil
}

// GetAIAgentConnectionsForScope lists the connections an agent of a scope may use: server-wide ones for a server
// agent; a community's own and the server-wide ones shared with it for a community's agent.
func (a *App) GetAIAgentConnectionsForScope(rctx request.CTX, scope string) ([]*model.AIConnectionInfo, *model.AppError) {
	scopes := []string{model.AIConnectionScopeServer}
	if scope != model.AIAgentScopeServer {
		scopes = append(scopes, scope)
	}
	connections, err := a.Srv().Store().AIConnection().GetForScopes(rctx, scopes)
	if err != nil {
		return nil, model.NewAppError("GetAIAgentConnectionsForScope", "app.ai.connection.get.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	infos := []*model.AIConnectionInfo{}
	for _, c := range connections {
		if aiAgentMayUseConnection(scope, c) {
			infos = append(infos, c.Info())
		}
	}
	slices.SortFunc(infos, func(x, y *model.AIConnectionInfo) int { return strings.Compare(x.Name, y.Name) })
	return infos, nil
}

// aiAgentMayUseConnection tells whether an agent of a scope may be set up on a connection. A server agent talks with
// people in every community and in direct messages: it needs a server-wide connection (whose sharing and audience
// still apply to each call). A community's agent uses the community's own or a server-wide one shared with it.
func aiAgentMayUseConnection(scope string, c *model.AIConnection) bool {
	if c.DeleteAt != 0 || c.NeedsAttention != "" || !slices.Contains(c.Capabilities(), "chat") {
		return false
	}
	if scope == model.AIAgentScopeServer {
		return c.IsServerScoped()
	}
	return c.SharedWithTeam(scope)
}

// GetAIAgentConnectionForScope gives a connection an agent of a scope may use.
func (a *App) GetAIAgentConnectionForScope(rctx request.CTX, scope, connectionID string) (*model.AIConnection, *model.AppError) {
	c, appErr := a.GetAIConnection(rctx, connectionID)
	if appErr != nil {
		return nil, appErr
	}
	if !aiAgentMayUseConnection(scope, c) {
		return nil, aiAgentError("GetAIAgentConnectionForScope", "connection", http.StatusBadRequest)
	}
	return c, nil
}

// checkAIAgentReferences checks what an agent's settings point to: its connection may serve its scope, its scope's
// community exists, and the communities, channels and people of its access exist (a community's agent only names
// its community's channels).
func (a *App) checkAIAgentReferences(rctx request.CTX, agent *model.AIAgent) *model.AppError {
	invalid := func(detail string) *model.AppError {
		return model.NewAppError("checkAIAgentReferences", "app.ai.agent.references.app_error", nil, detail, http.StatusBadRequest)
	}
	if !agent.IsServerScoped() {
		if team, err := a.Srv().Store().Team().Get(agent.Scope); err != nil || team.DeleteAt != 0 {
			return invalid("scope=" + agent.Scope)
		}
	}
	if !agent.External {
		if _, appErr := a.GetAIAgentConnectionForScope(rctx, agent.Scope, agent.ConnectionId); appErr != nil {
			return appErr
		}
	}
	for _, teamID := range agent.Access.TeamIDs {
		if team, err := a.Srv().Store().Team().Get(teamID); err != nil || team.DeleteAt != 0 {
			return invalid("team_id=" + teamID)
		}
	}
	for _, channelID := range agent.Access.ChannelIDs {
		channel, err := a.Srv().Store().Channel().Get(channelID, true)
		if err != nil || channel.DeleteAt != 0 {
			return invalid("channel_id=" + channelID)
		}
		if !agent.IsServerScoped() && channel.TeamId != agent.Scope {
			return invalid("channel_id=" + channelID + " belongs to another community")
		}
	}
	for _, userID := range agent.Access.UserIDs {
		if _, err := a.Srv().Store().User().Get(rctx, userID); err != nil {
			return invalid("user_id=" + userID)
		}
	}
	return nil
}

// CreateAIAgent makes an agent: its bot account (of kind agent, owned by Antimatter's agents rather than a person),
// its settings, and for a community's agent its membership of the community. The caller checked that the creator may
// make agents in the scope.
func (a *App) CreateAIAgent(rctx request.CTX, req *model.AIAgentRequest, creatorID string) (*model.AIAgentDetails, *model.AppError) {
	agent := &model.AIAgent{
		Scope:        req.Scope,
		External:     req.External,
		ConnectionId: req.ConnectionId,
		Model:        req.Model,
		SystemPrompt: req.SystemPrompt,
		Tools:        req.Tools,
		Reasoning:    req.Reasoning,
		MaxToolTurns: req.MaxToolTurns,
		Access:       req.Access,
		CreatorId:    creatorID,
		UpdaterId:    creatorID,
		// Checked with a placeholder ID before the account exists.
		UserId: model.NewId(),
	}
	agent.PreSave()
	if appErr := agent.IsValid(); appErr != nil {
		return nil, appErr
	}
	if appErr := a.checkAIAgentReferences(rctx, agent); appErr != nil {
		return nil, appErr
	}

	bot, appErr := a.createBotAccount(rctx, &model.Bot{
		Username:    strings.ToLower(strings.TrimSpace(req.Username)),
		DisplayName: strings.TrimSpace(req.DisplayName),
		Description: strings.TrimSpace(req.Description),
		OwnerId:     model.AIAgentBotOwnerId,
		Kind:        model.BotKindAgent,
	})
	if appErr != nil {
		return nil, appErr
	}
	agent.UserId = bot.UserId
	saved, err := a.Srv().Store().AIAgent().Save(rctx, agent)
	if err != nil {
		if delErr := a.PermanentDeleteBot(rctx, bot.UserId); delErr != nil {
			rctx.Logger().Error("Couldn't remove the account of an agent that couldn't be saved", mlog.String("user_id", bot.UserId), mlog.Err(delErr))
		}
		var vErr *model.AppError
		if errors.As(err, &vErr) {
			return nil, vErr
		}
		return nil, aiAgentError("CreateAIAgent", "save", http.StatusInternalServerError).Wrap(err)
	}
	if !saved.IsServerScoped() {
		// A community's agent belongs to its community, where people find and mention it.
		if _, appErr := a.AddTeamMember(rctx, saved.Scope, saved.UserId); appErr != nil {
			rctx.Logger().Warn("Couldn't add a community's agent to its community", mlog.String("user_id", saved.UserId), mlog.String("team_id", saved.Scope), mlog.Err(appErr))
		}
	}
	a.InvalidateCacheForUser(saved.UserId)
	a.invalidateAIAgents()
	return a.aiAgentDetails(rctx, saved)
}

// PatchAIAgent changes an agent's account and settings; any saved change clears what needed attention, the admin
// having reviewed it.
func (a *App) PatchAIAgent(rctx request.CTX, agent *model.AIAgent, patch *model.AIAgentPatch, updaterID string) (*model.AIAgentDetails, *model.AppError) {
	updated := agent.Clone()
	if patch.ConnectionId != nil {
		updated.ConnectionId = *patch.ConnectionId
	}
	if patch.Model != nil {
		updated.Model = *patch.Model
	}
	if patch.SystemPrompt != nil {
		updated.SystemPrompt = *patch.SystemPrompt
	}
	if patch.Tools != nil {
		updated.Tools = *patch.Tools
	}
	if patch.Reasoning != nil {
		updated.Reasoning = *patch.Reasoning
	}
	if patch.MaxToolTurns != nil {
		updated.MaxToolTurns = *patch.MaxToolTurns
	}
	if patch.Access != nil {
		updated.Access = *patch.Access
	}
	updated.NeedsAttention = ""
	updated.UpdaterId = updaterID
	updated.PreUpdate()
	if appErr := updated.IsValid(); appErr != nil {
		return nil, appErr
	}
	if appErr := a.checkAIAgentReferences(rctx, updated); appErr != nil {
		return nil, appErr
	}

	if patch.Username != nil || patch.DisplayName != nil || patch.Description != nil {
		botPatch := &model.BotPatch{DisplayName: patch.DisplayName, Description: patch.Description}
		if patch.Username != nil {
			username := strings.ToLower(strings.TrimSpace(*patch.Username))
			botPatch.Username = &username
		}
		if _, appErr := a.PatchBot(rctx, agent.UserId, botPatch); appErr != nil {
			return nil, appErr
		}
	}
	saved, err := a.Srv().Store().AIAgent().Update(rctx, updated)
	if err != nil {
		var vErr *model.AppError
		if errors.As(err, &vErr) {
			return nil, vErr
		}
		var nfErr *store.ErrNotFound
		if errors.As(err, &nfErr) {
			return nil, aiAgentError("PatchAIAgent", "not_found", http.StatusNotFound).Wrap(err)
		}
		return nil, aiAgentError("PatchAIAgent", "save", http.StatusInternalServerError).Wrap(err)
	}
	a.invalidateAIAgents()
	return a.aiAgentDetails(rctx, saved)
}

// SetAIAgentEnabled turns an agent's account on or off: a disabled agent can't sign in and nobody can talk to it.
func (a *App) SetAIAgentEnabled(rctx request.CTX, agent *model.AIAgent, enabled bool) (*model.AIAgentDetails, *model.AppError) {
	if _, appErr := a.UpdateBotActive(rctx, agent.UserId, enabled); appErr != nil {
		return nil, appErr
	}
	if !enabled {
		a.setAgentActivityQuietly(rctx, agent, &model.AgentActivity{UserId: agent.UserId, State: model.AgentStateOffline, UpdateAt: model.GetMillis()})
	} else {
		a.setAgentActivityQuietly(rctx, agent, &model.AgentActivity{UserId: agent.UserId, State: model.AgentStateIdle, UpdateAt: model.GetMillis()})
	}
	a.invalidateAIAgents()
	return a.aiAgentDetails(rctx, agent)
}

// DeleteAIAgent removes an agent: its settings are deleted and its account deactivated. Its messages stay, still
// shown as an agent's.
func (a *App) DeleteAIAgent(rctx request.CTX, agent *model.AIAgent) *model.AppError {
	if _, appErr := a.UpdateBotActive(rctx, agent.UserId, false); appErr != nil {
		return appErr
	}
	if err := a.Srv().Store().AIAgent().Delete(rctx, agent.UserId, model.GetMillis()); err != nil {
		var nfErr *store.ErrNotFound
		if errors.As(err, &nfErr) {
			return aiAgentError("DeleteAIAgent", "not_found", http.StatusNotFound).Wrap(err)
		}
		return aiAgentError("DeleteAIAgent", "save", http.StatusInternalServerError).Wrap(err)
	}
	if err := a.Srv().Store().AgentActivity().Delete(rctx, agent.UserId); err != nil {
		rctx.Logger().Warn("Couldn't forget a deleted agent's activity", mlog.String("user_id", agent.UserId), mlog.Err(err))
	}
	a.invalidateAIAgents()
	return nil
}

// SessionCanManageAIAgentUser tells whether a session manages the agent of a user ID (for its avatar): false for any
// account that isn't an agent's.
func (a *App) SessionCanManageAIAgentUser(rctx request.CTX, session model.Session, userID string) bool {
	agent, appErr := a.GetAIAgent(rctx, userID)
	if appErr != nil {
		return false
	}
	return a.SessionCanManageAIAgent(session, agent)
}

// aiAgentAllows tells whether a person may talk to an agent in a channel (nil for no channel): the agent's account is
// active, its scope reaches the person and the channel, and its access names them. Guests only when it allows them.
func (a *App) aiAgentAllows(rctx request.CTX, agent *model.AIAgent, user *model.User, channel *model.Channel) *model.AppError {
	const where = "aiAgentAllows"
	denied := func(detail string) *model.AppError {
		return model.NewAppError(where, "app.ai.agent.access.app_error", nil, detail, http.StatusForbidden)
	}
	if user == nil || user.DeleteAt != 0 || user.IsBot {
		return denied("not a person")
	}
	if user.IsGuest() && !agent.Access.AllowGuests {
		return denied("guest")
	}
	teamID := ""
	if channel != nil {
		teamID = channel.TeamId
	}
	if !agent.IsServerScoped() {
		if teamID != "" && teamID != agent.Scope {
			return denied("another community's channel")
		}
		teamID = agent.Scope
		if member, appErr := a.GetTeamMember(rctx, agent.Scope, user.Id); appErr != nil || member.DeleteAt != 0 {
			return denied("not a member of the agent's community")
		}
	}
	if agent.Access.Everyone {
		return nil
	}
	for _, id := range agent.Access.UserIDs {
		if id == user.Id {
			return nil
		}
	}
	for _, id := range agent.Access.TeamIDs {
		if member, appErr := a.GetTeamMember(rctx, id, user.Id); appErr == nil && member.DeleteAt == 0 {
			return nil
		}
	}
	for _, id := range agent.Access.ChannelIDs {
		if _, appErr := a.GetChannelMember(rctx, id, user.Id); appErr == nil {
			return nil
		}
	}
	if len(agent.Access.Roles) > 0 {
		roles := user.GetRoles()
		if teamID != "" {
			if member, appErr := a.GetTeamMember(rctx, teamID, user.Id); appErr == nil && member.DeleteAt == 0 {
				roles = append(roles, member.GetRoles()...)
				if member.SchemeAdmin {
					roles = append(roles, model.TeamAdminRoleId)
				}
				if member.SchemeUser {
					roles = append(roles, model.TeamUserRoleId)
				}
			}
		}
		for _, role := range agent.Access.Roles {
			if slices.Contains(roles, role) {
				return nil
			}
		}
	}
	return denied("not in the agent's access")
}

// GetAIAgentsForUser lists the built-in agents a person may talk to (for the AI chat and the composer's mentions):
// enabled, with their accounts, sorted by name.
func (a *App) GetAIAgentsForUser(rctx request.CTX, userID string) ([]*model.AIAgent, []*model.User, *model.AppError) {
	user, appErr := a.GetUser(rctx, userID)
	if appErr != nil {
		return nil, nil, appErr
	}
	// The node's list of agents: this runs for every mention autocomplete.
	agents := a.aiAgentDirectory(rctx).all()
	ids := make([]string, 0, len(agents))
	for _, agent := range agents {
		ids = append(ids, agent.UserId)
	}
	users := map[string]*model.User{}
	if len(ids) > 0 {
		list, appErr := a.GetUsers(rctx, ids)
		if appErr != nil {
			return nil, nil, appErr
		}
		for _, u := range list {
			users[u.Id] = u
		}
	}
	outAgents := []*model.AIAgent{}
	outUsers := []*model.User{}
	for _, agent := range agents {
		account := users[agent.UserId]
		if account == nil || account.DeleteAt != 0 || agent.External {
			continue
		}
		if a.aiAgentAllows(rctx, agent, user, nil) != nil {
			continue
		}
		outAgents = append(outAgents, agent)
		outUsers = append(outUsers, account)
	}
	order := make([]int, len(outAgents))
	for i := range order {
		order[i] = i
	}
	slices.SortStableFunc(order, func(x, y int) int {
		return strings.Compare(strings.ToLower(outUsers[x].Username), strings.ToLower(outUsers[y].Username))
	})
	sortedAgents := make([]*model.AIAgent, len(order))
	sortedUsers := make([]*model.User, len(order))
	for i, j := range order {
		sortedAgents[i], sortedUsers[i] = outAgents[j], outUsers[j]
	}
	return sortedAgents, sortedUsers, nil
}
