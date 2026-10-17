// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSpeechText(t *testing.T) {
	msg := "## Release **Thursday** :tada:\n\nSee [the roadmap](https://example.com/roadmap) or https://example.com.\n\n```go\nfmt.Println(1)\n```\n> Quoted _text_"
	assert.Equal(t, "Release Thursday\nSee the roadmap or\nQuoted text", SpeechText(msg, 4000))
	assert.Equal(t, "Hello", SpeechText("Hello world", 5))
	assert.Equal(t, "", SpeechText(":+1: https://example.com", 100))
}
