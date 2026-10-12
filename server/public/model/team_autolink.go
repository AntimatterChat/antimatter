// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

// Antimatter: a team's autolinks turn patterns of its messages into links when they're posted (an issue key into a
// link to the tracker, "#123" into a pull request...), as the community Autolink plugin did
// (github.com/mattermost-community/mattermost-plugin-autolink, Apache License 2.0), whose rules and replacement
// this follows.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

const (
	// TeamAutolinksMaxLength bounds a team's autolinks, as JSON.
	TeamAutolinksMaxLength = 65536
	TeamAutolinksMax       = 100
	TeamAutolinkNameMax    = 64
	TeamAutolinkPatternMax = 1000
	TeamAutolinkTargetMax  = 2000
	TeamAutolinkChannelMax = 50
)

// TeamAutolinks are a team's autolink rules.
type TeamAutolinks struct {
	Links []*TeamAutolink `json:"links"`
	// OnEdit applies the rules to edited messages too, not only new ones.
	OnEdit bool `json:"on_edit,omitempty"`
}

// TeamAutolink is a rule: the text matching Pattern (a regular expression, RE2 syntax) becomes Template, where $1
// or ${name} stand for the pattern's groups.
type TeamAutolink struct {
	Name     string `json:"name"`
	Disabled bool   `json:"disabled,omitempty"`
	Pattern  string `json:"pattern"`
	Template string `json:"template"`
	// Channels limits the rule to some of the team's channels; empty for all of them.
	Channels []string `json:"channels,omitempty"`
	// WordMatch matches whole words only (\b around the pattern).
	WordMatch bool `json:"word_match,omitempty"`
	// Without WordMatch, a match must start after a space, an opening parenthesis or emphasis (or the start of the
	// text), and end before a space, punctuation or emphasis, unless these are turned off.
	DisableNonWordPrefix bool `json:"disable_non_word_prefix,omitempty"`
	DisableNonWordSuffix bool `json:"disable_non_word_suffix,omitempty"`
	// ProcessBotPosts applies the rule to the messages of bots too.
	ProcessBotPosts bool `json:"process_bot_posts,omitempty"`
}

// CompiledAutolink is a rule ready to replace text.
type CompiledAutolink struct {
	Rule          *TeamAutolink
	re            *regexp.Regexp
	template      string
	canReplaceAll bool
}

// ParseTeamAutolinks reads a team's autolinks; an empty string has none.
func ParseTeamAutolinks(data string) (*TeamAutolinks, error) {
	links := &TeamAutolinks{}
	if strings.TrimSpace(data) == "" {
		return links, nil
	}
	if err := json.Unmarshal([]byte(data), links); err != nil {
		return nil, err
	}
	return links, nil
}

// IsValid checks a team's autolinks: their number, sizes and patterns.
func (a *TeamAutolinks) IsValid() *AppError {
	invalid := func(detail string) *AppError {
		return NewAppError("TeamAutolinks.IsValid", "model.team.is_valid.autolinks.app_error", nil, detail, http.StatusBadRequest)
	}
	if len(a.Links) > TeamAutolinksMax {
		return invalid(fmt.Sprintf("at most %d autolinks", TeamAutolinksMax))
	}
	for i, l := range a.Links {
		if l == nil {
			return invalid(fmt.Sprintf("autolink %d is empty", i+1))
		}
		if len(l.Name) > TeamAutolinkNameMax || len(l.Pattern) > TeamAutolinkPatternMax || len(l.Template) > TeamAutolinkTargetMax {
			return invalid(fmt.Sprintf("autolink %d is too long", i+1))
		}
		if strings.TrimSpace(l.Pattern) == "" || strings.TrimSpace(l.Template) == "" {
			return invalid(fmt.Sprintf("autolink %d needs a pattern and a template", i+1))
		}
		if len(l.Channels) > TeamAutolinkChannelMax {
			return invalid(fmt.Sprintf("autolink %d applies to too many channels", i+1))
		}
		for _, id := range l.Channels {
			if !IsValidId(id) {
				return invalid(fmt.Sprintf("autolink %d has an invalid channel", i+1))
			}
		}
		if _, err := l.Compile(); err != nil {
			return invalid(fmt.Sprintf("autolink %d: %s", i+1, err.Error()))
		}
		if re, err := regexp.Compile(l.Pattern); err == nil && re.MatchString("") {
			return invalid(fmt.Sprintf("autolink %d: the pattern matches empty text", i+1))
		}
	}
	return nil
}

// Compile prepares a rule. Disabled rules compile to nil.
func (l *TeamAutolink) Compile() (*CompiledAutolink, error) {
	if l.Disabled || l.Pattern == "" || l.Template == "" {
		return nil, nil
	}

	// The rule's pattern is wrapped in a group, AutolinkMatch, which $0 stands for: the matched text without the
	// space or punctuation around it. \b doesn't consume the characters around a match, so all matches can be
	// replaced at once; the custom prefix and suffix do, and their matches are replaced one at a time. The groups
	// added before the rule's own shift its numbered groups ($1...), which the template follows.
	pattern := `(?P<AutolinkMatch>` + l.Pattern + `)`
	added := 1
	prefix := ""
	canReplaceAll := false
	if !l.DisableNonWordPrefix {
		if l.WordMatch {
			pattern = `\b` + pattern
			canReplaceAll = true
		} else {
			pattern = `(?P<AutolinkNonWordPrefix>^|[\s(*_~])` + pattern
			prefix = `${AutolinkNonWordPrefix}`
			added++
		}
	}
	template := prefix + shiftGroupRefs(l.Template, added)
	if !l.DisableNonWordSuffix {
		if l.WordMatch {
			pattern += `\b`
			canReplaceAll = true
		} else {
			pattern += `(?P<AutolinkNonWordSuffix>$|[\s.!?,;:)*_~])`
			template += `${AutolinkNonWordSuffix}`
		}
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	return &CompiledAutolink{Rule: l, re: re, template: template, canReplaceAll: canReplaceAll}, nil
}

// shiftGroupRefs renumbers the numbered group references of a template ($1, ${2}) by some groups added before
// them, and turns $0 into the rule's whole match (AutolinkMatch). Named references and $$ are kept.
func shiftGroupRefs(template string, by int) string {
	var out strings.Builder
	for i := 0; i < len(template); i++ {
		c := template[i]
		if c != '$' || i+1 >= len(template) {
			out.WriteByte(c)
			continue
		}
		if template[i+1] == '$' {
			out.WriteString("$$")
			i++
			continue
		}
		name, end := "", i+1
		if template[i+1] == '{' {
			closing := strings.IndexByte(template[i+2:], '}')
			if closing < 0 {
				out.WriteByte(c)
				continue
			}
			name, end = template[i+2:i+2+closing], i+2+closing+1
		} else {
			for end < len(template) && (isWordByte(template[end])) {
				end++
			}
			name = template[i+1 : end]
		}
		if n, err := strconv.Atoi(name); err == nil && name != "" && n >= 0 {
			if n == 0 {
				out.WriteString("${AutolinkMatch}")
			} else {
				fmt.Fprintf(&out, "${%d}", n+by)
			}
		} else {
			out.WriteString(template[i:end])
		}
		i = end - 1
	}
	return out.String()
}

func isWordByte(c byte) bool {
	return c == '_' || (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// Replace replaces the rule's matches in a text.
func (c *CompiledAutolink) Replace(text string) string {
	out, _ := c.ReplaceWithin(text, -1)
	return out
}

// ReplaceWithin replaces the rule's matches in a text, unless the result grows past limit bytes (a negative limit
// is none): then it gives the text back as it was, and false. A template can be much longer than what it matches,
// and a message goes through every rule in turn, so without a bound a few rules could make a message of gigabytes.
func (c *CompiledAutolink) ReplaceWithin(text string, limit int) (string, bool) {
	if c == nil || c.re == nil {
		return text, true
	}
	tooLong := func(n int) bool { return limit >= 0 && n > limit }
	if c.canReplaceAll {
		if limit < 0 {
			return c.re.ReplaceAllString(text, c.template), true
		}
		// ReplaceAllString, match by match, to stop as soon as the result is too long.
		var out []byte
		last := 0
		for _, match := range c.re.FindAllStringSubmatchIndex(text, -1) {
			out = append(out, text[last:match[0]]...)
			out = c.re.ExpandString(out, c.template, text, match)
			last = match[1]
			if tooLong(len(out)) {
				return text, false
			}
		}
		out = append(out, text[last:]...)
		if tooLong(len(out)) {
			return text, false
		}
		return string(out), true
	}

	in := []byte(text)
	out := []byte{}
	for len(in) > 0 {
		match := c.re.FindSubmatchIndex(in)
		if match == nil {
			break
		}
		if match[1] == match[0] {
			// An empty match (refused by IsValid, but older rules may have one): move on by one character.
			if match[0] >= len(in) {
				break
			}
			out = append(out, in[:match[0]+1]...)
			in = in[match[0]+1:]
			continue
		}
		out = append(out, in[:match[0]]...)
		out = c.re.Expand(out, []byte(c.template), in, match)
		in = in[match[1]:]
		if tooLong(len(out) + len(in)) {
			return text, false
		}
	}
	out = append(out, in...)
	if tooLong(len(out)) {
		return text, false
	}
	return string(out), true
}

// AppliesTo tells whether the rule applies in a channel.
func (c *CompiledAutolink) AppliesTo(channelID string) bool {
	if len(c.Rule.Channels) == 0 {
		return true
	}
	return slices.Contains(c.Rule.Channels, channelID)
}
