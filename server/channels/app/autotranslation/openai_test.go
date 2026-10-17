// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package autotranslation

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeChatAPI emulates an OpenAI-compatible chat completions API. It answers "[<prompt's last line>]", after some
// thinking when think is set.
type fakeChatAPI struct {
	*httptest.Server
	think bool

	mu       sync.Mutex
	requests []map[string]any
	auth     []string
}

func newFakeChatAPI(t *testing.T) *fakeChatAPI {
	f := &fakeChatAPI{}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" || r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		f.mu.Lock()
		f.requests = append(f.requests, body)
		f.auth = append(f.auth, r.Header.Get("Authorization"))
		f.mu.Unlock()

		if body["model"] != "hy-mt" {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":{"message":"model not found"}}`))
			return
		}
		prompt := body["messages"].([]any)[0].(map[string]any)["content"].(string)
		lines := strings.Split(prompt, "\n")
		answer := "[" + lines[len(lines)-1] + "]"
		if f.think {
			answer = "<think>\nLet me translate.\n</think>\n\n" + answer
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{"message": map[string]any{"role": "assistant", "content": answer}}},
		})
	}))
	t.Cleanup(f.Close)
	return f
}

func TestOpenAIProvider(t *testing.T) {
	f := newFakeChatAPI(t)
	p := newOpenAIProvider(f.Client(), f.URL+"/v1/", "secret", "hy-mt", 5*time.Second)
	assert.Equal(t, providerOpenAI, p.Name())

	t.Run("translates each segment, leaving those without words", func(t *testing.T) {
		res, err := p.Translate(context.Background(), []string{"Bonjour {{0}}", " ", "à demain"}, "fr", "en")
		require.NoError(t, err)
		assert.Equal(t, []string{"[Bonjour {{0}}]", " ", "[à demain]"}, res.Texts)
		assert.Empty(t, res.SourceLang)

		f.mu.Lock()
		defer f.mu.Unlock()
		require.Len(t, f.requests, 2)
		assert.Equal(t, "Bearer secret", f.auth[0])
		assert.Equal(t, map[string]any{"enable_thinking": false}, f.requests[0]["chat_template_kwargs"])
		prompt := f.requests[0]["messages"].([]any)[0].(map[string]any)["content"]
		assert.Equal(t, "Translate the following segment into English, without additional explanation.\n\nBonjour {{0}}", prompt)
	})

	t.Run("prompts in Chinese for Chinese", func(t *testing.T) {
		res, err := p.Translate(context.Background(), []string{"Good morning"}, "en", "zh")
		require.NoError(t, err)
		assert.Equal(t, []string{"[Good morning]"}, res.Texts)
		f.mu.Lock()
		defer f.mu.Unlock()
		prompt := f.requests[len(f.requests)-1]["messages"].([]any)[0].(map[string]any)["content"].(string)
		assert.True(t, strings.HasPrefix(prompt, "将以下文本翻译为中文"))
	})

	t.Run("drops the model's thinking and detects the source", func(t *testing.T) {
		f.think = true
		defer func() { f.think = false }()
		res, err := p.Translate(context.Background(), []string{"Das ist ein ziemlich langer deutscher Satz über das Wetter heute."}, "", "en")
		require.NoError(t, err)
		assert.Equal(t, []string{"[Das ist ein ziemlich langer deutscher Satz über das Wetter heute.]"}, res.Texts)
		assert.Equal(t, "de", res.SourceLang)
		require.NotNil(t, res.Confidence)
	})

	t.Run("reports the API's error", func(t *testing.T) {
		bad := newOpenAIProvider(f.Client(), f.URL+"/v1", "", "other", 5*time.Second)
		_, err := bad.Translate(context.Background(), []string{"Hello"}, "en", "fr")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "model not found")
		f.mu.Lock()
		defer f.mu.Unlock()
		assert.Empty(t, f.auth[len(f.auth)-1])
	})
}

func TestOpenAIProviderTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	}))
	defer srv.Close()
	p := newOpenAIProvider(srv.Client(), srv.URL, "", "hy-mt", 50*time.Millisecond)
	_, err := p.Translate(context.Background(), []string{"Hello"}, "en", "fr")
	require.Error(t, err)
	assert.True(t, errors.Is(err, context.DeadlineExceeded))
}
