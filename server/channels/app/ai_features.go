// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package app

import (
	"encoding/json"
)

// The server's own AI features (Antimatter) — the composer's rewrite, recaps, auto-translation and text to speech —
// run on the model connections of AISettings through the AI service, not through the AI chat app.

// testAIFeatureBridge gives the bridge tests install in place of a model (the WithAgentsBridge option of the unit
// tests, or the end-to-end tests' AI helper, which needs ServiceSettings.EnableTesting): the built-in features use it
// so that those tests keep stubbing the model. A live server has neither, and its features use their connections.
func (a *App) testAIFeatureBridge() AgentsBridge {
	if a.Srv().agentsBridgeOverride != nil && a.ch.agentsBridge != nil {
		if _, live := a.ch.agentsBridge.(*liveAgentsBridge); !live {
			return a.ch.agentsBridge
		}
	}
	if bridge, ok := a.ch.agentsBridge.(*e2eAgentsBridge); ok {
		return bridge
	}
	return nil
}

// mustMarshalJSONSchema gives a schema written as a Go value as JSON, for package-level schemas.
func mustMarshalJSONSchema(schema map[string]any) json.RawMessage {
	data, err := json.Marshal(schema)
	if err != nil {
		panic(err)
	}
	return data
}
