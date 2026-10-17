// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package model

import (
	"context"
	"net/url"

	"github.com/mattermost/mattermost/server/public/ai"
)

// Agents (Antimatter): /api/v4/ai/agents.

func (c *Client4) aiAgentsRoute() clientRoute {
	return newClientRoute("ai").Join("agents")
}

func (c *Client4) aiAgentRoute(userID string) clientRoute {
	return c.aiAgentsRoute().Join(userID)
}

// GetAIAgents lists the agents the caller manages: those of a scope ("server" or a team ID), or all of them with an
// empty scope.
func (c *Client4) GetAIAgents(ctx context.Context, scope string) ([]*AIAgentDetails, *Response, error) {
	query := url.Values{}
	if scope != "" {
		query.Set("scope", scope)
	}
	r, err := c.doAPIGetWithQuery(ctx, c.aiAgentsRoute(), query, "")
	if err != nil {
		return nil, BuildResponse(r), err
	}
	defer closeBody(r)
	return DecodeJSONFromResponse[[]*AIAgentDetails](r)
}

// CreateAIAgent makes an agent and its account.
func (c *Client4) CreateAIAgent(ctx context.Context, request *AIAgentRequest) (*AIAgentDetails, *Response, error) {
	r, err := c.doAPIPostJSON(ctx, c.aiAgentsRoute(), request)
	if err != nil {
		return nil, BuildResponse(r), err
	}
	defer closeBody(r)
	return DecodeJSONFromResponse[*AIAgentDetails](r)
}

// GetAIAgent gives an agent the caller manages.
func (c *Client4) GetAIAgent(ctx context.Context, userID string) (*AIAgentDetails, *Response, error) {
	r, err := c.doAPIGet(ctx, c.aiAgentRoute(userID), "")
	if err != nil {
		return nil, BuildResponse(r), err
	}
	defer closeBody(r)
	return DecodeJSONFromResponse[*AIAgentDetails](r)
}

// PatchAIAgent changes an agent.
func (c *Client4) PatchAIAgent(ctx context.Context, userID string, patch *AIAgentPatch) (*AIAgentDetails, *Response, error) {
	r, err := c.doAPIPatchJSON(ctx, c.aiAgentRoute(userID), patch)
	if err != nil {
		return nil, BuildResponse(r), err
	}
	defer closeBody(r)
	return DecodeJSONFromResponse[*AIAgentDetails](r)
}

// DeleteAIAgent deletes an agent; its account is deactivated.
func (c *Client4) DeleteAIAgent(ctx context.Context, userID string) (*Response, error) {
	r, err := c.doAPIDelete(ctx, c.aiAgentRoute(userID))
	if err != nil {
		return BuildResponse(r), err
	}
	defer closeBody(r)
	return BuildResponse(r), nil
}

// SetAIAgentEnabled turns an agent's account on or off.
func (c *Client4) SetAIAgentEnabled(ctx context.Context, userID string, enabled bool) (*AIAgentDetails, *Response, error) {
	action := "disable"
	if enabled {
		action = "enable"
	}
	r, err := c.doAPIPost(ctx, c.aiAgentRoute(userID).Join(action), "")
	if err != nil {
		return nil, BuildResponse(r), err
	}
	defer closeBody(r)
	return DecodeJSONFromResponse[*AIAgentDetails](r)
}

// GetAIAgentConnections lists the connections an agent of a scope may use.
func (c *Client4) GetAIAgentConnections(ctx context.Context, scope string) ([]*AIConnectionInfo, *Response, error) {
	r, err := c.doAPIGetWithQuery(ctx, c.aiAgentsRoute().Join("connections"), url.Values{"scope": {scope}}, "")
	if err != nil {
		return nil, BuildResponse(r), err
	}
	defer closeBody(r)
	return DecodeJSONFromResponse[[]*AIConnectionInfo](r)
}

// GetAIAgentConnectionModels lists the models of a connection an agent of a scope may use.
func (c *Client4) GetAIAgentConnectionModels(ctx context.Context, scope, connectionID string) ([]ai.ModelInfo, *Response, error) {
	r, err := c.doAPIGetWithQuery(ctx, c.aiAgentsRoute().Join("connections", connectionID, "models"), url.Values{"scope": {scope}}, "")
	if err != nil {
		return nil, BuildResponse(r), err
	}
	defer closeBody(r)
	return DecodeJSONFromResponse[[]ai.ModelInfo](r)
}

// GetAgentActivity gives an agent's activity.
func (c *Client4) GetAgentActivity(ctx context.Context, userID string) (*AgentActivity, *Response, error) {
	r, err := c.doAPIGet(ctx, c.aiAgentRoute(userID).Join("activity"), "")
	if err != nil {
		return nil, BuildResponse(r), err
	}
	defer closeBody(r)
	return DecodeJSONFromResponse[*AgentActivity](r)
}

// SetAgentActivity sets an agent's activity: with the agent's own token, or as its manager.
func (c *Client4) SetAgentActivity(ctx context.Context, userID string, request *AgentActivityRequest) (*AgentActivity, *Response, error) {
	r, err := c.doAPIPutJSON(ctx, c.aiAgentRoute(userID).Join("activity"), request)
	if err != nil {
		return nil, BuildResponse(r), err
	}
	defer closeBody(r)
	return DecodeJSONFromResponse[*AgentActivity](r)
}

// GetAgentActivitiesByIds gives the activities of the agents among user IDs.
func (c *Client4) GetAgentActivitiesByIds(ctx context.Context, userIDs []string) ([]*AgentActivity, *Response, error) {
	r, err := c.doAPIPostJSON(ctx, c.aiAgentsRoute().Join("activity", "ids"), userIDs)
	if err != nil {
		return nil, BuildResponse(r), err
	}
	defer closeBody(r)
	return DecodeJSONFromResponse[[]*AgentActivity](r)
}
