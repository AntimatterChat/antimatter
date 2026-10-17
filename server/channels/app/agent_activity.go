// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package app

import (
	"errors"
	"net/http"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

// Agent activity (Antimatter): what an agent is doing now (idle, thinking, working, waiting for someone, blocked,
// offline), a few words on its task and the thread it works in. The agent sets it (an external agent with its token,
// the runtime for built-in ones), or its managers do. Everyone who can see the agent gets it live: a server agent's
// to everyone, a community agent's to its community, and guests only for an agent they can see (as user_updated). The
// thread is never sent live; it's given only to those who may read it.

// SetAgentActivity sets an agent's activity from a request and tells everyone who can see the agent.
func (a *App) SetAgentActivity(rctx request.CTX, agent *model.AIAgent, req *model.AgentActivityRequest) (*model.AgentActivity, *model.AppError) {
	activity, appErr := req.Validate(agent.UserId, model.GetMillis())
	if appErr != nil {
		return nil, appErr
	}
	if err := a.Srv().Store().AgentActivity().Save(rctx, activity); err != nil {
		return nil, model.NewAppError("SetAgentActivity", "app.ai.agent.activity.save.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	a.publishAgentActivity(agent, activity)
	return activity, nil
}

// setAgentActivityQuietly sets an activity the server decided on (the runtime's, a disabled agent's), logging what
// fails instead of failing.
func (a *App) setAgentActivityQuietly(rctx request.CTX, agent *model.AIAgent, activity *model.AgentActivity) {
	if err := a.Srv().Store().AgentActivity().Save(rctx, activity); err != nil {
		rctx.Logger().Warn("Couldn't save an agent's activity", mlog.String("user_id", agent.UserId), mlog.Err(err))
		return
	}
	a.publishAgentActivity(agent, activity)
}

// agentActivityEvent is agent_activity_changed for everyone who can see the agent (a community agent's for its
// community), without the thread.
func agentActivityEvent(agent *model.AIAgent, activity *model.AgentActivity) *model.WebSocketEvent {
	message := model.NewWebSocketEvent(model.WebsocketEventAgentActivityChanged, agent.TeamId(), "", "", nil, "")
	message.Add("user_id", agent.UserId)
	message.Add("activity", activity.WithoutThread())
	return message
}

// publishAgentActivity sends an agent's activity live.
func (a *App) publishAgentActivity(agent *model.AIAgent, activity *model.AgentActivity) {
	a.Publish(agentActivityEvent(agent, activity))
}

// readableAgentActivity gives an activity as a reader sees it: idle once expired, and without the thread unless they
// may read it.
func (a *App) readableAgentActivity(rctx request.CTX, activity *model.AgentActivity, readerID string) *model.AgentActivity {
	effective := activity.Effective(model.GetMillis())
	if effective.ThreadPostId != "" && !a.HasPermissionToChannelByPost(rctx, readerID, effective.ThreadPostId, model.PermissionReadChannelContent) {
		effective = effective.WithoutThread()
	}
	return effective
}

// GetAgentActivity gives an agent's activity as a reader sees it; idle when it never set one.
func (a *App) GetAgentActivity(rctx request.CTX, agentID, readerID string) (*model.AgentActivity, *model.AppError) {
	activity, err := a.Srv().Store().AgentActivity().Get(rctx, agentID)
	if err != nil {
		var nfErr *store.ErrNotFound
		if errors.As(err, &nfErr) {
			return &model.AgentActivity{UserId: agentID, State: model.AgentStateIdle}, nil
		}
		return nil, model.NewAppError("GetAgentActivity", "app.ai.agent.activity.get.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	return a.readableAgentActivity(rctx, activity, readerID), nil
}

// GetAgentActivities gives the activities of the agents among userIDs that the reader can see; the other IDs are left
// out.
func (a *App) GetAgentActivities(rctx request.CTX, userIDs []string, readerID string) ([]*model.AgentActivity, *model.AppError) {
	directory := a.aiAgentDirectory(rctx)
	agentIDs := make([]string, 0, len(userIDs))
	for _, id := range userIDs {
		if directory.get(id) == nil {
			continue
		}
		if canSee, appErr := a.UserCanSeeOtherUser(rctx, readerID, id); appErr != nil || !canSee {
			continue
		}
		agentIDs = append(agentIDs, id)
	}
	if len(agentIDs) == 0 {
		return []*model.AgentActivity{}, nil
	}
	stored, err := a.Srv().Store().AgentActivity().GetMany(rctx, agentIDs)
	if err != nil {
		return nil, model.NewAppError("GetAgentActivities", "app.ai.agent.activity.get.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	byID := make(map[string]*model.AgentActivity, len(stored))
	for _, activity := range stored {
		byID[activity.UserId] = activity
	}
	out := make([]*model.AgentActivity, 0, len(agentIDs))
	for _, id := range agentIDs {
		if activity := byID[id]; activity != nil {
			out = append(out, a.readableAgentActivity(rctx, activity, readerID))
		} else {
			out = append(out, &model.AgentActivity{UserId: id, State: model.AgentStateIdle})
		}
	}
	return out, nil
}

// SessionCanSetAgentActivity tells whether a session may set an agent's activity: the agent itself (its token), or
// whoever manages it.
func (a *App) SessionCanSetAgentActivity(session model.Session, agent *model.AIAgent) bool {
	if session.UserId == agent.UserId {
		return true
	}
	return a.SessionCanManageAIAgent(session, agent)
}
