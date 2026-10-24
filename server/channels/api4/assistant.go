// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package api4

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/mattermost/mattermost/server/public/model"
)

// Antimatter: the assistant (stage 1 of the ambient assistant), at /api/v4/assistant. Every route is the session's
// person's own: their feed in a channel they may read (a member, for private channels and direct and group
// messages), their panel's presence, their switch per channel, their cards and muted tools. No route gives anyone
// else's feed, to anyone. A community's settings are its admins' (manage_team_ai_connections), when the server lets
// them change any. The routes are the server's own: they don't depend on any app (the panel is an app of its own).

func (api *API) InitAssistant() {
	root := api.BaseRoutes.APIRoot.PathPrefix("/assistant").Subrouter()
	root.Handle("/muted_tools", api.APISessionRequired(getAssistantMutedTools)).Methods(http.MethodGet)
	root.Handle("/muted_tools", api.APISessionRequired(setAssistantMutedTools)).Methods(http.MethodPut)
	root.Handle("/communities/{team_id:[A-Za-z0-9]+}/settings", api.APISessionRequired(getAssistantCommunitySettings)).Methods(http.MethodGet)
	root.Handle("/communities/{team_id:[A-Za-z0-9]+}/settings", api.APISessionRequired(setAssistantCommunitySettings)).Methods(http.MethodPut)

	channel := root.PathPrefix("/channels/{channel_id:[A-Za-z0-9]+}").Subrouter()
	channel.Handle("/feed", api.APISessionRequired(getAssistantFeed)).Methods(http.MethodGet)
	channel.Handle("/presence", api.APISessionRequired(setAssistantPresence)).Methods(http.MethodPost)
	channel.Handle("/enabled", api.APISessionRequired(setAssistantChannelOn)).Methods(http.MethodPut)

	card := channel.PathPrefix("/cards/{card_id:[A-Za-z0-9]+}").Subrouter()
	card.Handle("/dismiss", api.APISessionRequired(dismissAssistantCard)).Methods(http.MethodPost)
	card.Handle("/pin", api.APISessionRequired(pinAssistantCard)).Methods(http.MethodPost)
	card.Handle("/mute", api.APISessionRequired(muteAssistantCard)).Methods(http.MethodPost)
	card.Handle("/claim", api.APISessionRequired(claimAssistantCard)).Methods(http.MethodPost)
	card.Handle("/run", api.APISessionRequired(runAssistantCard)).Methods(http.MethodPost)
	card.Handle("/post", api.APISessionRequired(postAssistantCard)).Methods(http.MethodPost)
}

// assistantChannelID gives the channel of the request.
func assistantChannelID(c *Context, r *http.Request) string {
	id := mux.Vars(r)["channel_id"]
	if !model.IsValidId(id) {
		c.SetInvalidURLParam("channel_id")
		return ""
	}
	return id
}

// assistantCardIDs gives the channel and the card of the request.
func assistantCardIDs(c *Context, r *http.Request) (string, string) {
	channelID := assistantChannelID(c, r)
	if channelID == "" {
		return "", ""
	}
	cardID := mux.Vars(r)["card_id"]
	if !model.IsValidId(cardID) {
		c.SetInvalidURLParam("card_id")
		return "", ""
	}
	return channelID, cardID
}

func decodeAssistantBody(c *Context, w http.ResponseWriter, r *http.Request, name string, into any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64*1024)).Decode(into); err != nil {
		c.SetInvalidParamWithErr(name, err)
		return false
	}
	return true
}

func getAssistantFeed(c *Context, w http.ResponseWriter, r *http.Request) {
	channelID := assistantChannelID(c, r)
	if channelID == "" {
		return
	}
	feed, appErr := c.App.GetAssistantFeed(c.AppContext, c.AppContext.Session().UserId, channelID)
	if appErr != nil {
		c.Err = appErr
		return
	}
	writeJSON(c, w, feed)
}

func setAssistantPresence(c *Context, w http.ResponseWriter, r *http.Request) {
	channelID := assistantChannelID(c, r)
	if channelID == "" {
		return
	}
	var req model.AssistantPresenceRequest
	if !decodeAssistantBody(c, w, r, "presence", &req) {
		return
	}
	presence, appErr := c.App.SetAssistantPresence(c.AppContext, c.AppContext.Session().UserId, channelID, req.Open)
	if appErr != nil {
		c.Err = appErr
		return
	}
	writeJSON(c, w, presence)
}

func setAssistantChannelOn(c *Context, w http.ResponseWriter, r *http.Request) {
	channelID := assistantChannelID(c, r)
	if channelID == "" {
		return
	}
	var req model.AssistantChannelSwitch
	if !decodeAssistantBody(c, w, r, "enabled", &req) {
		return
	}
	if appErr := c.App.SetAssistantChannelOn(c.AppContext, c.AppContext.Session().UserId, channelID, req.On); appErr != nil {
		c.Err = appErr
		return
	}
	writeJSON(c, w, req)
}

func dismissAssistantCard(c *Context, w http.ResponseWriter, r *http.Request) {
	channelID, cardID := assistantCardIDs(c, r)
	if cardID == "" {
		return
	}
	card, appErr := c.App.DismissAssistantCard(c.AppContext, c.AppContext.Session().UserId, channelID, cardID)
	if appErr != nil {
		c.Err = appErr
		return
	}
	writeJSON(c, w, card)
}

func pinAssistantCard(c *Context, w http.ResponseWriter, r *http.Request) {
	channelID, cardID := assistantCardIDs(c, r)
	if cardID == "" {
		return
	}
	var req model.AssistantPinRequest
	if !decodeAssistantBody(c, w, r, "pin", &req) {
		return
	}
	card, appErr := c.App.PinAssistantCard(c.AppContext, c.AppContext.Session().UserId, channelID, cardID, req.Pinned)
	if appErr != nil {
		c.Err = appErr
		return
	}
	writeJSON(c, w, card)
}

func muteAssistantCard(c *Context, w http.ResponseWriter, r *http.Request) {
	channelID, cardID := assistantCardIDs(c, r)
	if cardID == "" {
		return
	}
	card, appErr := c.App.MuteAssistantCardTool(c.AppContext, c.AppContext.Session().UserId, channelID, cardID)
	if appErr != nil {
		c.Err = appErr
		return
	}
	writeJSON(c, w, card)
}

func claimAssistantCard(c *Context, w http.ResponseWriter, r *http.Request) {
	channelID, cardID := assistantCardIDs(c, r)
	if cardID == "" {
		return
	}
	card, appErr := c.App.ClaimAssistantCard(c.AppContext, c.AppContext.Session().UserId, channelID, cardID)
	if appErr != nil {
		c.Err = appErr
		return
	}
	writeJSON(c, w, card)
}

func runAssistantCard(c *Context, w http.ResponseWriter, r *http.Request) {
	channelID, cardID := assistantCardIDs(c, r)
	if cardID == "" {
		return
	}
	result, appErr := c.App.RunAssistantCard(c.AppContext, c.AppContext.Session().UserId, channelID, cardID)
	if appErr != nil {
		c.Err = appErr
		return
	}
	writeJSON(c, w, result)
}

func postAssistantCard(c *Context, w http.ResponseWriter, r *http.Request) {
	channelID, cardID := assistantCardIDs(c, r)
	if cardID == "" {
		return
	}
	post, appErr := c.App.PostAssistantCard(c.AppContext, c.AppContext.Session().UserId, channelID, cardID)
	if appErr != nil {
		c.Err = appErr
		return
	}
	w.WriteHeader(http.StatusCreated)
	writeJSON(c, w, post)
}

func getAssistantMutedTools(c *Context, w http.ResponseWriter, r *http.Request) {
	writeJSON(c, w, c.App.GetAssistantMutedTools(c.AppContext.Session().UserId))
}

func setAssistantMutedTools(c *Context, w http.ResponseWriter, r *http.Request) {
	var req model.AssistantMutedTools
	if !decodeAssistantBody(c, w, r, "muted_tools", &req) {
		return
	}
	muted, appErr := c.App.SetAssistantMutedTools(c.AppContext, c.AppContext.Session().UserId, &req)
	if appErr != nil {
		c.Err = appErr
		return
	}
	writeJSON(c, w, muted)
}

// assistantCommunity gives the community of the request when the session manages its AI settings.
func assistantCommunity(c *Context, r *http.Request) string {
	teamID := mux.Vars(r)["team_id"]
	if !model.IsValidId(teamID) {
		c.SetInvalidURLParam("team_id")
		return ""
	}
	if !c.App.SessionCanManageAIConnectionScope(*c.AppContext.Session(), teamID) {
		c.SetPermissionError(model.PermissionManageTeamAIConnections)
		return ""
	}
	return teamID
}

func getAssistantCommunitySettings(c *Context, w http.ResponseWriter, r *http.Request) {
	teamID := assistantCommunity(c, r)
	if teamID == "" {
		return
	}
	writeJSON(c, w, c.App.GetAssistantCommunitySettings(teamID))
}

func setAssistantCommunitySettings(c *Context, w http.ResponseWriter, r *http.Request) {
	teamID := assistantCommunity(c, r)
	if teamID == "" {
		return
	}
	var req model.AssistantCommunitySettings
	if !decodeAssistantBody(c, w, r, "settings", &req) {
		return
	}
	auditRec := c.MakeAuditRecord(model.AuditEventUpdateAssistantCommunity, model.AuditStatusFail)
	defer c.LogAuditRec(auditRec)
	model.AddEventParameterToAuditRec(auditRec, "team_id", teamID)
	info, appErr := c.App.SetAssistantCommunitySettings(c.AppContext, teamID, &req)
	if appErr != nil {
		c.Err = appErr
		return
	}
	model.AddEventParameterToAuditRec(auditRec, "engine", req.Engine)
	model.AddEventParameterToAuditRec(auditRec, "connection_id", req.ConnectionId)
	auditRec.Success()
	writeJSON(c, w, info)
}
