// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

// Ported from the Agents plugin's llm package (Apache-2.0): estimate.go (EstimateTokens), json_sanitize.go
// (StripMarkdownCodeFencing), tools.go (SanitizeNonPrintableChars), provider_error.go (SanitizeProviderError),
// structured_output_fallback.go (JSONOutputInstruction) and completion_request.go (TruncateMessages), adapted to
// this package's types.

package ai

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// EstimateTokens is a fast, provider-agnostic approximation of a text's tokens, for providers that don't report
// usage and for budgets checked before a call.
func EstimateTokens(text string) int {
	charCount := float64(len(text)) / 4.0
	wordCount := float64(len(strings.Fields(text))) / 0.75
	return int((charCount + wordCount) / 2.0)
}

// EstimateMessagesTokens estimates the tokens of a conversation.
func EstimateMessagesTokens(messages []Message) int {
	total := 0
	for _, m := range messages {
		total += EstimateTokens(m.Content)
		for _, tc := range m.ToolCalls {
			total += EstimateTokens(tc.Name) + EstimateTokens(string(tc.Arguments))
		}
	}
	return total
}

// TruncateMessages keeps the latest messages that fit in maxTokens (by countTokens), cutting the oldest one that
// doesn't fit at its start. System messages are always kept: they carry the instructions. It tells whether
// anything was cut.
func TruncateMessages(messages []Message, maxTokens int, countTokens func(string) int) ([]Message, bool) {
	if countTokens == nil {
		countTokens = EstimateTokens
	}
	total := 0
	for _, m := range messages {
		if m.Role == RoleSystem {
			total += countTokens(m.Content)
		}
	}
	keep := make([]bool, len(messages))
	contents := make([]string, len(messages))
	truncated := false
	for i, m := range slices.Backward(messages) {
		contents[i] = m.Content
		if m.Role == RoleSystem {
			keep[i] = true
			continue
		}
		if truncated {
			continue
		}
		tokens := countTokens(m.Content)
		if total+tokens <= maxTokens {
			total += tokens
			keep[i] = true
			continue
		}
		truncated = true
		// Keep the end of the message that crosses the budget, roughly four characters a token.
		room := maxTokens - total
		if room > 0 && m.Role != RoleTool && len(m.ToolCalls) == 0 {
			cut := min(len(m.Content), max(0, len(m.Content)-room*4))
			for cut < len(m.Content) && !utf8.RuneStart(m.Content[cut]) {
				cut++
			}
			contents[i] = strings.TrimSpace(m.Content[cut:])
			keep[i] = contents[i] != ""
			total = maxTokens
		}
	}
	out := make([]Message, 0, len(messages))
	for i, m := range messages {
		if keep[i] {
			m.Content = contents[i]
			out = append(out, m)
		}
	}
	return out, truncated
}

// StripMarkdownCodeFencing removes the code fence (```json ... ```) models sometimes put around a JSON answer
// despite being told not to.
func StripMarkdownCodeFencing(s string) string {
	trimmed := strings.TrimSpace(s)
	if !strings.HasPrefix(trimmed, "```") {
		return s
	}
	content := strings.TrimPrefix(trimmed, "```")
	if firstNewline := strings.Index(content, "\n"); firstNewline != -1 {
		content = content[firstNewline+1:]
	} else {
		// A single-line fenced payload, e.g. ```json {"a":1}```
		content = strings.TrimSpace(content)
		if spaceIdx := strings.IndexAny(content, " \t"); spaceIdx != -1 {
			if isLanguageTag(content[:spaceIdx]) {
				content = strings.TrimSpace(content[spaceIdx:])
			}
		}
	}
	if idx := strings.LastIndex(content, "```"); idx != -1 {
		content = content[:idx]
	}
	return strings.TrimSpace(content)
}

func isLanguageTag(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		isLetter := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
		isDigit := r >= '0' && r <= '9'
		isSpecial := r == '-' || r == '_' || r == '+'
		if !isLetter && !isDigit && !isSpecial {
			return false
		}
	}
	return true
}

// JSONOutputInstruction asks a model, in its instructions, for raw JSON following schema: the structured output of
// providers and models that can't take a schema natively.
func JSONOutputInstruction(schema json.RawMessage) string {
	shape := "value"
	var probe struct {
		Type string `json:"type"`
	}
	if json.Unmarshal(schema, &probe) == nil && probe.Type == "object" {
		shape = "object"
	}
	base := fmt.Sprintf("Respond with a single valid JSON %s. Output raw JSON only: no markdown code fences, no explanation, no text before or after.", shape)
	if shape == "object" {
		base += " Your response must start with { and end with }."
	}
	if len(schema) == 0 || !json.Valid(schema) {
		return base
	}
	return fmt.Sprintf("%s The JSON must conform to the following JSON schema:\n%s", base, schema)
}

// WithJSONInstruction gives the messages with JSONOutputInstruction added to the first system message (or a new
// one first), leaving the caller's slice untouched.
func WithJSONInstruction(messages []Message, schema json.RawMessage) []Message {
	instruction := JSONOutputInstruction(schema)
	out := make([]Message, 0, len(messages)+1)
	added := false
	for _, m := range messages {
		if !added && m.Role == RoleSystem {
			m.Content += "\n\n" + instruction
			added = true
		}
		out = append(out, m)
	}
	if !added {
		out = append([]Message{{Role: RoleSystem, Content: instruction}}, out...)
	}
	return out
}

// SanitizeNonPrintableChars replaces non-printable Unicode characters with [U+XXXX], against spoofing such as
// bidirectional text that makes a link look like another. Newline, tab and carriage return stay.
func SanitizeNonPrintableChars(s string) string {
	needsEscape := false
	for _, r := range s {
		if !isSafeRune(r) {
			needsEscape = true
			break
		}
	}
	if !needsEscape {
		return s
	}
	var result strings.Builder
	result.Grow(len(s))
	for _, r := range s {
		if isSafeRune(r) {
			result.WriteRune(r)
		} else {
			fmt.Fprintf(&result, "[U+%04X]", r)
		}
	}
	return result.String()
}

func isSafeRune(r rune) bool {
	if r >= 0x20 && r <= 0x7E {
		return true
	}
	if r == '\n' || r == '\t' || r == '\r' {
		return true
	}
	if r > 0x7E {
		return unicode.IsPrint(r) &&
			!unicode.Is(unicode.Variation_Selector, r) &&
			!unicode.Is(unicode.Other_Default_Ignorable_Code_Point, r)
	}
	return false
}

const providerErrorRedacted = "[REDACTED]"

var (
	authHeaderPattern   = regexp.MustCompile(`(?i)(Authorization:\s*Bearer\s+)(\S+)`)
	apiKeyHeaderPattern = regexp.MustCompile(`(?i)((?:x-)?api-key:\s*)(\S+)`)
	jsonAPIKeyPattern   = regexp.MustCompile(`(?i)("api(?:_|-)?key"\s*:\s*")([^"]+)(")`)
	incorrectKeyPattern = regexp.MustCompile(`(?i)(Incorrect API key provided)(?::\s*[^"\r\n]+?)?(\.?\s+You can find your API key|["\r\n]|$)`)
	openAIKeyPattern    = regexp.MustCompile(`\bsk(?:-proj)?-[A-Za-z0-9_-]{10,}\b`)
	anthropicKeyPattern = regexp.MustCompile(`\bsk-ant-[A-Za-z0-9_-]{20,}\b`)
)

// SanitizeProviderErrorMessage removes keys, bearer tokens and the like from an endpoint's error before it's
// logged or returned: the known key formats, and each of secrets (the connection's key and header values) wherever
// it appears.
func SanitizeProviderErrorMessage(message string, secrets ...string) string {
	sanitized := authHeaderPattern.ReplaceAllString(message, `${1}`+providerErrorRedacted)
	sanitized = apiKeyHeaderPattern.ReplaceAllString(sanitized, `${1}`+providerErrorRedacted)
	sanitized = jsonAPIKeyPattern.ReplaceAllString(sanitized, `${1}`+providerErrorRedacted+`${3}`)
	sanitized = incorrectKeyPattern.ReplaceAllString(sanitized, `${1}`+`${2}`)
	sanitized = openAIKeyPattern.ReplaceAllString(sanitized, providerErrorRedacted)
	sanitized = anthropicKeyPattern.ReplaceAllString(sanitized, providerErrorRedacted)
	for _, secret := range secrets {
		// Short values (a header like "1") would redact half the message; keys are long.
		if secret = strings.TrimSpace(secret); len(secret) >= 6 {
			sanitized = strings.ReplaceAll(sanitized, secret, providerErrorRedacted)
		}
	}
	return SanitizeNonPrintableChars(sanitized)
}
