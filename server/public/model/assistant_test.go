// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package model

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// The assistant's preferences: a channel's switch (on or off, named by the channel) and the muted tools.
func TestAssistantPreferences(t *testing.T) {
	userID, channelID := NewId(), NewId()
	valid := []Preference{
		{UserId: userID, Category: PreferenceCategoryAssistant, Name: channelID, Value: AssistantChannelOff},
		{UserId: userID, Category: PreferenceCategoryAssistant, Name: channelID, Value: AssistantChannelOn},
		{UserId: userID, Category: PreferenceCategoryAssistant, Name: PreferenceNameAssistantMutedTools, Value: `{"tools":["mcp.grafana.query_prometheus"]}`},
	}
	for _, p := range valid {
		assert.Nil(t, p.IsValid(), "%s=%s", p.Name, p.Value)
	}
	invalid := []Preference{
		{UserId: userID, Category: PreferenceCategoryAssistant, Name: channelID, Value: "maybe"},
		{UserId: userID, Category: PreferenceCategoryAssistant, Name: "not-an-id", Value: AssistantChannelOff},
		{UserId: userID, Category: PreferenceCategoryAssistant, Name: PreferenceNameAssistantMutedTools, Value: `{"tools":["has space"]}`},
		{UserId: userID, Category: PreferenceCategoryAssistant, Name: PreferenceNameAssistantMutedTools, Value: `not json`},
	}
	for _, p := range invalid {
		assert.NotNil(t, p.IsValid(), "%s=%s", p.Name, p.Value)
	}

	muted := ParseAssistantMutedTools(`{"tools":["a.b"]}`)
	assert.True(t, muted.Mutes("a.b"))
	assert.False(t, muted.Mutes("a.c"))
	assert.Empty(t, ParseAssistantMutedTools("broken").Tools, "a broken value mutes nothing")
}

func TestAssistantCommunitySettingsIsValid(t *testing.T) {
	assert.Nil(t, (&AssistantCommunitySettings{}).IsValid())
	assert.Nil(t, (&AssistantCommunitySettings{Enabled: NewPointer(false), Engine: AssistantEngineModel, ConnectionId: NewId()}).IsValid())
	assert.NotNil(t, (&AssistantCommunitySettings{Engine: "chat"}).IsValid())
	assert.NotNil(t, (&AssistantCommunitySettings{ConnectionId: "x"}).IsValid())
}

// The apps whose posts start the assistant: the Alertmanager app by default, a list of app IDs (none twice, at most
// AssistantMaxTriggeringApps), and the hourly cap of their windows per channel.
func TestAssistantSettingsTriggeringApps(t *testing.T) {
	defaults := func() *AssistantSettings {
		s := &AssistantSettings{}
		s.SetDefaults()
		return s
	}
	s := defaults()
	assert.Equal(t, []string{"com.antimatterchat.alertmanager"}, s.TriggeringApps)
	assert.Equal(t, 30, *s.AppWindowsPerChannelHour)
	assert.Nil(t, s.isValid())

	t.Run("an empty list stays empty", func(t *testing.T) {
		s := &AssistantSettings{TriggeringApps: []string{}}
		s.SetDefaults()
		assert.Empty(t, s.TriggeringApps, "an admin who turned every app off keeps it so")
		assert.Nil(t, s.isValid())
	})

	many := make([]string, AssistantMaxTriggeringApps+1)
	for i := range many {
		many[i] = "com.example.app" + strings.Repeat("x", i)
	}
	for name, tc := range map[string]struct {
		apps  []string
		cap   int
		valid bool
	}{
		"two apps":                {[]string{"com.antimatterchat.alertmanager", "com.example.uptime"}, 30, true},
		"no cap: no app windows":  {[]string{"com.antimatterchat.alertmanager"}, 0, true},
		"the most apps":           {many[:AssistantMaxTriggeringApps], 30, true},
		"too many apps":           {many, 30, false},
		"an app twice":            {[]string{"com.example.uptime", "com.example.uptime"}, 30, false},
		"an empty ID":             {[]string{""}, 30, false},
		"an ID with a space":      {[]string{"com.example uptime"}, 30, false},
		"an ID with a colon":      {[]string{"antimatter:assistant"}, 30, false},
		"an ID too short":         {[]string{"ab"}, 30, false},
		"a negative cap":          {[]string{"com.example.uptime"}, -1, false},
		"a cap past ten thousand": {[]string{"com.example.uptime"}, 10001, false},
	} {
		t.Run(name, func(t *testing.T) {
			s := defaults()
			s.TriggeringApps = tc.apps
			s.AppWindowsPerChannelHour = NewPointer(tc.cap)
			appErr := s.isValid()
			if tc.valid {
				assert.Nil(t, appErr)
				return
			}
			if assert.NotNil(t, appErr) {
				assert.Equal(t, "model.config.is_valid.assistant.app_error", appErr.Id)
			}
		})
	}
}

// A pattern's slots become groups that capture the entity's index; plain regular-expression braces stay.
func TestCompileAssistantPatternExpression(t *testing.T) {
	re, slots, err := CompileAssistantPatternExpression(`\bcpu.{0,40}on {host|service} and {date}`)
	assert.NoError(t, err)
	assert.Equal(t, []string{"slot__0", "slot__1"}, slots)
	assert.True(t, re.MatchString("CPU usage on ⟦service:3⟧ and ⟦date:4⟧"))
	assert.False(t, re.MatchString("cpu on ⟦person:3⟧ and ⟦date:4⟧"), "a slot takes only its types")
}
