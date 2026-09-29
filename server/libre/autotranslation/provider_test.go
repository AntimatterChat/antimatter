// Copyright (c) 2026-present Mattermost Libre contributors.
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

	"github.com/mattermost/mattermost/server/v8/channels/app"
)

// fakeLibreTranslate emulates the parts of the LibreTranslate API used here.
// Translations are "<target>:" + input.
type fakeLibreTranslate struct {
	*httptest.Server
	apiKey string

	mu        sync.Mutex
	requests  []map[string]any
	failWith  int
	translate int
	detect    int
}

func newFakeLibreTranslate(t *testing.T, apiKey string) *fakeLibreTranslate {
	f := &fakeLibreTranslate{apiKey: apiKey}
	f.Server = httptest.NewServer(http.HandlerFunc(f.handle))
	t.Cleanup(f.Close)
	return f
}

func (f *fakeLibreTranslate) handle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.URL.Path == "/languages" {
		_, _ = w.Write([]byte(`[{"code":"en"},{"code":"es"},{"code":"pt"},{"code":"zh-Hant"},{"code":"zh-Hans"}]`))
		return
	}

	var body map[string]any
	_ = json.NewDecoder(r.Body).Decode(&body)
	f.mu.Lock()
	f.requests = append(f.requests, body)
	failWith := f.failWith
	f.mu.Unlock()

	if failWith != 0 {
		w.WriteHeader(failWith)
		_, _ = w.Write([]byte(`{"error":"boom"}`))
		return
	}
	if f.apiKey != "" && body["api_key"] != f.apiKey {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"Invalid API key"}`))
		return
	}

	detected := func(s string) string {
		if strings.Contains(s, "Hola") {
			return "es"
		}
		return "en"
	}

	switch r.URL.Path {
	case "/detect":
		f.mu.Lock()
		f.detect++
		f.mu.Unlock()
		q, _ := body["q"].(string)
		_, _ = w.Write([]byte(`[{"language":"` + detected(q) + `","confidence":92.0}]`))
	case "/translate":
		f.mu.Lock()
		f.translate++
		f.mu.Unlock()
		target, _ := body["target"].(string)
		resp := map[string]any{}
		switch q := body["q"].(type) {
		case string:
			resp["translatedText"] = target + ":" + q
			if body["source"] == "auto" {
				resp["detectedLanguage"] = map[string]any{"language": detected(q), "confidence": 80.0}
			}
		case []any:
			out := make([]string, len(q))
			dets := make([]map[string]any, len(q))
			for i, s := range q {
				out[i] = target + ":" + s.(string)
				dets[i] = map[string]any{"language": detected(s.(string)), "confidence": 70.0}
			}
			resp["translatedText"] = out
			if body["source"] == "auto" {
				resp["detectedLanguage"] = dets
			}
		}
		_ = json.NewEncoder(w).Encode(resp)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (f *fakeLibreTranslate) lastRequest() map[string]any {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.requests[len(f.requests)-1]
}

func (f *fakeLibreTranslate) counts() (translate, detect int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.translate, f.detect
}

func TestLibreTranslateProvider(t *testing.T) {
	lt := newFakeLibreTranslate(t, "secret")
	p := newLibreTranslateProvider(http.DefaultClient, lt.URL+"/", "secret", 2*time.Second)
	ctx := context.Background()

	t.Run("single text with detection", func(t *testing.T) {
		res, err := p.Translate(ctx, []string{"Hello {{0}}"}, "", "es")
		require.NoError(t, err)
		assert.Equal(t, []string{"es:Hello {{0}}"}, res.Texts)
		assert.Equal(t, "en", res.SourceLang)
		require.NotNil(t, res.Confidence)
		assert.InDelta(t, 0.8, *res.Confidence, 0.001)

		req := lt.lastRequest()
		assert.Equal(t, "auto", req["source"])
		assert.Equal(t, "text", req["format"])
		assert.Equal(t, "secret", req["api_key"])
	})

	t.Run("several texts", func(t *testing.T) {
		res, err := p.Translate(ctx, []string{"one", "two"}, "en", "pt")
		require.NoError(t, err)
		assert.Equal(t, []string{"pt:one", "pt:two"}, res.Texts)
		assert.Equal(t, "en", lt.lastRequest()["source"])
	})

	t.Run("language code mapping", func(t *testing.T) {
		_, err := p.Translate(ctx, []string{"hello"}, "en", "zh")
		require.NoError(t, err)
		assert.Equal(t, "zh-Hans", lt.lastRequest()["target"])

		_, err = p.Translate(ctx, []string{"hello"}, "en", "fr")
		require.Error(t, err, "unsupported target language")
	})

	t.Run("detect", func(t *testing.T) {
		lang, conf, err := p.Detect(ctx, "Hola amigos")
		require.NoError(t, err)
		assert.Equal(t, "es", lang)
		require.NotNil(t, conf)
		assert.InDelta(t, 0.92, *conf, 0.001)
	})

	t.Run("error status", func(t *testing.T) {
		lt.mu.Lock()
		lt.failWith = http.StatusTooManyRequests
		lt.mu.Unlock()
		defer func() {
			lt.mu.Lock()
			lt.failWith = 0
			lt.mu.Unlock()
		}()
		_, err := p.Translate(ctx, []string{"hello"}, "en", "es")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "429")
		assert.Contains(t, err.Error(), "boom")
	})

	t.Run("wrong api key", func(t *testing.T) {
		bad := newLibreTranslateProvider(http.DefaultClient, lt.URL, "wrong", time.Second)
		_, err := bad.Translate(ctx, []string{"hello"}, "en", "es")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Invalid API key")
	})
}

func TestLibreTranslateTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	}))
	defer srv.Close()

	p := newLibreTranslateProvider(http.DefaultClient, srv.URL, "", 50*time.Millisecond)
	_, err := p.Translate(context.Background(), []string{"hello"}, "en", "es")
	require.Error(t, err)
	assert.True(t, errors.Is(err, context.DeadlineExceeded))
}

func TestAgentsProvider(t *testing.T) {
	h := &fakeHost{agentsAvailable: true}
	p := newAgentsProvider(h, "openai")
	ctx := context.Background()

	t.Run("translate", func(t *testing.T) {
		h.agentsReply = func(serviceID string, req app.BridgeCompletionRequest) (string, error) {
			assert.Equal(t, "openai", serviceID)
			assert.Equal(t, app.BridgeOperationAutoTranslate, req.Operation)
			require.Len(t, req.Messages, 2)
			var input struct {
				Target string   `json:"target_language"`
				Source string   `json:"source_language"`
				Texts  []string `json:"texts"`
			}
			require.NoError(t, json.Unmarshal([]byte(req.Messages[1].Message), &input))
			assert.Equal(t, "es", input.Target)
			assert.Equal(t, "auto", input.Source)
			assert.Equal(t, []string{"Hello {{0}}"}, input.Texts)
			return "```json\n{\"translations\":[\"Hola {{0}}\"],\"source_language\":\"en-US\",\"confidence\":0.97}\n```", nil
		}
		res, err := p.Translate(ctx, []string{"Hello {{0}}"}, "", "es")
		require.NoError(t, err)
		assert.Equal(t, []string{"Hola {{0}}"}, res.Texts)
		assert.Equal(t, "en", res.SourceLang)
		require.NotNil(t, res.Confidence)
		assert.InDelta(t, 0.97, *res.Confidence, 0.001)
	})

	t.Run("wrong number of translations", func(t *testing.T) {
		h.agentsReply = func(string, app.BridgeCompletionRequest) (string, error) {
			return `{"translations":[],"source_language":"en","confidence":1}`, nil
		}
		_, err := p.Translate(ctx, []string{"Hello"}, "", "es")
		require.Error(t, err)
	})

	t.Run("detect", func(t *testing.T) {
		h.agentsReply = func(string, app.BridgeCompletionRequest) (string, error) {
			return `{"language":"fr","confidence":0.9}`, nil
		}
		lang, conf, err := p.Detect(ctx, "Bonjour")
		require.NoError(t, err)
		assert.Equal(t, "fr", lang)
		assert.InDelta(t, 0.9, *conf, 0.001)
	})

	t.Run("bridge unavailable", func(t *testing.T) {
		h.agentsAvailable = false
		defer func() { h.agentsAvailable = true }()
		_, err := p.Translate(ctx, []string{"Hello"}, "", "es")
		require.ErrorIs(t, err, errAgentsUnavailable)
	})
}
