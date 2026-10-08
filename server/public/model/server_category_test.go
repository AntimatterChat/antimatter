// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServerCategoriesUpdateIsValid(t *testing.T) {
	teamID := NewId()
	update := func(names ...string) *ServerCategoriesUpdate {
		u := &ServerCategoriesUpdate{}
		for _, n := range names {
			u.Categories = append(u.Categories, &ServerCategory{Name: n})
		}
		return u
	}

	t.Run("trims names and sets the team", func(t *testing.T) {
		u := update("  Rules ", "Community")
		require.Nil(t, u.IsValid(teamID))
		assert.Equal(t, "Rules", u.Categories[0].Name)
		assert.Equal(t, teamID, u.Categories[1].TeamId)
	})

	t.Run("refuses empty, long and repeated names", func(t *testing.T) {
		assert.NotNil(t, update(" ").IsValid(teamID))
		assert.NotNil(t, update(strings.Repeat("é", ServerCategoryNameMaxRunes+1)).IsValid(teamID))
		assert.Nil(t, update(strings.Repeat("é", ServerCategoryNameMaxRunes)).IsValid(teamID))
		assert.Equal(t, "model.server_category.duplicate.app_error", update("Rules", "RULES").IsValid(teamID).Id)
	})

	t.Run("refuses too many categories", func(t *testing.T) {
		names := make([]string, ServerCategoriesMaxPerTeam+1)
		for i := range names {
			names[i] = NewId()
		}
		assert.Equal(t, "model.server_category.too_many.app_error", update(names...).IsValid(teamID).Id)
	})

	t.Run("finds a category ignoring case and spaces", func(t *testing.T) {
		categories := []*ServerCategory{{Name: "Rules"}, {Name: "Open space"}}
		assert.Equal(t, "Open space", FindServerCategory(categories, " open SPACE ").Name)
		assert.Nil(t, FindServerCategory(categories, "Elsewhere"))
	})
}
