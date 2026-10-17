// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package autotranslation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/abadojack/whatlanggo"
	"golang.org/x/text/language"
	"golang.org/x/text/language/display"
)

const providerOpenAI = "openai"

// openAIProvider translates through an OpenAI-compatible chat completions API, reached directly: a dedicated
// translation model (such as Tencent's HY-MT) or a general LLM. Each segment is sent with the prompt translation
// models are trained on, which general models follow as well; languages are detected locally.
type openAIProvider struct {
	client  *http.Client
	baseURL string
	apiKey  string
	model   string
	timeout time.Duration
}

func newOpenAIProvider(client *http.Client, baseURL, apiKey, model string, timeout time.Duration) *openAIProvider {
	if client == nil {
		client = &http.Client{}
	}
	return &openAIProvider{
		client:  client,
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		model:   model,
		timeout: timeout,
	}
}

func (p *openAIProvider) Name() string { return providerOpenAI }

// languageName gives the English name of a language code, which models follow better than a code.
func languageName(code string) string {
	if tag, err := language.Parse(code); err == nil {
		if name := display.English.Languages().Name(tag); name != "" {
			return name
		}
	}
	return code
}

// translatePrompt is the prompt of Tencent's HY-MT translation models, in Chinese for a translation into Chinese.
func translatePrompt(text, dst string) string {
	if dst == "zh" {
		return "将以下文本翻译为中文，注意只需要输出翻译后的结果，不要额外解释：\n\n" + text
	}
	return "Translate the following segment into " + languageName(dst) + ", without additional explanation.\n\n" + text
}

// thinking matches the reasoning some models write before their answer.
var thinking = regexp.MustCompile(`(?s)^\s*<think>.*?</think>\s*`)

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// complete asks the model for a completion of one prompt.
func (p *openAIProvider) complete(ctx context.Context, prompt string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	body, err := json.Marshal(map[string]any{
		"model":       p.model,
		"messages":    []map[string]string{{"role": "user", "content": prompt}},
		"temperature": 0.2,
		"stream":      false,
		// Reasoning models answer without thinking first: a translation needs none.
		"chat_template_kwargs": map[string]any{"enable_thinking": false},
	})
	if err != nil {
		return "", err
	}
	send := func() (*http.Response, error) {
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(body))
		if reqErr != nil {
			return nil, reqErr
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		if p.apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+p.apiKey)
		}
		return p.client.Do(req)
	}

	// Servers such as llama.cpp close idle connections after a few seconds, and a POST on a connection closing
	// as it's reused fails without having reached the model: it's sent once more.
	resp, err := send()
	if err != nil && ctx.Err() == nil {
		resp, err = send()
	}
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxProviderResponseBytes))
	if err != nil {
		return "", err
	}

	var out chatResponse
	decodeErr := json.Unmarshal(data, &out)
	if resp.StatusCode != http.StatusOK {
		if decodeErr == nil && out.Error != nil && out.Error.Message != "" {
			return "", fmt.Errorf("the translation API returned status %d: %s", resp.StatusCode, out.Error.Message)
		}
		return "", fmt.Errorf("the translation API returned status %d", resp.StatusCode)
	}
	if decodeErr != nil || len(out.Choices) == 0 {
		return "", errors.New("invalid response from the translation API")
	}
	return strings.TrimSpace(thinking.ReplaceAllString(out.Choices[0].Message.Content, "")), nil
}

func (p *openAIProvider) Translate(ctx context.Context, texts []string, src, dst string) (*translateResult, error) {
	result := &translateResult{Texts: make([]string, len(texts))}
	for i, text := range texts {
		// Segments without words (spaces, punctuation, placeholders) stay as they are.
		if !hasLetters(text) {
			result.Texts[i] = text
			continue
		}
		translated, err := p.complete(ctx, translatePrompt(text, dst))
		if err != nil {
			return nil, err
		}
		result.Texts[i] = translated
	}

	if src == "" {
		if lang, confidence, err := p.Detect(ctx, strings.Join(texts, "\n")); err == nil {
			result.SourceLang = lang
			result.Confidence = confidence
		}
	}
	return result, nil
}

// Detect detects the language locally: translation models don't tell it, and the local detector's best guess beats
// asking a general model for every short message.
func (p *openAIProvider) Detect(_ context.Context, text string) (string, *float64, error) {
	info := whatlanggo.Detect(truncateBytes(text, maxDetectionBytes))
	if info.Lang < 0 || info.Lang.Iso6391() == "" {
		return "", nil, errors.New("the language of the text could not be detected")
	}
	confidence := info.Confidence
	return info.Lang.Iso6391(), &confidence, nil
}
