// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validTeam() *Team {
	return &Team{Id: NewId(), CreateAt: GetMillis(), UpdateAt: GetMillis(), DisplayName: "Antimatter", Name: "antimatter", Email: "team@example.com", Type: TeamOpen, InviteId: NewId()}
}

func TestTeamOnboardingIsValid(t *testing.T) {
	t.Run("none is fine", func(t *testing.T) {
		require.Nil(t, validTeam().IsValid())
	})

	t.Run("JSON is fine", func(t *testing.T) {
		team := validTeam()
		team.Onboarding = `{"welcome":"Hi","questions":[]}`
		require.Nil(t, team.IsValid())
	})

	t.Run("anything else isn't", func(t *testing.T) {
		team := validTeam()
		team.Onboarding = `{"welcome":`
		appErr := team.IsValid()
		require.NotNil(t, appErr)
		assert.Equal(t, "model.team.is_valid.onboarding.app_error", appErr.Id)
	})

	t.Run("up to its length", func(t *testing.T) {
		team := validTeam()
		pad := TeamOnboardingMaxLength - len(`{"welcome":""}`)
		team.Onboarding = `{"welcome":"` + strings.Repeat("a", pad) + `"}`
		require.Nil(t, team.IsValid())
		team.Onboarding = `{"welcome":"` + strings.Repeat("a", pad+1) + `"}`
		require.NotNil(t, team.IsValid())
	})
}

func TestTeamPatchAntimatterFields(t *testing.T) {
	t.Run("sets what the patch has", func(t *testing.T) {
		team := validTeam()
		team.Patch(&TeamPatch{HideRoleColors: NewPointer(true), RequireTeamBadges: NewPointer(true), Onboarding: NewPointer(`{"welcome":"Hi"}`)})
		assert.True(t, team.HideRoleColors)
		assert.True(t, team.RequireTeamBadges)
		assert.Equal(t, `{"welcome":"Hi"}`, team.Onboarding)
	})

	t.Run("keeps what the patch lacks", func(t *testing.T) {
		team := validTeam()
		team.HideRoleColors = true
		team.RequireTeamBadges = true
		team.Onboarding = `{"welcome":"Hi"}`
		team.Patch(&TeamPatch{DisplayName: NewPointer("Renamed")})
		assert.True(t, team.HideRoleColors)
		assert.True(t, team.RequireTeamBadges)
		assert.Equal(t, `{"welcome":"Hi"}`, team.Onboarding)
	})

	t.Run("clears the onboarding with an empty one", func(t *testing.T) {
		team := validTeam()
		team.Onboarding = `{"welcome":"Hi"}`
		team.Patch(&TeamPatch{Onboarding: NewPointer("")})
		assert.Empty(t, team.Onboarding)
	})
}
