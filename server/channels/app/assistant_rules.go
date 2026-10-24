// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

// The assistant's rules engine (no model): a dictionary of entities and intent patterns, both data.
//
//   - The dictionary has the seed entities of the settings and what the read-only tools list (the dictionary's
//     sources: dashboards, data sources...), run as the person and kept for ten minutes; recognisers add the
//     entities that have a shape of their own: @mentions (people), ticket keys (ABC-123), numbers (#12) and dates.
//   - A message's entities become slots ("have you seen the cpu on ⟦host:0⟧?"), and the patterns' expressions are
//     matched against that. A match picks the first tool of the pattern the person may use (a tool that reads, for an
//     insight), fills its arguments from the templates and the entities, and checks them against the tool's schema.
//   - It's deterministic: the same message, dictionary and tools give the same candidates, and each says which words
//     matched.

const (
	assistantDictionaryTTL      = 10 * time.Minute
	assistantDictionaryMaxWords = 6
	assistantSourceMaxValues    = 2000
	assistantMaxRuleCandidates  = 5
)

// assistantDefaultPatterns are the built-in intent patterns: operations first (resources, health, errors, deploys)
// and developers' (pull requests, issues, dashboards).
func assistantDefaultPatterns() []*model.AssistantPattern {
	return []*model.AssistantPattern{
		{
			Id: "ops.resource_usage", Kind: model.AssistantCardKindInsight,
			Match: []string{
				`\b(?P<metric>cpu|memory|mem|ram|disk|load|latency|traffic)\b[^.?!\n]{0,40}?\b(?:on|of|for|at)\s+(?:the\s+)?{host|service}`,
				`{host|service}(?:'s)?\s+(?P<metric>cpu|memory|mem|ram|disk|load|latency|traffic)\b`,
			},
			Tools: []string{"*host_metrics*", "*query_prometheus*", "*metrics*"},
		},
		{
			Id: "ops.is_down", Kind: model.AssistantCardKindInsight,
			Match: []string{
				`\bis\s+(?:the\s+)?{host|service}\s+(?P<state>down|up|ok|alive|working|broken|reachable)\b`,
				`{host|service}\s+(?:is|seems|looks)\s+(?:to\s+be\s+)?(?P<state>down|dead|unreachable|broken|slow)\b`,
			},
			Tools: []string{"*list_alert*", "*alert*", "*health*", "*status*"},
		},
		{
			Id: "ops.errors", Kind: model.AssistantCardKindInsight,
			Match: []string{
				`\b(?:errors?|exceptions?|5xx|500s)\b[^.?!\n]{0,30}?\b(?:on|in|from|of)\s+(?:the\s+)?{host|service}`,
			},
			Tools: []string{"*query_loki*", "*logs*", "*errors*"},
		},
		{
			Id: "dev.pull_request", Kind: model.AssistantCardKindInsight,
			Match: []string{
				`\b(?:pr|pull\s+request|mr|merge\s+request)\s*{number}`,
			},
			Tools: []string{"*get_pull_request*", "*pull_request*", "*merge_request*"},
		},
		{
			Id: "dev.issue", Kind: model.AssistantCardKindInsight,
			Match: []string{
				`\b(?:issue|ticket|bug)\s*{number}`,
				`{ticket}`,
			},
			Tools: []string{"*get_issue*", "*issue*", "*ticket*"},
		},
		{
			Id: "ops.dashboard", Kind: model.AssistantCardKindInsight,
			Match: []string{
				`\bdashboard\b[^.?!\n]{0,20}?{dashboard}`,
				`{dashboard}\s+dashboard\b`,
			},
			Tools: []string{"*get_dashboard*", "*search_dashboards*"},
		},
		{
			Id: "ops.deploy", Kind: model.AssistantCardKindAction,
			Match: []string{
				`\b(?:deploy|redeploy|roll\s*out|ship|release)\b[^.?!\n]{0,20}?(?:the\s+)?{service|repo}`,
			},
			Tools: []string{"*deploy*", "*run_pipeline*", "*trigger_pipeline*", "*workflow_dispatch*"},
		},
		{
			Id: "ops.restart", Kind: model.AssistantCardKindAction,
			Match: []string{
				`\brestart\b[^.?!\n]{0,20}?(?:the\s+)?{service|host}`,
			},
			Tools: []string{"*restart*"},
		},
	}
}

// assistantDefaultDictionarySources fill the dictionary from listings Grafana's MCP server (mcp-grafana) has.
var assistantDefaultDictionarySources = []model.AssistantDictionarySource{
	{Tool: "*search_dashboards", Type: "dashboard", Field: "title"},
	{Tool: "*list_datasources", Type: "service", Field: "name"},
}

// assistantPropertyEntityTypes say which entity fills an argument by the property's name (lower case, without "_"
// and "-"), when no template gives it.
var assistantPropertyEntityTypes = map[string]string{
	"host": "host", "hostname": "host", "instance": "host", "node": "host", "server": "host", "target": "host", "machine": "host",
	"service": "service", "job": "service", "app": "service", "application": "service", "datasource": "service",
	"dashboard": "dashboard", "dashboardtitle": "dashboard", "title": "dashboard",
	"repo": "repo", "repository": "repo", "project": "repo",
	"number": "number", "pr": "number", "prnumber": "number", "pullnumber": "number", "pullrequest": "number",
	"issue": "number", "issuenumber": "number", "iid": "number", "mergerequest": "number",
	"ticket": "ticket", "key": "ticket", "issuekey": "ticket", "ticketid": "ticket",
	"user": "person", "username": "person", "person": "person", "assignee": "person",
	"date": "date", "day": "date",
	"metric": "$metric", "state": "$state",
}

// assistantCompiledPattern is a pattern ready to match: its expressions and its tools' patterns.
type assistantCompiledPattern struct {
	pattern *model.AssistantPattern
	exprs   []*regexp.Regexp
	tools   []*regexp.Regexp
}

// assistantToolGlob compiles a pattern of tool names: "*" matches anything, dots included; nothing else is special.
func assistantToolGlob(pattern string) *regexp.Regexp {
	parts := strings.Split(pattern, "*")
	for i, part := range parts {
		parts[i] = regexp.QuoteMeta(part)
	}
	return regexp.MustCompile("^" + strings.Join(parts, ".*") + "$")
}

func compileAssistantPatterns(patterns []*model.AssistantPattern) []*assistantCompiledPattern {
	out := make([]*assistantCompiledPattern, 0, len(patterns))
	for _, p := range patterns {
		if !p.IsValid() {
			continue
		}
		cp := &assistantCompiledPattern{pattern: p}
		for _, m := range p.Match {
			re, _, err := model.CompileAssistantPatternExpression(m)
			if err != nil {
				continue
			}
			cp.exprs = append(cp.exprs, re)
		}
		for _, t := range p.Tools {
			cp.tools = append(cp.tools, assistantToolGlob(t))
		}
		out = append(out, cp)
	}
	return out
}

// assistantDictionary maps a phrase (lower case, single spaces) to its entity.
type assistantDictionary struct {
	phrases  map[string]model.AssistantEntityMatch
	maxWords int
}

func newAssistantDictionary() *assistantDictionary {
	return &assistantDictionary{phrases: map[string]model.AssistantEntityMatch{}, maxWords: 1}
}

func assistantPhrase(text string) string {
	return strings.ToLower(strings.Join(strings.Fields(text), " "))
}

// add puts an entity in the dictionary under its value and its aliases; the first one in keeps a phrase.
func (d *assistantDictionary) add(entityType, value string, aliases ...string) {
	value = strings.TrimSpace(value)
	if value == "" || utf8.RuneCountInString(value) > model.AssistantEntityMaxRunes || strings.ContainsAny(value, model.AssistantEntityTokenOpen+model.AssistantEntityTokenClose) {
		return
	}
	for _, name := range append([]string{value}, aliases...) {
		phrase := assistantPhrase(name)
		if phrase == "" || utf8.RuneCountInString(phrase) < 2 {
			continue
		}
		if _, taken := d.phrases[phrase]; taken {
			continue
		}
		d.phrases[phrase] = model.AssistantEntityMatch{Type: entityType, Value: value}
		if words := len(strings.Fields(phrase)); words > d.maxWords {
			d.maxWords = min(words, assistantDictionaryMaxWords)
		}
	}
}

var (
	assistantMentionRE = regexp.MustCompile(`(?:^|[^\w@./-])(@[a-zA-Z0-9][a-zA-Z0-9._-]*)`)
	assistantTicketRE  = regexp.MustCompile(`\b[A-Z][A-Z0-9]{1,9}-[0-9]{1,7}\b`)
	assistantNumberRE  = regexp.MustCompile(`(?:#|!)?\b[0-9]{1,7}\b`)
	assistantDateRE    = regexp.MustCompile(`(?i)\b(?:today|tomorrow|yesterday|monday|tuesday|wednesday|thursday|friday|saturday|sunday|[0-9]{4}-[0-9]{2}-[0-9]{2})\b`)
	assistantWordRE    = regexp.MustCompile(`[\p{L}\p{N}][\p{L}\p{N}._/-]*`)
)

// assistantSpan is an entity found at [start, end) of a message.
type assistantSpan struct {
	start, end int
	entity     model.AssistantEntityMatch
}

func assistantOverlaps(spans []assistantSpan, start, end int) bool {
	return slices.ContainsFunc(spans, func(s assistantSpan) bool { return start < s.end && s.start < end })
}

// assistantNormalise finds a message's entities and gives the message with each of them as a slot (⟦type:index⟧),
// and the entities in the order of their slots.
func assistantNormalise(text string, dict *assistantDictionary) (string, []model.AssistantEntityMatch) {
	text = strings.NewReplacer(model.AssistantEntityTokenOpen, " ", model.AssistantEntityTokenClose, " ").Replace(text)
	var spans []assistantSpan
	addMatches := func(re *regexp.Regexp, entityType string, group int, value func(string) string) {
		for _, m := range re.FindAllStringSubmatchIndex(text, -1) {
			start, end := m[2*group], m[2*group+1]
			if start < 0 || assistantOverlaps(spans, start, end) {
				continue
			}
			word := text[start:end]
			spans = append(spans, assistantSpan{start: start, end: end, entity: model.AssistantEntityMatch{Type: entityType, Value: value(word), Text: word}})
		}
	}
	addMatches(assistantMentionRE, "person", 1, func(word string) string { return strings.ToLower(strings.TrimPrefix(word, "@")) })

	if dict != nil && len(dict.phrases) > 0 {
		words := assistantWordRE.FindAllStringIndex(text, -1)
		for i, w := range words {
			words[i][1] = w[0] + len(strings.TrimRight(text[w[0]:w[1]], "._/-"))
		}
		for i := 0; i < len(words); {
			matched := 0
			for n := min(dict.maxWords, len(words)-i); n >= 1 && matched == 0; n-- {
				start, end := words[i][0], words[i+n-1][1]
				if n > 1 && strings.TrimSpace(gapsBetween(text, words[i:i+n])) != "" {
					// The words of a phrase are only apart by spaces.
					continue
				}
				if entity, ok := dict.phrases[assistantPhrase(text[start:end])]; ok && !assistantOverlaps(spans, start, end) {
					entity.Text = text[start:end]
					spans = append(spans, assistantSpan{start: start, end: end, entity: entity})
					matched = n
				}
			}
			i += max(matched, 1)
		}
	}

	// The dictionary's entities win over the shapes: a host called WEB-1 isn't a ticket.
	addMatches(assistantTicketRE, "ticket", 0, strings.ToUpper)
	addMatches(assistantDateRE, "date", 0, strings.ToLower)
	addMatches(assistantNumberRE, "number", 0, func(word string) string { return strings.TrimLeft(word, "#!") })

	sort.Slice(spans, func(i, j int) bool { return spans[i].start < spans[j].start })
	var out strings.Builder
	entities := make([]model.AssistantEntityMatch, 0, len(spans))
	last := 0
	for i, s := range spans {
		out.WriteString(text[last:s.start])
		fmt.Fprintf(&out, "%s%s:%d%s", model.AssistantEntityTokenOpen, s.entity.Type, i, model.AssistantEntityTokenClose)
		entities = append(entities, s.entity)
		last = s.end
	}
	out.WriteString(text[last:])
	return out.String(), entities
}

// gapsBetween gives the text between consecutive words.
func gapsBetween(text string, words [][]int) string {
	var gaps strings.Builder
	for i := 1; i < len(words); i++ {
		gaps.WriteString(text[words[i-1][1]:words[i][0]])
	}
	return gaps.String()
}

var assistantSlotTokenRE = regexp.MustCompile(model.AssistantEntityTokenOpen + `[a-z0-9_]+:([0-9]+)` + model.AssistantEntityTokenClose)

// assistantDenormalise puts the entities' own words back in a part of the normalised message.
func assistantDenormalise(normalised string, entities []model.AssistantEntityMatch) string {
	return assistantSlotTokenRE.ReplaceAllStringFunc(normalised, func(token string) string {
		index, err := strconv.Atoi(assistantSlotTokenRE.FindStringSubmatch(token)[1])
		if err != nil || index >= len(entities) {
			return token
		}
		return entities[index].Text
	})
}

// assistantRuleMatch is a pattern that matched a message: the entities of its slots, its named groups, and the
// words.
type assistantRuleMatch struct {
	pattern  *assistantCompiledPattern
	entities []model.AssistantEntityMatch
	all      []model.AssistantEntityMatch
	groups   map[string]string
	words    string
}

// assistantMatchRules matches the patterns against a message: at most one match per pattern, in the patterns' order.
func assistantMatchRules(text string, dict *assistantDictionary, patterns []*assistantCompiledPattern) []*assistantRuleMatch {
	normalised, entities := assistantNormalise(text, dict)
	var matches []*assistantRuleMatch
	for _, p := range patterns {
		for _, re := range p.exprs {
			m := re.FindStringSubmatchIndex(normalised)
			if m == nil {
				continue
			}
			match := &assistantRuleMatch{pattern: p, all: entities, groups: map[string]string{}}
			for gi, name := range re.SubexpNames() {
				if name == "" || m[2*gi] < 0 {
					continue
				}
				value := normalised[m[2*gi]:m[2*gi+1]]
				if strings.HasPrefix(name, model.AssistantSlotGroupPrefix) {
					if index, err := strconv.Atoi(value); err == nil && index < len(entities) {
						match.entities = append(match.entities, entities[index])
					}
					continue
				}
				match.groups[name] = strings.ToLower(value)
			}
			match.words = strings.TrimSpace(assistantDenormalise(normalised[m[0]:m[1]], entities))
			matches = append(matches, match)
			break
		}
	}
	return matches
}

// entity gives the first entity of a type: one of the match's slots, else any in the message.
func (m *assistantRuleMatch) entity(entityType string) (string, bool) {
	for _, list := range [][]model.AssistantEntityMatch{m.entities, m.all} {
		for _, e := range list {
			if e.Type == entityType {
				return e.Value, true
			}
		}
	}
	return "", false
}

var assistantTemplateRE = regexp.MustCompile(`\{(\$?[a-z][a-z0-9_]*)\}`)

// value gives what a template or an entity type ("$name" for a named group) stands for.
func (m *assistantRuleMatch) value(name string) (string, bool) {
	if group, ok := strings.CutPrefix(name, "$"); ok {
		v, ok := m.groups[group]
		return v, ok && v != ""
	}
	return m.entity(name)
}

func (m *assistantRuleMatch) expand(template string) (string, bool) {
	ok := true
	out := assistantTemplateRE.ReplaceAllStringFunc(template, func(placeholder string) string {
		v, found := m.value(assistantTemplateRE.FindStringSubmatch(placeholder)[1])
		if !found {
			ok = false
		}
		return v
	})
	return out, ok
}

// assistantSchema is the part of a tool's input schema the assistant checks: the properties' types and allowed
// values, and the required ones.
type assistantSchema struct {
	Properties map[string]assistantSchemaProperty `json:"properties"`
	Required   []string                           `json:"required"`
}

type assistantSchemaProperty struct {
	Type        any    `json:"type"`
	Enum        []any  `json:"enum"`
	Description string `json:"description"`
}

func parseAssistantSchema(raw json.RawMessage) *assistantSchema {
	schema := &assistantSchema{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, schema)
	}
	return schema
}

// types gives the JSON types a property takes (none: any).
func (p assistantSchemaProperty) types() []string {
	switch t := p.Type.(type) {
	case string:
		return []string{t}
	case []any:
		var out []string
		for _, item := range t {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

func (p assistantSchemaProperty) takes(jsonType string) bool {
	types := p.types()
	return len(types) == 0 || slices.Contains(types, jsonType) || jsonType == "integer" && slices.Contains(types, "number")
}

// propertyNames gives a schema's properties, sorted (so that the engines are deterministic).
func (s *assistantSchema) propertyNames() []string {
	names := make([]string, 0, len(s.Properties))
	for name := range s.Properties {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// assistantConvert turns a text into a property's value: a number for integer and number properties, an enum's own
// value (matched without case), else the text.
func assistantConvert(p assistantSchemaProperty, text string) (any, bool) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, false
	}
	if len(p.Enum) > 0 {
		for _, option := range p.Enum {
			if s, ok := option.(string); ok && strings.EqualFold(s, text) {
				return s, true
			}
		}
		return nil, false
	}
	switch {
	case p.takes("string"):
		return text, true
	case p.takes("integer"):
		n, err := strconv.ParseInt(strings.TrimLeft(text, "#!"), 10, 64)
		return n, err == nil
	case p.takes("number"):
		f, err := strconv.ParseFloat(text, 64)
		return f, err == nil
	}
	return nil, false
}

// validateAssistantArguments checks arguments against a tool's schema: every required one, of its type, one of
// its allowed values, and none it doesn't have.
func validateAssistantArguments(raw json.RawMessage, args map[string]any) error {
	schema := parseAssistantSchema(raw)
	for _, name := range schema.Required {
		if _, ok := args[name]; !ok {
			return fmt.Errorf("the argument %q is required", name)
		}
	}
	for name, value := range args {
		p, ok := schema.Properties[name]
		if !ok {
			if len(schema.Properties) > 0 {
				return fmt.Errorf("the tool has no argument %q", name)
			}
			continue
		}
		jsonType := ""
		switch v := value.(type) {
		case string:
			jsonType = "string"
		case bool:
			jsonType = "boolean"
		case int64, int:
			jsonType = "integer"
		case float64:
			jsonType = "number"
			if v == math.Trunc(v) {
				jsonType = "integer"
			}
		default:
			return fmt.Errorf("the argument %q has an unexpected type", name)
		}
		if !p.takes(jsonType) && !(jsonType == "integer" && p.takes("number")) {
			return fmt.Errorf("the argument %q isn't a %s", name, strings.Join(p.types(), " or "))
		}
		if len(p.Enum) > 0 && !slices.ContainsFunc(p.Enum, func(option any) bool { return fmt.Sprint(option) == fmt.Sprint(value) }) {
			return fmt.Errorf("the argument %q isn't one of its values", name)
		}
	}
	return nil
}

// assistantFillArguments fills a tool's arguments for a match: the pattern's templates first, then the properties
// the entities fill by their names, and checks them against the schema.
func assistantFillArguments(schemaRaw json.RawMessage, templates map[string]string, m *assistantRuleMatch) (json.RawMessage, error) {
	schema := parseAssistantSchema(schemaRaw)
	args := map[string]any{}
	names := make([]string, 0, len(templates))
	for name := range templates {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		text, ok := m.expand(templates[name])
		if !ok {
			continue
		}
		p, known := schema.Properties[name]
		if !known && len(schema.Properties) > 0 {
			continue
		}
		if value, ok := assistantConvert(p, text); ok {
			args[name] = value
		}
	}
	for _, name := range schema.propertyNames() {
		if _, set := args[name]; set {
			continue
		}
		entityType, ok := assistantPropertyEntityTypes[strings.NewReplacer("_", "", "-", "").Replace(strings.ToLower(name))]
		if !ok {
			continue
		}
		if text, ok := m.value(entityType); ok {
			if value, ok := assistantConvert(schema.Properties[name], text); ok {
				args[name] = value
			}
		}
	}
	if err := validateAssistantArguments(schemaRaw, args); err != nil {
		return nil, err
	}
	out, err := json.Marshal(args)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// assistantPickTool gives the first of a pattern's tools the person may use: one that reads, for an insight.
func assistantPickTool(p *assistantCompiledPattern, tools []*aiResolvedTool) *aiResolvedTool {
	for _, glob := range p.tools {
		for _, rt := range tools {
			if !glob.MatchString(rt.tool.Name) {
				continue
			}
			if p.pattern.Kind == model.AssistantCardKindInsight && !rt.tool.IsRead() {
				continue
			}
			return rt
		}
	}
	return nil
}

// assistantCardKind is a tool's kind of card: an insight for a tool that reads, else an action.
func assistantCardKind(tool *model.AITool) string {
	if tool.IsRead() {
		return model.AssistantCardKindInsight
	}
	return model.AssistantCardKindAction
}

// assistantRuleCandidates gives the rules engine's candidates for one message.
func assistantRuleCandidates(post *model.Post, dict *assistantDictionary, patterns []*assistantCompiledPattern, tools []*aiResolvedTool) []*assistantCandidate {
	var out []*assistantCandidate
	seen := map[string]bool{}
	for _, m := range assistantMatchRules(post.Message, dict, patterns) {
		rt := assistantPickTool(m.pattern, tools)
		if rt == nil {
			continue
		}
		args, err := assistantFillArguments(rt.tool.InputSchema, m.pattern.pattern.Arguments, m)
		if err != nil {
			continue
		}
		c := &assistantCandidate{
			AssistantCandidate: model.AssistantCandidate{
				ChannelId: post.ChannelId, PostIds: []string{post.Id}, AuthorId: post.UserId,
				Kind: assistantCardKind(rt.tool), Tool: rt.tool.Name, Arguments: args,
				Why: model.AssistantWhy{Engine: model.AssistantEngineRules, Pattern: m.pattern.pattern.Id, Words: []string{m.words}, Entities: m.entities},
			},
			tool: rt,
		}
		c.DedupeKey = model.AssistantDedupeKey(c.ChannelId, c.Tool, c.Arguments)
		if seen[c.DedupeKey] {
			continue
		}
		seen[c.DedupeKey] = true
		out = append(out, c)
		if len(out) == assistantMaxRuleCandidates {
			break
		}
	}
	return out
}

// assistantDictionaryCache is a source's values, for a while.
type assistantDictionaryCache struct {
	values  []string
	expires time.Time
}

// assistantListingOwner is whose listing a tool gives: everyone's for an MCP server without sign-ins of its own (a
// service account, or none), else the person's (their own sign-in, or the server's and the apps' tools, which check
// their permissions).
func assistantListingOwner(rt *aiResolvedTool, userID string) string {
	if rt.mcp != nil && rt.mcp.Auth != model.AIMCPAuthOAuth {
		return ""
	}
	return userID
}

// assistantSourceValues gives a listing's strings under a key named field, anywhere in its JSON.
func assistantSourceValues(result, field string) []string {
	var data any
	if json.Unmarshal([]byte(result), &data) != nil {
		return nil
	}
	var values []string
	var walk func(v any)
	walk = func(v any) {
		if len(values) >= assistantSourceMaxValues {
			return
		}
		switch t := v.(type) {
		case map[string]any:
			for key, item := range t {
				if s, ok := item.(string); ok && strings.EqualFold(key, field) {
					values = append(values, s)
					continue
				}
				walk(item)
			}
		case []any:
			for _, item := range t {
				walk(item)
			}
		case string:
			// A text result may carry JSON (MCP servers often give their JSON as text).
			if strings.HasPrefix(strings.TrimSpace(t), "{") || strings.HasPrefix(strings.TrimSpace(t), "[") {
				values = append(values, assistantSourceValues(t, field)...)
			}
		}
	}
	walk(data)
	return values
}

// assistantDictionaryFor builds a person's dictionary in a channel: the seeds, then the listings of the dictionary's
// sources they may run (as them, kept ten minutes per tool and owner of the listing).
func (a *App) assistantDictionaryFor(rctx request.CTX, user *model.User, channel *model.Channel, tools []*aiResolvedTool, cfg *assistantConfig) *assistantDictionary {
	dict := newAssistantDictionary()
	for _, e := range cfg.seeds {
		dict.add(e.Type, e.Value, e.Aliases...)
	}
	st := a.assistant()
	for _, source := range cfg.sources {
		glob := assistantToolGlob(source.Tool)
		for _, rt := range tools {
			if !rt.tool.IsRead() || !glob.MatchString(rt.tool.Name) {
				continue
			}
			key := rt.tool.Name + "|" + string(source.Arguments) + "|" + source.Field + "|" + assistantListingOwner(rt, user.Id)
			st.mu.Lock()
			cached, ok := st.dicts[key]
			st.mu.Unlock()
			if !ok || time.Now().After(cached.expires) {
				args := source.Arguments
				if len(args) == 0 {
					args = json.RawMessage(`{}`)
				}
				text, err := a.runAssistantTool(rctx, user, channel, rt, args)
				cached = &assistantDictionaryCache{expires: time.Now().Add(assistantDictionaryTTL)}
				if err != nil {
					rctx.Logger().Debug("The assistant couldn't list a tool's entities", mlog.String("tool", rt.tool.Name), mlog.Err(err))
				} else {
					cached.values = assistantSourceValues(text, source.Field)
				}
				st.mu.Lock()
				if st.dicts == nil {
					st.dicts = map[string]*assistantDictionaryCache{}
				}
				st.dicts[key] = cached
				st.mu.Unlock()
			}
			for _, value := range cached.values {
				dict.add(source.Type, value)
			}
		}
	}
	return dict
}

var errAssistantNotARead = errors.New("the assistant only runs tools that read on its own")
