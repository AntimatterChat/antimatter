// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package model

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
)

// Antimatter community profiles (CommunityProfile): someone's nickname and picture in a community.

// GetCommunityProfiles lists the profiles set in a community.
func (c *Client4) GetCommunityProfiles(ctx context.Context, teamId string) ([]*CommunityProfile, *Response, error) {
	r, err := c.doAPIGet(ctx, c.teamRoute(teamId).Join("profiles"), "")
	if err != nil {
		return nil, BuildResponse(r), err
	}
	defer closeBody(r)
	return DecodeJSONFromResponse[[]*CommunityProfile](r)
}

// PatchCommunityProfile changes one's own nickname in a community.
func (c *Client4) PatchCommunityProfile(ctx context.Context, teamId, userId string, patch *CommunityProfilePatch) (*CommunityProfile, *Response, error) {
	r, err := c.doAPIPatchJSON(ctx, c.teamMemberRoute(teamId, userId).Join("profile"), patch)
	if err != nil {
		return nil, BuildResponse(r), err
	}
	defer closeBody(r)
	return DecodeJSONFromResponse[*CommunityProfile](r)
}

// ResetCommunityProfile takes a nickname and/or a picture off a community profile: one's own, or a member's for the
// community's moderators.
func (c *Client4) ResetCommunityProfile(ctx context.Context, teamId, userId string, reset *CommunityProfileReset) (*CommunityProfile, *Response, error) {
	r, err := c.doAPIPostJSON(ctx, c.teamMemberRoute(teamId, userId).Join("profile", "reset"), reset)
	if err != nil {
		return nil, BuildResponse(r), err
	}
	defer closeBody(r)
	return DecodeJSONFromResponse[*CommunityProfile](r)
}

// SetCommunityAvatar sets one's own picture for a community.
func (c *Client4) SetCommunityAvatar(ctx context.Context, teamId, userId string, data []byte) (*CommunityProfile, *Response, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("image", "avatar.png")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err = io.Copy(part, bytes.NewBuffer(data)); err != nil {
		return nil, nil, fmt.Errorf("failed to copy data to form file: %w", err)
	}
	if err = writer.Close(); err != nil {
		return nil, nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}
	r, err := c.doAPIRequestReaderRoute(ctx, http.MethodPost, c.teamMemberRoute(teamId, userId).Join("profile", "image"), writer.FormDataContentType(), body, nil)
	if err != nil {
		return nil, BuildResponse(r), err
	}
	defer closeBody(r)
	return DecodeJSONFromResponse[*CommunityProfile](r)
}

// GetCommunityAvatar gives someone's picture for a community (404 when they use their profile picture there).
func (c *Client4) GetCommunityAvatar(ctx context.Context, teamId, userId, etag string) ([]byte, *Response, error) {
	r, err := c.doAPIGet(ctx, c.teamMemberRoute(teamId, userId).Join("profile", "image"), etag)
	if err != nil {
		return nil, BuildResponse(r), err
	}
	defer closeBody(r)
	return ReadBytesFromResponse(r)
}
