// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package autotranslation

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
)

const (
	libreTranslateLanguagesTTL = time.Hour
	maxProviderResponseBytes   = 8 << 20
)

// libreTranslateProvider talks to a LibreTranslate server
// (https://github.com/LibreTranslate/LibreTranslate) over its HTTP API.
type libreTranslateProvider struct {
	client  *http.Client
	baseURL string
	apiKey  string
	timeout time.Duration

	mu          sync.Mutex
	languages   map[string]string // base code -> provider code
	languagesAt time.Time
}

func newLibreTranslateProvider(client *http.Client, baseURL, apiKey string, timeout time.Duration) *libreTranslateProvider {
	if client == nil {
		client = &http.Client{}
	}
	return &libreTranslateProvider{
		client:  client,
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		timeout: timeout,
	}
}

func (p *libreTranslateProvider) Name() string { return providerLibreTranslate }

type ltDetection struct {
	Language   string  `json:"language"`
	Confidence float64 `json:"confidence"`
}

type ltTranslateResponse struct {
	TranslatedText   json.RawMessage `json:"translatedText"`
	DetectedLanguage json.RawMessage `json:"detectedLanguage"`
	Error            string          `json:"error"`
}

type ltError struct {
	Error string `json:"error"`
}

// post sends a JSON request and decodes the JSON response into out.
func (p *libreTranslateProvider) do(ctx context.Context, method, path string, body any, out any) error {
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, p.baseURL+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxProviderResponseBytes))
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		var e ltError
		if json.Unmarshal(data, &e) == nil && e.Error != "" {
			return fmt.Errorf("libretranslate returned status %d: %s", resp.StatusCode, e.Error)
		}
		return fmt.Errorf("libretranslate returned status %d", resp.StatusCode)
	}

	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("invalid libretranslate response: %w", err)
	}
	return nil
}

func (p *libreTranslateProvider) withKey(body map[string]any) map[string]any {
	if p.apiKey != "" {
		body["api_key"] = p.apiKey
	}
	return body
}

// languageCodes returns the provider's supported languages keyed by base
// code. A nil map means the list is unknown.
func (p *libreTranslateProvider) languageCodes(ctx context.Context) map[string]string {
	p.mu.Lock()
	if p.languages != nil && time.Since(p.languagesAt) < libreTranslateLanguagesTTL {
		langs := p.languages
		p.mu.Unlock()
		return langs
	}
	p.mu.Unlock()

	var list []struct {
		Code string `json:"code"`
	}
	if err := p.do(ctx, http.MethodGet, "/languages", nil, &list); err != nil || len(list) == 0 {
		return nil
	}

	langs := make(map[string]string, len(list))
	for _, l := range list {
		base := normalizeLang(l.Code)
		if base == "" {
			continue
		}
		// Prefer the exact base code ("zh") over a regional or script
		// variant ("zh-Hant") when the server offers both.
		if existing, ok := langs[base]; !ok || (existing != base && (l.Code == base || l.Code == "zh-Hans")) {
			langs[base] = l.Code
		}
	}

	p.mu.Lock()
	p.languages = langs
	p.languagesAt = time.Now()
	p.mu.Unlock()
	return langs
}

// providerCode maps a base language code to the code the server expects.
func (p *libreTranslateProvider) providerCode(ctx context.Context, lang string) (string, error) {
	langs := p.languageCodes(ctx)
	if langs == nil {
		return lang, nil
	}
	code, ok := langs[lang]
	if !ok {
		return "", fmt.Errorf("language %q is not supported by the libretranslate server", lang)
	}
	return code, nil
}

func (p *libreTranslateProvider) Translate(ctx context.Context, texts []string, src, dst string) (*translateResult, error) {
	if len(texts) == 0 {
		return &translateResult{}, nil
	}

	target, err := p.providerCode(ctx, dst)
	if err != nil {
		return nil, err
	}
	source := "auto"
	if src != "" {
		if source, err = p.providerCode(ctx, src); err != nil {
			return nil, err
		}
	}

	// Older servers only accept a single string, so arrays are sent only
	// when there is more than one segment.
	var q any = texts[0]
	if len(texts) > 1 {
		q = texts
	}
	body := p.withKey(map[string]any{
		"q":      q,
		"source": source,
		"target": target,
		"format": "text",
	})

	var resp ltTranslateResponse
	if err := p.do(ctx, http.MethodPost, "/translate", body, &resp); err != nil {
		return nil, err
	}
	if resp.Error != "" {
		return nil, fmt.Errorf("libretranslate error: %s", resp.Error)
	}

	result := &translateResult{}
	if len(texts) == 1 {
		var s string
		if err := json.Unmarshal(resp.TranslatedText, &s); err != nil {
			var arr []string
			if err2 := json.Unmarshal(resp.TranslatedText, &arr); err2 != nil || len(arr) != 1 {
				return nil, fmt.Errorf("invalid translatedText in libretranslate response: %w", err)
			}
			s = arr[0]
		}
		result.Texts = []string{s}
	} else {
		if err := json.Unmarshal(resp.TranslatedText, &result.Texts); err != nil {
			return nil, fmt.Errorf("invalid translatedText in libretranslate response: %w", err)
		}
		if len(result.Texts) != len(texts) {
			return nil, fmt.Errorf("libretranslate returned %d translations for %d texts", len(result.Texts), len(texts))
		}
	}

	if det := parseLTDetection(resp.DetectedLanguage); det != nil && det.Language != "" {
		result.SourceLang = normalizeLang(det.Language)
		conf := normalizeConfidence(det.Confidence)
		result.Confidence = &conf
	}

	return result, nil
}

// parseLTDetection reads detectedLanguage, which is an object for a single
// input and an array for several; the most confident entry wins.
func parseLTDetection(raw json.RawMessage) *ltDetection {
	if len(raw) == 0 {
		return nil
	}
	var single ltDetection
	if err := json.Unmarshal(raw, &single); err == nil {
		return &single
	}
	var list []ltDetection
	if err := json.Unmarshal(raw, &list); err != nil || len(list) == 0 {
		return nil
	}
	best := list[0]
	for _, d := range list[1:] {
		if d.Confidence > best.Confidence {
			best = d
		}
	}
	return &best
}

func (p *libreTranslateProvider) Detect(ctx context.Context, text string) (string, *float64, error) {
	var list []ltDetection
	if err := p.do(ctx, http.MethodPost, "/detect", p.withKey(map[string]any{"q": text}), &list); err != nil {
		return "", nil, err
	}
	if len(list) == 0 || list[0].Language == "" {
		return "", nil, fmt.Errorf("libretranslate could not detect the language")
	}
	best := list[0]
	for _, d := range list[1:] {
		if d.Confidence > best.Confidence {
			best = d
		}
	}
	conf := normalizeConfidence(best.Confidence)
	return normalizeLang(best.Language), &conf, nil
}

// normalizeConfidence converts LibreTranslate's 0-100 scale to [0, 1].
func normalizeConfidence(c float64) float64 {
	if c > 1 {
		c /= 100
	}
	if c < 0 {
		return 0
	}
	if c > 1 {
		return 1
	}
	return c
}
