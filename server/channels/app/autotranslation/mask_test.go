// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package autotranslation

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

func TestMaskText(t *testing.T) {
	text := "Hi @alice.smith, see https://example.com/a?b=1 and `go test` :tada: in ~town-square #release {{0}}"
	m := maskText(text)

	for _, protected := range []string{"@alice.smith", "https://example.com/a?b=1", "`go test`", ":tada:", "~town-square", "#release", "{{0}}"} {
		assert.Contains(t, m.Masks, protected)
	}
	assert.Equal(t, "Hi {{0}}, see {{1}} and {{2}} {{3}} in {{4}} {{5}} {{6}}", m.Text)
	assert.Equal(t, "{{0}}", m.Masks[6], "a literal placeholder is protected too")
	assert.Contains(t, m.Text, "Hi ")
	assert.True(t, m.translatable())

	restored, err := m.unmask(m.Text)
	require.NoError(t, err)
	assert.Equal(t, text, restored)
}

func TestMaskFencedCodeAndLinks(t *testing.T) {
	text := "Look:\n```\nfoo := bar()\n```\n[the docs](https://docs.example.com \"title\") please"
	m := maskText(text)
	assert.Contains(t, m.Masks, "```\nfoo := bar()\n```")
	assert.Contains(t, m.Masks, "](https://docs.example.com \"title\")")
	assert.Contains(t, m.Text, "[the docs")

	restored, err := m.unmask(m.Text)
	require.NoError(t, err)
	assert.Equal(t, text, restored)
}

func TestUnmask(t *testing.T) {
	m := maskText("Hello @bob and @carol")
	require.Len(t, m.Masks, 2)

	t.Run("tolerates whitespace inside placeholders", func(t *testing.T) {
		out, err := m.unmask("Hola {{ 0 }} y {{1}}")
		require.NoError(t, err)
		assert.Equal(t, "Hola @bob y @carol", out)
	})

	t.Run("reorders placeholders", func(t *testing.T) {
		out, err := m.unmask("{{1}} y {{0}}, hola")
		require.NoError(t, err)
		assert.Equal(t, "@carol y @bob, hola", out)
	})

	t.Run("appends dropped placeholders", func(t *testing.T) {
		out, err := m.unmask("Hola {{0}}")
		require.NoError(t, err)
		assert.Equal(t, "Hola @bob @carol", out)
	})

	t.Run("rejects unknown placeholders", func(t *testing.T) {
		_, err := m.unmask("Hola {{7}}")
		require.Error(t, err)
	})
}

func TestNotTranslatable(t *testing.T) {
	for _, text := range []string{"https://example.com", "@alice :smile:", "`x`", "123 456", "   "} {
		assert.False(t, maskText(text).translatable(), text)
	}
}

func TestNormalizeLang(t *testing.T) {
	assert.Equal(t, "pt", normalizeLang("pt-BR"))
	assert.Equal(t, "zh", normalizeLang("zh_TW"))
	assert.Equal(t, "en", normalizeLang(" EN "))
	assert.Equal(t, "", normalizeLang(""))
	assert.Equal(t, map[string]bool{"en": true, "pt": true}, targetLanguageSet([]string{"en", "pt-BR", ""}))
}

func TestDetectLocal(t *testing.T) {
	lang, _ := detectLocal("This is a fairly long English sentence that should be detected reliably by the detector.")
	assert.Equal(t, "en", lang)

	lang, _ = detectLocal("ok")
	assert.Equal(t, "", lang, "short texts are left to the provider")
}

func TestExtractContent(t *testing.T) {
	t.Run("post", func(t *testing.T) {
		c, appErr := extractContent(&model.Post{Message: "Good morning @team, the build is green"})
		require.Nil(t, appErr)
		assert.Equal(t, model.TranslationTypeObject, c.Type)
		require.Len(t, c.Segments, 1)
		assert.NotEmpty(t, c.NormHash)

		text, obj, err := c.build([]string{"Buenos días {{0}}, la compilación está verde"})
		require.NoError(t, err)
		assert.Empty(t, text)
		assert.JSONEq(t, `{"message":"Buenos días @team, la compilación está verde"}`, string(obj))
	})

	t.Run("hash ignores trailing whitespace", func(t *testing.T) {
		a, _ := extractContent(&model.Post{Message: "hello world"})
		b, _ := extractContent(&model.Post{Message: "hello world  \n"})
		c, _ := extractContent(&model.Post{Message: "hello there"})
		assert.Equal(t, a.NormHash, b.NormHash)
		assert.NotEqual(t, a.NormHash, c.NormHash)
	})

	t.Run("system post", func(t *testing.T) {
		_, appErr := extractContent(&model.Post{Message: "joined", Type: model.PostTypeJoinChannel})
		require.NotNil(t, appErr)
		assert.Equal(t, "ent.autotranslation.no_translatable_content", appErr.Id)
	})

	t.Run("only a link", func(t *testing.T) {
		_, appErr := extractContent(&model.Post{Message: "https://mattermost.com"})
		require.NotNil(t, appErr)
		assert.Equal(t, "ent.autotranslation.no_translatable_content", appErr.Id)
	})

	t.Run("string", func(t *testing.T) {
		c, appErr := extractContent("Bonjour tout le monde")
		require.Nil(t, appErr)
		assert.Equal(t, model.TranslationTypeString, c.Type)
		text, obj, err := c.build([]string{"Hello everyone"})
		require.NoError(t, err)
		assert.Equal(t, "Hello everyone", text)
		assert.Nil(t, obj)
	})

	t.Run("object", func(t *testing.T) {
		raw := json.RawMessage(`{"id":"abcdefghijklmnopqrstuvwxyz","title":"Incident report","summary":{"text":"Service restored","owner_id":"x"},"count":3}`)
		c, appErr := extractContent(raw)
		require.Nil(t, appErr)
		require.Len(t, c.Segments, 2)
		assert.Equal(t, [][]string{{"summary", "text"}, {"title"}}, c.Paths)

		_, obj, err := c.build([]string{"Servicio restablecido", "Informe de incidente"})
		require.NoError(t, err)
		assert.JSONEq(t, `{"id":"abcdefghijklmnopqrstuvwxyz","title":"Informe de incidente","summary":{"text":"Servicio restablecido","owner_id":"x"},"count":3}`, string(obj))
	})

	t.Run("map", func(t *testing.T) {
		c, appErr := extractContent(map[string]any{"name": "Weekly sync"})
		require.Nil(t, appErr)
		assert.Len(t, c.Segments, 1)
	})

	t.Run("invalid", func(t *testing.T) {
		_, appErr := extractContent(nil)
		require.NotNil(t, appErr)
		assert.Equal(t, "ent.autotranslation.validate_content.nil_content", appErr.Id)

		_, appErr = extractContent(42)
		require.NotNil(t, appErr)
		assert.Equal(t, "ent.autotranslation.validate_content.invalid_type", appErr.Id)

		_, appErr = extractContent(json.RawMessage(`[1,2]`))
		require.NotNil(t, appErr)
		assert.Equal(t, "ent.autotranslation.validate_content.invalid_json", appErr.Id)

		_, appErr = extractContent(strings.Repeat("a ", maxContentBytes))
		require.NotNil(t, appErr)
		assert.Equal(t, http.StatusRequestEntityTooLarge, appErr.StatusCode)
	})
}
