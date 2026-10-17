// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package aiservice

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/ai"
)

const testKey = "sk-test-0123456789abcdef"

// fakeEndpoint is an httptest server recording each request's path, headers and JSON body.
type fakeEndpoint struct {
	*httptest.Server
	t        *testing.T
	mu       sync.Mutex
	requests []recordedRequest
}

type recordedRequest struct {
	Path   string
	Header http.Header
	Body   map[string]any
	Raw    []byte
}

func newFakeEndpoint(t *testing.T, handler func(w http.ResponseWriter, r *http.Request, body map[string]any)) *fakeEndpoint {
	f := &fakeEndpoint{t: t}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		if strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			_ = json.Unmarshal(raw, &body)
		}
		f.mu.Lock()
		f.requests = append(f.requests, recordedRequest{Path: r.URL.Path, Header: r.Header.Clone(), Body: body, Raw: raw})
		f.mu.Unlock()
		handler(w, r, body)
	}))
	t.Cleanup(f.Close)
	return f
}

func (f *fakeEndpoint) last() recordedRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	require.NotEmpty(f.t, f.requests)
	return f.requests[len(f.requests)-1]
}

func newOpenAI(t *testing.T, url string, dialect ai.DecisionsDialect) ai.Provider {
	t.Helper()
	p, err := New(Config{Provider: ProviderOpenAICompatible, BaseURL: url + "/v1", APIKey: testKey, Headers: map[string]string{"X-Team": "lumen"}, DecisionsDialect: dialect})
	require.NoError(t, err)
	return p
}

func writeSSE(w http.ResponseWriter, events ...string) {
	w.Header().Set("Content-Type", "text/event-stream")
	flusher, _ := w.(http.Flusher)
	for _, e := range events {
		fmt.Fprintf(w, "data: %s\n\n", e)
		if flusher != nil {
			flusher.Flush()
		}
	}
}

func TestOpenAIChat(t *testing.T) {
	f := newFakeEndpoint(t, func(w http.ResponseWriter, r *http.Request, body map[string]any) {
		fmt.Fprint(w, `{"model":"gpt-test","choices":[{"message":{"role":"assistant","content":"<think>hmm</think>Hello!"},"finish_reason":"stop"}],"usage":{"prompt_tokens":12,"completion_tokens":3}}`)
	})
	p := newOpenAI(t, f.URL, "")
	temp := 0.2
	resp, err := p.Chat(context.Background(), ai.ChatRequest{
		Model:       "gpt-test",
		Messages:    []ai.Message{{Role: ai.RoleSystem, Content: "Be brief."}, {Role: ai.RoleUser, Content: "Hi", Images: []ai.Image{{MimeType: "image/png", Data: []byte{1, 2}}}}},
		MaxTokens:   50,
		Temperature: &temp,
	})
	require.NoError(t, err)
	assert.Equal(t, "Hello!", resp.Message.Content, "the reasoning block is left out")
	assert.Equal(t, ai.FinishReasonStop, resp.FinishReason)
	assert.Equal(t, ai.Usage{InputTokens: 12, OutputTokens: 3}, resp.Usage)

	req := f.last()
	assert.Equal(t, "/v1/chat/completions", req.Path)
	assert.Equal(t, "Bearer "+testKey, req.Header.Get("Authorization"))
	assert.Equal(t, "lumen", req.Header.Get("X-Team"), "the connection's extra headers")
	assert.Equal(t, float64(50), req.Body["max_tokens"], "compatible servers take max_tokens")
	messages := req.Body["messages"].([]any)
	parts := messages[1].(map[string]any)["content"].([]any)
	assert.Equal(t, "image_url", parts[1].(map[string]any)["type"])
	assert.True(t, strings.HasPrefix(parts[1].(map[string]any)["image_url"].(map[string]any)["url"].(string), "data:image/png;base64,"))
}

func TestOpenAIChatTools(t *testing.T) {
	f := newFakeEndpoint(t, func(w http.ResponseWriter, r *http.Request, body map[string]any) {
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":null,"tool_calls":[{"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{\"q\":\"gdp\"}"}}]},"finish_reason":"tool_calls"}]}`)
	})
	p := newOpenAI(t, f.URL, "")
	resp, err := p.Chat(context.Background(), ai.ChatRequest{
		Model: "m",
		Messages: []ai.Message{
			{Role: ai.RoleUser, Content: "Europe's GDP?"},
			{Role: ai.RoleAssistant, ToolCalls: []ai.ToolCall{{ID: "call_0", Name: "lookup", Arguments: json.RawMessage(`{"q":"x"}`)}}},
			{Role: ai.RoleTool, ToolCallID: "call_0", Content: "nothing"},
		},
		Tools:      []ai.Tool{{Name: "lookup", Description: "Look it up", Parameters: json.RawMessage(`{"type":"object","properties":{"q":{"type":"string"}}}`)}},
		ToolChoice: "lookup",
	})
	require.NoError(t, err)
	require.Len(t, resp.Message.ToolCalls, 1)
	assert.Equal(t, "lookup", resp.Message.ToolCalls[0].Name)
	assert.JSONEq(t, `{"q":"gdp"}`, string(resp.Message.ToolCalls[0].Arguments))
	assert.Equal(t, ai.FinishReasonToolCalls, resp.FinishReason)
	assert.Positive(t, resp.Usage.InputTokens, "estimated when the endpoint doesn't say")

	body := f.last().Body
	tools := body["tools"].([]any)
	assert.Equal(t, "function", tools[0].(map[string]any)["type"])
	assert.Equal(t, "lookup", body["tool_choice"].(map[string]any)["function"].(map[string]any)["name"])
	messages := body["messages"].([]any)
	assert.Equal(t, "call_0", messages[2].(map[string]any)["tool_call_id"])
	assert.Equal(t, `{"q":"x"}`, messages[1].(map[string]any)["tool_calls"].([]any)[0].(map[string]any)["function"].(map[string]any)["arguments"])
}

func TestOpenAIStructuredOutput(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"rewritten_text":{"type":"string"}},"required":["rewritten_text"],"additionalProperties":false}`)

	t.Run("native json_schema", func(t *testing.T) {
		f := newFakeEndpoint(t, func(w http.ResponseWriter, r *http.Request, body map[string]any) {
			fmt.Fprint(w, `{"choices":[{"message":{"content":"{\"rewritten_text\":\"Hi\"}"},"finish_reason":"stop"}]}`)
		})
		resp, err := newOpenAI(t, f.URL, "").Chat(context.Background(), ai.ChatRequest{Model: "m", Messages: []ai.Message{{Role: ai.RoleUser, Content: "hi"}}, JSONSchema: schema, JSONSchemaName: "rewrite"})
		require.NoError(t, err)
		assert.JSONEq(t, `{"rewritten_text":"Hi"}`, resp.Message.Content)
		format := f.last().Body["response_format"].(map[string]any)
		assert.Equal(t, "json_schema", format["type"])
		assert.Equal(t, "rewrite", format["json_schema"].(map[string]any)["name"])
		assert.Equal(t, true, format["json_schema"].(map[string]any)["strict"])
	})

	t.Run("falls back to the instructions when the server refuses the schema", func(t *testing.T) {
		f := newFakeEndpoint(t, func(w http.ResponseWriter, r *http.Request, body map[string]any) {
			if body["response_format"] != nil {
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprint(w, `{"error":{"message":"response_format json_schema is not supported"}}`)
				return
			}
			fmt.Fprint(w, "{\"choices\":[{\"message\":{\"content\":\"```json\\n{\\\"rewritten_text\\\":\\\"Hi\\\"}\\n```\"},\"finish_reason\":\"stop\"}]}")
		})
		resp, err := newOpenAI(t, f.URL, "").Chat(context.Background(), ai.ChatRequest{Model: "m", Messages: []ai.Message{{Role: ai.RoleSystem, Content: "Rewrite."}, {Role: ai.RoleUser, Content: "hi"}}, JSONSchema: schema})
		require.NoError(t, err)
		assert.JSONEq(t, `{"rewritten_text":"Hi"}`, resp.Message.Content, "code fences removed")
		require.Len(t, f.requests, 2)
		system := f.last().Body["messages"].([]any)[0].(map[string]any)["content"].(string)
		assert.Contains(t, system, "JSON schema")
	})
}

func TestOpenAIChatStream(t *testing.T) {
	f := newFakeEndpoint(t, func(w http.ResponseWriter, r *http.Request, body map[string]any) {
		writeSSE(w,
			`{"choices":[{"delta":{"role":"assistant","content":"Hel"}}]}`,
			`{"choices":[{"delta":{"content":"lo"}}]}`,
			`{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{\"q\":"}}]}}]}`,
			`{"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"gdp\"}"}}]}}]}`,
			`{"choices":[{"delta":{},"finish_reason":"tool_calls"}]}`,
			`{"choices":[],"usage":{"prompt_tokens":7,"completion_tokens":5}}`,
			`[DONE]`,
		)
	})
	stream, err := newOpenAI(t, f.URL, "").ChatStream(context.Background(), ai.ChatRequest{Model: "m", Messages: []ai.Message{{Role: ai.RoleUser, Content: "hi"}}})
	require.NoError(t, err)

	var texts []string
	var final ai.StreamEvent
	var calls []ai.ToolCall
	var usage *ai.Usage
	for ev := range stream.Events {
		switch ev.Type {
		case ai.EventText:
			texts = append(texts, ev.Text)
		case ai.EventToolCalls:
			calls = ev.ToolCalls
		case ai.EventUsage:
			usage = ev.Usage
		case ai.EventEnd, ai.EventError:
			final = ev
		}
	}
	assert.Equal(t, []string{"Hel", "lo"}, texts, "text arrives as it's written")
	require.Len(t, calls, 1)
	assert.JSONEq(t, `{"q":"gdp"}`, string(calls[0].Arguments), "tool call deltas joined")
	assert.Equal(t, &ai.Usage{InputTokens: 7, OutputTokens: 5}, usage)
	assert.Equal(t, ai.EventEnd, final.Type)
	assert.Equal(t, ai.FinishReasonToolCalls, final.FinishReason)

	body := f.last().Body
	assert.Equal(t, true, body["stream"])
	assert.Equal(t, true, body["stream_options"].(map[string]any)["include_usage"])
}

func TestOpenAIChatStreamThinkingAndErrors(t *testing.T) {
	t.Run("a leading think block is reasoning", func(t *testing.T) {
		f := newFakeEndpoint(t, func(w http.ResponseWriter, r *http.Request, body map[string]any) {
			writeSSE(w, `{"choices":[{"delta":{"content":"<thi"}}]}`, `{"choices":[{"delta":{"content":"nk>pondering</think>Answer"}}]}`, `[DONE]`)
		})
		stream, err := newOpenAI(t, f.URL, "").ChatStream(context.Background(), ai.ChatRequest{Model: "m"})
		require.NoError(t, err)
		var text, reasoning strings.Builder
		for ev := range stream.Events {
			switch ev.Type {
			case ai.EventText:
				text.WriteString(ev.Text)
			case ai.EventReasoning:
				reasoning.WriteString(ev.Text)
			}
		}
		assert.Equal(t, "Answer", text.String())
		assert.Equal(t, "pondering", reasoning.String())
	})

	t.Run("an error event ends the stream, without the key", func(t *testing.T) {
		f := newFakeEndpoint(t, func(w http.ResponseWriter, r *http.Request, body map[string]any) {
			writeSSE(w, `{"error":{"message":"invalid key `+testKey+`"}}`)
		})
		stream, err := newOpenAI(t, f.URL, "").ChatStream(context.Background(), ai.ChatRequest{Model: "m"})
		require.NoError(t, err)
		_, err = stream.ReadAll()
		require.Error(t, err)
		assert.NotContains(t, err.Error(), testKey)
	})

	t.Run("a refused call is a provider error without the key", func(t *testing.T) {
		f := newFakeEndpoint(t, func(w http.ResponseWriter, r *http.Request, body map[string]any) {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, `{"error":{"message":"Incorrect API key provided: `+testKey+`."}}`)
		})
		_, err := newOpenAI(t, f.URL, "").ChatStream(context.Background(), ai.ChatRequest{Model: "m"})
		var pe *ai.ProviderError
		require.ErrorAs(t, err, &pe)
		assert.Equal(t, http.StatusUnauthorized, pe.StatusCode)
		assert.NotContains(t, pe.Error(), testKey)
	})

	t.Run("closing the stream stops the call", func(t *testing.T) {
		release := make(chan struct{})
		f := newFakeEndpoint(t, func(w http.ResponseWriter, r *http.Request, body map[string]any) {
			writeSSE(w, `{"choices":[{"delta":{"content":"first"}}]}`)
			select {
			case <-r.Context().Done():
			case <-release:
			}
		})
		defer close(release)
		stream, err := newOpenAI(t, f.URL, "").ChatStream(context.Background(), ai.ChatRequest{Model: "m"})
		require.NoError(t, err)
		first := <-stream.Events
		assert.Equal(t, "first", first.Text)
		stream.Close()
		var final ai.StreamEvent
		for ev := range stream.Events {
			final = ev
		}
		assert.Equal(t, ai.EventError, final.Type)
	})

	t.Run("an endpoint that stops sending times out", func(t *testing.T) {
		release := make(chan struct{})
		f := newFakeEndpoint(t, func(w http.ResponseWriter, r *http.Request, body map[string]any) {
			writeSSE(w, `{"choices":[{"delta":{"content":"first"}}]}`)
			select {
			case <-r.Context().Done():
			case <-release:
			}
		})
		defer close(release)
		p, err := New(Config{Provider: ProviderOpenAICompatible, BaseURL: f.URL + "/v1", StreamIdleTimeout: 100 * time.Millisecond})
		require.NoError(t, err)
		stream, err := p.ChatStream(context.Background(), ai.ChatRequest{Model: "m"})
		require.NoError(t, err)
		_, err = stream.ReadAll()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "stopped sending")
	})
}

func TestOpenAIMaxCompletionTokens(t *testing.T) {
	p := &openAIProvider{base: newBase(Config{BaseURL: "https://api.openai.com/v1"})}
	body := p.buildChat(ai.ChatRequest{Model: "m", MaxTokens: 10}, false, false)
	assert.Equal(t, 10, body.MaxCompletionTokens, "OpenAI's own API takes max_completion_tokens")
	assert.Zero(t, body.MaxTokens)
}

func TestOpenAIEmbeddings(t *testing.T) {
	f := newFakeEndpoint(t, func(w http.ResponseWriter, r *http.Request, body map[string]any) {
		fmt.Fprint(w, `{"model":"embed-test","data":[{"index":1,"embedding":[0.3,0.4]},{"index":0,"embedding":[0.1,0.2]}],"usage":{"prompt_tokens":4,"total_tokens":4}}`)
	})
	resp, err := newOpenAI(t, f.URL, "").Embed(context.Background(), ai.EmbedRequest{Model: "embed-test", Input: []string{"a", "b"}, Dimensions: 2})
	require.NoError(t, err)
	assert.Equal(t, [][]float32{{0.1, 0.2}, {0.3, 0.4}}, resp.Embeddings, "back in the inputs' order")
	assert.Equal(t, int64(4), resp.Usage.InputTokens)
	req := f.last()
	assert.Equal(t, "/v1/embeddings", req.Path)
	assert.Equal(t, float64(2), req.Body["dimensions"])
	assert.Equal(t, []any{"a", "b"}, req.Body["input"])
}

func TestOpenAITranscription(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/audio/transcriptions", r.URL.Path)
		assert.Equal(t, "Bearer "+testKey, r.Header.Get("Authorization"))
		if !assert.NoError(t, r.ParseMultipartForm(1<<20)) {
			return
		}
		assert.Equal(t, "whisper-1", r.FormValue("model"))
		assert.Equal(t, "fr", r.FormValue("language"))
		assert.Equal(t, "verbose_json", r.FormValue("response_format"))
		file, header, err := r.FormFile("file")
		if assert.NoError(t, err) {
			data, _ := io.ReadAll(file)
			assert.Equal(t, []byte("audio"), data)
			assert.Equal(t, "call.webm", header.Filename)
		}
		fmt.Fprint(w, `{"text":" Bonjour ","language":"french","duration":1.5,"segments":[{"start":0,"end":1.5,"text":"Bonjour"}]}`)
	}))
	defer server.Close()
	resp, err := newOpenAI(t, server.URL, "").Transcribe(context.Background(), ai.TranscribeRequest{Model: "whisper-1", Audio: []byte("audio"), FileName: "call.webm", Language: "fr", Segments: true})
	require.NoError(t, err)
	assert.Equal(t, "Bonjour", resp.Text)
	assert.Equal(t, 1.5, resp.Duration)
	require.Len(t, resp.Segments, 1)
}

func TestOpenAISpeechAndModels(t *testing.T) {
	f := newFakeEndpoint(t, func(w http.ResponseWriter, r *http.Request, body map[string]any) {
		switch r.URL.Path {
		case "/v1/audio/speech":
			w.Header().Set("Content-Type", "audio/mpeg")
			fmt.Fprint(w, "mp3data")
		case "/v1/models":
			fmt.Fprint(w, `{"data":[{"id":"zeta","owned_by":"me"},{"id":"alpha"}]}`)
		}
	})
	p := newOpenAI(t, f.URL, "")
	speech, err := p.(ai.SpeechProvider).Speak(context.Background(), ai.SpeechRequest{Model: "tts", Input: "Hello", Voice: "alloy"})
	require.NoError(t, err)
	assert.Equal(t, []byte("mp3data"), speech.Audio)
	assert.Equal(t, "audio/mpeg", speech.ContentType)
	assert.Equal(t, "alloy", f.last().Body["voice"])

	models, err := p.ListModels(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []ai.ModelInfo{{ID: "alpha"}, {ID: "zeta", OwnedBy: "me"}}, models)
}

func decisionQuestions() []ai.DecisionQuestion {
	return []ai.DecisionQuestion{
		{Type: ai.DecisionPredicate, Name: "is_refund_request", Instructions: "Is the customer asking for a refund?"},
		{Type: ai.DecisionChoice, Name: "urgency", Instructions: "How urgent is this?", Choices: []ai.DecisionOption{{Value: "low", Description: "can wait a week"}, {Value: "high", Description: "needs action today"}}},
		{Type: ai.DecisionScore, Name: "frustration", Instructions: "How frustrated is the customer?", Levels: []ai.DecisionLevel{{Label: "calm"}, {Label: "mildly annoyed"}, {Label: "angry"}}},
	}
}

func TestOpenAIDecisions(t *testing.T) {
	f := newFakeEndpoint(t, func(w http.ResponseWriter, r *http.Request, body map[string]any) {
		fmt.Fprint(w, `{"model":"gpt-6-luna","answers":[
			{"type":"predicate","name":"is_refund_request","probability":0.99},
			{"type":"choice","name":"urgency","choice":"high","confidence":0.9,"probabilities":[{"value":"low","probability":0.1},{"value":"high","probability":0.9}]},
			{"type":"score","name":"frustration","score":0.95,"confidence":0.86,"probabilities":[{"label":"calm","probability":0.07},{"label":"mildly annoyed","probability":0.91},{"label":"angry","probability":0.02}]}
		],"usage":{"input_tokens":391,"output_tokens":0}}`)
	})
	resp, err := newOpenAI(t, f.URL, ai.DecisionsDialectOpenAI).Decide(context.Background(), ai.DecideRequest{Model: "gpt-6-luna", Input: "I was charged twice", Questions: decisionQuestions()})
	require.NoError(t, err)

	req := f.last()
	assert.Equal(t, "/v1/decisions", req.Path)
	assert.Equal(t, "I was charged twice", req.Body["input"])
	questions := req.Body["questions"].([]any)
	require.Len(t, questions, 3)
	assert.Equal(t, "predicate", questions[0].(map[string]any)["type"])
	assert.Equal(t, "high", questions[1].(map[string]any)["choices"].([]any)[1].(map[string]any)["value"])
	assert.Equal(t, "angry", questions[2].(map[string]any)["levels"].([]any)[2].(map[string]any)["label"])

	refund, _ := resp.Answer("is_refund_request")
	require.NotNil(t, refund.Probability)
	assert.InDelta(t, 0.99, *refund.Probability, 1e-9)
	urgency, _ := resp.Answer("urgency")
	assert.Equal(t, "high", urgency.Choice)
	assert.Len(t, urgency.Probabilities, 2)
	frustration, _ := resp.Answer("frustration")
	assert.InDelta(t, 0.95, *frustration.Score, 1e-9)
	assert.Equal(t, "1", frustration.Probabilities[1].Value, "levels get their index")
	assert.Equal(t, int64(391), resp.Usage.InputTokens)
}

func TestSystemOneDecisions(t *testing.T) {
	f := newFakeEndpoint(t, func(w http.ResponseWriter, r *http.Request, body map[string]any) {
		fmt.Fprint(w, `{"model":"jev-1.13.0","answers":{
			"is_refund_request":{"type":"noul","noul":0.99},
			"urgency":{"type":"choice","choice":"high","confidence":1.0,"probabilities":{"low":0.0,"high":1.0}},
			"frustration":{"type":"score","score":0.95,"confidence":0.86,"legend":{"0":"calm","1":"mildly annoyed","2":"angry"},"probabilities":{"0":0.07,"1":0.91,"2":0.02}}
		},"usage":{"input_tokens":391,"output_tokens":67}}`)
	})
	resp, err := newOpenAI(t, f.URL, ai.DecisionsDialectSystemOne).Decide(context.Background(), ai.DecideRequest{Model: "jev", Input: "I was charged twice", Questions: decisionQuestions()})
	require.NoError(t, err)

	req := f.last()
	assert.Equal(t, "/v1/systemone", req.Path)
	assert.Equal(t, "I was charged twice", req.Body["state"])
	questions := req.Body["questions"].(map[string]any)
	assert.Equal(t, "noul", questions["is_refund_request"].(map[string]any)["type"])
	assert.Equal(t, map[string]any{"low": "can wait a week", "high": "needs action today"}, questions["urgency"].(map[string]any)["criteria"])
	assert.Equal(t, []any{"calm", "mildly annoyed", "angry"}, questions["frustration"].(map[string]any)["criteria"])

	require.Len(t, resp.Answers, 3)
	assert.Equal(t, "is_refund_request", resp.Answers[0].Name, "answers in the questions' order")
	assert.Equal(t, ai.DecisionPredicate, resp.Answers[0].Type)
	assert.InDelta(t, 0.99, *resp.Answers[0].Probability, 1e-9)
	assert.Equal(t, "high", resp.Answers[1].Choice)
	assert.Equal(t, []ai.DecisionProbability{{Value: "low", Probability: 0}, {Value: "high", Probability: 1}}, resp.Answers[1].Probabilities)
	assert.Equal(t, ai.DecisionProbability{Value: "1", Label: "mildly annoyed", Probability: 0.91}, resp.Answers[2].Probabilities[1])
	assert.Equal(t, ai.Usage{InputTokens: 391, OutputTokens: 67}, resp.Usage)

	_, err = newOpenAI(t, f.URL, ai.DecisionsDialectSystemOne).Decide(context.Background(), ai.DecideRequest{Model: "jev", Input: "x"})
	assert.Error(t, err, "a request without questions is refused before any call")
}

// A server that ignores "stream" and answers with JSON still gives a stream.
func TestOpenAIChatStreamFromAWholeAnswer(t *testing.T) {
	f := newFakeEndpoint(t, func(w http.ResponseWriter, r *http.Request, body map[string]any) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"content":"All at once"},"finish_reason":"stop"}],"usage":{"prompt_tokens":4,"completion_tokens":3}}`)
	})
	stream, err := newOpenAI(t, f.URL, "").ChatStream(context.Background(), ai.ChatRequest{Model: "m"})
	require.NoError(t, err)
	resp, err := stream.ReadAll()
	require.NoError(t, err)
	assert.Equal(t, "All at once", resp.Message.Content)
	assert.Equal(t, ai.Usage{InputTokens: 4, OutputTokens: 3}, resp.Usage)
	assert.Equal(t, ai.FinishReasonStop, resp.FinishReason)
}
