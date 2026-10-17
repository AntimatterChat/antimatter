// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

// The stream events follow the Agents plugin's llm.TextStreamResult (Apache-2.0), with typed fields in place of
// its `any` value so that they cross the plugin RPC boundary.

package ai

import (
	"encoding/json"
	"errors"
	"strings"
	"sync"
)

// EventType is the kind of a streamed event.
type EventType string

const (
	// EventText carries a chunk of the answer's text.
	EventText EventType = "text"
	// EventReasoning carries a chunk of a model's reasoning summary, for models that share it.
	EventReasoning EventType = "reasoning"
	// EventToolCalls carries the complete tool calls of the answer, once the model finished them.
	EventToolCalls EventType = "tool_calls"
	// EventUsage carries the tokens the call used.
	EventUsage EventType = "usage"
	// EventEnd ends a stream that went well; FinishReason says why the model stopped.
	EventEnd EventType = "end"
	// EventError ends a stream that failed; Error says why (free of keys).
	EventError EventType = "error"
)

// StreamEvent is one event of a streamed chat answer. A stream ends with exactly one EventEnd or EventError.
type StreamEvent struct {
	Type         EventType  `json:"type"`
	Text         string     `json:"text,omitempty"`
	ToolCalls    []ToolCall `json:"tool_calls,omitempty"`
	Usage        *Usage     `json:"usage,omitempty"`
	FinishReason string     `json:"finish_reason,omitempty"`
	Error        string     `json:"error,omitempty"`
}

// Stream is a streamed chat answer. Read Events until it's closed; call Close to stop early (it cancels the call).
type Stream struct {
	Events <-chan StreamEvent

	closeOnce sync.Once
	cancel    func()
}

// NewStream wraps a channel of events; cancel (may be nil) stops the producer and is called once by Close.
func NewStream(events <-chan StreamEvent, cancel func()) *Stream {
	return &Stream{Events: events, cancel: cancel}
}

// Close stops the stream. Events already sent may still be read; the channel is closed by the producer.
func (s *Stream) Close() {
	s.closeOnce.Do(func() {
		if s.cancel != nil {
			s.cancel()
		}
	})
}

// StreamFromResponse turns a complete answer into a stream, for providers or tests without streaming.
func StreamFromResponse(resp *ChatResponse) *Stream {
	events := make(chan StreamEvent, 4)
	if resp.Message.Content != "" {
		events <- StreamEvent{Type: EventText, Text: resp.Message.Content}
	}
	if len(resp.Message.ToolCalls) > 0 {
		events <- StreamEvent{Type: EventToolCalls, ToolCalls: resp.Message.ToolCalls}
	}
	usage := resp.Usage
	events <- StreamEvent{Type: EventUsage, Usage: &usage}
	events <- StreamEvent{Type: EventEnd, FinishReason: resp.FinishReason}
	close(events)
	return NewStream(events, nil)
}

// ReadAll reads the stream to its end and gives the whole answer, or the stream's error.
func (s *Stream) ReadAll() (*ChatResponse, error) {
	defer s.Close()
	var text strings.Builder
	resp := &ChatResponse{Message: Message{Role: RoleAssistant}}
	for event := range s.Events {
		switch event.Type {
		case EventText:
			text.WriteString(event.Text)
		case EventToolCalls:
			resp.Message.ToolCalls = append(resp.Message.ToolCalls, event.ToolCalls...)
		case EventUsage:
			if event.Usage != nil {
				resp.Usage = *event.Usage
			}
		case EventError:
			if event.Error == "" {
				return nil, errors.New("the stream failed")
			}
			return nil, errors.New(event.Error)
		case EventEnd:
			resp.FinishReason = event.FinishReason
			resp.Message.Content = text.String()
			return resp, nil
		}
	}
	return nil, errors.New("the stream ended without an answer")
}

// MarshalToolArguments gives a tool call's arguments as a JSON object, "{}" when empty.
func MarshalToolArguments(args any) json.RawMessage {
	if args == nil {
		return json.RawMessage("{}")
	}
	data, err := json.Marshal(args)
	if err != nil || len(data) == 0 || string(data) == "null" {
		return json.RawMessage("{}")
	}
	return data
}
