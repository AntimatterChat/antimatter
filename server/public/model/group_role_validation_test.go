// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGroupRoleValidation(t *testing.T) {
	role := func(edit func(g *Group)) *Group {
		g := &Group{Name: NewPointer("mods"), DisplayName: "Mods", Source: GroupSourceCustom, TeamId: NewId(), Color: "#e67e22", Icon: "shield", Position: 3}
		edit(g)
		return g
	}

	assert.Nil(t, role(func(*Group) {}).IsValidForCreate())

	t.Run("a place among the roles, within bounds", func(t *testing.T) {
		assert.NotNil(t, role(func(g *Group) { g.Position = -1 }).IsValidForCreate())
		assert.Nil(t, role(func(g *Group) { g.Position = GroupPositionMax }).IsValidForCreate())
		assert.NotNil(t, role(func(g *Group) { g.Position = GroupPositionMax + 1 }).IsValidForCreate())
	})

	t.Run("an icon is an emoji name", func(t *testing.T) {
		assert.Nil(t, role(func(g *Group) { g.Icon = "thumbs_up-2+1" }).IsValidForCreate())
		assert.Nil(t, role(func(g *Group) { g.Icon = "" }).IsValidForCreate())
		assert.NotNil(t, role(func(g *Group) { g.Icon = "<img src=x>" }).IsValidForCreate())
		assert.NotNil(t, role(func(g *Group) { g.Icon = strings.Repeat("a", GroupIconMaxLength+1) }).IsValidForCreate())
	})

	t.Run("a colour is #rrggbb", func(t *testing.T) {
		assert.NotNil(t, role(func(g *Group) { g.Color = "red" }).IsValidForCreate())
		assert.NotNil(t, role(func(g *Group) { g.Color = "#fff" }).IsValidForCreate())
	})
}

func TestNormalizeGroupTag(t *testing.T) {
	assert.Equal(t, "DET", NormalizeGroupTag("  det "))
	assert.Equal(t, "", NormalizeGroupTag("   "))

	g := &Group{}
	g.Patch(&GroupPatch{Tag: NewPointer(" ops")})
	assert.Equal(t, "OPS", g.Tag)
}

func TestServerCategoriesUpdateBounds(t *testing.T) {
	teamID := NewId()
	many := make([]string, ServerCategoriesMaxPerTeam+1)
	for i := range many {
		many[i] = NewId()
	}
	assert.NotNil(t, (&ServerCategoriesUpdate{Removed: many}).IsValid(teamID))
	assert.Nil(t, (&ServerCategoriesUpdate{Removed: many[:1]}).IsValid(teamID))
}
