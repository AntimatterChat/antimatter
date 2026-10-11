// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package utils

import (
	"regexp"
	"strings"
)

var (
	speechCodeBlock = regexp.MustCompile("(?s)```.*?```")
	speechImage     = regexp.MustCompile(`!\[([^\]]*)\]\([^)]*\)`)
	speechLink      = regexp.MustCompile(`\[([^\]]+)\]\([^)]*\)`)
	speechURL       = regexp.MustCompile(`https?://\S+`)
	speechEmoji     = regexp.MustCompile(`:[a-z0-9_+\-]+:`)
	speechMarkup    = regexp.MustCompile("[*_~`#>|]+")
	speechSpaces    = regexp.MustCompile(`[ \t]+`)
)

// SpeechText turns a message's Markdown into the text to read aloud: link texts without their addresses, no code
// blocks, emoji shortcodes or formatting characters.
func SpeechText(message string, maxRunes int) string {
	s := speechCodeBlock.ReplaceAllString(message, " ")
	s = speechImage.ReplaceAllString(s, "$1")
	s = speechLink.ReplaceAllString(s, "$1")
	s = speechURL.ReplaceAllString(s, " ")
	s = speechEmoji.ReplaceAllString(s, " ")
	s = speechMarkup.ReplaceAllString(s, " ")
	s = speechSpaces.ReplaceAllString(s, " ")
	lines := strings.Split(s, "\n")
	out := lines[:0]
	for _, l := range lines {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	s = strings.Join(out, "\n")
	if r := []rune(s); len(r) > maxRunes {
		s = string(r[:maxRunes])
	}
	return s
}
