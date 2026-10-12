package model

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGroupAuditable(t *testing.T) {
	t.Run("zero value", func(t *testing.T) {
		var g Group
		m := g.Auditable()
		require.NotNil(t, m)
		assert.Equal(t, "", m["remote_id"])
	})

	t.Run("values set", func(t *testing.T) {
		id := NewId()
		leadId := NewId()
		channelId := NewId()
		teamId := NewId()
		now := GetMillis()
		g := Group{
			Id:             id,
			Name:           new("some name"),
			DisplayName:    "some display name",
			Source:         GroupSourceLdap,
			RemoteId:       new("some_remote"),
			CreateAt:       now,
			UpdateAt:       now,
			DeleteAt:       now,
			HasSyncables:   true,
			MemberCount:    new(10),
			AllowReference: true,
			Tag:            "DET",
			LeadId:         leadId,
			ChannelId:      channelId,
			TeamId:         teamId,
			Color:          "#3b82f6",
			Icon:           "star",
			Position:       2,
			Hoist:          true,
			SelfAssignable: true,
		}
		m := g.Auditable()

		expected := map[string]any{
			"id":              id,
			"source":          GroupSourceLdap,
			"remote_id":       "some_remote",
			"create_at":       now,
			"update_at":       now,
			"delete_at":       now,
			"has_syncables":   true,
			"member_count":    10,
			"allow_reference": true,
			"is_team":         false,
			"tag":             "DET",
			"lead_id":         leadId,
			"channel_id":      channelId,
			"team_id":         teamId,
			"color":           "#3b82f6",
			"icon":            "star",
			"position":        2,
			"hoist":           true,
			"self_assignable": true,
		}

		assert.Equal(t, expected, m)
	})
}

func TestGroupLogClone(t *testing.T) {
	t.Run("zero value", func(t *testing.T) {
		var g Group
		l := g.LogClone()
		require.NotNil(t, l)

		m, ok := l.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "", m["remote_id"])
	})

	t.Run("values set", func(t *testing.T) {
		id := NewId()
		leadId := NewId()
		channelId := NewId()
		teamId := NewId()
		now := GetMillis()
		g := Group{
			Id:             id,
			Name:           new("some name"),
			DisplayName:    "some display name",
			Source:         GroupSourceLdap,
			RemoteId:       new("some_remote"),
			CreateAt:       now,
			UpdateAt:       now,
			DeleteAt:       now,
			HasSyncables:   true,
			MemberCount:    new(10),
			AllowReference: true,
			Tag:            "DET",
			LeadId:         leadId,
			ChannelId:      channelId,
			TeamId:         teamId,
			Color:          "#3b82f6",
			Icon:           "star",
			Position:       2,
			Hoist:          true,
			SelfAssignable: true,
		}
		l := g.LogClone()
		m, ok := l.(map[string]any)
		require.True(t, ok)

		expected := map[string]any{
			"id":              id,
			"name":            "some name",
			"display_name":    "some display name",
			"source":          GroupSourceLdap,
			"remote_id":       "some_remote",
			"create_at":       now,
			"update_at":       now,
			"delete_at":       now,
			"has_syncables":   true,
			"member_count":    10,
			"allow_reference": true,
			"is_team":         false,
			"tag":             "DET",
			"lead_id":         leadId,
			"channel_id":      channelId,
			"team_id":         teamId,
			"color":           "#3b82f6",
			"icon":            "star",
			"position":        2,
			"hoist":           true,
			"self_assignable": true,
		}

		assert.Equal(t, expected, m)
	})
}

func TestGroupTeamFields(t *testing.T) {
	valid := func() Group {
		return Group{DisplayName: "Detector team", Name: new("detector"), Source: GroupSourceCustom, RemoteId: new(NewId()), AllowReference: true}
	}

	t.Run("patch sets and normalizes the team fields", func(t *testing.T) {
		g := valid()
		lead, channel := NewId(), NewId()
		g.Patch(&GroupPatch{Tag: new(" det "), LeadId: &lead, ChannelId: &channel})
		assert.Equal(t, "DET", g.Tag)
		assert.Equal(t, lead, g.LeadId)
		assert.Equal(t, channel, g.ChannelId)
	})

	t.Run("the team fields are optional", func(t *testing.T) {
		g := valid()
		assert.Nil(t, g.IsValidForCreate())
	})

	t.Run("a tag is at most 8 characters", func(t *testing.T) {
		g := valid()
		g.Tag = "ÉQUIPE42"
		assert.Nil(t, g.IsValidForCreate())
		g.Tag = "TOOLONGTAG"
		require.NotNil(t, g.IsValidForCreate())
		assert.Equal(t, "model.group.tag.app_error", g.IsValidForCreate().Id)
	})

	t.Run("the lead and the channel must be ids", func(t *testing.T) {
		g := valid()
		g.LeadId = "someone"
		assert.Equal(t, "model.group.lead_id.app_error", g.IsValidForCreate().Id)
		g.LeadId = ""
		g.ChannelId = "somewhere"
		assert.Equal(t, "model.group.channel_id.app_error", g.IsValidForCreate().Id)
	})
}

func TestGroupRoleFields(t *testing.T) {
	valid := func() Group {
		return Group{DisplayName: "Moderators", Name: new("moderators"), Source: GroupSourceCustom, AllowReference: true, TeamId: NewId()}
	}

	t.Run("a group of a team is a role", func(t *testing.T) {
		g := valid()
		assert.True(t, g.IsRole())
		assert.True(t, g.BelongsToTeam())
		g.TeamId = ""
		assert.False(t, g.IsRole())
		assert.False(t, g.BelongsToTeam())
	})

	t.Run("a team of a community isn't a role, and needs its community", func(t *testing.T) {
		g := valid()
		g.IsTeam = true
		assert.False(t, g.IsRole())
		assert.True(t, g.BelongsToTeam())
		assert.Nil(t, g.IsValidForCreate())
		g.TeamId = ""
		assert.Equal(t, "model.group.is_team.app_error", g.IsValidForCreate().Id)
	})

	t.Run("old clients' JSON still makes roles", func(t *testing.T) {
		var g Group
		require.NoError(t, json.Unmarshal([]byte(`{"team_id":"`+NewId()+`"}`), &g))
		assert.True(t, g.IsRole())
		assert.False(t, g.IsTeam)
	})

	t.Run("a role colour is a hex colour", func(t *testing.T) {
		g := valid()
		g.Color = "#3B82f6"
		assert.Nil(t, g.IsValidForCreate())
		g.Color = "blue"
		assert.Equal(t, "model.group.color.app_error", g.IsValidForCreate().Id)
	})

	t.Run("the team must be an id, the position not negative", func(t *testing.T) {
		g := valid()
		g.TeamId = "a-team"
		assert.Equal(t, "model.group.team_id.app_error", g.IsValidForCreate().Id)
		g = valid()
		g.Position = -1
		assert.Equal(t, "model.group.role.app_error", g.IsValidForCreate().Id)
	})

	t.Run("patch sets the role fields", func(t *testing.T) {
		g := valid()
		color, icon, position, hoist := "#ff0000", "shield", 3, true
		g.Patch(&GroupPatch{Color: &color, Icon: &icon, Position: &position, Hoist: &hoist})
		assert.Equal(t, "#ff0000", g.Color)
		assert.Equal(t, "shield", g.Icon)
		assert.Equal(t, 3, g.Position)
		assert.True(t, g.Hoist)
	})
}
