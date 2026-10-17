// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mattermost/mattermost/server/v8/channels/app/autotranslation"
)

func TestAutoTranslationBridgeRequest(t *testing.T) {
	format := map[string]any{"type": "object"}
	req := autoTranslationBridgeRequest(autotranslation.CompletionRequest{
		Messages: []autotranslation.CompletionMessage{
			{Role: "system", Message: "Translate"},
			{Role: "user", Message: "Hello"},
		},
		JSONOutputFormat: format,
	})

	assert.Equal(t, BridgeOperationAutoTranslate, req.Operation)
	assert.Equal(t, string(BridgeOperationAutoTranslate), req.ClientOperation)
	assert.Equal(t, []BridgeMessage{{Role: "system", Message: "Translate"}, {Role: "user", Message: "Hello"}}, req.Messages)
	assert.Equal(t, format, req.JSONOutputFormat)
	assert.Empty(t, req.UserID)
	assert.Empty(t, req.ChannelID)
}
