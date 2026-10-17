// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAISettingsDefaultsAndValidation(t *testing.T) {
	cfg := Config{}
	cfg.SetDefaults()
	require.NotNil(t, cfg.AISettings.Rewrite)
	assert.Equal(t, "", *cfg.AISettings.Rewrite.ConnectionId, "the AI features are off until an admin picks a connection")
	assert.Equal(t, "", *cfg.AISettings.Recaps.Model)
	assert.False(t, cfg.AISettings.AutoTranslation.IsConfigured())
	require.Nil(t, cfg.isValidAISettings())

	cfg.AISettings.Rewrite.ConnectionId = NewPointer("not-an-id")
	require.NotNil(t, cfg.isValidAISettings())
	cfg.AISettings.Rewrite.ConnectionId = NewPointer(NewId())
	require.Nil(t, cfg.isValidAISettings())

	t.Run("auto-translation by connection needs one", func(t *testing.T) {
		cfg := Config{}
		cfg.SetDefaults()
		*cfg.AutoTranslationSettings.Enable = true
		*cfg.AutoTranslationSettings.Provider = AutoTranslationProviderConnection
		require.NotNil(t, cfg.isValidAISettings())
		require.Nil(t, cfg.AutoTranslationSettings.isValid(), "the provider itself is known")

		id := NewId()
		cfg.AISettings.AutoTranslation.ConnectionId = NewPointer(id)
		cfg.AISettings.AutoTranslation.Model = NewPointer("hy-mt")
		require.Nil(t, cfg.isValidAISettings())
		connectionID, modelName := cfg.AutoTranslationConnection()
		assert.Equal(t, id, connectionID)
		assert.Equal(t, "hy-mt", modelName)
	})

	t.Run("the older agents provider names the migrated connection", func(t *testing.T) {
		cfg := Config{}
		cfg.SetDefaults()
		serviceID := NewId()
		*cfg.AutoTranslationSettings.Enable = true
		*cfg.AutoTranslationSettings.Provider = "agents"
		cfg.AutoTranslationSettings.Agents.LLMServiceID = NewPointer(serviceID)
		require.Nil(t, cfg.AutoTranslationSettings.isValid())
		connectionID, _ := cfg.AutoTranslationConnection()
		assert.Equal(t, serviceID, connectionID)
	})

	t.Run("text to speech through a connection needs no URL", func(t *testing.T) {
		cfg := Config{}
		cfg.SetDefaults()
		*cfg.TextToSpeechSettings.Enable = true
		*cfg.TextToSpeechSettings.Model = "tts-1"
		require.NotNil(t, cfg.TextToSpeechSettings.isValid(cfg.TextToSpeechUsesConnection()))
		cfg.AISettings.TextToSpeech.ConnectionId = NewPointer(NewId())
		require.Nil(t, cfg.TextToSpeechSettings.isValid(cfg.TextToSpeechUsesConnection()))
	})
}
