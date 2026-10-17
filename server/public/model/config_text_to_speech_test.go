// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTextToSpeechSettingsIsValid(t *testing.T) {
	s := TextToSpeechSettings{}
	s.SetDefaults()
	require.Nil(t, s.isValid(false), "off, nothing is required")

	s.Enable = NewPointer(true)
	require.NotNil(t, s.isValid(false))

	s.URL = NewPointer("http://tts.example.com/v1")
	appErr := s.isValid(false)
	require.NotNil(t, appErr)
	require.Equal(t, "model.config.is_valid.text_to_speech.model.app_error", appErr.Id)

	s.Model = NewPointer("tts-1")
	require.Nil(t, s.isValid(false))

	cfg := Config{}
	cfg.SetDefaults()
	cfg.TextToSpeechSettings.APIKey = NewPointer("secret")
	cfg.Sanitize(nil, nil)
	require.Equal(t, FakeSetting, *cfg.TextToSpeechSettings.APIKey)
}
