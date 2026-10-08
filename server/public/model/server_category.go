// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"net/http"
	"strings"
	"unicode/utf8"
)

const (
	ServerCategoryNameMaxRunes = 64
	ServerCategoriesMaxPerTeam = 100
)

// ServerCategory is a sidebar category an admin sets up for everyone in a team (Antimatter). Channels are filed in it
// by name, through their managed category. An open category takes channels from anyone who can manage the channel, and
// people may move them to their own categories; a locked one takes channels from team admins only, and keeps them.
type ServerCategory struct {
	TeamId    string `json:"team_id"`
	Name      string `json:"name"`
	SortOrder int    `json:"sort_order"`
	Locked    bool   `json:"locked"`
	CreateAt  int64  `json:"create_at"`
	UpdateAt  int64  `json:"update_at"`
}

// ServerCategoriesUpdate replaces a team's server categories, in order. Renames maps old names to new ones, so that the
// channels filed under an old name follow it; the channels of a category left out are unfiled.
type ServerCategoriesUpdate struct {
	Categories []*ServerCategory `json:"categories"`
	Renames    map[string]string `json:"renames,omitempty"`
}

func (c *ServerCategory) IsValid() *AppError {
	if !IsValidId(c.TeamId) {
		return NewAppError("ServerCategory.IsValid", "model.server_category.team_id.app_error", nil, "", http.StatusBadRequest)
	}
	if c.Name == "" || c.Name != strings.TrimSpace(c.Name) || utf8.RuneCountInString(c.Name) > ServerCategoryNameMaxRunes {
		return NewAppError("ServerCategory.IsValid", "model.server_category.name.app_error", map[string]any{"MaxRunes": ServerCategoryNameMaxRunes}, "", http.StatusBadRequest)
	}
	return nil
}

// IsValid checks the update for a team: valid categories, no name twice (ignoring case), and at most
// ServerCategoriesMaxPerTeam of them.
func (u *ServerCategoriesUpdate) IsValid(teamID string) *AppError {
	if len(u.Categories) > ServerCategoriesMaxPerTeam {
		return NewAppError("ServerCategoriesUpdate.IsValid", "model.server_category.too_many.app_error", map[string]any{"Max": ServerCategoriesMaxPerTeam}, "", http.StatusBadRequest)
	}
	seen := make(map[string]bool, len(u.Categories))
	for _, c := range u.Categories {
		if c == nil {
			return NewAppError("ServerCategoriesUpdate.IsValid", "model.server_category.name.app_error", map[string]any{"MaxRunes": ServerCategoryNameMaxRunes}, "", http.StatusBadRequest)
		}
		c.TeamId = teamID
		c.Name = strings.TrimSpace(c.Name)
		if appErr := c.IsValid(); appErr != nil {
			return appErr
		}
		key := strings.ToLower(c.Name)
		if seen[key] {
			return NewAppError("ServerCategoriesUpdate.IsValid", "model.server_category.duplicate.app_error", map[string]any{"Name": c.Name}, "", http.StatusBadRequest)
		}
		seen[key] = true
	}
	return nil
}

// FindServerCategory finds a category by name, ignoring case.
func FindServerCategory(categories []*ServerCategory, name string) *ServerCategory {
	for _, c := range categories {
		if strings.EqualFold(c.Name, strings.TrimSpace(name)) {
			return c
		}
	}
	return nil
}
