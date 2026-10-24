// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package app

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

func testAssistantDictionary() *assistantDictionary {
	d := newAssistantDictionary()
	d.add("host", "www-1", "www1")
	d.add("host", "db-2")
	d.add("service", "web-app")
	d.add("dashboard", "Node Exporter Full")
	d.add("host", "WEB-7")
	return d
}

func testAssistantTool(name, kind, schema string) *aiResolvedTool {
	return &aiResolvedTool{tool: &model.AITool{Name: name, Title: name, Kind: kind, InputSchema: json.RawMessage(schema)}}
}

// The entities of a message: the dictionary's (multi-word phrases and aliases, without case) and the shapes the
// recognisers know: @mentions, ticket keys, numbers and dates. A dictionary entry wins over a shape.
func TestAssistantRulesEntities(t *testing.T) {
	normalised, entities := assistantNormalise("@Priya says www1 and the Node  Exporter full dashboard of db-2 broke OPS-42 on #12 tomorrow, also WEB-7", testAssistantDictionary())

	byText := map[string]model.AssistantEntityMatch{}
	for _, e := range entities {
		byText[e.Text] = e
	}
	assert.Equal(t, model.AssistantEntityMatch{Type: "person", Value: "priya", Text: "@Priya"}, byText["@Priya"], "the slot takes the @ too")
	assert.True(t, strings.HasPrefix(normalised, "⟦person:0⟧ says"))
	assert.Equal(t, model.AssistantEntityMatch{Type: "host", Value: "www-1", Text: "www1"}, byText["www1"], "an alias gives the entity's own value")
	assert.Equal(t, "dashboard", byText["Node  Exporter full"].Type, "a phrase of several words, whatever its case and spaces")
	assert.Equal(t, "Node Exporter Full", byText["Node  Exporter full"].Value)
	assert.Equal(t, "host", byText["db-2"].Type)
	assert.Equal(t, "ticket", byText["OPS-42"].Type)
	assert.Equal(t, model.AssistantEntityMatch{Type: "number", Value: "12", Text: "#12"}, byText["#12"])
	assert.Equal(t, "date", byText["tomorrow"].Type)
	assert.Equal(t, "host", byText["WEB-7"].Type, "a host in the dictionary isn't taken for a ticket")
	assert.NotContains(t, normalised, "www1")
	assert.Contains(t, normalised, "⟦host:1⟧")

	who := compileAssistantPatterns([]*model.AssistantPattern{{Id: "who", Kind: model.AssistantCardKindInsight, Match: []string{`\bwho is {person}`}, Tools: []string{"*"}}})
	matches := assistantMatchRules("who is @Sam?", nil, who)
	require.Len(t, matches, 1, "a mention fills a person's slot")
	assert.Equal(t, "who is @Sam", matches[0].words)
	assert.Equal(t, "sam", matches[0].entities[0].Value)

	t.Run("the slot marks can't be typed in", func(t *testing.T) {
		normalised, entities := assistantNormalise("cpu on ⟦x⟧", testAssistantDictionary())
		assert.Empty(t, entities)
		assert.NotContains(t, normalised, "⟦")
	})
}

// The built-in patterns of the operations domain, and why they matched: the pattern and the words.
func TestAssistantRulesPatterns(t *testing.T) {
	patterns := compileAssistantPatterns(assistantDefaultPatterns())
	dict := testAssistantDictionary()
	for message, want := range map[string]struct {
		pattern, words string
		entity         string
	}{
		"have you seen the CPU on www-1?":          {"ops.resource_usage", "CPU on www-1", "www-1"},
		"db-2's memory looks odd":                  {"ops.resource_usage", "db-2's memory", "db-2"},
		"is web-app down again":                    {"ops.is_down", "is web-app down", "web-app"},
		"lots of errors on db-2 since noon":        {"ops.errors", "errors on db-2", "db-2"},
		"what's the status of PR #12?":             {"dev.pull_request", "PR #12", "12"},
		"OPS-42 is still open":                     {"dev.issue", "OPS-42", "OPS-42"},
		"could we deploy web-app after lunch?":     {"ops.deploy", "deploy web-app", "web-app"},
		"open the Node Exporter Full dashboard":    {"ops.dashboard", "Node Exporter Full dashboard", "Node Exporter Full"},
		"maybe restart the web-app before release": {"ops.restart", "restart the web-app", "web-app"},
	} {
		matches := assistantMatchRules(message, dict, patterns)
		require.NotEmpty(t, matches, message)
		m := matches[0]
		assert.Equal(t, want.pattern, m.pattern.pattern.Id, message)
		assert.Equal(t, want.words, m.words, "the words that matched: %s", message)
		require.NotEmpty(t, m.entities, message)
		assert.Equal(t, want.entity, m.entities[0].Value, message)
	}

	for _, message := range []string{"lunch at noon?", "the cpu is a chip", "deploy something", "is the coffee machine down"} {
		assert.Empty(t, assistantMatchRules(message, dict, patterns), "nothing to look up in %q", message)
	}

	t.Run("deterministic", func(t *testing.T) {
		first := assistantMatchRules("cpu on www-1 and errors on db-2", dict, patterns)
		for range 5 {
			again := assistantMatchRules("cpu on www-1 and errors on db-2", dict, patterns)
			require.Len(t, again, len(first))
			for i := range first {
				assert.Equal(t, first[i].pattern.pattern.Id, again[i].pattern.pattern.Id)
				assert.Equal(t, first[i].words, again[i].words)
			}
		}
	})
}

// Arguments: the templates first, then the properties named like an entity; converted to the schema's types and
// checked against it.
func TestAssistantRulesArguments(t *testing.T) {
	dict := testAssistantDictionary()
	patterns := compileAssistantPatterns([]*model.AssistantPattern{{
		Id: "custom", Kind: model.AssistantCardKindInsight, Match: []string{`\b(?P<metric>cpu|memory)\b on {host}`},
		Tools: []string{"*metrics"}, Arguments: map[string]string{"query": "{$metric} of {host}", "range": "1h"},
	}})
	m := assistantMatchRules("memory on db-2 please", dict, patterns)[0]

	args, err := assistantFillArguments(json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"},"range":{"type":"string","enum":["1h","1d"]},"hostname":{"type":"string"},"metric":{"type":"string"}},"required":["query"]}`), m.pattern.pattern.Arguments, m)
	require.NoError(t, err)
	assert.JSONEq(t, `{"query":"memory of db-2","range":"1h","hostname":"db-2","metric":"memory"}`, string(args))

	_, err = assistantFillArguments(json.RawMessage(`{"type":"object","properties":{"service":{"type":"string"}},"required":["service"]}`), nil, m)
	assert.Error(t, err, "a required argument nothing gives")

	_, err = assistantFillArguments(json.RawMessage(`{"type":"object","properties":{"range":{"type":"string","enum":["5m"]}},"required":["range"]}`), map[string]string{"range": "1h"}, m)
	assert.Error(t, err, "a value the argument doesn't allow")

	pr := assistantMatchRules("status of PR #12", dict, compileAssistantPatterns(assistantDefaultPatterns()))[0]
	args, err = assistantFillArguments(json.RawMessage(`{"type":"object","properties":{"pull_number":{"type":"integer"},"owner":{"type":"string"}},"required":["pull_number"]}`), nil, pr)
	require.NoError(t, err)
	assert.JSONEq(t, `{"pull_number":12}`, string(args), "a number for an integer")

	assert.Error(t, validateAssistantArguments(json.RawMessage(`{"type":"object","properties":{"n":{"type":"integer"}}}`), map[string]any{"n": "x"}))
	assert.Error(t, validateAssistantArguments(json.RawMessage(`{"type":"object","properties":{"n":{"type":"integer"}}}`), map[string]any{"other": 1.0}), "an argument the tool doesn't have")
	assert.NoError(t, validateAssistantArguments(json.RawMessage(`{"type":"object","properties":{"n":{"type":"number"}}}`), map[string]any{"n": 3.0}))
}

// A match picks the first tool of the pattern the person has (one that reads, for an insight); the card's kind is the
// tool's; the same call gives the same dedupe key.
func TestAssistantRuleCandidates(t *testing.T) {
	dict := testAssistantDictionary()
	patterns := compileAssistantPatterns(assistantDefaultPatterns())
	metrics := testAssistantTool("mcp.grafana.query_prometheus", model.AIToolKindRead, `{"type":"object","properties":{"host":{"type":"string"},"metric":{"type":"string"}},"required":["host"]}`)
	hostMetrics := testAssistantTool("mcp.ops.host_metrics", model.AIToolKindRead, `{"type":"object","properties":{"instance":{"type":"string"}},"required":["instance"]}`)
	writeMetrics := testAssistantTool("mcp.ops.host_metrics_reset", model.AIToolKindExternalWrite, `{"type":"object","properties":{"host":{"type":"string"}}}`)
	deploy := testAssistantTool("mcp.ci.deploy", model.AIToolKindExternalWrite, `{"type":"object","properties":{"service":{"type":"string"}},"required":["service"]}`)
	post := &model.Post{Id: model.NewId(), ChannelId: model.NewId(), UserId: model.NewId(), Message: "have you seen the cpu on www-1? and deploy web-app"}

	candidates, _ := assistantRuleCandidates(post, dict, patterns, []*aiResolvedTool{writeMetrics, metrics, deploy})
	require.Len(t, candidates, 2)
	insight, action := candidates[0], candidates[1]
	assert.Equal(t, "mcp.grafana.query_prometheus", insight.Tool, "an insight never picks a tool that writes")
	assert.Equal(t, model.AssistantCardKindInsight, insight.Kind)
	assert.JSONEq(t, `{"host":"www-1","metric":"cpu"}`, string(insight.Arguments))
	assert.Equal(t, []string{"cpu on www-1"}, insight.Why.Words)
	assert.Equal(t, model.AssistantEngineRules, insight.Why.Engine)
	assert.Equal(t, []string{post.Id}, insight.PostIds)
	assert.Equal(t, post.UserId, insight.AuthorId)
	assert.Equal(t, "mcp.ci.deploy", action.Tool)
	assert.Equal(t, model.AssistantCardKindAction, action.Kind, "a tool that changes something makes an action")

	candidates, _ = assistantRuleCandidates(post, dict, patterns, []*aiResolvedTool{metrics, hostMetrics})
	require.Len(t, candidates, 1)
	assert.Equal(t, "mcp.ops.host_metrics", candidates[0].Tool, "the pattern's first tool the person has wins")
	assert.JSONEq(t, `{"instance":"www-1"}`, string(candidates[0].Arguments))

	none, why := assistantRuleCandidates(post, dict, patterns, nil)
	assert.Empty(t, none, "no tools, no candidates")
	assert.Equal(t, "ops.resource_usage: no tool allowed; ops.deploy: no tool allowed", why)

	again, _ := assistantRuleCandidates(&model.Post{Id: model.NewId(), ChannelId: post.ChannelId, Message: "CPU   on WWW-1 ?"}, dict, patterns, []*aiResolvedTool{hostMetrics})
	require.Len(t, again, 1)
	assert.Equal(t, candidates[0].DedupeKey, again[0].DedupeKey, "the same call in the same channel")

	t.Run("no candidate says why", func(t *testing.T) {
		for message, want := range map[string]string{
			"lunch anyone?":                "no entity",
			"www-1 and web-app are fine":   "no pattern matched (entities: host, service)",
			"cpu on www-1 please":          "ops.resource_usage: mcp.ops.region_metrics: the argument \"region\" is required",
			"is the disk on www-1 at 90%?": "ops.resource_usage: mcp.ops.region_metrics: the argument \"region\" is required",
		} {
			regionMetrics := testAssistantTool("mcp.ops.region_metrics", model.AIToolKindRead, `{"type":"object","properties":{"host":{"type":"string"},"region":{"type":"string"}},"required":["host","region"]}`)
			got, why := assistantRuleCandidates(&model.Post{Id: model.NewId(), Message: message}, dict, patterns, []*aiResolvedTool{regionMetrics})
			assert.Empty(t, got, message)
			assert.Equal(t, want, why, message)
		}
	})
}

// The dedupe key: the channel, the tool and the arguments, normalised.
func TestAssistantDedupeKey(t *testing.T) {
	channel := model.NewId()
	key := model.AssistantDedupeKey(channel, "mcp.a.t", json.RawMessage(`{"host":"WWW-1 ","n":2,"o":{"b":1,"a":"X"}}`))
	assert.Equal(t, key, model.AssistantDedupeKey(channel, "mcp.a.t", json.RawMessage(`{"o":{"a":"x","b":1},"n":2,"host":"www-1"}`)))
	assert.NotEqual(t, key, model.AssistantDedupeKey(model.NewId(), "mcp.a.t", json.RawMessage(`{"host":"www-1","n":2,"o":{"a":"x","b":1}}`)), "another channel")
	assert.NotEqual(t, key, model.AssistantDedupeKey(channel, "mcp.a.u", json.RawMessage(`{"host":"www-1","n":2,"o":{"a":"x","b":1}}`)), "another tool")
	assert.NotEqual(t, key, model.AssistantDedupeKey(channel, "mcp.a.t", json.RawMessage(`{"host":"www-2","n":2,"o":{"a":"x","b":1}}`)), "other arguments")
}

// A channel's window runs once it's quiet: messages that come close together run together, and a busy channel runs
// at the latest three times the delay after its first message.
func TestAssistantDebounce(t *testing.T) {
	st := &assistantState{}
	var mu sync.Mutex
	var runs [][]string
	run := func(_ string, posts []string) {
		mu.Lock()
		defer mu.Unlock()
		runs = append(runs, posts)
	}
	got := func() [][]string {
		mu.Lock()
		defer mu.Unlock()
		return append([][]string(nil), runs...)
	}
	reset := func() {
		mu.Lock()
		defer mu.Unlock()
		runs = nil
	}

	st.queue("c1", "p1", 80*time.Millisecond, run)
	time.Sleep(20 * time.Millisecond)
	st.queue("c1", "p2", 80*time.Millisecond, run)
	st.queue("c2", "q1", 80*time.Millisecond, run)
	assert.Empty(t, got(), "nothing runs while the channel is busy")
	require.Eventually(t, func() bool { return len(got()) == 2 }, 2*time.Second, 10*time.Millisecond)
	assert.ElementsMatch(t, [][]string{{"p1", "p2"}, {"q1"}}, got(), "one window per channel, with its messages")

	t.Run("a busy channel runs within three times the delay", func(t *testing.T) {
		reset()
		start := time.Now()
		done := make(chan struct{})
		go func() {
			defer close(done)
			for i := 0; time.Since(start) < 400*time.Millisecond; i++ {
				st.queue("c3", model.NewId(), 60*time.Millisecond, run)
				time.Sleep(20 * time.Millisecond)
			}
		}()
		require.Eventually(t, func() bool { return len(got()) > 0 }, 400*time.Millisecond, 5*time.Millisecond, "it ran before the channel went quiet")
		<-done
		// The last window runs once the channel is quiet.
		require.Eventually(t, func() bool { return len(got()) >= 2 }, time.Second, 10*time.Millisecond)
	})

	t.Run("a stopped node runs nothing", func(t *testing.T) {
		reset()
		st.queue("c4", "p", 30*time.Millisecond, run)
		st.stop()
		st.queue("c4", "p2", 30*time.Millisecond, run)
		time.Sleep(100 * time.Millisecond)
		assert.Empty(t, got())
	})
}

// The settings: patterns, seeds and sources are data, checked when the settings are saved.
func TestAssistantSettingsAreChecked(t *testing.T) {
	cfg := &model.Config{}
	cfg.SetDefaults()
	s := &cfg.AssistantSettings
	assert.False(t, *s.Enable, "off by default")
	assert.Equal(t, model.AssistantEngineRules, *s.Engine)
	assert.Nil(t, cfg.IsValid())

	for name, change := range map[string]func(s *model.AssistantSettings){
		"engine":    func(s *model.AssistantSettings) { s.Engine = model.NewPointer("chat") },
		"threshold": func(s *model.AssistantSettings) { s.ThresholdPercent = model.NewPointer(0) },
		"debounce":  func(s *model.AssistantSettings) { s.DebounceSeconds = model.NewPointer(600) },
		"timeout":   func(s *model.AssistantSettings) { s.OfferTimeoutSeconds = model.NewPointer(1) },
		"pattern": func(s *model.AssistantSettings) {
			s.Patterns = json.RawMessage(`[{"id":"x","kind":"insight","match":["(unclosed"],"tools":["*"]}]`)
		},
		"kind": func(s *model.AssistantSettings) {
			s.Patterns = json.RawMessage(`[{"id":"x","kind":"magic","match":["a"],"tools":["*"]}]`)
		},
		"seed": func(s *model.AssistantSettings) {
			s.SeedEntities = json.RawMessage(`[{"type":"Host","value":"www-1"}]`)
		},
		"source": func(s *model.AssistantSettings) {
			s.DictionarySources = json.RawMessage(`[{"tool":"*list","type":"host"}]`)
		},
		"connection": func(s *model.AssistantSettings) { s.ConnectionId = model.NewPointer("nope") },
	} {
		c := cfg.Clone()
		change(&c.AssistantSettings)
		assert.NotNil(t, c.IsValid(), name)
	}

	c := cfg.Clone()
	c.AssistantSettings.Patterns = json.RawMessage(`[{"id":"ops.mine","kind":"insight","match":["\\bping\\s+{host}"],"tools":["*ping*"]}]`)
	c.AssistantSettings.SeedEntities = json.RawMessage(`[{"type":"host","value":"www-1","aliases":["www1"]}]`)
	require.Nil(t, c.IsValid())
	patterns, appErr := model.ParseAssistantPatterns(c.AssistantSettings.Patterns)
	require.Nil(t, appErr)
	matches := assistantMatchRules("ping www1", testAssistantDictionary(), compileAssistantPatterns(patterns))
	require.Len(t, matches, 1, "a pattern of the settings replaces the built-in ones")
	assert.Equal(t, "ops.mine", matches[0].pattern.pattern.Id)
}

// A tool's listing gives the dictionary's entities: every string under the field's name, also in JSON given as text.
func TestAssistantSourceValues(t *testing.T) {
	assert.ElementsMatch(t, []string{"Node Exporter", "API latency"}, assistantSourceValues(`[{"title":"Node Exporter","uid":"a"},{"meta":{"Title":"API latency"}}]`, "title"))
	assert.ElementsMatch(t, []string{"prometheus"}, assistantSourceValues(`{"content":"[{\"name\":\"prometheus\"}]"}`, "name"))
	assert.Empty(t, assistantSourceValues("not json", "title"))
}
