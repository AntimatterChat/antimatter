// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package app

import (
	"encoding/json"
)

// The server's own AI features (Antimatter) — the composer's rewrite, recaps, auto-translation and text to speech —
// run on the model connections of AISettings through the AI service, not through the AI chat app.

// mustMarshalJSONSchema gives a schema written as a Go value as JSON, for package-level schemas.
func mustMarshalJSONSchema(schema map[string]any) json.RawMessage {
	data, err := json.Marshal(schema)
	if err != nil {
		panic(err)
	}
	return data
}
