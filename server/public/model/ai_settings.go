// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package model

import (
	"net/http"
)

// AutoTranslationProviderConnection translates with a model connection (AISettings.AutoTranslation). The older
// value "agents" still works and means the same, falling back to AutoTranslationSettings.Agents.LLMServiceID as the
// connection's ID (connections migrated from the Agents plugin keep its services' IDs).
const AutoTranslationProviderConnection = "connection"

// AIFeatureSettings picks the model connection and model a built-in feature uses. An empty ConnectionId turns the
// feature's AI off; an empty Model uses the connection's chat model. Only server-wide connections serve built-in
// features.
type AIFeatureSettings struct {
	ConnectionId *string `access:"integrations_integration_management"`
	Model        *string `access:"integrations_integration_management"`
}

func (s *AIFeatureSettings) SetDefaults() {
	if s.ConnectionId == nil {
		s.ConnectionId = NewPointer("")
	}
	if s.Model == nil {
		s.Model = NewPointer("")
	}
}

// IsConfigured tells whether the feature has a connection.
func (s *AIFeatureSettings) IsConfigured() bool {
	return s != nil && s.ConnectionId != nil && *s.ConnectionId != ""
}

func (s *AIFeatureSettings) isValid(feature string) *AppError {
	if s.ConnectionId != nil && *s.ConnectionId != "" && !IsValidId(*s.ConnectionId) {
		return NewAppError("Config.IsValid", "model.config.is_valid.ai.connection_id.app_error", map[string]any{"Feature": feature}, "", http.StatusBadRequest)
	}
	if s.Model != nil && len(*s.Model) > AIModelNameMaxLength {
		return NewAppError("Config.IsValid", "model.config.is_valid.ai.model.app_error", map[string]any{"Feature": feature}, "", http.StatusBadRequest)
	}
	return nil
}

// AISettings says which model connection each of the server's own AI features uses: the composer's rewrite, recaps,
// auto-translation (when its provider is a connection) and text to speech (when it names a connection, in place of
// TextToSpeechSettings' URL and key). They no longer go through the AI chat app.
type AISettings struct {
	Rewrite         *AIFeatureSettings `access:"integrations_integration_management"`
	Recaps          *AIFeatureSettings `access:"integrations_integration_management"`
	AutoTranslation *AIFeatureSettings `access:"integrations_integration_management"`
	TextToSpeech    *AIFeatureSettings `access:"integrations_integration_management"`
}

func (s *AISettings) SetDefaults() {
	if s.Rewrite == nil {
		s.Rewrite = &AIFeatureSettings{}
	}
	s.Rewrite.SetDefaults()
	if s.Recaps == nil {
		s.Recaps = &AIFeatureSettings{}
	}
	s.Recaps.SetDefaults()
	if s.AutoTranslation == nil {
		s.AutoTranslation = &AIFeatureSettings{}
	}
	s.AutoTranslation.SetDefaults()
	if s.TextToSpeech == nil {
		s.TextToSpeech = &AIFeatureSettings{}
	}
	s.TextToSpeech.SetDefaults()
}

func (o *Config) isValidAISettings() *AppError {
	s := &o.AISettings
	for feature, settings := range map[string]*AIFeatureSettings{
		"Rewrite": s.Rewrite, "Recaps": s.Recaps, "AutoTranslation": s.AutoTranslation, "TextToSpeech": s.TextToSpeech,
	} {
		if settings == nil {
			continue
		}
		if appErr := settings.isValid(feature); appErr != nil {
			return appErr
		}
	}

	at := o.AutoTranslationSettings
	if at.Enable != nil && *at.Enable && at.Provider != nil {
		switch *at.Provider {
		case AutoTranslationProviderConnection:
			if !s.AutoTranslation.IsConfigured() {
				return NewAppError("Config.IsValid", "model.config.is_valid.autotranslation.connection.app_error", nil, "", http.StatusBadRequest)
			}
		case "agents":
			if connectionID, _ := o.AutoTranslationConnection(); connectionID == "" {
				return NewAppError("Config.IsValid", "model.config.is_valid.autotranslation.agents.llm_service_id.app_error", nil, "", http.StatusBadRequest)
			}
		}
	}
	return nil
}

// TextToSpeechUsesConnection tells whether reading aloud goes through a model connection (which wins over the URL of
// TextToSpeechSettings).
func (o *Config) TextToSpeechUsesConnection() bool {
	return o.AISettings.TextToSpeech.IsConfigured()
}

// AutoTranslationConnection gives the connection and model auto-translation uses when its provider is a model
// connection ("connection", or the older "agents"): AISettings.AutoTranslation, else the Agents plugin's service ID.
func (o *Config) AutoTranslationConnection() (connectionID, modelName string) {
	if s := o.AISettings.AutoTranslation; s.IsConfigured() {
		return *s.ConnectionId, SafeDereference(s.Model)
	}
	if o.AutoTranslationSettings.Agents != nil {
		modelName := ""
		if o.AISettings.AutoTranslation != nil {
			modelName = SafeDereference(o.AISettings.AutoTranslation.Model)
		}
		return SafeDereference(o.AutoTranslationSettings.Agents.LLMServiceID), modelName
	}
	return "", ""
}
