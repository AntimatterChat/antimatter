// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package model

import (
	"regexp"
	"slices"
	"strings"
)

// These helpers compute the "has:link", "has:code", "has:embed" and
// "mentions:@user" search filters directly from a post's message, so that the
// database search and Antimatter's own Elasticsearch/OpenSearch engine agree
// on exactly the same definition.

// urlRegexp matches a bare http(s):// or www. URL, the same links the message
// renderer turns into clickable links and, when outside code, into previews.
var urlRegexp = regexp.MustCompile(`(?i)\b(?:https?://|www\.)\S+`)

// fencedCodeRegexp matches a ``` fenced code block, and inlineCodeRegexp a
// `single backtick` code span.
var fencedCodeRegexp = regexp.MustCompile("(?s)```.*?```")
var inlineCodeRegexp = regexp.MustCompile("`[^`\n]+`")

// atMentionRegexp finds @username mentions in a message. It mirrors the
// pattern the server uses to detect possible mentions when sending a post.
var atMentionRegexp = regexp.MustCompile(`\B@[[:alnum:]][[:alnum:]\.\-_:]*`)

// HasLink reports whether message contains a bare URL, anywhere (including
// inside code).
func HasLink(message string) bool {
	return urlRegexp.MatchString(message)
}

// HasCode reports whether message contains a fenced code block or an inline
// code span.
func HasCode(message string) bool {
	return fencedCodeRegexp.MatchString(message) || inlineCodeRegexp.MatchString(message)
}

// stripCode removes fenced code blocks and inline code spans from message,
// so that links inside code (which never get an automatic preview) are
// ignored.
func stripCode(message string) string {
	message = fencedCodeRegexp.ReplaceAllString(message, " ")
	message = inlineCodeRegexp.ReplaceAllString(message, " ")
	return message
}

// HasEmbed reports whether message contains a URL outside of code, which is
// what can trigger an automatic link preview (an image or OpenGraph embed).
func HasEmbed(message string) bool {
	return HasLink(stripCode(message))
}

// PossibleAtMentions returns the usernames that look like valid @ mentions in
// message, normalized and deduplicated, in the order they first appear. It
// does not check whether the user actually exists or was notified; it is
// used to match the "mentions:@user" search filter against the literal text
// of a message, consistently with how hashtag search matches literal #tags.
func PossibleAtMentions(message string) []string {
	if !strings.Contains(message, "@") {
		return nil
	}

	var names []string
	seen := make(map[string]bool)
	for _, match := range atMentionRegexp.FindAllString(message, -1) {
		name := NormalizeUsername(match[1:])
		if !seen[name] && IsValidUsernameAllowRemote(name) {
			names = append(names, name)
			seen[name] = true
		}
	}
	return names
}

// PostHasFile reports whether post has attachments, including the legacy
// Filenames ones, like the database search's has:file clause.
func PostHasFile(post *Post) bool {
	return len(post.FileIds) > 0 || len(post.Filenames) > 0
}

// postHasFlag reports whether post has the given "has:" subtype. ReplyCount
// must already be populated (by the same resolved-thread-root subquery the
// database search and GetPostsByIds use) for "thread" to be meaningful.
func postHasFlag(post *Post, flag string) bool {
	switch flag {
	case "link":
		return HasLink(post.Message)
	case "code":
		return HasCode(post.Message)
	case "embed":
		return HasEmbed(post.Message)
	case "file":
		return PostHasFile(post)
	case "thread":
		return post.ReplyCount > 0
	default:
		return false
	}
}

// PostMatchesSearchFilters reports whether post satisfies the has:,
// pinned: and mentions: filters of params. It is the single definition
// shared by the database search and Antimatter's own Elasticsearch/
// OpenSearch engine, applied as a final check after their own query or
// index-level filtering (which, for has:file and pinned:, already narrows
// the candidates; this is what remains self-contained in the post's own
// fields and is safe to re-check unconditionally).
func PostMatchesSearchFilters(post *Post, params *SearchParams) bool {
	if params.Pinned != nil && post.IsPinned != *params.Pinned {
		return false
	}
	for _, flag := range params.HasFlags {
		if !postHasFlag(post, flag) {
			return false
		}
	}
	for _, flag := range params.ExcludedHasFlags {
		if postHasFlag(post, flag) {
			return false
		}
	}
	if len(params.Mentions) > 0 || len(params.ExcludedMentions) > 0 {
		mentioned := PossibleAtMentions(post.Message)
		for _, username := range params.Mentions {
			if !slices.Contains(mentioned, username) {
				return false
			}
		}
		for _, username := range params.ExcludedMentions {
			if slices.Contains(mentioned, username) {
				return false
			}
		}
	}
	return true
}
