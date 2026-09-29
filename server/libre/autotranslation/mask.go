// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package autotranslation

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Parts of a message that must reach the reader unchanged (code, links,
// mentions, emoji, ...) are replaced with numbered placeholders before the
// text is sent to the provider, and restored afterwards.

// maskTokenFormat is the placeholder written in place of a protected span.
// Braces and digits survive both neural machine translation and LLMs well.
const maskTokenFormat = "{{%d}}"

// maskTokenRe matches a placeholder in provider output, tolerating the
// whitespace some engines insert around it.
var maskTokenRe = regexp.MustCompile(`\{\{\s*(\d+)\s*\}\}`)

// protectedPatterns lists the spans that are never translated, in priority
// order: when two matches overlap, the one found by the earlier pattern wins.
var protectedPatterns = []*regexp.Regexp{
	// Fenced code blocks.
	regexp.MustCompile("(?s)```.*?(```|$)"),
	regexp.MustCompile("(?s)~~~.*?(~~~|$)"),
	// Inline code.
	regexp.MustCompile("`[^`\n]+`"),
	// Anything already shaped like a placeholder, so a literal "{{1}}" typed
	// by a user round-trips untouched.
	maskTokenRe,
	// Markdown link and image targets: keep "(url "title")" as is.
	regexp.MustCompile(`\]\([^)\s]+(\s+"[^"]*")?\)`),
	// Autolinks and bare URLs.
	regexp.MustCompile(`<[a-zA-Z][a-zA-Z0-9+.\-]*:[^>\s]+>`),
	regexp.MustCompile(`(?i)\b(?:[a-z][a-z0-9+.\-]*://|www\.|mailto:)[^\s<>()\[\]]*[^\s<>()\[\].,;:!?'"]`),
	// E-mail addresses.
	regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`),
	// @mentions, ~channel links and #hashtags.
	regexp.MustCompile(`(^|[^\w@])(@[a-zA-Z0-9][a-zA-Z0-9._\-]*[a-zA-Z0-9_]|@[a-zA-Z0-9])`),
	regexp.MustCompile(`(^|[^\w~])(~[a-z0-9][a-z0-9_\-]*)`),
	regexp.MustCompile(`(^|[^\w#&])(#[\p{L}\p{N}_][\p{L}\p{N}_.\-]*[\p{L}\p{N}_])`),
	// :emoji: shortcodes.
	regexp.MustCompile(`:[a-z0-9_+\-]+:`),
	// HTML entities.
	regexp.MustCompile(`&[a-zA-Z]+;|&#[0-9]+;`),
}

// maskedText is a text whose protected spans have been replaced with
// placeholders.
type maskedText struct {
	Text  string   `json:"text"`
	Masks []string `json:"masks,omitempty"`
}

type span struct{ start, end int }

// maskText replaces every protected span of text with a placeholder.
func maskText(text string) maskedText {
	var spans []span
	overlaps := func(s span) bool {
		for _, o := range spans {
			if s.start < o.end && o.start < s.end {
				return true
			}
		}
		return false
	}

	for _, re := range protectedPatterns {
		for _, m := range re.FindAllStringSubmatchIndex(text, -1) {
			s := span{m[0], m[1]}
			// Patterns with a leading context group protect only the
			// second group so the preceding character stays translatable.
			if re.NumSubexp() >= 2 && m[4] >= 0 && (strings.HasPrefix(re.String(), "(^|")) {
				s = span{m[4], m[5]}
			}
			if s.end <= s.start || overlaps(s) {
				continue
			}
			spans = append(spans, s)
		}
	}

	if len(spans) == 0 {
		return maskedText{Text: text}
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i].start < spans[j].start })

	var b strings.Builder
	masks := make([]string, 0, len(spans))
	last := 0
	for _, s := range spans {
		b.WriteString(text[last:s.start])
		fmt.Fprintf(&b, maskTokenFormat, len(masks))
		masks = append(masks, text[s.start:s.end])
		last = s.end
	}
	b.WriteString(text[last:])

	return maskedText{Text: b.String(), Masks: masks}
}

// translatable reports whether anything is left to translate once the
// placeholders are removed.
func (m maskedText) translatable() bool {
	return hasLetters(maskTokenRe.ReplaceAllString(m.Text, ""))
}

// errUnknownMaskToken is returned when the provider output references a
// placeholder that was never issued.
type errUnknownMaskToken struct{ index int }

func (e *errUnknownMaskToken) Error() string {
	return fmt.Sprintf("translation references unknown placeholder %d", e.index)
}

// unmask restores the protected spans in a translated text. Placeholders the
// provider dropped are appended so no link, mention or code is lost.
func (m maskedText) unmask(translated string) (string, error) {
	if len(m.Masks) == 0 {
		return translated, nil
	}

	used := make([]bool, len(m.Masks))
	var unknown error
	out := maskTokenRe.ReplaceAllStringFunc(translated, func(tok string) string {
		sub := maskTokenRe.FindStringSubmatch(tok)
		idx, err := strconv.Atoi(sub[1])
		if err != nil || idx < 0 || idx >= len(m.Masks) {
			if unknown == nil {
				unknown = &errUnknownMaskToken{index: idx}
			}
			return tok
		}
		used[idx] = true
		return m.Masks[idx]
	})
	if unknown != nil {
		return "", unknown
	}

	var missing []string
	for i, u := range used {
		if !u {
			missing = append(missing, m.Masks[i])
		}
	}
	if len(missing) > 0 {
		out = strings.TrimRight(out, " ") + " " + strings.Join(missing, " ")
	}
	return out, nil
}
