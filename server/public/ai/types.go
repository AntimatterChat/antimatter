// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

// Package ai holds the request and response types of Antimatter's AI service: chat (with or without streaming,
// tools and structured output), embeddings, transcription, decisions and speech. The server runs every call against
// a model connection an admin set up; apps (plugins) reach the service through the plugin API (AIChat,
// AIChatStream, AIEmbed, AITranscribe, AIDecide, AIConnectionsForUser) with these types, and never see a key.
package ai

import (
	"encoding/json"
	"errors"
)

// Capability is what a model connection can be used for. Each has its own default model on the connection.
type Capability string

const (
	CapabilityChat          Capability = "chat"
	CapabilityEmbeddings    Capability = "embeddings"
	CapabilityTranscription Capability = "transcription"
	CapabilityDecisions     Capability = "decisions"
)

// Purpose names a built-in server feature calling the AI service on its own (with the connection an admin chose
// for it), as opposed to a call for a person. Only the server uses them: apps always call for a person.
type Purpose string

const (
	PurposeRecaps          Purpose = "recaps"
	PurposeRewrite         Purpose = "rewrite"
	PurposeAutoTranslation Purpose = "auto_translation"
	PurposeTextToSpeech    Purpose = "text_to_speech"
)

// BuiltInPurposes are the purposes of the server's own features.
var BuiltInPurposes = []Purpose{PurposeRecaps, PurposeRewrite, PurposeAutoTranslation, PurposeTextToSpeech}

// IsBuiltIn tells whether p is one of the server's own features.
func (p Purpose) IsBuiltIn() bool {
	for _, b := range BuiltInPurposes {
		if p == b {
			return true
		}
	}
	return false
}

// Role is who wrote a message of a conversation.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	// RoleTool carries a tool's result back to the model; ToolCallID says which call it answers.
	RoleTool Role = "tool"
)

// Image is a picture attached to a user message, for models that read images.
type Image struct {
	MimeType string `json:"mime_type"` // image/jpeg, image/png, image/gif or image/webp
	Data     []byte `json:"data"`
}

// IsSupportedImageMimeType tells whether the providers accept images of this type.
func IsSupportedImageMimeType(mimeType string) bool {
	switch mimeType {
	case "image/jpeg", "image/png", "image/gif", "image/webp":
		return true
	default:
		return false
	}
}

// Message is one turn of a conversation.
type Message struct {
	Role    Role    `json:"role"`
	Content string  `json:"content"`
	Images  []Image `json:"images,omitempty"`

	// ToolCalls are the tools an assistant message asked to run.
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	// ToolCallID is the call a RoleTool message answers.
	ToolCallID string `json:"tool_call_id,omitempty"`
}

// Tool is a function the model may ask to call. Parameters is its input as a JSON schema (an object).
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

// ToolCall is a model's request to run a tool, with its arguments as a JSON object.
type ToolCall struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// Tool choices: let the model decide, forbid tools, require one, or require a named tool (any other value).
const (
	ToolChoiceAuto     = "auto"
	ToolChoiceNone     = "none"
	ToolChoiceRequired = "required"
)

// ChatRequest is a chat completion. ConnectionID picks the model connection (empty: the caller's default, for the
// server's own features) and Model one of its models (empty: the connection's chat model).
type ChatRequest struct {
	ConnectionID string    `json:"connection_id"`
	Model        string    `json:"model,omitempty"`
	Messages     []Message `json:"messages"`

	Tools      []Tool `json:"tools,omitempty"`
	ToolChoice string `json:"tool_choice,omitempty"`

	// JSONSchema asks for structured output: the answer is a JSON value that follows this schema. Providers that
	// can't enforce a schema get it as an instruction, and the answer's code fences are removed.
	JSONSchema     json.RawMessage `json:"json_schema,omitempty"`
	JSONSchemaName string          `json:"json_schema_name,omitempty"`

	MaxTokens   int      `json:"max_tokens,omitempty"`
	Temperature *float64 `json:"temperature,omitempty"`

	// Operation is a short label for usage and audit records (e.g. "summarize_thread"); never content.
	Operation string `json:"operation,omitempty"`
}

// Usage counts the tokens a call used, as the provider reports them (or estimated when it doesn't).
type Usage struct {
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`
}

// Add adds other's tokens to u.
func (u *Usage) Add(other Usage) {
	u.InputTokens += other.InputTokens
	u.OutputTokens += other.OutputTokens
}

// Total is the tokens in and out.
func (u Usage) Total() int64 {
	return u.InputTokens + u.OutputTokens
}

// ChatResponse is the model's answer: an assistant message (text, tool calls or both).
type ChatResponse struct {
	Model        string  `json:"model"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason,omitempty"`
	Usage        Usage   `json:"usage"`
}

// Finish reasons, normalised across providers.
const (
	FinishReasonStop      = "stop"
	FinishReasonLength    = "length"
	FinishReasonToolCalls = "tool_calls"
)

// EmbedRequest asks for the embeddings of a batch of texts.
type EmbedRequest struct {
	ConnectionID string   `json:"connection_id"`
	Model        string   `json:"model,omitempty"`
	Input        []string `json:"input"`
	// Dimensions asks models that support it for shorter vectors; 0 keeps the model's own size.
	Dimensions int    `json:"dimensions,omitempty"`
	Operation  string `json:"operation,omitempty"`
}

// EmbedResponse holds one vector per input text, in order.
type EmbedResponse struct {
	Model      string      `json:"model"`
	Embeddings [][]float32 `json:"embeddings"`
	Usage      Usage       `json:"usage"`
}

// TranscribeRequest turns speech into text.
type TranscribeRequest struct {
	ConnectionID string `json:"connection_id"`
	Model        string `json:"model,omitempty"`
	Audio        []byte `json:"audio"`
	// FileName gives the audio's format by its extension (e.g. "call.webm"); it is sent to the provider as is.
	FileName string `json:"file_name"`
	Language string `json:"language,omitempty"` // ISO 639-1, empty to detect it
	Prompt   string `json:"prompt,omitempty"`
	// Segments asks for the timed segments too (verbose_json), which not every server supports.
	Segments  bool   `json:"segments,omitempty"`
	Operation string `json:"operation,omitempty"`
}

// TranscriptSegment is a stretch of speech, with its times in seconds.
type TranscriptSegment struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
}

// TranscribeResponse is the text of the audio.
type TranscribeResponse struct {
	Model    string              `json:"model"`
	Text     string              `json:"text"`
	Language string              `json:"language,omitempty"`
	Duration float64             `json:"duration,omitempty"`
	Segments []TranscriptSegment `json:"segments,omitempty"`
	Usage    Usage               `json:"usage"`
}

// SpeechRequest reads a text aloud (OpenAI-compatible /audio/speech).
type SpeechRequest struct {
	ConnectionID string `json:"connection_id"`
	Model        string `json:"model,omitempty"`
	Input        string `json:"input"`
	Voice        string `json:"voice,omitempty"`
	Format       string `json:"format,omitempty"` // mp3 (default), opus, wav...
}

// SpeechResponse is the audio of a SpeechRequest.
type SpeechResponse struct {
	Audio       []byte `json:"audio"`
	ContentType string `json:"content_type"`
}

// ModelInfo is a model a connection's endpoint offers.
type ModelInfo struct {
	ID      string `json:"id"`
	OwnedBy string `json:"owned_by,omitempty"`
}

// ErrUnsupported is returned by a provider for an operation it doesn't have (e.g. embeddings on Anthropic).
var ErrUnsupported = errors.New("the provider doesn't support this operation")

// ProviderError is an error the model's endpoint answered, its message already free of keys.
type ProviderError struct {
	StatusCode int
	Message    string
}

func (e *ProviderError) Error() string {
	return e.Message
}
