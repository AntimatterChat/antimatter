// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package app

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/mattermost/mattermost/server/public/ai"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/app/aiservice"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

// Model connections (Antimatter): system admins (manage_ai_connections) manage the server-wide ones; a community's
// admins (manage_team_ai_connections) manage their community's own, and only those. Keys are sealed before they're
// stored and never given back.

// SessionCanManageAIConnectionScope tells whether a session may manage the connections of a scope: "server" or a
// team ID.
func (a *App) SessionCanManageAIConnectionScope(session model.Session, scope string) bool {
	if scope == model.AIConnectionScopeServer {
		return a.SessionHasPermissionTo(session, model.PermissionManageAIConnections)
	}
	if !model.IsValidId(scope) {
		return false
	}
	return a.SessionHasPermissionToTeam(session, scope, model.PermissionManageTeamAIConnections)
}

// GetAIConnection gives a connection that isn't deleted.
func (a *App) GetAIConnection(rctx request.CTX, id string) (*model.AIConnection, *model.AppError) {
	c, err := a.Srv().Store().AIConnection().Get(rctx, id)
	if err != nil {
		var nfErr *store.ErrNotFound
		if errors.As(err, &nfErr) {
			return nil, model.NewAppError("GetAIConnection", "app.ai.connection.not_found.app_error", nil, "", http.StatusNotFound).Wrap(err)
		}
		return nil, model.NewAppError("GetAIConnection", "app.ai.connection.get.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	if c.DeleteAt != 0 {
		return nil, model.NewAppError("GetAIConnection", "app.ai.connection.not_found.app_error", nil, "", http.StatusNotFound)
	}
	return c, nil
}

// GetAIConnectionsForManager lists the connections a session manages: those of a scope ("server" or a team ID), or,
// with an empty scope, every one it may manage (the server's and its communities').
func (a *App) GetAIConnectionsForManager(rctx request.CTX, session model.Session, scope string) ([]*model.AIConnection, *model.AppError) {
	var scopes []string
	switch {
	case scope != "":
		if !a.SessionCanManageAIConnectionScope(session, scope) {
			return nil, model.NewAppError("GetAIConnectionsForManager", "app.ai.connection.manage_permission.app_error", nil, "", http.StatusForbidden)
		}
		scopes = []string{scope}
	case a.SessionHasPermissionTo(session, model.PermissionManageAIConnections) && a.SessionHasPermissionTo(session, model.PermissionManageSystem):
		connections, err := a.Srv().Store().AIConnection().GetAll(rctx)
		if err != nil {
			return nil, model.NewAppError("GetAIConnectionsForManager", "app.ai.connection.get.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
		}
		return connections, nil
	default:
		if a.SessionHasPermissionTo(session, model.PermissionManageAIConnections) {
			scopes = append(scopes, model.AIConnectionScopeServer)
		}
		members, appErr := a.GetTeamMembersForUser(rctx, session.UserId, "", false)
		if appErr != nil {
			return nil, appErr
		}
		for _, m := range members {
			if m.DeleteAt == 0 && a.SessionHasPermissionToTeam(session, m.TeamId, model.PermissionManageTeamAIConnections) {
				scopes = append(scopes, m.TeamId)
			}
		}
	}
	connections, err := a.Srv().Store().AIConnection().GetForScopes(rctx, scopes)
	if err != nil {
		return nil, model.NewAppError("GetAIConnectionsForManager", "app.ai.connection.get.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	return connections, nil
}

// checkAIConnectionReferences checks what a connection points to: its scope's community exists, the communities it's
// shared with exist, and its audience's groups exist (and, for a community's connection, are that community's).
func (a *App) checkAIConnectionReferences(rctx request.CTX, c *model.AIConnection) *model.AppError {
	invalid := func(detail string) *model.AppError {
		return model.NewAppError("checkAIConnectionReferences", "app.ai.connection.references.app_error", nil, detail, http.StatusBadRequest)
	}
	if !c.IsServerScoped() {
		if team, err := a.Srv().Store().Team().Get(c.Scope); err != nil || team.DeleteAt != 0 {
			return invalid("scope=" + c.Scope)
		}
	}
	for _, teamID := range c.Sharing.TeamIDs {
		if team, err := a.Srv().Store().Team().Get(teamID); err != nil || team.DeleteAt != 0 {
			return invalid("team_id=" + teamID)
		}
	}
	for _, groupID := range c.Audience.GroupIDs {
		group, err := a.Srv().Store().Group().Get(groupID)
		if err != nil || group.DeleteAt != 0 {
			return invalid("group_id=" + groupID)
		}
		if !c.IsServerScoped() && group.TeamId != c.Scope {
			return invalid("group_id=" + groupID + " belongs to another community")
		}
	}
	return nil
}

func aiConnectionStoreError(where string, err error) *model.AppError {
	var appErr *model.AppError
	if errors.As(err, &appErr) {
		return appErr
	}
	var nfErr *store.ErrNotFound
	if errors.As(err, &nfErr) {
		return model.NewAppError(where, "app.ai.connection.not_found.app_error", nil, "", http.StatusNotFound).Wrap(err)
	}
	return model.NewAppError(where, "app.ai.connection.save.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
}

// newAIConnectionFromRequest builds an unsaved connection from a request, its secrets sealed.
func (a *App) newAIConnectionFromRequest(req *model.AIConnectionRequest, creatorID string) (*model.AIConnection, *model.AppError) {
	if appErr := req.Validate(); appErr != nil {
		return nil, appErr
	}
	c := &model.AIConnection{
		Id:               model.NewId(),
		Name:             req.Name,
		Scope:            req.Scope,
		Provider:         req.Provider,
		BaseURL:          req.BaseURL,
		Models:           req.Models,
		DecisionsDialect: req.DecisionsDialect,
		Limits:           req.Limits,
		Sharing:          req.Sharing,
		Audience:         req.Audience,
		CreatorId:        creatorID,
		UpdaterId:        creatorID,
	}
	headers := req.ExtraHeaders
	if headers == nil {
		headers = map[string]string{}
	}
	apiKey := strings.TrimSpace(req.APIKey)
	if appErr := a.sealAIConnectionSecrets(c, &apiKey, &headers); appErr != nil {
		return nil, appErr
	}
	c.PreSave()
	return c, nil
}

// CreateAIConnection adds a connection. The caller checked that its creator may manage its scope.
func (a *App) CreateAIConnection(rctx request.CTX, req *model.AIConnectionRequest, creatorID string) (*model.AIConnection, *model.AppError) {
	c, appErr := a.newAIConnectionFromRequest(req, creatorID)
	if appErr != nil {
		return nil, appErr
	}
	if appErr := c.IsValid(); appErr != nil {
		return nil, appErr
	}
	if appErr := a.checkAIConnectionReferences(rctx, c); appErr != nil {
		return nil, appErr
	}
	saved, err := a.Srv().Store().AIConnection().Save(rctx, c)
	if err != nil {
		return nil, aiConnectionStoreError("CreateAIConnection", err)
	}
	return saved, nil
}

// PatchAIConnection changes a connection; any saved change clears what needed attention, the admin having reviewed
// it.
func (a *App) PatchAIConnection(rctx request.CTX, c *model.AIConnection, patch *model.AIConnectionPatch, updaterID string) (*model.AIConnection, *model.AppError) {
	updated := *c
	if patch.Name != nil {
		updated.Name = *patch.Name
	}
	if patch.Provider != nil {
		updated.Provider = *patch.Provider
	}
	if patch.BaseURL != nil {
		updated.BaseURL = *patch.BaseURL
	}
	if patch.Models != nil {
		updated.Models = *patch.Models
	}
	if patch.DecisionsDialect != nil {
		updated.DecisionsDialect = *patch.DecisionsDialect
	}
	if patch.Limits != nil {
		updated.Limits = *patch.Limits
	}
	if patch.Sharing != nil {
		updated.Sharing = *patch.Sharing
	}
	if patch.Audience != nil {
		updated.Audience = *patch.Audience
	}
	var apiKey *string
	if patch.APIKey != nil {
		key := strings.TrimSpace(*patch.APIKey)
		if appErr := (&model.AIConnectionRequest{APIKey: key}).Validate(); appErr != nil {
			return nil, appErr
		}
		apiKey = &key
	}
	if patch.ExtraHeaders != nil {
		if appErr := model.ValidateAIExtraHeaders(*patch.ExtraHeaders); appErr != nil {
			return nil, appErr
		}
	}
	if appErr := a.sealAIConnectionSecrets(&updated, apiKey, patch.ExtraHeaders); appErr != nil {
		return nil, appErr
	}
	updated.NeedsAttention = ""
	updated.UpdaterId = updaterID
	updated.PreUpdate()
	if appErr := updated.IsValid(); appErr != nil {
		return nil, appErr
	}
	if appErr := a.checkAIConnectionReferences(rctx, &updated); appErr != nil {
		return nil, appErr
	}
	saved, err := a.Srv().Store().AIConnection().Update(rctx, &updated)
	if err != nil {
		return nil, aiConnectionStoreError("PatchAIConnection", err)
	}
	return saved, nil
}

// DeleteAIConnection deletes a connection; its key and headers are forgotten. Features configured with it stop
// using AI until an admin picks another one.
func (a *App) DeleteAIConnection(rctx request.CTX, c *model.AIConnection) *model.AppError {
	if err := a.Srv().Store().AIConnection().Delete(rctx, c.Id, model.GetMillis()); err != nil {
		return aiConnectionStoreError("DeleteAIConnection", err)
	}
	return nil
}

// aiHTTPClient gives the client of a connection's calls: a server-wide connection's address is a system admin's and
// may be on the local network; a community's is its admins', and may not reach internal addresses (unless
// ServiceSettings.AllowedUntrustedInternalConnections allows them).
func (a *App) aiHTTPClient(c *model.AIConnection) *http.Client {
	return &http.Client{Transport: a.HTTPService().MakeTransport(c.IsServerScoped())}
}

// aiProvider builds the provider of a connection, with its secrets in clear for this call only.
func (a *App) aiProvider(c *model.AIConnection) (ai.Provider, *model.AppError) {
	apiKey, headers, appErr := a.openAIConnectionSecrets(c)
	if appErr != nil {
		return nil, appErr
	}
	provider, err := aiservice.New(aiservice.Config{
		Provider:         c.Provider,
		BaseURL:          c.BaseURL,
		APIKey:           apiKey,
		Headers:          headers,
		DecisionsDialect: ai.DecisionsDialect(c.DecisionsDialect),
		HTTPClient:       a.aiHTTPClient(c),
	})
	if err != nil {
		return nil, model.NewAppError("aiProvider", "app.ai.connection.provider.app_error", nil, "", http.StatusBadRequest).Wrap(err)
	}
	return provider, nil
}

const aiConnectionTestTimeout = 20 * time.Second

// TestAIConnection checks a connection's endpoint and key with a cheap call: the list of its models, else (for
// endpoints without one) a one-token chat with its chat model.
func (a *App) TestAIConnection(rctx request.CTX, c *model.AIConnection) *model.AIConnectionTestResult {
	result := &model.AIConnectionTestResult{}
	provider, appErr := a.aiProvider(c)
	if appErr != nil {
		result.Error = appErr.Message
		return result
	}
	ctx, cancel := context.WithTimeout(context.Background(), aiConnectionTestTimeout)
	defer cancel()
	start := time.Now()
	models, err := provider.ListModels(ctx)
	if err != nil && c.Models.Chat != "" {
		var pe *ai.ProviderError
		if !errors.As(err, &pe) || pe.StatusCode == http.StatusNotFound || pe.StatusCode == http.StatusMethodNotAllowed {
			_, err = provider.Chat(ctx, ai.ChatRequest{Model: c.Models.Chat, MaxTokens: 1, Messages: []ai.Message{{Role: ai.RoleUser, Content: "ping"}}})
		}
	}
	result.LatencyMs = time.Since(start).Milliseconds()
	if err != nil {
		result.Error = ai.SanitizeProviderErrorMessage(err.Error())
		var pe *ai.ProviderError
		if errors.As(err, &pe) {
			result.StatusCode = pe.StatusCode
		}
		rctx.Logger().Info("A model connection's test failed", mlog.String("connection_id", c.Id), mlog.Int("status", result.StatusCode))
		return result
	}
	result.OK = true
	result.ModelCount = len(models)
	return result
}

// TestAIConnectionRequest tests a connection before it's saved; nothing is stored.
func (a *App) TestAIConnectionRequest(rctx request.CTX, req *model.AIConnectionRequest) (*model.AIConnectionTestResult, *model.AppError) {
	c, appErr := a.newAIConnectionFromRequest(req, "")
	if appErr != nil {
		return nil, appErr
	}
	if appErr := c.IsValid(); appErr != nil {
		return nil, appErr
	}
	return a.TestAIConnection(rctx, c), nil
}

// ListAIConnectionModels gives the models a connection's endpoint offers.
func (a *App) ListAIConnectionModels(rctx request.CTX, c *model.AIConnection) ([]ai.ModelInfo, *model.AppError) {
	provider, appErr := a.aiProvider(c)
	if appErr != nil {
		return nil, appErr
	}
	ctx, cancel := context.WithTimeout(context.Background(), aiConnectionTestTimeout)
	defer cancel()
	models, err := provider.ListModels(ctx)
	if err != nil {
		return nil, aiCallError("ListAIConnectionModels", err)
	}
	return models, nil
}

// ListAIConnectionRequestModels gives the models a connection's endpoint offers before it's saved; nothing is
// stored. Its calls go out as a saved connection of its scope's would (a community's can't reach internal addresses).
func (a *App) ListAIConnectionRequestModels(rctx request.CTX, req *model.AIConnectionRequest) ([]ai.ModelInfo, *model.AppError) {
	c, appErr := a.newAIConnectionFromRequest(req, "")
	if appErr != nil {
		return nil, appErr
	}
	if appErr := c.IsValid(); appErr != nil {
		return nil, appErr
	}
	return a.ListAIConnectionModels(rctx, c)
}

// GetAIConnectionUsage gives a connection's usage this month, per day and purpose.
func (a *App) GetAIConnectionUsage(rctx request.CTX, c *model.AIConnection) (*model.AIConnectionUsage, *model.AppError) {
	now := time.Now()
	monthStart := model.AIUsageMonthStart(now)
	rows, err := a.Srv().Store().AIConnection().GetUsage(rctx, c.Id, monthStart)
	if err != nil {
		return nil, model.NewAppError("GetAIConnectionUsage", "app.ai.connection.usage.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	usage := &model.AIConnectionUsage{
		ConnectionId:       c.Id,
		Month:              monthStart / 100,
		MonthlyTokenBudget: c.Limits.MonthlyTokenBudget,
		Days:               rows,
		ByPurpose:          map[string]model.AIUsageTotals{},
	}
	for _, r := range rows {
		usage.Totals.Requests += r.Requests
		usage.Totals.InputTokens += r.InputTokens
		usage.Totals.OutputTokens += r.OutputTokens
		purpose := r.Purpose
		if purpose == "" {
			purpose = "user"
		}
		t := usage.ByPurpose[purpose]
		t.Requests += r.Requests
		t.InputTokens += r.InputTokens
		t.OutputTokens += r.OutputTokens
		usage.ByPurpose[purpose] = t
	}
	return usage, nil
}

// GetAIConnectionsForUser lists the connections a person may use in a channel or community (both may be empty: then
// only server-wide connections shared with every community). An app asks it for a person the server verified.
func (a *App) GetAIConnectionsForUser(rctx request.CTX, userID, channelID, teamID string) ([]*model.AIConnectionInfo, *model.AppError) {
	user, teamID, appErr := a.aiCallContext(rctx, userID, channelID, teamID)
	if appErr != nil {
		return nil, appErr
	}
	scopes := []string{model.AIConnectionScopeServer}
	if teamID != "" {
		scopes = append(scopes, teamID)
	}
	connections, err := a.Srv().Store().AIConnection().GetForScopes(rctx, scopes)
	if err != nil {
		return nil, model.NewAppError("GetAIConnectionsForUser", "app.ai.connection.get.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	infos := []*model.AIConnectionInfo{}
	for _, c := range connections {
		if c.NeedsAttention != "" {
			continue
		}
		if a.aiAudienceAllows(rctx, c, user, teamID) == nil {
			infos = append(infos, c.Info())
		}
	}
	slices.SortFunc(infos, func(x, y *model.AIConnectionInfo) int { return strings.Compare(x.Name, y.Name) })
	return infos, nil
}
