// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/mattermost/mattermost/server/public/ai"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

// The server's AI service (Antimatter): every call runs on a model connection, for a principal, and is checked,
// limited, counted and audited here.
//
// The principal is a person the caller verified (a session's user, or the user an app got from a request's
// Mattermost-User-Id or a hook context), or a built-in purpose (recaps, rewrite, auto-translation, text to speech)
// with the connection an admin chose for it. A person's call is held to the connection's audience in its context: a
// community's connection only in that community's channels and for its members, a server-wide one only in the
// communities it's shared with (direct and group messages need it shared with all), and only for those its audience
// names (guests only when allowed). There are no anonymous calls.

// AICall is who a call is for, and where.
type AICall struct {
	// UserID is the person the call is for, verified by the caller. Built-in features set it when there's one (the
	// person rewriting, the owner of a recap), for their limits and usage.
	UserID string
	// Purpose names a built-in feature; empty for a person's own call (an app's, for instance).
	Purpose ai.Purpose
	// PluginID is the app making the call for UserID, set by the server's plugin API.
	PluginID string
	// ChannelID and TeamID are where the call happens; both may be empty. A channel's community wins: a team ID that
	// isn't the channel's is refused.
	ChannelID string
	TeamID    string
}

// usagePurpose is what the usage and audit records say the call was for.
func (c AICall) usagePurpose() string {
	switch {
	case c.Purpose != "":
		return string(c.Purpose)
	case c.PluginID != "":
		return "plugin:" + c.PluginID
	default:
		return ""
	}
}

// aiPreparedCall is a call that passed the checks: its connection, model and provider.
type aiPreparedCall struct {
	call       AICall
	connection *model.AIConnection
	model      string
	capability ai.Capability
	operation  string
	provider   ai.Provider
	started    time.Time
}

// aiCallContext checks a person and where they call from, and gives the community of the call ("" for direct and
// group messages, or no context).
func (a *App) aiCallContext(rctx request.CTX, userID, channelID, teamID string) (*model.User, string, *model.AppError) {
	if !model.IsValidId(userID) {
		return nil, "", model.NewAppError("aiCallContext", "app.ai.call.no_user.app_error", nil, "", http.StatusBadRequest)
	}
	user, appErr := a.GetUser(rctx, userID)
	if appErr != nil {
		return nil, "", model.NewAppError("aiCallContext", "app.ai.call.user.app_error", nil, "user_id="+userID, http.StatusForbidden)
	}
	if user.DeleteAt != 0 {
		return nil, "", model.NewAppError("aiCallContext", "app.ai.call.user.app_error", nil, "deactivated user_id="+userID, http.StatusForbidden)
	}
	if channelID != "" {
		channel, appErr := a.GetChannel(rctx, channelID)
		if appErr != nil {
			return nil, "", model.NewAppError("aiCallContext", "app.ai.call.channel.app_error", nil, "channel_id="+channelID, http.StatusForbidden)
		}
		if teamID != "" && teamID != channel.TeamId {
			return nil, "", model.NewAppError("aiCallContext", "app.ai.call.team_mismatch.app_error", nil, "", http.StatusBadRequest)
		}
		if ok, _ := a.HasPermissionToChannel(rctx, userID, channelID, model.PermissionReadChannelContent); !ok {
			return nil, "", model.NewAppError("aiCallContext", "app.ai.call.channel.app_error", nil, "no read access to channel_id="+channelID, http.StatusForbidden)
		}
		teamID = channel.TeamId
	}
	if teamID != "" {
		member, appErr := a.GetTeamMember(rctx, teamID, userID)
		if appErr != nil || member.DeleteAt != 0 {
			return nil, "", model.NewAppError("aiCallContext", "app.ai.call.team.app_error", nil, "not a member of team_id="+teamID, http.StatusForbidden)
		}
	}
	return user, teamID, nil
}

// aiAudienceAllows tells whether a person may use a connection in a community ("" for none): the connection must
// be available there, and its audience must name them.
func (a *App) aiAudienceAllows(rctx request.CTX, c *model.AIConnection, user *model.User, teamID string) *model.AppError {
	if !c.SharedWithTeam(teamID) {
		return model.NewAppError("aiAudienceAllows", "app.ai.call.not_available_here.app_error", nil, "connection_id="+c.Id, http.StatusForbidden)
	}
	if user.IsGuest() && !c.Audience.AllowGuests {
		return model.NewAppError("aiAudienceAllows", "app.ai.call.audience.app_error", nil, "guest", http.StatusForbidden)
	}
	if c.Audience.Everyone {
		return nil
	}
	if len(c.Audience.Roles) > 0 {
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
				if member.SchemeGuest {
					roles = append(roles, model.TeamGuestRoleId)
				}
			}
		}
		for _, role := range c.Audience.Roles {
			if slices.Contains(roles, role) {
				return nil
			}
		}
	}
	for _, groupID := range c.Audience.GroupIDs {
		group, err := a.Srv().Store().Group().Get(groupID)
		if err != nil || group.DeleteAt != 0 {
			continue
		}
		// A community's group only counts in its community.
		if group.TeamId != "" && group.TeamId != teamID && !c.IsServerScoped() {
			continue
		}
		if member, err := a.Srv().Store().Group().GetMember(groupID, user.Id); err == nil && member.DeleteAt == 0 {
			return nil
		}
	}
	return model.NewAppError("aiAudienceAllows", "app.ai.call.audience.app_error", nil, "connection_id="+c.Id, http.StatusForbidden)
}

// prepareAICall runs the checks of a call: the principal, the connection, the audience, the model, the limits and
// the budgets.
func (a *App) prepareAICall(rctx request.CTX, call AICall, connectionID, requestedModel string, capability ai.Capability, operation string) (*aiPreparedCall, *model.AppError) {
	const where = "prepareAICall"
	if call.Purpose != "" && !call.Purpose.IsBuiltIn() {
		return nil, model.NewAppError(where, "app.ai.call.purpose.app_error", nil, "purpose="+string(call.Purpose), http.StatusBadRequest)
	}
	if call.Purpose != "" && call.PluginID != "" {
		// Apps call for a person, never as a built-in feature.
		return nil, model.NewAppError(where, "app.ai.call.purpose.app_error", nil, "plugin_id="+call.PluginID, http.StatusBadRequest)
	}
	if call.Purpose == "" && call.UserID == "" {
		return nil, model.NewAppError(where, "app.ai.call.no_user.app_error", nil, "", http.StatusBadRequest)
	}
	if !model.IsValidId(connectionID) {
		return nil, model.NewAppError(where, "app.ai.connection.not_found.app_error", nil, "", http.StatusNotFound)
	}
	c, appErr := a.GetAIConnection(rctx, connectionID)
	if appErr != nil {
		return nil, appErr
	}
	if c.NeedsAttention != "" {
		return nil, model.NewAppError(where, "app.ai.connection.needs_attention.app_error", nil, "connection_id="+c.Id, http.StatusConflict)
	}

	if call.Purpose != "" {
		// A built-in feature uses the connection a system admin chose for it: a server-wide one.
		if !c.IsServerScoped() {
			return nil, model.NewAppError(where, "app.ai.call.builtin_server_only.app_error", nil, "connection_id="+c.Id, http.StatusForbidden)
		}
		if call.UserID != "" {
			if user, appErr := a.GetUser(rctx, call.UserID); appErr != nil || user.DeleteAt != 0 {
				return nil, model.NewAppError(where, "app.ai.call.user.app_error", nil, "user_id="+call.UserID, http.StatusForbidden)
			}
		}
	} else {
		user, teamID, appErr := a.aiCallContext(rctx, call.UserID, call.ChannelID, call.TeamID)
		if appErr != nil {
			return nil, appErr
		}
		if appErr := a.aiAudienceAllows(rctx, c, user, teamID); appErr != nil {
			return nil, appErr
		}
		call.TeamID = teamID
	}

	if !slices.Contains(c.Capabilities(), string(capability)) {
		return nil, model.NewAppError(where, "app.ai.call.unsupported.app_error", nil, "capability="+string(capability), http.StatusBadRequest)
	}
	modelName := strings.TrimSpace(requestedModel)
	if modelName == "" {
		switch capability {
		case ai.CapabilityChat:
			modelName = c.Models.Chat
		case ai.CapabilityEmbeddings:
			modelName = c.Models.Embeddings
		case ai.CapabilityTranscription:
			modelName = c.Models.Transcription
		case ai.CapabilityDecisions:
			modelName = c.Models.Decisions
		}
	}
	if modelName == "" {
		return nil, model.NewAppError(where, "app.ai.call.no_model.app_error", map[string]any{"Capability": string(capability)}, "", http.StatusBadRequest)
	}
	if len(modelName) > model.AIModelNameMaxLength {
		return nil, model.NewAppError(where, "app.ai.call.no_model.app_error", map[string]any{"Capability": string(capability)}, "too long", http.StatusBadRequest)
	}

	if appErr := a.checkAILimits(rctx, c, call.UserID); appErr != nil {
		return nil, appErr
	}

	provider, appErr := a.aiProvider(c)
	if appErr != nil {
		return nil, appErr
	}
	return &aiPreparedCall{call: call, connection: c, model: modelName, capability: capability, operation: operation, provider: provider, started: time.Now()}, nil
}

// checkAILimits applies a connection's requests per minute (for it and for the person) and its monthly token
// budgets (for it and for the person). The requests per minute are counted on each node, in memory: a cluster of n
// nodes lets through up to n times the rate. The budgets are counted in the database, for the whole cluster (a few
// calls running at once may go slightly past them).
func (a *App) checkAILimits(rctx request.CTX, c *model.AIConnection, userID string) *model.AppError {
	const where = "checkAILimits"
	limits := map[string]int{"aiconnection:" + c.Id: c.Limits.RequestsPerMinute}
	if userID != "" {
		limits["aiconnection:"+c.Id+":"+userID] = c.Limits.PerUserRequestsPerMinute
	}
	if ok, wait := a.Srv().aiLimiter().AllowAll(limits); !ok {
		appErr := model.NewAppError(where, "app.ai.call.rate_limited.app_error", nil, fmt.Sprintf("retry_after=%s", wait.Round(time.Second)), http.StatusTooManyRequests)
		return appErr
	}
	monthStart := model.AIUsageMonthStart(time.Now())
	if c.Limits.MonthlyTokenBudget > 0 {
		totals, err := a.Srv().Store().AIConnection().GetUsageTotals(rctx, c.Id, monthStart)
		if err != nil {
			return model.NewAppError(where, "app.ai.connection.usage.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
		}
		if totals.Tokens() >= c.Limits.MonthlyTokenBudget {
			return model.NewAppError(where, "app.ai.call.budget_exceeded.app_error", nil, "connection", http.StatusTooManyRequests)
		}
	}
	if userID != "" && c.Limits.PerUserMonthlyTokenBudget > 0 {
		totals, err := a.Srv().Store().AIConnection().GetUserUsageTotals(rctx, c.Id, userID, monthStart)
		if err != nil {
			return model.NewAppError(where, "app.ai.connection.usage.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
		}
		if totals.Tokens() >= c.Limits.PerUserMonthlyTokenBudget {
			return model.NewAppError(where, "app.ai.call.user_budget_exceeded.app_error", nil, "user", http.StatusTooManyRequests)
		}
	}
	return nil
}

// finishAICall counts a call's usage and, for a person's or an app's call, writes its audit record: who, the
// connection, the purpose, the model and the tokens; never what was said. The server's own features (recaps,
// rewrite, auto-translation, text to speech) are counted but not audited call by call: auto-translation alone makes
// one call per message and language; their usage per day and purpose is the record.
func (a *App) finishAICall(rctx request.CTX, p *aiPreparedCall, usage ai.Usage, callErr error) {
	if err := a.Srv().Store().AIConnection().AddUsage(rctx, &model.AIUsage{
		ConnectionId: p.connection.Id,
		UserId:       p.call.UserID,
		Purpose:      p.call.usagePurpose(),
		Day:          model.AIUsageDay(time.Now()),
		Requests:     1,
		InputTokens:  usage.InputTokens,
		OutputTokens: usage.OutputTokens,
	}); err != nil {
		rctx.Logger().Warn("Couldn't count the use of a model connection", mlog.String("connection_id", p.connection.Id), mlog.Err(err))
	}

	if p.call.Purpose != "" {
		return
	}

	rec := a.MakeAuditRecord(rctx, model.AuditEventAICall, model.AuditStatusFail)
	rec.Actor.UserId = p.call.UserID
	model.AddEventParameterToAuditRec(rec, "connection_id", p.connection.Id)
	model.AddEventParameterToAuditRec(rec, "scope", p.connection.Scope)
	model.AddEventParameterToAuditRec(rec, "purpose", p.call.usagePurpose())
	model.AddEventParameterToAuditRec(rec, "capability", string(p.capability))
	model.AddEventParameterToAuditRec(rec, "model", p.model)
	model.AddEventParameterToAuditRec(rec, "operation", p.operation)
	model.AddEventParameterToAuditRec(rec, "channel_id", p.call.ChannelID)
	model.AddEventParameterToAuditRec(rec, "team_id", p.call.TeamID)
	model.AddEventParameterToAuditRec(rec, "input_tokens", usage.InputTokens)
	model.AddEventParameterToAuditRec(rec, "output_tokens", usage.OutputTokens)
	model.AddEventParameterToAuditRec(rec, "duration_ms", time.Since(p.started).Milliseconds())
	if callErr == nil {
		rec.Success()
		a.LogAuditRecWithLevel(rctx, rec, mlog.LvlAuditAPI, nil)
		return
	}
	rec.AddErrorDesc(callErr.Error())
	a.LogAuditRecWithLevel(rctx, rec, mlog.LvlAuditAPI, nil)
}

// aiCallError turns a provider's error into an AppError; its message is already free of the key.
func aiCallError(where string, err error) *model.AppError {
	var pe *ai.ProviderError
	switch {
	case errors.Is(err, ai.ErrUnsupported):
		return model.NewAppError(where, "app.ai.call.unsupported.app_error", nil, "", http.StatusBadRequest)
	case errors.Is(err, context.DeadlineExceeded):
		return model.NewAppError(where, "app.ai.call.timeout.app_error", nil, "", http.StatusGatewayTimeout)
	case errors.As(err, &pe):
		switch {
		case pe.StatusCode == http.StatusTooManyRequests:
			return model.NewAppError(where, "app.ai.call.provider_rate_limited.app_error", nil, pe.Message, http.StatusTooManyRequests)
		case pe.StatusCode == http.StatusUnauthorized || pe.StatusCode == http.StatusForbidden:
			return model.NewAppError(where, "app.ai.call.provider_refused.app_error", nil, pe.Message, http.StatusBadGateway)
		case pe.StatusCode >= 400 && pe.StatusCode < 500:
			return model.NewAppError(where, "app.ai.call.provider_bad_request.app_error", nil, pe.Message, http.StatusBadRequest)
		}
		return model.NewAppError(where, "app.ai.call.provider_failed.app_error", nil, pe.Message, http.StatusBadGateway)
	default:
		return model.NewAppError(where, "app.ai.call.provider_failed.app_error", nil, ai.SanitizeProviderErrorMessage(err.Error()), http.StatusBadGateway)
	}
}

// AIChat runs a chat completion for a principal.
func (a *App) AIChat(rctx request.CTX, call AICall, req ai.ChatRequest) (*ai.ChatResponse, *model.AppError) {
	p, appErr := a.prepareAICall(rctx, call, req.ConnectionID, req.Model, ai.CapabilityChat, req.Operation)
	if appErr != nil {
		return nil, appErr
	}
	req.Model = p.model
	resp, err := p.provider.Chat(rctx.Context(), req)
	if err != nil {
		appErr := aiCallError("AIChat", err)
		a.finishAICall(rctx, p, ai.Usage{InputTokens: int64(ai.EstimateMessagesTokens(req.Messages))}, appErr)
		return nil, appErr
	}
	a.finishAICall(rctx, p, resp.Usage, nil)
	return resp, nil
}

// AIChatStream streams a chat completion for a principal. The stream's events are the provider's; its usage is
// counted when it ends. Closing the stream stops the call.
func (a *App) AIChatStream(rctx request.CTX, call AICall, req ai.ChatRequest) (*ai.Stream, *model.AppError) {
	p, appErr := a.prepareAICall(rctx, call, req.ConnectionID, req.Model, ai.CapabilityChat, req.Operation)
	if appErr != nil {
		return nil, appErr
	}
	req.Model = p.model
	ctx, cancel := context.WithCancel(context.WithoutCancel(rctx.Context()))
	upstream, err := p.provider.ChatStream(ctx, req)
	if err != nil {
		cancel()
		appErr := aiCallError("AIChatStream", err)
		a.finishAICall(rctx, p, ai.Usage{InputTokens: int64(ai.EstimateMessagesTokens(req.Messages))}, appErr)
		return nil, appErr
	}
	events := make(chan ai.StreamEvent, 16)
	go func() {
		defer close(events)
		defer upstream.Close()
		var usage ai.Usage
		var streamErr error
		for ev := range upstream.Events {
			switch ev.Type {
			case ai.EventUsage:
				if ev.Usage != nil {
					usage = *ev.Usage
				}
			case ai.EventError:
				streamErr = errors.New(ev.Error)
			}
			select {
			case events <- ev:
			case <-ctx.Done():
				// The reader stopped: drain what the provider still sends so that it ends.
			}
		}
		if streamErr == nil && ctx.Err() != nil {
			streamErr = ctx.Err()
		}
		a.finishAICall(rctx, p, usage, streamErr)
	}()
	return ai.NewStream(events, cancel), nil
}

// AIEmbed computes embeddings for a principal.
func (a *App) AIEmbed(rctx request.CTX, call AICall, req ai.EmbedRequest) (*ai.EmbedResponse, *model.AppError) {
	if len(req.Input) == 0 {
		return &ai.EmbedResponse{Embeddings: [][]float32{}}, nil
	}
	p, appErr := a.prepareAICall(rctx, call, req.ConnectionID, req.Model, ai.CapabilityEmbeddings, req.Operation)
	if appErr != nil {
		return nil, appErr
	}
	req.Model = p.model
	resp, err := p.provider.Embed(rctx.Context(), req)
	if err != nil {
		appErr := aiCallError("AIEmbed", err)
		a.finishAICall(rctx, p, ai.Usage{}, appErr)
		return nil, appErr
	}
	a.finishAICall(rctx, p, resp.Usage, nil)
	return resp, nil
}

// MaxAITranscriptionBytes bounds the audio of a transcription.
const MaxAITranscriptionBytes = 25 * 1024 * 1024

// AITranscribe turns speech into text for a principal.
func (a *App) AITranscribe(rctx request.CTX, call AICall, req ai.TranscribeRequest) (*ai.TranscribeResponse, *model.AppError) {
	if len(req.Audio) == 0 || len(req.Audio) > MaxAITranscriptionBytes {
		return nil, model.NewAppError("AITranscribe", "app.ai.call.audio_size.app_error", nil, "", http.StatusBadRequest)
	}
	p, appErr := a.prepareAICall(rctx, call, req.ConnectionID, req.Model, ai.CapabilityTranscription, req.Operation)
	if appErr != nil {
		return nil, appErr
	}
	req.Model = p.model
	resp, err := p.provider.Transcribe(rctx.Context(), req)
	if err != nil {
		appErr := aiCallError("AITranscribe", err)
		a.finishAICall(rctx, p, ai.Usage{}, appErr)
		return nil, appErr
	}
	a.finishAICall(rctx, p, resp.Usage, nil)
	return resp, nil
}

// AIDecide asks a decision model its questions for a principal.
func (a *App) AIDecide(rctx request.CTX, call AICall, req ai.DecideRequest) (*ai.DecideResponse, *model.AppError) {
	if err := req.Validate(); err != nil {
		return nil, model.NewAppError("AIDecide", "app.ai.call.decision.app_error", nil, err.Error(), http.StatusBadRequest)
	}
	p, appErr := a.prepareAICall(rctx, call, req.ConnectionID, req.Model, ai.CapabilityDecisions, req.Operation)
	if appErr != nil {
		return nil, appErr
	}
	req.Model = p.model
	resp, err := p.provider.Decide(rctx.Context(), req)
	if err != nil {
		appErr := aiCallError("AIDecide", err)
		a.finishAICall(rctx, p, ai.Usage{InputTokens: int64(ai.EstimateTokens(req.Input))}, appErr)
		return nil, appErr
	}
	a.finishAICall(rctx, p, resp.Usage, nil)
	return resp, nil
}

// AISpeak reads a text aloud for a principal, with a connection's OpenAI-compatible speech endpoint. The model is
// required: connections have no default speech model.
func (a *App) AISpeak(rctx request.CTX, call AICall, req ai.SpeechRequest) (*ai.SpeechResponse, *model.AppError) {
	if strings.TrimSpace(req.Model) == "" {
		return nil, model.NewAppError("AISpeak", "app.ai.call.no_model.app_error", map[string]any{"Capability": "speech"}, "", http.StatusBadRequest)
	}
	p, appErr := a.prepareAICall(rctx, call, req.ConnectionID, req.Model, ai.CapabilityChat, "speech")
	if appErr != nil {
		return nil, appErr
	}
	speaker, ok := p.provider.(ai.SpeechProvider)
	if !ok {
		appErr := model.NewAppError("AISpeak", "app.ai.call.unsupported.app_error", nil, "speech", http.StatusBadRequest)
		a.finishAICall(rctx, p, ai.Usage{}, appErr)
		return nil, appErr
	}
	req.Model = p.model
	resp, err := speaker.Speak(rctx.Context(), req)
	if err != nil {
		appErr := aiCallError("AISpeak", err)
		a.finishAICall(rctx, p, ai.Usage{}, appErr)
		return nil, appErr
	}
	a.finishAICall(rctx, p, ai.Usage{InputTokens: int64(ai.EstimateTokens(req.Input))}, nil)
	return resp, nil
}

// aiFeatureCall runs a chat for a built-in feature with the connection and model of its settings.
func (a *App) aiFeatureCall(rctx request.CTX, purpose ai.Purpose, settings *model.AIFeatureSettings, userID, channelID string, req ai.ChatRequest) (*ai.ChatResponse, *model.AppError) {
	if !settings.IsConfigured() {
		return nil, model.NewAppError("aiFeatureCall", "app.ai.feature.not_configured.app_error", map[string]any{"Feature": string(purpose)}, "", http.StatusNotImplemented)
	}
	req.ConnectionID = *settings.ConnectionId
	if req.Model == "" {
		req.Model = model.SafeDereference(settings.Model)
	}
	return a.AIChat(rctx, AICall{UserID: userID, Purpose: purpose, ChannelID: channelID}, req)
}
