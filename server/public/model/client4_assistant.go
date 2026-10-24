// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package model

import (
	"context"
)

// The assistant (Antimatter): /api/v4/assistant.

func (c *Client4) assistantRoute() clientRoute {
	return newClientRoute("assistant")
}

func (c *Client4) assistantChannelRoute(channelID string) clientRoute {
	return c.assistantRoute().Join("channels", channelID)
}

func (c *Client4) assistantCardRoute(channelID, cardID string) clientRoute {
	return c.assistantChannelRoute(channelID).Join("cards", cardID)
}

// GetAssistantFeed gives the caller's own feed in a channel.
func (c *Client4) GetAssistantFeed(ctx context.Context, channelID string) (*AssistantFeed, *Response, error) {
	r, err := c.doAPIGet(ctx, c.assistantChannelRoute(channelID).Join("feed"), "")
	if err != nil {
		return nil, BuildResponse(r), err
	}
	defer closeBody(r)
	return DecodeJSONFromResponse[*AssistantFeed](r)
}

// SetAssistantPresence says the caller's panel has the assistant open on a channel (a heartbeat), or closed it.
func (c *Client4) SetAssistantPresence(ctx context.Context, channelID string, open bool) (*AssistantPresence, *Response, error) {
	r, err := c.doAPIPostJSON(ctx, c.assistantChannelRoute(channelID).Join("presence"), &AssistantPresenceRequest{Open: open})
	if err != nil {
		return nil, BuildResponse(r), err
	}
	defer closeBody(r)
	return DecodeJSONFromResponse[*AssistantPresence](r)
}

// SetAssistantChannelOn is the caller's switch for the assistant in a channel.
func (c *Client4) SetAssistantChannelOn(ctx context.Context, channelID string, on bool) (*Response, error) {
	r, err := c.doAPIPutJSON(ctx, c.assistantChannelRoute(channelID).Join("enabled"), &AssistantChannelSwitch{On: on})
	if err != nil {
		return BuildResponse(r), err
	}
	defer closeBody(r)
	return BuildResponse(r), nil
}

func (c *Client4) assistantCardAction(ctx context.Context, channelID, cardID, action string, body any) (*AssistantCard, *Response, error) {
	if body == nil {
		body = map[string]any{}
	}
	r, err := c.doAPIPostJSON(ctx, c.assistantCardRoute(channelID, cardID).Join(action), body)
	if err != nil {
		return nil, BuildResponse(r), err
	}
	defer closeBody(r)
	return DecodeJSONFromResponse[*AssistantCard](r)
}

// DismissAssistantCard hides one of the caller's cards (and passes its action on, if it was offered to them).
func (c *Client4) DismissAssistantCard(ctx context.Context, channelID, cardID string) (*AssistantCard, *Response, error) {
	return c.assistantCardAction(ctx, channelID, cardID, "dismiss", nil)
}

// PinAssistantCard pins or unpins one of the caller's cards.
func (c *Client4) PinAssistantCard(ctx context.Context, channelID, cardID string, pinned bool) (*AssistantCard, *Response, error) {
	return c.assistantCardAction(ctx, channelID, cardID, "pin", &AssistantPinRequest{Pinned: pinned})
}

// MuteAssistantCard is "don't suggest this kind again": the card's tool is muted for the caller.
func (c *Client4) MuteAssistantCard(ctx context.Context, channelID, cardID string) (*AssistantCard, *Response, error) {
	return c.assistantCardAction(ctx, channelID, cardID, "mute", nil)
}

// ClaimAssistantCard takes an action offered to nobody.
func (c *Client4) ClaimAssistantCard(ctx context.Context, channelID, cardID string) (*AssistantCard, *Response, error) {
	return c.assistantCardAction(ctx, channelID, cardID, "claim", nil)
}

// RunAssistantCard runs an action offered to the caller.
func (c *Client4) RunAssistantCard(ctx context.Context, channelID, cardID string) (*AssistantRunResult, *Response, error) {
	r, err := c.doAPIPostJSON(ctx, c.assistantCardRoute(channelID, cardID).Join("run"), map[string]any{})
	if err != nil {
		return nil, BuildResponse(r), err
	}
	defer closeBody(r)
	return DecodeJSONFromResponse[*AssistantRunResult](r)
}

// PostAssistantCard posts an insight card's content to its channel as the caller.
func (c *Client4) PostAssistantCard(ctx context.Context, channelID, cardID string) (*Post, *Response, error) {
	r, err := c.doAPIPostJSON(ctx, c.assistantCardRoute(channelID, cardID).Join("post"), map[string]any{})
	if err != nil {
		return nil, BuildResponse(r), err
	}
	defer closeBody(r)
	return DecodeJSONFromResponse[*Post](r)
}

// GetAssistantMutedTools gives the tools the caller muted.
func (c *Client4) GetAssistantMutedTools(ctx context.Context) (*AssistantMutedTools, *Response, error) {
	r, err := c.doAPIGet(ctx, c.assistantRoute().Join("muted_tools"), "")
	if err != nil {
		return nil, BuildResponse(r), err
	}
	defer closeBody(r)
	return DecodeJSONFromResponse[*AssistantMutedTools](r)
}

// SetAssistantMutedTools replaces the tools the caller muted.
func (c *Client4) SetAssistantMutedTools(ctx context.Context, muted *AssistantMutedTools) (*AssistantMutedTools, *Response, error) {
	r, err := c.doAPIPutJSON(ctx, c.assistantRoute().Join("muted_tools"), muted)
	if err != nil {
		return nil, BuildResponse(r), err
	}
	defer closeBody(r)
	return DecodeJSONFromResponse[*AssistantMutedTools](r)
}

// GetAssistantCommunitySettings gives a community's assistant settings, for its admins.
func (c *Client4) GetAssistantCommunitySettings(ctx context.Context, teamID string) (*AssistantCommunitySettingsInfo, *Response, error) {
	r, err := c.doAPIGet(ctx, c.assistantRoute().Join("communities", teamID, "settings"), "")
	if err != nil {
		return nil, BuildResponse(r), err
	}
	defer closeBody(r)
	return DecodeJSONFromResponse[*AssistantCommunitySettingsInfo](r)
}

// SetAssistantCommunitySettings changes a community's assistant settings.
func (c *Client4) SetAssistantCommunitySettings(ctx context.Context, teamID string, settings *AssistantCommunitySettings) (*AssistantCommunitySettingsInfo, *Response, error) {
	r, err := c.doAPIPutJSON(ctx, c.assistantRoute().Join("communities", teamID, "settings"), settings)
	if err != nil {
		return nil, BuildResponse(r), err
	}
	defer closeBody(r)
	return DecodeJSONFromResponse[*AssistantCommunitySettingsInfo](r)
}
