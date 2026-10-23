// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package model

import (
	"context"
	"net/url"
)

// Agents' tools and MCP servers (Antimatter): /api/v4/ai/agents/{agent_id}/tools, /api/v4/ai/agents/tools/catalogue,
// /api/v4/ai/tools/calls and /api/v4/ai/mcp_servers.

func (c *Client4) aiMCPServersRoute() clientRoute {
	return newClientRoute("ai").Join("mcp_servers")
}

func (c *Client4) aiMCPServerRoute(serverID string) clientRoute {
	return c.aiMCPServersRoute().Join(serverID)
}

// GetAIAgentTools lists the tools an agent may use for the caller (in a channel, or none).
func (c *Client4) GetAIAgentTools(ctx context.Context, agentID, channelID string) ([]*AIToolSourceInfo, *Response, error) {
	query := url.Values{}
	if channelID != "" {
		query.Set("channel_id", channelID)
	}
	r, err := c.doAPIGetWithQuery(ctx, c.aiAgentRoute(agentID).Join("tools"), query, "")
	if err != nil {
		return nil, BuildResponse(r), err
	}
	defer closeBody(r)
	return DecodeJSONFromResponse[[]*AIToolSourceInfo](r)
}

// GetAIToolCatalogue lists what the agents of a scope ("server" or a team ID) may be given.
func (c *Client4) GetAIToolCatalogue(ctx context.Context, scope string) (*AIToolCatalogue, *Response, error) {
	query := url.Values{}
	query.Set("scope", scope)
	r, err := c.doAPIGetWithQuery(ctx, c.aiAgentsRoute().Join("tools", "catalogue"), query, "")
	if err != nil {
		return nil, BuildResponse(r), err
	}
	defer closeBody(r)
	return DecodeJSONFromResponse[*AIToolCatalogue](r)
}

// AnswerAIToolCall confirms (or declines) a tool call an agent proposed to the caller.
func (c *Client4) AnswerAIToolCall(ctx context.Context, callID string, confirm bool) (*AIToolCallSummary, *Response, error) {
	answer := "decline"
	if confirm {
		answer = "confirm"
	}
	r, err := c.doAPIPostJSON(ctx, newClientRoute("ai").Join("tools", "calls", callID, answer), nil)
	if err != nil {
		return nil, BuildResponse(r), err
	}
	defer closeBody(r)
	return DecodeJSONFromResponse[*AIToolCallSummary](r)
}

// GetAIMCPServers lists the MCP servers the caller manages: those of a scope, or all of them with an empty scope.
func (c *Client4) GetAIMCPServers(ctx context.Context, scope string) ([]*AIMCPServer, *Response, error) {
	query := url.Values{}
	if scope != "" {
		query.Set("scope", scope)
	}
	r, err := c.doAPIGetWithQuery(ctx, c.aiMCPServersRoute(), query, "")
	if err != nil {
		return nil, BuildResponse(r), err
	}
	defer closeBody(r)
	return DecodeJSONFromResponse[[]*AIMCPServer](r)
}

// CreateAIMCPServer adds an MCP server.
func (c *Client4) CreateAIMCPServer(ctx context.Context, request *AIMCPServerRequest) (*AIMCPServer, *Response, error) {
	r, err := c.doAPIPostJSON(ctx, c.aiMCPServersRoute(), request)
	if err != nil {
		return nil, BuildResponse(r), err
	}
	defer closeBody(r)
	return DecodeJSONFromResponse[*AIMCPServer](r)
}

// GetAIMCPServer gets an MCP server the caller manages.
func (c *Client4) GetAIMCPServer(ctx context.Context, serverID string) (*AIMCPServer, *Response, error) {
	r, err := c.doAPIGet(ctx, c.aiMCPServerRoute(serverID), "")
	if err != nil {
		return nil, BuildResponse(r), err
	}
	defer closeBody(r)
	return DecodeJSONFromResponse[*AIMCPServer](r)
}

// PatchAIMCPServer changes an MCP server.
func (c *Client4) PatchAIMCPServer(ctx context.Context, serverID string, patch *AIMCPServerPatch) (*AIMCPServer, *Response, error) {
	r, err := c.doAPIPatchJSON(ctx, c.aiMCPServerRoute(serverID), patch)
	if err != nil {
		return nil, BuildResponse(r), err
	}
	defer closeBody(r)
	return DecodeJSONFromResponse[*AIMCPServer](r)
}

// DeleteAIMCPServer deletes an MCP server and everyone's sign-in to it.
func (c *Client4) DeleteAIMCPServer(ctx context.Context, serverID string) (*Response, error) {
	r, err := c.doAPIDelete(ctx, c.aiMCPServerRoute(serverID))
	if err != nil {
		return BuildResponse(r), err
	}
	defer closeBody(r)
	return BuildResponse(r), nil
}

// ConnectAIMCPServer starts the caller's sign-in to an MCP server: it gives where to send them.
func (c *Client4) ConnectAIMCPServer(ctx context.Context, serverID string) (*AIMCPOAuthConnect, *Response, error) {
	r, err := c.doAPIPostJSON(ctx, c.aiMCPServerRoute(serverID).Join("oauth"), nil)
	if err != nil {
		return nil, BuildResponse(r), err
	}
	defer closeBody(r)
	return DecodeJSONFromResponse[*AIMCPOAuthConnect](r)
}

// DisconnectAIMCPServer forgets the caller's sign-in to an MCP server.
func (c *Client4) DisconnectAIMCPServer(ctx context.Context, serverID string) (*Response, error) {
	r, err := c.doAPIDelete(ctx, c.aiMCPServerRoute(serverID).Join("oauth"))
	if err != nil {
		return BuildResponse(r), err
	}
	defer closeBody(r)
	return BuildResponse(r), nil
}
