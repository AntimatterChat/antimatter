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

func TestPostMatchesSearchFilters(t *testing.T) {
	post := &Post{
		Message:    "see https://example.com and `code` @alice @bob",
		FileIds:    StringArray{NewId()},
		IsPinned:   true,
		ReplyCount: 2,
	}
	plain := &Post{Message: "nothing special"}

	for _, tc := range []struct {
		Name   string
		Params *SearchParams
		Post   bool
		Plain  bool
	}{
		{"no filter", &SearchParams{}, true, true},
		{"has:link", &SearchParams{HasFlags: []string{"link"}}, true, false},
		{"has:code", &SearchParams{HasFlags: []string{"code"}}, true, false},
		{"has:embed", &SearchParams{HasFlags: []string{"embed"}}, true, false},
		{"has:file", &SearchParams{HasFlags: []string{"file"}}, true, false},
		{"has:thread", &SearchParams{HasFlags: []string{"thread"}}, true, false},
		{"-has:link", &SearchParams{ExcludedHasFlags: []string{"link"}}, false, true},
		{"pinned:true", &SearchParams{Pinned: NewPointer(true)}, true, false},
		{"pinned:false", &SearchParams{Pinned: NewPointer(false)}, false, true},
		{"mentions:alice", &SearchParams{Mentions: []string{"alice"}}, true, false},
		{"mentions: needs every user", &SearchParams{Mentions: []string{"alice", "carol"}}, false, false},
		{"-mentions:bob", &SearchParams{ExcludedMentions: []string{"bob"}}, false, true},
	} {
		t.Run(tc.Name, func(t *testing.T) {
			assert.Equal(t, tc.Post, PostMatchesSearchFilters(post, tc.Params))
			assert.Equal(t, tc.Plain, PostMatchesSearchFilters(plain, tc.Params))
		})
	}

	t.Run("has:file counts legacy Filenames", func(t *testing.T) {
		legacy := &Post{Message: "old attachment", Filenames: StringArray{"/old/file.png"}}
		assert.True(t, PostMatchesSearchFilters(legacy, &SearchParams{HasFlags: []string{"file"}}))
		assert.False(t, PostMatchesSearchFilters(legacy, &SearchParams{ExcludedHasFlags: []string{"file"}}))
	})
}
