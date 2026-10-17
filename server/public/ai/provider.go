// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package ai

import "context"

// Provider is a model endpoint: OpenAI-compatible or Anthropic. The server builds one per call from a model
// connection, with its address and key; nothing outside the server sees it. A provider returns ErrUnsupported for
// what its endpoint can't do, and a *ProviderError for what the endpoint refused.
type Provider interface {
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
	// ChatStream streams the answer; the stream's producer stops when ctx ends or the stream is closed.
	ChatStream(ctx context.Context, req ChatRequest) (*Stream, error)
	Embed(ctx context.Context, req EmbedRequest) (*EmbedResponse, error)
	Transcribe(ctx context.Context, req TranscribeRequest) (*TranscribeResponse, error)
	Decide(ctx context.Context, req DecideRequest) (*DecideResponse, error)
	// ListModels lists the models the endpoint offers (GET /models).
	ListModels(ctx context.Context) ([]ModelInfo, error)
}

// SpeechProvider is a provider that reads text aloud (OpenAI-compatible /audio/speech).
type SpeechProvider interface {
	Speak(ctx context.Context, req SpeechRequest) (*SpeechResponse, error)
}
