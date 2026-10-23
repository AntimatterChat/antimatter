// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package app

import (
	"errors"
	"net/http"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

// Agent statuses (Antimatter): a built-in agent's status (online, away, offline) is the server's to set, never a
// person's. It says whether the agent can answer:
//
//   - online: it's on, and its model connection is set up (a connection it may use, with a chat model) and answers;
//   - away: it's on and set up, but the connection's endpoint doesn't answer (unreachable, refused key, timeout);
//   - offline: it's off, or it has no usable connection or model.
//
// The server sets it when an agent or a connection changes, when an admin tests a saved connection, when an agent's
// call to its model reaches the endpoint or fails to, and every agentStatusInterval on the cluster leader. The status
// is saved as manual, so that no automatic change (a websocket closing, inactivity) overrides it.
//
// An external agent is a program that signs in with the account's token: its status follows its own connection, like
// any account's, and only its own token may change it. Nobody else may set an agent's status: not its managers, not
// system admins, not apps.

// agentStatusInterval is how often the cluster leader checks the agents' connections.
const agentStatusInterval = 10 * time.Minute

// aiAgentStatusFor gives the status of a built-in agent: offline when it's off or not set up, else online or away as
// its connection answers.
func aiAgentStatusFor(active, setUp, reachable bool) string {
	switch {
	case !active || !setUp:
		return model.StatusOffline
	case reachable:
		return model.StatusOnline
	default:
		return model.StatusAway
	}
}

// aiAgentConnection gives the connection a built-in agent answers with when it's set up: one it may use (not deleted,
// not needing attention, with chat, serving its scope), and a model (its own or the connection's chat model). Nil when
// it isn't set up.
func (a *App) aiAgentConnection(rctx request.CTX, agent *model.AIAgent) *model.AIConnection {
	if agent.External || !model.IsValidId(agent.ConnectionId) {
		return nil
	}
	c, appErr := a.GetAIConnection(rctx, agent.ConnectionId)
	if appErr != nil || !aiAgentMayUseConnection(agent.Scope, c) {
		return nil
	}
	if agent.Model == "" && c.Models.Chat == "" {
		return nil
	}
	return c
}

// aiConnectionAnswers tells whether a connection's endpoint answers (its test). The end-to-end tests' stand-in model
// answers for its connection.
func (a *App) aiConnectionAnswers(rctx request.CTX, c *model.AIConnection) bool {
	if m := a.aiE2EModel(); m != nil && m.serves(c.Id) {
		return true
	}
	return a.testAIConnection(rctx, c).OK
}

// aiAgentAccountActive tells whether an agent's account is on (a disabled agent's is deactivated).
func (a *App) aiAgentAccountActive(rctx request.CTX, userID string) bool {
	user, appErr := a.GetUser(rctx, userID)
	return appErr == nil && user.DeleteAt == 0
}

// setAIAgentStatus saves and sends a status the server decided for an agent, when it changes.
func (a *App) setAIAgentStatus(userID, status string) {
	if !*a.Config().ServiceSettings.EnableUserStatuses {
		return
	}
	ps := a.Srv().Platform()
	if current, appErr := ps.GetStatus(userID); appErr == nil && current.Status == status && current.Manual {
		return
	}
	ps.SaveAndBroadcastStatus(&model.Status{UserId: userID, Status: status, Manual: true, LastActivityAt: model.GetMillis()})
}

// RefreshAIAgentStatuses sets the status of the built-in agents (those on connectionID only, when given), testing
// each connection they use once.
func (a *App) RefreshAIAgentStatuses(rctx request.CTX, connectionID string) {
	answers := map[string]bool{}
	for _, agent := range a.aiAgentDirectory(rctx).all() {
		if agent.External || (connectionID != "" && agent.ConnectionId != connectionID) {
			continue
		}
		a.refreshAIAgentStatus(rctx, agent, answers)
	}
}

// refreshAIAgentStatus sets one built-in agent's status; answers remembers the connections already tested.
func (a *App) refreshAIAgentStatus(rctx request.CTX, agent *model.AIAgent, answers map[string]bool) {
	if agent.External {
		return
	}
	active := a.aiAgentAccountActive(rctx, agent.UserId)
	c := a.aiAgentConnection(rctx, agent)
	reachable := false
	if active && c != nil {
		known, tested := answers[c.Id]
		if !tested {
			known = a.aiConnectionAnswers(rctx, c)
			answers[c.Id] = known
		}
		reachable = known
	}
	a.setAIAgentStatus(agent.UserId, aiAgentStatusFor(active, c != nil, reachable))
}

// refreshAIAgentStatusesLater refreshes the agents' statuses in the background (a connection's test may take a
// while): those on connectionID, or the one agent given.
func (a *App) refreshAIAgentStatusesLater(connectionID string, agent *model.AIAgent) {
	a.Srv().Go(func() {
		rctx := request.EmptyContext(a.Log())
		if agent != nil {
			a.refreshAIAgentStatus(rctx, agent, map[string]bool{})
			return
		}
		a.RefreshAIAgentStatuses(rctx, connectionID)
	})
}

// noteAIConnectionAnswers sets the status of the built-in agents on a connection from what is known of its endpoint
// (a test, or an agent's call to it), without testing it again.
func (a *App) noteAIConnectionAnswers(rctx request.CTX, connectionID string, reachable bool) {
	if !model.IsValidId(connectionID) {
		return
	}
	for _, agent := range a.aiAgentDirectory(rctx).all() {
		if agent.External || agent.ConnectionId != connectionID {
			continue
		}
		a.refreshAIAgentStatus(rctx, agent, map[string]bool{connectionID: reachable})
	}
}

// aiCallReachedEndpoint tells what a model call's outcome says of its endpoint (reachable), when it says anything
// (known): it answered (also when it refused the request itself or its rate), or it didn't answer or refused the key.
// An error from before the call (limits, budgets, access, a missing model) says nothing of it.
func aiCallReachedEndpoint(appErr *model.AppError) (reachable, known bool) {
	if appErr == nil {
		return true, true
	}
	switch appErr.Id {
	case "app.ai.call.provider_failed.app_error", "app.ai.call.timeout.app_error", "app.ai.call.provider_refused.app_error":
		return false, true
	case "app.ai.call.provider_rate_limited.app_error", "app.ai.call.provider_bad_request.app_error":
		return true, true
	}
	return false, false
}

// SessionMaySetUserStatus tells whether a session may set an account's status (online, away, do not disturb,
// offline, and its custom status), on top of the usual permission: always for an account that isn't an agent's; for
// an external agent, only with its own token; for a built-in agent, never (the server sets it). When it can't tell,
// it refuses.
func (a *App) SessionMaySetUserStatus(rctx request.CTX, session model.Session, userID string) *model.AppError {
	// The node's list of agents answers first (clients send their activity often); the database when it couldn't load.
	agent, loaded := a.aiAgentDirectory(rctx).lookup(userID)
	if !loaded {
		stored, err := a.Srv().Store().AIAgent().Get(rctx, userID)
		if err != nil {
			var nfErr *store.ErrNotFound
			if errors.As(err, &nfErr) {
				return nil
			}
			return model.NewAppError("SessionMaySetUserStatus", "app.ai.agent.get.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
		}
		agent = stored
	}
	if agent == nil {
		return nil
	}
	// A deleted agent's account is deactivated: its status stays the server's too.
	if agent.DeleteAt == 0 && agent.External && session.UserId == agent.UserId {
		return nil
	}
	rctx.Logger().Debug("Refused to set an agent's status: the server sets it", mlog.String("user_id", userID), mlog.String("session_user_id", session.UserId))
	return model.NewAppError("SessionMaySetUserStatus", "api.status.agent.app_error", nil, "", http.StatusForbidden)
}
