// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package aiservice

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/mattermost/mattermost/server/public/ai"
)

// anthropicProvider speaks Anthropic's Messages API (POST /messages, streamed over SSE), with tools. Structured
// output forces a tool whose input is the schema: every Claude model supports it, and the tool's input is the
// answer. Anthropic has no embeddings, transcription or decisions.
type anthropicProvider struct {
	base
}

const (
	anthropicVersion = "2023-06-01"
	// anthropicDefaultMaxTokens is sent when the caller sets none: the API requires max_tokens.
	anthropicDefaultMaxTokens = 4096
	// anthropicOutputTool is the forced tool of structured output.
	anthropicOutputTool = "structured_output"
)

func (p *anthropicProvider) setAuth(h http.Header) {
	if p.cfg.APIKey != "" {
		h.Set("x-api-key", p.cfg.APIKey)
	}
	h.Set("anthropic-version", anthropicVersion)
}

func anthropicErrorMessage(data []byte) string {
	var body struct {
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(data, &body) != nil {
		return ""
	}
	return body.Error.Message
}

type anImageSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

type anBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	Source    *anImageSource  `json:"source,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   string          `json:"content,omitempty"`
}

type anMessage struct {
	Role    string    `json:"role"`
	Content []anBlock `json:"content"`
}

type anTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema"`
}

type anRequest struct {
	Model       string      `json:"model"`
	MaxTokens   int         `json:"max_tokens"`
	System      string      `json:"system,omitempty"`
	Messages    []anMessage `json:"messages"`
	Tools       []anTool    `json:"tools,omitempty"`
	ToolChoice  any         `json:"tool_choice,omitempty"`
	Temperature *float64    `json:"temperature,omitempty"`
	Stream      bool        `json:"stream,omitempty"`
}

type anUsage struct {
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`
}

type anResponse struct {
	Model      string    `json:"model"`
	Content    []anBlock `json:"content"`
	StopReason string    `json:"stop_reason"`
	Usage      anUsage   `json:"usage"`
}

// outputMode says how a request's structured output goes to Anthropic.
type outputMode int

const (
	// The zero value: no structured output.
	_ outputMode = iota
	// outputTool forces the structured_output tool; wrapped says the schema isn't an object and sits in "value".
	outputTool
	// outputPrompt puts the schema in the instructions, when the caller has tools of its own.
	outputPrompt
)

type anthropicPlan struct {
	mode    outputMode
	wrapped bool
}

func isObjectSchema(schema json.RawMessage) bool {
	var probe struct {
		Type string `json:"type"`
	}
	return json.Unmarshal(schema, &probe) == nil && probe.Type == "object"
}

func (p *anthropicProvider) build(req ai.ChatRequest, stream bool) (anRequest, anthropicPlan) {
	plan := anthropicPlan{}
	messages := req.Messages
	if len(req.JSONSchema) > 0 {
		if len(req.Tools) > 0 {
			plan.mode = outputPrompt
			messages = ai.WithJSONInstruction(messages, req.JSONSchema)
		} else {
			plan.mode = outputTool
			plan.wrapped = !isObjectSchema(req.JSONSchema)
		}
	}

	out := anRequest{Model: req.Model, MaxTokens: req.MaxTokens, Temperature: req.Temperature, Stream: stream}
	if out.MaxTokens <= 0 {
		out.MaxTokens = anthropicDefaultMaxTokens
	}
	var system []string
	for _, m := range messages {
		switch m.Role {
		case ai.RoleSystem:
			if m.Content != "" {
				system = append(system, m.Content)
			}
			continue
		case ai.RoleTool:
			out.Messages = appendAnthropic(out.Messages, "user", anBlock{Type: "tool_result", ToolUseID: m.ToolCallID, Content: m.Content})
			continue
		}
		role := "user"
		if m.Role == ai.RoleAssistant {
			role = "assistant"
		}
		var blocks []anBlock
		if m.Content != "" {
			blocks = append(blocks, anBlock{Type: "text", Text: m.Content})
		}
		for _, img := range m.Images {
			blocks = append(blocks, anBlock{Type: "image", Source: &anImageSource{Type: "base64", MediaType: img.MimeType, Data: base64.StdEncoding.EncodeToString(img.Data)}})
		}
		for _, tc := range m.ToolCalls {
			input := tc.Arguments
			if len(input) == 0 || !json.Valid(input) {
				input = json.RawMessage("{}")
			}
			blocks = append(blocks, anBlock{Type: "tool_use", ID: tc.ID, Name: tc.Name, Input: input})
		}
		if len(blocks) == 0 {
			blocks = []anBlock{{Type: "text", Text: "…"}}
		}
		out.Messages = appendAnthropic(out.Messages, role, blocks...)
	}
	out.System = strings.Join(system, "\n\n")

	for _, t := range req.Tools {
		schema := t.Parameters
		if len(schema) == 0 {
			schema = json.RawMessage(`{"type":"object","properties":{}}`)
		}
		out.Tools = append(out.Tools, anTool{Name: t.Name, Description: t.Description, InputSchema: schema})
	}
	switch req.ToolChoice {
	case "", ai.ToolChoiceAuto:
	case ai.ToolChoiceNone:
		out.ToolChoice = map[string]string{"type": "none"}
	case ai.ToolChoiceRequired:
		out.ToolChoice = map[string]string{"type": "any"}
	default:
		out.ToolChoice = map[string]string{"type": "tool", "name": req.ToolChoice}
	}
	if len(out.Tools) == 0 {
		out.ToolChoice = nil
	}

	if plan.mode == outputTool {
		schema := req.JSONSchema
		if plan.wrapped {
			schema = json.RawMessage(`{"type":"object","properties":{"value":` + string(req.JSONSchema) + `},"required":["value"]}`)
		}
		out.Tools = []anTool{{Name: anthropicOutputTool, Description: "Give the answer, following this schema.", InputSchema: schema}}
		out.ToolChoice = map[string]string{"type": "tool", "name": anthropicOutputTool}
	}
	return out, plan
}

// appendAnthropic adds blocks to the conversation, in the last message when it has the same role: Anthropic wants
// the roles to alternate, and tool results sit in the user's turn.
func appendAnthropic(messages []anMessage, role string, blocks ...anBlock) []anMessage {
	if n := len(messages); n > 0 && messages[n-1].Role == role {
		messages[n-1].Content = append(messages[n-1].Content, blocks...)
		return messages
	}
	return append(messages, anMessage{Role: role, Content: blocks})
}

// structuredAnswer gives the JSON answer of the structured_output tool's input.
func structuredAnswer(input json.RawMessage, plan anthropicPlan) (string, error) {
	if !plan.wrapped {
		return string(input), nil
	}
	var wrapper struct {
		Value json.RawMessage `json:"value"`
	}
	if err := json.Unmarshal(input, &wrapper); err != nil {
		return "", fmt.Errorf("the model's structured answer isn't valid: %w", err)
	}
	return string(wrapper.Value), nil
}

func (p *anthropicProvider) post(ctx context.Context, payload anRequest) (*http.Response, error) {
	body, err := jsonBody(payload)
	if err != nil {
		return nil, err
	}
	req, err := p.newRequest(ctx, http.MethodPost, "/messages", body, "application/json", p.setAuth)
	if err != nil {
		return nil, err
	}
	return p.do(req, anthropicErrorMessage)
}

func (p *anthropicProvider) Chat(ctx context.Context, req ai.ChatRequest) (*ai.ChatResponse, error) {
	ctx, cancel := p.withCallTimeout(ctx)
	defer cancel()
	payload, plan := p.build(req, false)
	resp, err := p.post(ctx, payload)
	if err != nil {
		return nil, err
	}
	var body anResponse
	if err := decodeJSON(resp, maxResponseBytes, &body); err != nil {
		return nil, err
	}
	out := &ai.ChatResponse{
		Model:        body.Model,
		Message:      ai.Message{Role: ai.RoleAssistant},
		FinishReason: normaliseFinishReason(body.StopReason),
		Usage:        ai.Usage{InputTokens: body.Usage.InputTokens, OutputTokens: body.Usage.OutputTokens},
	}
	var text strings.Builder
	for _, block := range body.Content {
		switch block.Type {
		case "text":
			text.WriteString(block.Text)
		case "tool_use":
			if plan.mode == outputTool && block.Name == anthropicOutputTool {
				answer, err := structuredAnswer(block.Input, plan)
				if err != nil {
					return nil, err
				}
				text.Reset()
				text.WriteString(answer)
				out.FinishReason = ai.FinishReasonStop
				continue
			}
			out.Message.ToolCalls = append(out.Message.ToolCalls, ai.ToolCall{ID: block.ID, Name: block.Name, Arguments: ai.MarshalToolArguments(block.Input)})
		}
	}
	out.Message.Content = text.String()
	if plan.mode == outputPrompt {
		out.Message.Content = ai.StripMarkdownCodeFencing(out.Message.Content)
	}
	if out.Model == "" {
		out.Model = req.Model
	}
	return out, nil
}

type anStreamEvent struct {
	Type    string `json:"type"`
	Index   int    `json:"index"`
	Message *struct {
		Model string  `json:"model"`
		Usage anUsage `json:"usage"`
	} `json:"message"`
	ContentBlock *anBlock `json:"content_block"`
	Delta        *struct {
		Type        string `json:"type"`
		Text        string `json:"text"`
		PartialJSON string `json:"partial_json"`
		Thinking    string `json:"thinking"`
		StopReason  string `json:"stop_reason"`
	} `json:"delta"`
	Usage *anUsage `json:"usage"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

func (p *anthropicProvider) ChatStream(ctx context.Context, req ai.ChatRequest) (*ai.Stream, error) {
	runner := newStreamRunner(ctx, p.cfg.StreamIdleTimeout)
	payload, plan := p.build(req, true)
	resp, err := p.post(runner.ctx, payload)
	if err != nil {
		runner.idle.Stop()
		runner.cancel()
		return nil, err
	}
	go p.readStream(runner, resp, plan)
	return runner.stream(), nil
}

func (p *anthropicProvider) readStream(runner *streamRunner, resp *http.Response, plan anthropicPlan) {
	defer resp.Body.Close()
	type toolBuilder struct {
		id, name string
		input    strings.Builder
	}
	var (
		tools        = map[int]*toolBuilder{}
		usage        ai.Usage
		finishReason string
		streamErr    error
		promptText   strings.Builder
	)
	err := readSSE(resp.Body, runner.touch, func(ev sseEvent) bool {
		if ev.Data == "" {
			return true
		}
		var event anStreamEvent
		if err := json.Unmarshal([]byte(ev.Data), &event); err != nil {
			streamErr = fmt.Errorf("the AI endpoint sent an invalid event: %w", err)
			return false
		}
		switch event.Type {
		case "message_start":
			if event.Message != nil {
				usage.InputTokens = event.Message.Usage.InputTokens
				usage.OutputTokens = event.Message.Usage.OutputTokens
			}
		case "content_block_start":
			if event.ContentBlock != nil && event.ContentBlock.Type == "tool_use" {
				tools[event.Index] = &toolBuilder{id: event.ContentBlock.ID, name: event.ContentBlock.Name}
			}
		case "content_block_delta":
			if event.Delta == nil {
				return true
			}
			switch event.Delta.Type {
			case "text_delta":
				if plan.mode == outputPrompt {
					// The answer is JSON: sent once complete, without its code fences.
					promptText.WriteString(event.Delta.Text)
					return true
				}
				return runner.send(ai.StreamEvent{Type: ai.EventText, Text: event.Delta.Text})
			case "thinking_delta":
				return runner.send(ai.StreamEvent{Type: ai.EventReasoning, Text: event.Delta.Thinking})
			case "input_json_delta":
				if b := tools[event.Index]; b != nil {
					b.input.WriteString(event.Delta.PartialJSON)
				}
			}
		case "message_delta":
			if event.Delta != nil && event.Delta.StopReason != "" {
				finishReason = normaliseFinishReason(event.Delta.StopReason)
			}
			if event.Usage != nil {
				usage.OutputTokens = event.Usage.OutputTokens
				if event.Usage.InputTokens > 0 {
					usage.InputTokens = event.Usage.InputTokens
				}
			}
		case "message_stop":
			return false
		case "error":
			msg := "the AI endpoint failed"
			if event.Error != nil && event.Error.Message != "" {
				msg = event.Error.Message
			}
			streamErr = errors.New(p.sanitize(msg))
			return false
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

	if plan.mode == outputPrompt && promptText.Len() > 0 {
		runner.send(ai.StreamEvent{Type: ai.EventText, Text: ai.StripMarkdownCodeFencing(promptText.String())})
	}
	indexes := make([]int, 0, len(tools))
	for i := range tools {
		indexes = append(indexes, i)
	}
	slices.Sort(indexes)
	var calls []ai.ToolCall
	for _, i := range indexes {
		b := tools[i]
		input := json.RawMessage(strings.TrimSpace(b.input.String()))
		if len(input) == 0 || !json.Valid(input) {
			input = json.RawMessage("{}")
		}
		if plan.mode == outputTool && b.name == anthropicOutputTool {
			answer, err := structuredAnswer(input, plan)
			if err != nil {
				runner.finish(runner.fail(err))
				return
			}
			runner.send(ai.StreamEvent{Type: ai.EventText, Text: answer})
			finishReason = ai.FinishReasonStop
			continue
		}
		calls = append(calls, ai.ToolCall{ID: b.id, Name: b.name, Arguments: input})
	}
	if len(calls) > 0 {
		runner.send(ai.StreamEvent{Type: ai.EventToolCalls, ToolCalls: calls})
	}
	runner.send(ai.StreamEvent{Type: ai.EventUsage, Usage: &usage})
	runner.finish(ai.StreamEvent{Type: ai.EventEnd, FinishReason: finishReason})
}

func (p *anthropicProvider) Embed(context.Context, ai.EmbedRequest) (*ai.EmbedResponse, error) {
	return nil, ai.ErrUnsupported
}

func (p *anthropicProvider) Transcribe(context.Context, ai.TranscribeRequest) (*ai.TranscribeResponse, error) {
	return nil, ai.ErrUnsupported
}

func (p *anthropicProvider) Decide(context.Context, ai.DecideRequest) (*ai.DecideResponse, error) {
	return nil, ai.ErrUnsupported
}

func (p *anthropicProvider) ListModels(ctx context.Context) ([]ai.ModelInfo, error) {
	ctx, cancel := p.withCallTimeout(ctx)
	defer cancel()
	req, err := p.newRequest(ctx, http.MethodGet, "/models?limit=1000", nil, "", p.setAuth)
	if err != nil {
		return nil, err
	}
	resp, err := p.do(req, anthropicErrorMessage)
	if err != nil {
		return nil, err
	}
	var body struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := decodeJSON(resp, maxResponseBytes, &body); err != nil {
		return nil, err
	}
	models := make([]ai.ModelInfo, 0, len(body.Data))
	for _, m := range body.Data {
		if m.ID != "" {
			models = append(models, ai.ModelInfo{ID: m.ID, OwnedBy: "anthropic"})
		}
	}
	slices.SortFunc(models, func(a, b ai.ModelInfo) int { return strings.Compare(a.ID, b.ID) })
	return models, nil
}
