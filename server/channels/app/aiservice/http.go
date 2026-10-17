// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

// Package aiservice holds the parts of the server's AI service that don't need the app: the providers (an
// OpenAI-compatible endpoint, Anthropic's Messages API), the encryption of the connections' keys, and the rate
// limiter. The app (channels/app/ai_*.go) adds connections, permissions, audiences, budgets, usage and audit.
//
// No cloud SDK: Antimatter only talks to an OpenAI-compatible or an Anthropic endpoint. Cloud services with their own
// sign-in (Bedrock, Vertex, Azure with Entra ID) are reached through their OpenAI-compatible endpoints or a gateway
// such as LiteLLM or OpenRouter.
package aiservice

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/mattermost/mattermost/server/public/ai"
)

// Provider names, as stored on a model connection.
const (
	ProviderOpenAICompatible = "openai_compatible"
	ProviderAnthropic        = "anthropic"
)

// Default addresses when a connection leaves its base URL empty.
const (
	DefaultOpenAIBaseURL    = "https://api.openai.com/v1"
	DefaultAnthropicBaseURL = "https://api.anthropic.com/v1"
)

const (
	// DefaultCallTimeout bounds a call that doesn't stream.
	DefaultCallTimeout = 3 * time.Minute
	// DefaultStreamIdleTimeout ends a stream that sends nothing for this long.
	DefaultStreamIdleTimeout = 5 * time.Minute

	maxResponseBytes  = 16 * 1024 * 1024
	maxEmbedBytes     = 64 * 1024 * 1024
	maxAudioBytes     = 32 * 1024 * 1024
	maxErrorBodyBytes = 8 * 1024
	maxSSELineBytes   = 4 * 1024 * 1024
)

// Config is what a provider needs from a model connection. The key and headers are in clear here: build a Config
// only for the call, and never log it.
type Config struct {
	Provider         string
	BaseURL          string
	APIKey           string
	Headers          map[string]string
	DecisionsDialect ai.DecisionsDialect
	// HTTPClient makes the requests. Its Timeout should be 0: calls are bounded by their context (CallTimeout) and
	// streams by StreamIdleTimeout. Community connections get a client that refuses internal addresses.
	HTTPClient        *http.Client
	CallTimeout       time.Duration
	StreamIdleTimeout time.Duration
}

// New builds the provider of a connection.
func New(cfg Config) (ai.Provider, error) {
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{}
	}
	if cfg.CallTimeout <= 0 {
		cfg.CallTimeout = DefaultCallTimeout
	}
	if cfg.StreamIdleTimeout <= 0 {
		cfg.StreamIdleTimeout = DefaultStreamIdleTimeout
	}
	if !cfg.DecisionsDialect.IsValid() {
		cfg.DecisionsDialect = ai.DecisionsDialectOpenAI
	}
	switch cfg.Provider {
	case ProviderOpenAICompatible:
		if cfg.BaseURL == "" {
			cfg.BaseURL = DefaultOpenAIBaseURL
		}
		return &openAIProvider{base: newBase(cfg)}, nil
	case ProviderAnthropic:
		if cfg.BaseURL == "" {
			cfg.BaseURL = DefaultAnthropicBaseURL
		}
		return &anthropicProvider{base: newBase(cfg)}, nil
	default:
		return nil, fmt.Errorf("unknown AI provider %q", cfg.Provider)
	}
}

// base holds what both providers share: the endpoint, its secrets (for redaction) and the HTTP plumbing.
type base struct {
	cfg     Config
	baseURL string
	secrets []string
}

func newBase(cfg Config) base {
	secrets := []string{cfg.APIKey}
	for _, v := range cfg.Headers {
		secrets = append(secrets, v)
	}
	return base{cfg: cfg, baseURL: strings.TrimRight(cfg.BaseURL, "/"), secrets: secrets}
}

// sanitize removes the connection's secrets from a message.
func (b *base) sanitize(message string) string {
	return ai.SanitizeProviderErrorMessage(message, b.secrets...)
}

// transportError turns a failed request into an error free of secrets (a URL may carry a key in a query).
func (b *base) transportError(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return errors.New(b.sanitize(err.Error()))
}

// newRequest builds a request to path, with auth set by the provider's setAuth and the connection's headers last.
func (b *base) newRequest(ctx context.Context, method, path string, body io.Reader, contentType string, setAuth func(http.Header)) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, b.baseURL+path, body)
	if err != nil {
		return nil, errors.New(b.sanitize(err.Error()))
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if setAuth != nil {
		setAuth(req.Header)
	}
	for name, value := range b.cfg.Headers {
		req.Header.Set(name, value)
	}
	return req, nil
}

// do sends a request and returns its response when the status is 2xx, else a *ai.ProviderError from its body.
func (b *base) do(req *http.Request, errorMessage func([]byte) string) (*http.Response, error) {
	resp, err := b.cfg.HTTPClient.Do(req)
	if err != nil {
		return nil, b.transportError(err)
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return resp, nil
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
	message := ""
	if errorMessage != nil {
		message = errorMessage(data)
	}
	if message == "" {
		message = strings.TrimSpace(string(data))
	}
	if message == "" {
		message = http.StatusText(resp.StatusCode)
	}
	return nil, &ai.ProviderError{StatusCode: resp.StatusCode, Message: b.sanitize(fmt.Sprintf("%d: %s", resp.StatusCode, message))}
}

// decodeJSON reads a JSON response of at most limit bytes into out.
func decodeJSON(resp *http.Response, limit int64, out any) error {
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return err
	}
	if int64(len(data)) > limit {
		return errors.New("the AI endpoint's answer is too large")
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("the AI endpoint's answer isn't valid JSON: %w", err)
	}
	return nil
}

func jsonBody(v any) (io.Reader, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}

// withCallTimeout bounds a call that doesn't stream, unless the caller already set a deadline.
func (b *base) withCallTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, b.cfg.CallTimeout)
}

// sseEvent is one server-sent event: its name (may be empty) and its data lines joined.
type sseEvent struct {
	Event string
	Data  string
}

// readSSE reads server-sent events from r and calls onEvent for each; it stops at the end of r, on an error, or
// when onEvent returns false. Each line read resets the idle timer (touch).
func readSSE(r io.Reader, touch func(), onEvent func(sseEvent) bool) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), maxSSELineBytes)
	var event sseEvent
	var data []string
	dispatch := func() bool {
		if len(data) == 0 && event.Event == "" {
			return true
		}
		event.Data = strings.Join(data, "\n")
		ok := onEvent(event)
		event, data = sseEvent{}, nil
		return ok
	}
	for scanner.Scan() {
		if touch != nil {
			touch()
		}
		line := scanner.Text()
		switch {
		case line == "":
			if !dispatch() {
				return nil
			}
		case strings.HasPrefix(line, ":"):
			// A comment, sent as a keep-alive.
		case strings.HasPrefix(line, "data:"):
			data = append(data, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		case strings.HasPrefix(line, "event:"):
			event.Event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	dispatch()
	return nil
}

// streamRunner runs a stream's producer: it owns the events channel, the idle watchdog and the cancellation, and
// sends exactly one final event (end or error).
type streamRunner struct {
	events      chan ai.StreamEvent
	ctx         context.Context
	cancel      context.CancelFunc
	idle        *time.Timer
	idleTimeout time.Duration
	timedOut    atomic.Bool
}

func newStreamRunner(ctx context.Context, idleTimeout time.Duration) *streamRunner {
	ctx, cancel := context.WithCancel(ctx)
	r := &streamRunner{events: make(chan ai.StreamEvent, 16), ctx: ctx, cancel: cancel, idleTimeout: idleTimeout}
	r.idle = time.AfterFunc(idleTimeout, func() {
		r.timedOut.Store(true)
		cancel()
	})
	return r
}

// touch tells the watchdog the endpoint is still sending.
func (r *streamRunner) touch() {
	r.idle.Reset(r.idleTimeout)
}

// fail gives the final event of a stream that failed with err.
func (r *streamRunner) fail(err error) ai.StreamEvent {
	if r.timedOut.Load() {
		return ai.StreamEvent{Type: ai.EventError, Error: "the AI endpoint stopped sending"}
	}
	return errorEvent(err)
}

// send delivers an event unless the stream was stopped; it tells whether to go on.
func (r *streamRunner) send(event ai.StreamEvent) bool {
	select {
	case r.events <- event:
		return true
	case <-r.ctx.Done():
		return false
	}
}

// finish sends the final event (best effort: a reader that stopped doesn't block it) and closes the stream.
func (r *streamRunner) finish(final ai.StreamEvent) {
	r.idle.Stop()
	select {
	case r.events <- final:
	default:
		select {
		case r.events <- final:
		case <-time.After(time.Second):
		}
	}
	close(r.events)
	r.cancel()
}

func (r *streamRunner) stream() *ai.Stream {
	return ai.NewStream(r.events, r.cancel)
}

func errorEvent(err error) ai.StreamEvent {
	if errors.Is(err, context.Canceled) {
		return ai.StreamEvent{Type: ai.EventError, Error: "the call was stopped"}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return ai.StreamEvent{Type: ai.EventError, Error: "the AI endpoint took too long to answer"}
	}
	return ai.StreamEvent{Type: ai.EventError, Error: err.Error()}
}
