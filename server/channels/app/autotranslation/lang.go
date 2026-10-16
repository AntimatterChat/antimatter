// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package autotranslation

import (
	"strings"
	"unicode"

	"github.com/abadojack/whatlanggo"
)

const (
	// minLocalDetectionLetters is the minimum number of letters a text needs
	// before the local detector's verdict is trusted. Short snippets are left
	// to the remote provider, which sees the whole text while translating.
	minLocalDetectionLetters = 24
	// localDetectionMinConfidence is the confidence the local detector must
	// reach for its result to be used.
	localDetectionMinConfidence = 0.9
	// maxDetectionBytes caps the amount of text handed to a detector.
	maxDetectionBytes = 4096
)

// normalizeLang reduces a locale such as "pt-BR", "zh_TW" or "EN" to its
// lower-case base language code ("pt", "zh", "en"). Translations are stored
// and looked up by this code, which is also what the web client uses as the
// key of Post.Metadata.Translations.
func normalizeLang(locale string) string {
	locale = strings.TrimSpace(locale)
	if i := strings.IndexAny(locale, "-_"); i >= 0 {
		locale = locale[:i]
	}
	return strings.ToLower(locale)
}

// targetLanguageSet returns the base language codes configured as allowed
// translation targets.
func targetLanguageSet(targets []string) map[string]bool {
	set := make(map[string]bool, len(targets))
	for _, t := range targets {
		if l := normalizeLang(t); l != "" {
			set[l] = true
		}
	}
	return set
}

// countLetters returns the number of letters in s.
func countLetters(s string) int {
	n := 0
	for _, r := range s {
		if unicode.IsLetter(r) {
			n++
		}
	}
	return n
}

// hasLetters reports whether s contains at least one letter.
func hasLetters(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) {
			return true
		}
	}
	return false
}

// truncateBytes shortens s to at most n bytes without splitting a rune.
func truncateBytes(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !isRuneStart(s[n]) {
		n--
	}
	return s[:n]
}

func isRuneStart(b byte) bool { return b&0xC0 != 0x80 }

// detectLocal detects the language of text with the embedded trigram
// detector. It returns an empty language when the verdict is not reliable
// enough to skip asking the provider.
func detectLocal(text string) (string, float64) {
	text = truncateBytes(text, maxDetectionBytes)
	if countLetters(text) < minLocalDetectionLetters {
		return "", 0
	}
	info := whatlanggo.Detect(text)
	if info.Lang < 0 || !info.IsReliable() || info.Confidence < localDetectionMinConfidence {
		return "", info.Confidence
	}
	code := info.Lang.Iso6391()
	if code == "" {
		return "", info.Confidence
	}
	return code, info.Confidence
}
