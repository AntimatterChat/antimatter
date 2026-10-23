// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package app

import (
	"net/http"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

// Sign-in warnings (Antimatter): when someone signs in to an account (a password, SSO or magic link sign-in, on the
// web, the desktop or the mobile app), the account's connected sessions hear of it (session_created), to warn the
// person in case it wasn't them. They then disconnect that session (the revoke-session API, session_revoked) or
// confirm it was them (session_acknowledged), and the warning goes from all their sessions. Personal access tokens,
// apps' OAuth tokens and sessions an app creates aren't sign-ins, and warn nobody.
//
// The events go to the account's own sessions only, never to anyone else's (system admins included), and carry the
// session's id and what the sessions list already shows of it (device, browser, time): never its token, its CSRF
// token or its device id (a push notification token).

// sessionCreatedEvent is the session_created event for a new session.
func sessionCreatedEvent(session *model.Session) *model.WebSocketEvent {
	message := model.NewWebSocketEvent(model.WebsocketEventSessionCreated, "", "", session.UserId, nil, "")
	message.Add("session_id", session.Id)
	message.Add("create_at", session.CreateAt)
	message.Add("platform", session.Props[model.SessionPropPlatform])
	message.Add("os", session.Props[model.SessionPropOs])
	message.Add("browser", session.Props[model.SessionPropBrowser])
	message.Add("mobile", session.IsMobileApp())
	return message
}

// warnsOfSignIn tells whether a new session is someone signing in, to warn of: not an OAuth app's token, a personal
// access token nor a bot's session.
func warnsOfSignIn(session *model.Session) bool {
	return session != nil && session.UserId != "" && session.Id != "" && !session.IsOAuth && !session.IsUserAccessToken() && !session.IsBotUser()
}

// publishSessionCreated warns the user's other sessions of a sign-in. The new session itself isn't told (see
// WebConn.ShouldSendEvent): it has no connection yet, and wouldn't get the warning if it had one.
func (a *App) publishSessionCreated(session *model.Session) {
	if !warnsOfSignIn(session) {
		return
	}
	a.Publish(sessionCreatedEvent(session))
}

// publishSessionHandled takes a sign-in's warning off the user's sessions: the session was revoked, or confirmed.
func (a *App) publishSessionHandled(event model.WebsocketEventType, userID, sessionID string) {
	if userID == "" || sessionID == "" {
		return
	}
	message := model.NewWebSocketEvent(event, "", "", userID, nil, "")
	message.Add("session_id", sessionID)
	a.Publish(message)
}

// AcknowledgeSession confirms that one of the user's own sessions was them signing in ("It's me"): its warning goes
// from all their sessions. A session that isn't theirs, or no longer exists, is refused the same way.
func (a *App) AcknowledgeSession(rctx request.CTX, userID, sessionID string) *model.AppError {
	session, appErr := a.GetSessionById(rctx, sessionID)
	if appErr != nil || session.UserId != userID {
		return model.NewAppError("AcknowledgeSession", "api.context.invalid_body_param.app_error", map[string]any{"Name": "session_id"}, "", http.StatusBadRequest)
	}
	a.publishSessionHandled(model.WebsocketEventSessionAcknowledged, userID, session.Id)
	return nil
}
