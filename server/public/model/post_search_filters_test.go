// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHasLink(t *testing.T) {
	for _, tc := range []struct {
		Name    string
		Message string
		Want    bool
	}{
		{"no link", "just some text", false},
		{"https link", "check out https://example.com/path", true},
		{"http link", "check out http://example.com", true},
		{"www link without scheme", "see www.example.com for details", true},
		{"link inside inline code still has a link", "`https://example.com`", true},
		{"email is not a link", "me@example.com", false},
		{"empty message", "", false},
	} {
		t.Run(tc.Name, func(t *testing.T) {
			assert.Equal(t, tc.Want, HasLink(tc.Message))
		})
	}
}

func TestHasCode(t *testing.T) {
	for _, tc := range []struct {
		Name    string
		Message string
		Want    bool
	}{
		{"no code", "just some text", false},
		{"inline code span", "run `go test` to check", true},
		{"fenced code block", "```\nfmt.Println(\"hi\")\n```", true},
		{"single backtick alone is not a span", "a ` b", false},
		{"empty message", "", false},
	} {
		t.Run(tc.Name, func(t *testing.T) {
			assert.Equal(t, tc.Want, HasCode(tc.Message))
		})
	}
}

func TestHasEmbed(t *testing.T) {
	for _, tc := range []struct {
		Name    string
		Message string
		Want    bool
	}{
		{"plain link embeds", "see https://example.com", true},
		{"link inside inline code does not embed", "`https://example.com`", false},
		{"link inside a fenced block does not embed", "```\nhttps://example.com\n```", false},
		{"link outside code still embeds even with other code present", "run `go test`, see https://example.com", true},
		{"no link at all", "just text", false},
	} {
		t.Run(tc.Name, func(t *testing.T) {
			assert.Equal(t, tc.Want, HasEmbed(tc.Message))
		})
	}
}

func TestPossibleAtMentions(t *testing.T) {
	for _, tc := range []struct {
		Name    string
		Message string
		Want    []string
	}{
		{"no mention", "hello there", nil},
		{"single mention", "hello @bob", []string{"bob"}},
		{"mention is case-insensitive and normalized to lower case", "hello @Bob", []string{"bob"}},
		{"several distinct mentions in order", "@bob and @alice, thanks", []string{"bob", "alice"}},
		{"duplicate mentions are deduplicated", "@bob @bob @bob", []string{"bob"}},
		{"an email address is not a mention", "me@example.com", nil},
		{"empty message", "", nil},
	} {
		t.Run(tc.Name, func(t *testing.T) {
			assert.Equal(t, tc.Want, PossibleAtMentions(tc.Message))
		})
	}
}
