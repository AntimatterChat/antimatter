// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"regexp"
	"strings"
)

// channelMentionRegexp finds ~channel-name and, in Antimatter, #channel-name links. A # is not a link after a
// letter or digit (C#, page#anchor), and #name only becomes a link when a channel of that name exists; otherwise it
// stays a hashtag.
var channelMentionRegexp = regexp.MustCompile(`\B(?:~[a-zA-Z0-9\-_]+|#[a-zA-Z0-9][a-zA-Z0-9\-_]*)`)

// channelMentionNames returns the channel names linked in s, without their ~ or #, in order.
func channelMentionNames(s string) []string {
	if !strings.ContainsAny(s, "~#") {
		return nil
	}
	var names []string
	for _, loc := range channelMentionRegexp.FindAllStringIndex(s, -1) {
		// &#39; is an HTML entity and ##name a heading, not channels.
		if s[loc[0]] == '#' && loc[0] > 0 && (s[loc[0]-1] == '&' || s[loc[0]-1] == '#') {
			continue
		}
		names = append(names, s[loc[0]+1:loc[1]])
	}
	return names
}

func ChannelMentions(message string) []string {
	return ChannelMentionsFromStrings([]string{message})
}

// ChannelMentionsFromStrings extracts ~channel-name and #channel-name mentions from each string, deduplicating across
// all inputs. Callers typically pass post.AllStrings() so mentions are found in the message, message
// attachments, and interactive payloads (mm_blocks, blocks, cards) consistently with other post text.
func ChannelMentionsFromStrings(strs []string) []string {
	alreadyMentioned := make(map[string]bool)
	var names []string

	for _, s := range strs {
		for _, name := range channelMentionNames(s) {
			if !alreadyMentioned[name] {
				names = append(names, name)
				alreadyMentioned[name] = true
			}
		}
	}

	return names
}

// ChannelMentionsFromAttachments extracts channel mentions from attachment fields.
// It scans pretext, text, and field values (but not titles, as titles are labels).
func ChannelMentionsFromAttachments(attachments []*MessageAttachment) []string {
	alreadyMentioned := make(map[string]bool)
	var names []string

	for _, attachment := range attachments {
		if attachment == nil {
			continue
		}

		// Scan pretext
		for _, name := range channelMentionNames(attachment.Pretext) {
			if !alreadyMentioned[name] {
				names = append(names, name)
				alreadyMentioned[name] = true
			}
		}

		// Scan text
		for _, name := range channelMentionNames(attachment.Text) {
			if !alreadyMentioned[name] {
				names = append(names, name)
				alreadyMentioned[name] = true
			}
		}

		// Scan field values (not titles - titles are labels)
		for _, field := range attachment.Fields {
			if field == nil {
				continue
			}

			// Field value can be any type, convert to string
			var valueStr string
			switch v := field.Value.(type) {
			case string:
				valueStr = v
			default:
				// For non-string values, we don't scan for mentions
				continue
			}

			for _, name := range channelMentionNames(valueStr) {
				if !alreadyMentioned[name] {
					names = append(names, name)
					alreadyMentioned[name] = true
				}
			}
		}
	}

	return names
}
