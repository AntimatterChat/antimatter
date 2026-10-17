// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package model

import (
	"context"
	"net/url"

	"github.com/mattermost/mattermost/server/public/ai"
)

// Model connections (Antimatter): /api/v4/ai/connections.

func (c *Client4) aiConnectionsRoute() clientRoute {
	return newClientRoute("ai").Join("connections")
}

func (c *Client4) aiConnectionRoute(connectionID string) clientRoute {
	return c.aiConnectionsRoute().Join(connectionID)
}

// GetAIConnections lists the model connections the caller manages: those of a scope ("server" or a team ID), or all
// of them with an empty scope.
func (c *Client4) GetAIConnections(ctx context.Context, scope string) ([]*AIConnection, *Response, error) {
	query := url.Values{}
	if scope != "" {
		query.Set("scope", scope)
	}
	r, err := c.doAPIGetWithQuery(ctx, c.aiConnectionsRoute(), query, "")
	if err != nil {
		return nil, BuildResponse(r), err
	}
	defer closeBody(r)
	return DecodeJSONFromResponse[[]*AIConnection](r)
}

// GetUsableAIConnections lists the model connections the caller may use in a channel or community.
func (c *Client4) GetUsableAIConnections(ctx context.Context, channelID, teamID string) ([]*AIConnectionInfo, *Response, error) {
	query := url.Values{}
	if channelID != "" {
		query.Set("channel_id", channelID)
	}
	if teamID != "" {
		query.Set("team_id", teamID)
	}
	r, err := c.doAPIGetWithQuery(ctx, c.aiConnectionsRoute().Join("usable"), query, "")
	if err != nil {
		return nil, BuildResponse(r), err
	}
	defer closeBody(r)
	return DecodeJSONFromResponse[[]*AIConnectionInfo](r)
}

// CreateAIConnection adds a model connection.
func (c *Client4) CreateAIConnection(ctx context.Context, request *AIConnectionRequest) (*AIConnection, *Response, error) {
	r, err := c.doAPIPostJSON(ctx, c.aiConnectionsRoute(), request)
	if err != nil {
		return nil, BuildResponse(r), err
	}
	defer closeBody(r)
	return DecodeJSONFromResponse[*AIConnection](r)
}

// TestAIConnectionRequest tests a model connection before it's saved.
func (c *Client4) TestAIConnectionRequest(ctx context.Context, request *AIConnectionRequest) (*AIConnectionTestResult, *Response, error) {
	r, err := c.doAPIPostJSON(ctx, c.aiConnectionsRoute().Join("test"), request)
	if err != nil {
		return nil, BuildResponse(r), err
	}
	defer closeBody(r)
	return DecodeJSONFromResponse[*AIConnectionTestResult](r)
}

// GetAIConnectionRequestModels lists the models a connection's endpoint offers before it's saved.
func (c *Client4) GetAIConnectionRequestModels(ctx context.Context, request *AIConnectionRequest) ([]ai.ModelInfo, *Response, error) {
	r, err := c.doAPIPostJSON(ctx, c.aiConnectionsRoute().Join("models"), request)
	if err != nil {
		return nil, BuildResponse(r), err
	}
	defer closeBody(r)
	return DecodeJSONFromResponse[[]ai.ModelInfo](r)
}

// GetAIConnection gets a model connection, without its key.
func (c *Client4) GetAIConnection(ctx context.Context, connectionID string) (*AIConnection, *Response, error) {
	r, err := c.doAPIGet(ctx, c.aiConnectionRoute(connectionID), "")
	if err != nil {
		return nil, BuildResponse(r), err
	}
	defer closeBody(r)
	return DecodeJSONFromResponse[*AIConnection](r)
}

// PatchAIConnection changes a model connection.
func (c *Client4) PatchAIConnection(ctx context.Context, connectionID string, patch *AIConnectionPatch) (*AIConnection, *Response, error) {
	r, err := c.doAPIPatchJSON(ctx, c.aiConnectionRoute(connectionID), patch)
	if err != nil {
		return nil, BuildResponse(r), err
	}
	defer closeBody(r)
	return DecodeJSONFromResponse[*AIConnection](r)
}

// DeleteAIConnection deletes a model connection.
func (c *Client4) DeleteAIConnection(ctx context.Context, connectionID string) (*Response, error) {
	r, err := c.doAPIDelete(ctx, c.aiConnectionRoute(connectionID))
	if err != nil {
		return BuildResponse(r), err
	}
	defer closeBody(r)
	return BuildResponse(r), nil
}

// TestAIConnection tests a model connection's endpoint and key.
func (c *Client4) TestAIConnection(ctx context.Context, connectionID string) (*AIConnectionTestResult, *Response, error) {
	r, err := c.doAPIPostJSON(ctx, c.aiConnectionRoute(connectionID).Join("test"), nil)
	if err != nil {
		return nil, BuildResponse(r), err
	}
	defer closeBody(r)
	return DecodeJSONFromResponse[*AIConnectionTestResult](r)
}

// GetAIConnectionModels lists the models a connection's endpoint offers.
func (c *Client4) GetAIConnectionModels(ctx context.Context, connectionID string) ([]ai.ModelInfo, *Response, error) {
	r, err := c.doAPIGet(ctx, c.aiConnectionRoute(connectionID).Join("models"), "")
	if err != nil {
		return nil, BuildResponse(r), err
	}
	defer closeBody(r)
	return DecodeJSONFromResponse[[]ai.ModelInfo](r)
}

// GetAIConnectionUsage gets a connection's usage this month.
func (c *Client4) GetAIConnectionUsage(ctx context.Context, connectionID string) (*AIConnectionUsage, *Response, error) {
	r, err := c.doAPIGet(ctx, c.aiConnectionRoute(connectionID).Join("usage"), "")
	if err != nil {
		return nil, BuildResponse(r), err
	}
	defer closeBody(r)
	return DecodeJSONFromResponse[*AIConnectionUsage](r)
}
