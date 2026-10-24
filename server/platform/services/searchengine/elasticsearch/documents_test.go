// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package elasticsearch

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mattermost/mattermost/server/public/model"
)

func TestNewPostDocumentHasPinnedMentions(t *testing.T) {
	t.Run("a plain message has none of the has: flags", func(t *testing.T) {
		doc := newPostDocument(&model.Post{Message: "just text"}, "team1", "O")
		assert.False(t, doc.HasLink)
		assert.False(t, doc.HasCode)
		assert.False(t, doc.HasEmbed)
		assert.False(t, doc.HasFile)
		assert.False(t, doc.IsPinned)
		assert.Equal(t, []string{}, doc.Mentions)
	})

	t.Run("has_link is set from a bare URL", func(t *testing.T) {
		doc := newPostDocument(&model.Post{Message: "see https://example.com"}, "team1", "O")
		assert.True(t, doc.HasLink)
		assert.True(t, doc.HasEmbed)
	})

	t.Run("has_code and has_embed disagree on a link inside code", func(t *testing.T) {
		doc := newPostDocument(&model.Post{Message: "`https://example.com`"}, "team1", "O")
		assert.True(t, doc.HasLink)
		assert.True(t, doc.HasCode)
		assert.False(t, doc.HasEmbed, "a link inside code never gets a preview")
	})

	t.Run("has_file follows FileIds", func(t *testing.T) {
		doc := newPostDocument(&model.Post{Message: "a file", FileIds: model.StringArray{model.NewId()}}, "team1", "O")
		assert.True(t, doc.HasFile)
	})

	t.Run("is_pinned follows the post", func(t *testing.T) {
		doc := newPostDocument(&model.Post{Message: "x", IsPinned: true}, "team1", "O")
		assert.True(t, doc.IsPinned)
	})

	t.Run("mentions are extracted and lower-cased", func(t *testing.T) {
		doc := newPostDocument(&model.Post{Message: "hello @Bob and @alice"}, "team1", "O")
		assert.Equal(t, []string{"bob", "alice"}, doc.Mentions)
	})
}
