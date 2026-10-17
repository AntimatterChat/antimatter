// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/utils"
)

// Reading messages aloud, by an OpenAI-compatible text-to-speech API (TextToSpeechSettings).

const (
	speechMaxRunes      = 4000
	speechMaxAudioBytes = 32 * 1024 * 1024
	speechTimeout       = 2 * time.Minute
	speechCacheSize     = 64
)

type speechAudio struct {
	data        []byte
	contentType string
}

// speechCache keeps the last messages read, as they're often played again.
var speechCache = struct {
	sync.Mutex
	keys    []string
	entries map[string]speechAudio
}{entries: map[string]speechAudio{}}

func cachedSpeech(key string) (speechAudio, bool) {
	speechCache.Lock()
	defer speechCache.Unlock()
	a, ok := speechCache.entries[key]
	return a, ok
}

func cacheSpeech(key string, audio speechAudio) {
	speechCache.Lock()
	defer speechCache.Unlock()
	if _, ok := speechCache.entries[key]; ok {
		return
	}
	speechCache.entries[key] = audio
	speechCache.keys = append(speechCache.keys, key)
	if len(speechCache.keys) > speechCacheSize {
		delete(speechCache.entries, speechCache.keys[0])
		speechCache.keys = speechCache.keys[1:]
	}
}

// GetPostSpeech returns a message read aloud, as audio, and its content type.
func (a *App) GetPostSpeech(rctx request.CTX, post *model.Post) ([]byte, string, *model.AppError) {
	settings := a.Config().TextToSpeechSettings
	if !*settings.Enable {
		return nil, "", model.NewAppError("GetPostSpeech", "app.post.speech.disabled.app_error", nil, "", http.StatusNotImplemented)
	}
	text := utils.SpeechText(post.Message, speechMaxRunes)
	if text == "" {
		return nil, "", model.NewAppError("GetPostSpeech", "app.post.speech.empty.app_error", nil, "", http.StatusBadRequest)
	}

	key := fmt.Sprintf("%s:%d:%s:%s:%s", post.Id, post.EditAt, *settings.URL, *settings.Model, *settings.Voice)
	if audio, ok := cachedSpeech(key); ok {
		return audio.data, audio.contentType, nil
	}

	body, err := json.Marshal(map[string]any{
		"model":           *settings.Model,
		"input":           text,
		"voice":           *settings.Voice,
		"response_format": "mp3",
	})
	if err != nil {
		return nil, "", model.NewAppError("GetPostSpeech", "app.post.speech.failed.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), speechTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(*settings.URL, "/")+"/audio/speech", bytes.NewReader(body))
	if err != nil {
		return nil, "", model.NewAppError("GetPostSpeech", "app.post.speech.failed.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if *settings.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+*settings.APIKey)
	}

	// The API's address is the administrator's: it may be on the local network.
	resp, err := a.HTTPService().MakeClient(true).Do(req)
	if err != nil {
		return nil, "", model.NewAppError("GetPostSpeech", "app.post.speech.failed.app_error", nil, "", http.StatusBadGateway).Wrap(err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, speechMaxAudioBytes))
	if err != nil {
		return nil, "", model.NewAppError("GetPostSpeech", "app.post.speech.failed.app_error", nil, "", http.StatusBadGateway).Wrap(err)
	}
	if resp.StatusCode != http.StatusOK {
		rctx.Logger().Warn("The text-to-speech API failed", mlog.Int("status", resp.StatusCode), mlog.String("body", string(data[:min(len(data), 300)])))
		return nil, "", model.NewAppError("GetPostSpeech", "app.post.speech.failed.app_error", nil, fmt.Sprintf("status %d", resp.StatusCode), http.StatusBadGateway)
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "audio/") {
		contentType = "audio/mpeg"
	}
	cacheSpeech(key, speechAudio{data: data, contentType: contentType})
	return data, contentType, nil
}
