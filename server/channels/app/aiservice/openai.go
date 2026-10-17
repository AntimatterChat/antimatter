// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package aiservice

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/mattermost/mattermost/server/public/ai"
)

// openAIProvider speaks the OpenAI API as OpenAI, Azure OpenAI's v1 endpoint, Mistral, Gemini's compatible endpoint,
// Ollama, vLLM, LiteLLM, OpenRouter and the like serve it: Chat Completions (streamed over SSE, with tools and
// JSON-schema output), /embeddings, /audio/transcriptions, /audio/speech, /models, and decisions (/decisions, or
// /systemone for the SystemOne dialect).
type openAIProvider struct {
	base
}

var _ ai.SpeechProvider = (*openAIProvider)(nil)

func (p *openAIProvider) setAuth(h http.Header) {
	if p.cfg.APIKey != "" {
		h.Set("Authorization", "Bearer "+p.cfg.APIKey)
	}
}

func openAIErrorMessage(data []byte) string {
	var body struct {
		Error json.RawMessage `json:"error"`
	}
	if json.Unmarshal(data, &body) != nil || len(body.Error) == 0 {
		return ""
	}
	var detailed struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(body.Error, &detailed) == nil && detailed.Message != "" {
		return detailed.Message
	}
	var plain string
	if json.Unmarshal(body.Error, &plain) == nil {
		return plain
	}
	return ""
}

func (p *openAIProvider) post(ctx context.Context, path string, payload any) (*http.Response, error) {
	body, err := jsonBody(payload)
	if err != nil {
		return nil, err
	}
	req, err := p.newRequest(ctx, http.MethodPost, path, body, "application/json", p.setAuth)
	if err != nil {
		return nil, err
	}
	return p.do(req, openAIErrorMessage)
}

// The Chat Completions wire format.

type oaContentPart struct {
	Type     string      `json:"type"`
	Text     string      `json:"text,omitempty"`
	ImageURL *oaImageURL `json:"image_url,omitempty"`
}

type oaImageURL struct {
	URL string `json:"url"`
}

type oaFunctionCall struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments"`
}

type oaToolCall struct {
	Index    *int           `json:"index,omitempty"`
	ID       string         `json:"id,omitempty"`
	Type     string         `json:"type,omitempty"`
	Function oaFunctionCall `json:"function"`
}

type oaMessage struct {
	Role       string       `json:"role"`
	Content    any          `json:"content"`
	ToolCalls  []oaToolCall `json:"tool_calls,omitempty"`
	ToolCallID string       `json:"tool_call_id,omitempty"`
}

type oaTool struct {
	Type     string         `json:"type"`
	Function oaToolFunction `json:"function"`
}

type oaToolFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type oaJSONSchema struct {
	Name   string          `json:"name"`
	Schema json.RawMessage `json:"schema"`
	Strict bool            `json:"strict"`
}

type oaResponseFormat struct {
	Type       string        `json:"type"`
	JSONSchema *oaJSONSchema `json:"json_schema,omitempty"`
}

type oaStreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type oaChatRequest struct {
	Model               string            `json:"model"`
	Messages            []oaMessage       `json:"messages"`
	Tools               []oaTool          `json:"tools,omitempty"`
	ToolChoice          any               `json:"tool_choice,omitempty"`
	ResponseFormat      *oaResponseFormat `json:"response_format,omitempty"`
	MaxTokens           int               `json:"max_tokens,omitempty"`
	MaxCompletionTokens int               `json:"max_completion_tokens,omitempty"`
	Temperature         *float64          `json:"temperature,omitempty"`
	Stream              bool              `json:"stream,omitempty"`
	StreamOptions       *oaStreamOptions  `json:"stream_options,omitempty"`
}

type oaUsage struct {
	PromptTokens     int64 `json:"prompt_tokens"`
	CompletionTokens int64 `json:"completion_tokens"`
}

type oaChatResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		Message struct {
			Content          *string      `json:"content"`
			ReasoningContent string       `json:"reasoning_content"`
			ToolCalls        []oaToolCall `json:"tool_calls"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage *oaUsage `json:"usage"`
}

type oaChatChunk struct {
	Model   string `json:"model"`
	Choices []struct {
		Delta struct {
			Content          *string      `json:"content"`
			ReasoningContent string       `json:"reasoning_content"`
			ToolCalls        []oaToolCall `json:"tool_calls"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Usage *oaUsage        `json:"usage"`
	Error json.RawMessage `json:"error"`
}

// usesMaxCompletionTokens tells whether the endpoint is OpenAI's own, whose newer models only take
// max_completion_tokens; compatible servers take max_tokens.
func (p *openAIProvider) usesMaxCompletionTokens() bool {
	u, err := url.Parse(p.baseURL)
	return err == nil && strings.EqualFold(u.Hostname(), "api.openai.com")
}

func toOpenAIMessages(messages []ai.Message) []oaMessage {
	out := make([]oaMessage, 0, len(messages))
	for _, m := range messages {
		msg := oaMessage{Role: string(m.Role), ToolCallID: m.ToolCallID}
		if len(m.Images) > 0 && m.Role == ai.RoleUser {
			parts := []oaContentPart{}
			if m.Content != "" {
				parts = append(parts, oaContentPart{Type: "text", Text: m.Content})
			}
			for _, img := range m.Images {
				parts = append(parts, oaContentPart{Type: "image_url", ImageURL: &oaImageURL{
					URL: "data:" + img.MimeType + ";base64," + base64.StdEncoding.EncodeToString(img.Data),
				}})
			}
			msg.Content = parts
		} else {
			msg.Content = m.Content
		}
		for _, tc := range m.ToolCalls {
			args := string(tc.Arguments)
			if args == "" {
				args = "{}"
			}
			msg.ToolCalls = append(msg.ToolCalls, oaToolCall{ID: tc.ID, Type: "function", Function: oaFunctionCall{Name: tc.Name, Arguments: args}})
		}
		if m.Role == ai.RoleAssistant && len(m.ToolCalls) > 0 && m.Content == "" {
			msg.Content = nil
		}
		out = append(out, msg)
	}
	return out
}

func openAIToolChoice(choice string) any {
	switch choice {
	case "":
		return nil
	case ai.ToolChoiceAuto, ai.ToolChoiceNone, ai.ToolChoiceRequired:
		return choice
	default:
		return map[string]any{"type": "function", "function": map[string]string{"name": choice}}
	}
}

// strictSchema tells whether a schema can be enforced strictly: OpenAI's strict mode needs objects that forbid
// additional properties.
func strictSchema(schema json.RawMessage) bool {
	var probe struct {
		AdditionalProperties *bool `json:"additionalProperties"`
	}
	return json.Unmarshal(schema, &probe) == nil && probe.AdditionalProperties != nil && !*probe.AdditionalProperties
}

// buildChat builds the request; promptSchema puts the JSON schema in the instructions instead of response_format.
func (p *openAIProvider) buildChat(req ai.ChatRequest, stream, promptSchema bool) oaChatRequest {
	messages := req.Messages
	if len(req.JSONSchema) > 0 && promptSchema {
		messages = ai.WithJSONInstruction(messages, req.JSONSchema)
	}
	out := oaChatRequest{
		Model:       req.Model,
		Messages:    toOpenAIMessages(messages),
		ToolChoice:  openAIToolChoice(req.ToolChoice),
		Temperature: req.Temperature,
	}
	for _, t := range req.Tools {
		params := t.Parameters
		if len(params) == 0 {
			params = json.RawMessage(`{"type":"object","properties":{}}`)
		}
		out.Tools = append(out.Tools, oaTool{Type: "function", Function: oaToolFunction{Name: t.Name, Description: t.Description, Parameters: params}})
	}
	if len(out.Tools) == 0 {
		out.ToolChoice = nil
	}
	if len(req.JSONSchema) > 0 && !promptSchema {
		name := req.JSONSchemaName
		if name == "" {
			name = "response"
		}
		out.ResponseFormat = &oaResponseFormat{Type: "json_schema", JSONSchema: &oaJSONSchema{Name: name, Schema: req.JSONSchema, Strict: strictSchema(req.JSONSchema)}}
	}
	if req.MaxTokens > 0 {
		if p.usesMaxCompletionTokens() {
			out.MaxCompletionTokens = req.MaxTokens
		} else {
			out.MaxTokens = req.MaxTokens
		}
	}
	if stream {
		out.Stream = true
		out.StreamOptions = &oaStreamOptions{IncludeUsage: true}
	}
	return out
}

// schemaRefused tells whether the endpoint refused a native JSON schema, so the call is worth retrying with the
// schema in the instructions (older compatible servers don't know response_format json_schema).
func schemaRefused(err error) bool {
	var pe *ai.ProviderError
	if !errors.As(err, &pe) || (pe.StatusCode != http.StatusBadRequest && pe.StatusCode != http.StatusUnprocessableEntity) {
		return false
	}
	msg := strings.ToLower(pe.Message)
	return strings.Contains(msg, "response_format") || strings.Contains(msg, "json_schema") || strings.Contains(msg, "structured")
}

func normaliseFinishReason(reason string) string {
	switch reason {
	case "stop", "end_turn", "stop_sequence":
		return ai.FinishReasonStop
	case "length", "max_tokens":
		return ai.FinishReasonLength
	case "tool_calls", "function_call", "tool_use":
		return ai.FinishReasonToolCalls
	default:
		return reason
	}
}

// thinkBlock matches the reasoning some models write before their answer.
var thinkBlock = regexp.MustCompile(`(?s)^\s*<think>.*?</think>\s*`)

func toToolCalls(calls []oaToolCall) []ai.ToolCall {
	out := make([]ai.ToolCall, 0, len(calls))
	for _, c := range calls {
		args := strings.TrimSpace(c.Function.Arguments)
		if args == "" || !json.Valid([]byte(args)) {
			args = "{}"
		}
		out = append(out, ai.ToolCall{ID: c.ID, Name: c.Function.Name, Arguments: json.RawMessage(args)})
	}
	return out
}

func estimatedUsage(req ai.ChatRequest, answer string) ai.Usage {
	return ai.Usage{InputTokens: int64(ai.EstimateMessagesTokens(req.Messages)), OutputTokens: int64(ai.EstimateTokens(answer))}
}

func (p *openAIProvider) Chat(ctx context.Context, req ai.ChatRequest) (*ai.ChatResponse, error) {
	ctx, cancel := p.withCallTimeout(ctx)
	defer cancel()

	resp, err := p.post(ctx, "/chat/completions", p.buildChat(req, false, false))
	if err != nil && len(req.JSONSchema) > 0 && schemaRefused(err) {
		resp, err = p.post(ctx, "/chat/completions", p.buildChat(req, false, true))
	}
	if err != nil {
		return nil, err
	}
	var body oaChatResponse
	if err := decodeJSON(resp, maxResponseBytes, &body); err != nil {
		return nil, err
	}
	if len(body.Choices) == 0 {
		return nil, errors.New("the AI endpoint gave no answer")
	}
	choice := body.Choices[0]
	content := ""
	if choice.Message.Content != nil {
		content = thinkBlock.ReplaceAllString(*choice.Message.Content, "")
	}
	if len(req.JSONSchema) > 0 {
		content = ai.StripMarkdownCodeFencing(content)
	}
	out := &ai.ChatResponse{
		Model:        body.Model,
		Message:      ai.Message{Role: ai.RoleAssistant, Content: content, ToolCalls: toToolCalls(choice.Message.ToolCalls)},
		FinishReason: normaliseFinishReason(choice.FinishReason),
	}
	if len(out.Message.ToolCalls) == 0 {
		out.Message.ToolCalls = nil
	}
	if out.Model == "" {
		out.Model = req.Model
	}
	if body.Usage != nil {
		out.Usage = ai.Usage{InputTokens: body.Usage.PromptTokens, OutputTokens: body.Usage.CompletionTokens}
	} else {
		out.Usage = estimatedUsage(req, content)
	}
	return out, nil
}

func (p *openAIProvider) ChatStream(ctx context.Context, req ai.ChatRequest) (*ai.Stream, error) {
	runner := newStreamRunner(ctx, p.cfg.StreamIdleTimeout)
	resp, err := p.post(runner.ctx, "/chat/completions", p.buildChat(req, true, false))
	if err != nil && len(req.JSONSchema) > 0 && schemaRefused(err) {
		resp, err = p.post(runner.ctx, "/chat/completions", p.buildChat(req, true, true))
	}
	if err != nil {
		runner.idle.Stop()
		runner.cancel()
		return nil, err
	}
	go p.readChatStream(runner, resp, req)
	return runner.stream(), nil
}

// toolCallBuilder gathers a streamed tool call from its deltas.
type toolCallBuilder struct {
	id, name string
	args     strings.Builder
}

// thinkFilter routes a streamed "<think>…</think>" prefix (some reasoning models through compatible servers) to
// reasoning events instead of the answer.
type thinkFilter struct {
	decided  bool
	inThink  bool
	buffered strings.Builder
}

// feed takes a chunk of content and gives the answer text and the reasoning text in it.
func (f *thinkFilter) feed(chunk string) (text, reasoning string) {
	if !f.decided {
		f.buffered.WriteString(chunk)
		buf := f.buffered.String()
		trimmed := strings.TrimLeft(buf, " \t\r\n")
		if len(trimmed) < len("<think>") && strings.HasPrefix("<think>", trimmed) {
			return "", ""
		}
		f.decided = true
		f.buffered.Reset()
		if !strings.HasPrefix(trimmed, "<think>") {
			return buf, ""
		}
		f.inThink = true
		chunk = strings.TrimPrefix(trimmed, "<think>")
	}
	if f.inThink {
		if idx := strings.Index(chunk, "</think>"); idx >= 0 {
			f.inThink = false
			return strings.TrimLeft(chunk[idx+len("</think>"):], " \t\r\n"), chunk[:idx]
		}
		return "", chunk
	}
	return chunk, ""
}

// flush gives what's still buffered at the end of the stream.
func (f *thinkFilter) flush() string {
	if f.decided {
		return ""
	}
	return f.buffered.String()
}

func (p *openAIProvider) readChatStream(runner *streamRunner, resp *http.Response, req ai.ChatRequest) {
	defer resp.Body.Close()
	var (
		text         strings.Builder
		calls        = map[int]*toolCallBuilder{}
		finishReason string
		usage        *ai.Usage
		streamErr    error
		filter       thinkFilter
	)
	sendText := func(content string) bool {
		answer, reasoning := filter.feed(content)
		if reasoning != "" && !runner.send(ai.StreamEvent{Type: ai.EventReasoning, Text: reasoning}) {
			return false
		}
		if answer != "" {
			text.WriteString(answer)
			return runner.send(ai.StreamEvent{Type: ai.EventText, Text: answer})
		}
		return true
	}
	err := readSSE(resp.Body, runner.touch, func(ev sseEvent) bool {
		data := strings.TrimSpace(ev.Data)
		if data == "" {
			return true
		}
		if data == "[DONE]" {
			return false
		}
		var chunk oaChatChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			streamErr = fmt.Errorf("the AI endpoint sent an invalid event: %w", err)
			return false
		}
		if len(chunk.Error) > 0 {
			msg := openAIErrorMessage([]byte(`{"error":` + string(chunk.Error) + `}`))
			if msg == "" {
				msg = "the AI endpoint failed"
			}
			streamErr = errors.New(p.sanitize(msg))
			return false
		}
		if chunk.Usage != nil {
			usage = &ai.Usage{InputTokens: chunk.Usage.PromptTokens, OutputTokens: chunk.Usage.CompletionTokens}
		}
		for _, choice := range chunk.Choices {
			if choice.Delta.ReasoningContent != "" && !runner.send(ai.StreamEvent{Type: ai.EventReasoning, Text: choice.Delta.ReasoningContent}) {
				return false
			}
			if choice.Delta.Content != nil && *choice.Delta.Content != "" && !sendText(*choice.Delta.Content) {
				return false
			}
			for i, tc := range choice.Delta.ToolCalls {
				index := i
				if tc.Index != nil {
					index = *tc.Index
				}
				b := calls[index]
				if b == nil {
					b = &toolCallBuilder{}
					calls[index] = b
				}
				if tc.ID != "" {
					b.id = tc.ID
				}
				if tc.Function.Name != "" {
					b.name = tc.Function.Name
				}
				b.args.WriteString(tc.Function.Arguments)
			}
			if choice.FinishReason != nil && *choice.FinishReason != "" {
				finishReason = normaliseFinishReason(*choice.FinishReason)
			}
		}
		return true
	})
	if streamErr == nil && err != nil {
		streamErr = err
	}
	if streamErr == nil && runner.ctx.Err() != nil {
		streamErr = runner.ctx.Err()
	}
	if streamErr != nil {
		runner.finish(runner.fail(streamErr))
		return
	}
	if rest := filter.flush(); rest != "" {
		text.WriteString(rest)
		runner.send(ai.StreamEvent{Type: ai.EventText, Text: rest})
	}
	if len(calls) > 0 {
		indexes := make([]int, 0, len(calls))
		for i := range calls {
			indexes = append(indexes, i)
		}
		sort.Ints(indexes)
		raw := make([]oaToolCall, 0, len(calls))
		for _, i := range indexes {
			raw = append(raw, oaToolCall{ID: calls[i].id, Function: oaFunctionCall{Name: calls[i].name, Arguments: calls[i].args.String()}})
		}
		runner.send(ai.StreamEvent{Type: ai.EventToolCalls, ToolCalls: toToolCalls(raw)})
		if finishReason == "" {
			finishReason = ai.FinishReasonToolCalls
		}
	}
	if usage == nil {
		estimated := estimatedUsage(req, text.String())
		usage = &estimated
	}
	runner.send(ai.StreamEvent{Type: ai.EventUsage, Usage: usage})
	runner.finish(ai.StreamEvent{Type: ai.EventEnd, FinishReason: finishReason})
}

func (p *openAIProvider) Embed(ctx context.Context, req ai.EmbedRequest) (*ai.EmbedResponse, error) {
	ctx, cancel := p.withCallTimeout(ctx)
	defer cancel()
	payload := map[string]any{"model": req.Model, "input": req.Input, "encoding_format": "float"}
	if req.Dimensions > 0 {
		payload["dimensions"] = req.Dimensions
	}
	resp, err := p.post(ctx, "/embeddings", payload)
	if err != nil {
		return nil, err
	}
	var body struct {
		Model string `json:"model"`
		Data  []struct {
			Index     int       `json:"index"`
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
		Usage *struct {
			PromptTokens int64 `json:"prompt_tokens"`
			TotalTokens  int64 `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := decodeJSON(resp, maxEmbedBytes, &body); err != nil {
		return nil, err
	}
	if len(body.Data) != len(req.Input) {
		return nil, fmt.Errorf("the AI endpoint gave %d embeddings for %d texts", len(body.Data), len(req.Input))
	}
	out := &ai.EmbedResponse{Model: body.Model, Embeddings: make([][]float32, len(req.Input))}
	for i, d := range body.Data {
		index := d.Index
		if index < 0 || index >= len(req.Input) || out.Embeddings[index] != nil {
			index = i
		}
		out.Embeddings[index] = d.Embedding
	}
	if out.Model == "" {
		out.Model = req.Model
	}
	if body.Usage != nil {
		out.Usage.InputTokens = max(body.Usage.PromptTokens, body.Usage.TotalTokens)
	} else {
		for _, text := range req.Input {
			out.Usage.InputTokens += int64(ai.EstimateTokens(text))
		}
	}
	return out, nil
}

func (p *openAIProvider) Transcribe(ctx context.Context, req ai.TranscribeRequest) (*ai.TranscribeResponse, error) {
	ctx, cancel := p.withCallTimeout(ctx)
	defer cancel()
	var buf bytes.Buffer
	form := multipart.NewWriter(&buf)
	fileName := req.FileName
	if fileName == "" {
		fileName = "audio.webm"
	}
	part, err := form.CreateFormFile("file", fileName)
	if err != nil {
		return nil, err
	}
	if _, err := part.Write(req.Audio); err != nil {
		return nil, err
	}
	fields := map[string]string{"model": req.Model, "language": req.Language, "prompt": req.Prompt, "response_format": "json"}
	if req.Segments {
		fields["response_format"] = "verbose_json"
	}
	for name, value := range fields {
		if value == "" {
			continue
		}
		if err := form.WriteField(name, value); err != nil {
			return nil, err
		}
	}
	if err := form.Close(); err != nil {
		return nil, err
	}
	httpReq, err := p.newRequest(ctx, http.MethodPost, "/audio/transcriptions", &buf, form.FormDataContentType(), p.setAuth)
	if err != nil {
		return nil, err
	}
	resp, err := p.do(httpReq, openAIErrorMessage)
	if err != nil {
		return nil, err
	}
	var body struct {
		Text     string                 `json:"text"`
		Language string                 `json:"language"`
		Duration float64                `json:"duration"`
		Segments []ai.TranscriptSegment `json:"segments"`
		Usage    *struct {
			InputTokens  int64 `json:"input_tokens"`
			OutputTokens int64 `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := decodeJSON(resp, maxResponseBytes, &body); err != nil {
		return nil, err
	}
	out := &ai.TranscribeResponse{Model: req.Model, Text: strings.TrimSpace(body.Text), Language: body.Language, Duration: body.Duration, Segments: body.Segments}
	if body.Usage != nil {
		out.Usage = ai.Usage{InputTokens: body.Usage.InputTokens, OutputTokens: body.Usage.OutputTokens}
	} else {
		out.Usage.OutputTokens = int64(ai.EstimateTokens(out.Text))
	}
	return out, nil
}

func (p *openAIProvider) Speak(ctx context.Context, req ai.SpeechRequest) (*ai.SpeechResponse, error) {
	ctx, cancel := p.withCallTimeout(ctx)
	defer cancel()
	format := req.Format
	if format == "" {
		format = "mp3"
	}
	payload := map[string]any{"model": req.Model, "input": req.Input, "response_format": format}
	if req.Voice != "" {
		payload["voice"] = req.Voice
	}
	resp, err := p.post(ctx, "/audio/speech", payload)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxAudioBytes+1))
	if err != nil {
		return nil, p.transportError(err)
	}
	if len(data) > maxAudioBytes {
		return nil, errors.New("the speech is too long")
	}
	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "audio/") {
		contentType = "audio/mpeg"
	}
	return &ai.SpeechResponse{Audio: data, ContentType: contentType}, nil
}

func (p *openAIProvider) ListModels(ctx context.Context) ([]ai.ModelInfo, error) {
	ctx, cancel := p.withCallTimeout(ctx)
	defer cancel()
	req, err := p.newRequest(ctx, http.MethodGet, "/models", nil, "", p.setAuth)
	if err != nil {
		return nil, err
	}
	resp, err := p.do(req, openAIErrorMessage)
	if err != nil {
		return nil, err
	}
	var body struct {
		Data []struct {
			ID      string `json:"id"`
			OwnedBy string `json:"owned_by"`
		} `json:"data"`
	}
	if err := decodeJSON(resp, maxResponseBytes, &body); err != nil {
		return nil, err
	}
	models := make([]ai.ModelInfo, 0, len(body.Data))
	for _, m := range body.Data {
		if m.ID != "" {
			models = append(models, ai.ModelInfo{ID: m.ID, OwnedBy: m.OwnedBy})
		}
	}
	sort.Slice(models, func(i, j int) bool { return models[i].ID < models[j].ID })
	return models, nil
}

// Decisions.

type oaDecisionChoice struct {
	Value       string `json:"value"`
	Description string `json:"description,omitempty"`
}

type oaDecisionQuestion struct {
	Type         string             `json:"type"`
	Name         string             `json:"name"`
	Instructions string             `json:"instructions"`
	Choices      []oaDecisionChoice `json:"choices,omitempty"`
	Levels       []ai.DecisionLevel `json:"levels,omitempty"`
}

type oaDecisionProbability struct {
	Value       json.RawMessage `json:"value"`
	Label       string          `json:"label"`
	Probability float64         `json:"probability"`
}

type oaDecisionAnswer struct {
	Type          string                  `json:"type"`
	Name          string                  `json:"name"`
	Probability   *float64                `json:"probability"`
	Choice        json.RawMessage         `json:"choice"`
	Score         *float64                `json:"score"`
	Confidence    *float64                `json:"confidence"`
	Probabilities []oaDecisionProbability `json:"probabilities"`
}

type decisionUsage struct {
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`
}

// rawString gives a JSON scalar as text: a string unquoted, anything else (true, 3) as written.
func rawString(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	return string(raw)
}

func (p *openAIProvider) Decide(ctx context.Context, req ai.DecideRequest) (*ai.DecideResponse, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	ctx, cancel := p.withCallTimeout(ctx)
	defer cancel()
	if p.cfg.DecisionsDialect == ai.DecisionsDialectSystemOne {
		return p.decideSystemOne(ctx, req)
	}

	questions := make([]oaDecisionQuestion, 0, len(req.Questions))
	for _, q := range req.Questions {
		oq := oaDecisionQuestion{Type: string(q.Type), Name: q.Name, Instructions: q.Instructions, Levels: q.Levels}
		for _, c := range q.Choices {
			oq.Choices = append(oq.Choices, oaDecisionChoice{Value: c.Value, Description: c.Description})
		}
		questions = append(questions, oq)
	}
	resp, err := p.post(ctx, "/decisions", map[string]any{"model": req.Model, "input": req.Input, "questions": questions})
	if err != nil {
		return nil, err
	}
	var body struct {
		Model   string             `json:"model"`
		Answers []oaDecisionAnswer `json:"answers"`
		Usage   *decisionUsage     `json:"usage"`
	}
	if err := decodeJSON(resp, maxResponseBytes, &body); err != nil {
		return nil, err
	}
	byName := make(map[string]oaDecisionAnswer, len(body.Answers))
	for i, a := range body.Answers {
		if a.Name == "" && i < len(req.Questions) {
			// Answers come in the questions' order; a server that doesn't echo the names still lines up.
			a.Name = req.Questions[i].Name
		}
		byName[a.Name] = a
	}
	out := &ai.DecideResponse{Model: body.Model, Answers: make([]ai.DecisionAnswer, 0, len(req.Questions))}
	for _, q := range req.Questions {
		a, ok := byName[q.Name]
		if !ok {
			return nil, fmt.Errorf("the decision model didn't answer %q", q.Name)
		}
		answer := ai.DecisionAnswer{Name: q.Name, Type: q.Type, Probability: a.Probability, Choice: rawString(a.Choice), Score: a.Score, Confidence: a.Confidence}
		for i, prob := range a.Probabilities {
			dp := ai.DecisionProbability{Value: rawString(prob.Value), Label: prob.Label, Probability: prob.Probability}
			if q.Type == ai.DecisionScore {
				if dp.Label == "" && i < len(q.Levels) {
					dp.Label = q.Levels[i].Label
				}
				if dp.Value == "" {
					dp.Value = strconv.Itoa(i)
				}
			}
			answer.Probabilities = append(answer.Probabilities, dp)
		}
		out.Answers = append(out.Answers, answer)
	}
	if out.Model == "" {
		out.Model = req.Model
	}
	if body.Usage != nil {
		out.Usage = ai.Usage{InputTokens: body.Usage.InputTokens, OutputTokens: body.Usage.OutputTokens}
	} else {
		out.Usage.InputTokens = int64(ai.EstimateTokens(req.Input))
	}
	return out, nil
}

// The SystemOne dialect (POST /systemone, Jev and Laya): the input is the "state"; questions and answers are keyed
// by name; a predicate is a "noul"; a choice's options and a score's levels are its "criteria" (a map of value to
// description, and a list of labels); a score's probabilities are keyed by level index, with a legend.

type soQuestion struct {
	Type         string `json:"type"`
	Instructions string `json:"instructions"`
	Criteria     any    `json:"criteria,omitempty"`
}

type soAnswer struct {
	Type          string             `json:"type"`
	Noul          *float64           `json:"noul"`
	Choice        json.RawMessage    `json:"choice"`
	Score         *float64           `json:"score"`
	Confidence    *float64           `json:"confidence"`
	Legend        map[string]string  `json:"legend"`
	Probabilities map[string]float64 `json:"probabilities"`
}

func (p *openAIProvider) decideSystemOne(ctx context.Context, req ai.DecideRequest) (*ai.DecideResponse, error) {
	questions := make(map[string]soQuestion, len(req.Questions))
	for _, q := range req.Questions {
		sq := soQuestion{Instructions: q.Instructions}
		switch q.Type {
		case ai.DecisionPredicate:
			sq.Type = "noul"
		case ai.DecisionChoice:
			sq.Type = "choice"
			criteria := make(map[string]string, len(q.Choices))
			for _, c := range q.Choices {
				criteria[c.Value] = c.Description
			}
			sq.Criteria = criteria
		case ai.DecisionScore:
			sq.Type = "score"
			labels := make([]string, 0, len(q.Levels))
			for _, l := range q.Levels {
				label := l.Label
				if l.Description != "" {
					label += ": " + l.Description
				}
				labels = append(labels, label)
			}
			sq.Criteria = labels
		}
		questions[q.Name] = sq
	}
	resp, err := p.post(ctx, "/systemone", map[string]any{"model": req.Model, "state": req.Input, "questions": questions})
	if err != nil {
		return nil, err
	}
	var body struct {
		Model   string              `json:"model"`
		Answers map[string]soAnswer `json:"answers"`
		Usage   *decisionUsage      `json:"usage"`
	}
	if err := decodeJSON(resp, maxResponseBytes, &body); err != nil {
		return nil, err
	}
	out := &ai.DecideResponse{Model: body.Model, Answers: make([]ai.DecisionAnswer, 0, len(req.Questions))}
	for _, q := range req.Questions {
		a, ok := body.Answers[q.Name]
		if !ok {
			return nil, fmt.Errorf("the decision model didn't answer %q", q.Name)
		}
		answer := ai.DecisionAnswer{Name: q.Name, Type: q.Type, Confidence: a.Confidence}
		switch q.Type {
		case ai.DecisionPredicate:
			answer.Probability = a.Noul
		case ai.DecisionChoice:
			answer.Choice = rawString(a.Choice)
			for _, c := range q.Choices {
				if prob, ok := a.Probabilities[c.Value]; ok {
					answer.Probabilities = append(answer.Probabilities, ai.DecisionProbability{Value: c.Value, Probability: prob})
				}
			}
		case ai.DecisionScore:
			answer.Score = a.Score
			for i, l := range q.Levels {
				key := strconv.Itoa(i)
				if prob, ok := a.Probabilities[key]; ok {
					answer.Probabilities = append(answer.Probabilities, ai.DecisionProbability{Value: key, Label: l.Label, Probability: prob})
				}
			}
		}
		out.Answers = append(out.Answers, answer)
	}
	if out.Model == "" {
		out.Model = req.Model
	}
	if body.Usage != nil {
		out.Usage = ai.Usage{InputTokens: body.Usage.InputTokens, OutputTokens: body.Usage.OutputTokens}
	} else {
		out.Usage.InputTokens = int64(ai.EstimateTokens(req.Input))
	}
	return out, nil
}
