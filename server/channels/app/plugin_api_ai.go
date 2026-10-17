// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package app

import (
	"net/http"

	"github.com/mattermost/mattermost/server/public/ai"
	"github.com/mattermost/mattermost/server/public/model"
)

// The plugin API of the AI service (Antimatter). An app always calls for a person, never as a built-in feature: the
// call carries the app's ID, set here, and the person the app got from the server (see plugin.API). The service
// checks that the person may use the connection where the call happens.

func (api *PluginAPI) aiCall(actingUserID, channelID, teamID string) AICall {
	return AICall{UserID: actingUserID, PluginID: api.id, ChannelID: channelID, TeamID: teamID}
}

func aiMissingRequest(where string) *model.AppError {
	return model.NewAppError(where, "app.ai.call.no_request.app_error", nil, "", http.StatusBadRequest)
}

func (api *PluginAPI) AIChat(actingUserID, channelID, teamID string, request *ai.ChatRequest) (*ai.ChatResponse, *model.AppError) {
	if request == nil {
		return nil, aiMissingRequest("AIChat")
	}
	return api.app.AIChat(api.ctx, api.aiCall(actingUserID, channelID, teamID), *request)
}

func (api *PluginAPI) AIChatStream(actingUserID, channelID, teamID string, request *ai.ChatRequest) (*ai.Stream, *model.AppError) {
	if request == nil {
		return nil, aiMissingRequest("AIChatStream")
	}
	return api.app.AIChatStream(api.ctx, api.aiCall(actingUserID, channelID, teamID), *request)
}

func (api *PluginAPI) AIEmbed(actingUserID, channelID, teamID string, request *ai.EmbedRequest) (*ai.EmbedResponse, *model.AppError) {
	if request == nil {
		return nil, aiMissingRequest("AIEmbed")
	}
	return api.app.AIEmbed(api.ctx, api.aiCall(actingUserID, channelID, teamID), *request)
}

func (api *PluginAPI) AITranscribe(actingUserID, channelID, teamID string, request *ai.TranscribeRequest) (*ai.TranscribeResponse, *model.AppError) {
	if request == nil {
		return nil, aiMissingRequest("AITranscribe")
	}
	return api.app.AITranscribe(api.ctx, api.aiCall(actingUserID, channelID, teamID), *request)
}

func (api *PluginAPI) AIDecide(actingUserID, channelID, teamID string, request *ai.DecideRequest) (*ai.DecideResponse, *model.AppError) {
	if request == nil {
		return nil, aiMissingRequest("AIDecide")
	}
	return api.app.AIDecide(api.ctx, api.aiCall(actingUserID, channelID, teamID), *request)
}

func (api *PluginAPI) AIConnectionsForUser(userID, channelID, teamID string) ([]*model.AIConnectionInfo, *model.AppError) {
	return api.app.GetAIConnectionsForUser(api.ctx, userID, channelID, teamID)
}
