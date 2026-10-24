// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package model

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

// The assistant (Antimatter, stage 1 of the ambient assistant): a personal feed of cards for the conversation a person
// has the assistant open on. Two engines read the new messages and propose tool calls in one format (an
// AssistantCandidate): the rules engine (a dictionary of entities and intent patterns, no model) and the model engine
// (the decisions capability of a model connection, never its chat model). A read-only tool runs on its own, as the
// person, and its result becomes an insight card in their feed; a tool that changes something becomes an action card,
// offered to one person at a time and never run on its own.

const (
	AssistantEngineRules = "rules"
	AssistantEngineModel = "model"

	// The kinds of cards: an insight comes from a tool that only reads, an action from one that changes something.
	AssistantCardKindInsight = "insight"
	AssistantCardKindAction  = "action"

	// The states of a card in its owner's feed.
	AssistantCardStatusNew       = "new"
	AssistantCardStatusDismissed = "dismissed"
	AssistantCardStatusPosted    = "posted"

	// The states of an action's offer, shared by everyone who has the card.
	AssistantOfferStatusOpen    = "open"
	AssistantOfferStatusDone    = "done"
	AssistantOfferStatusExpired = "expired"

	// PreferenceCategoryAssistant keeps a person's choices: per channel (named by the channel's ID) "on" or "off",
	// and the tools they don't want suggested any more (PreferenceNameAssistantMutedTools, a JSON list).
	PreferenceCategoryAssistant       = "assistant"
	PreferenceNameAssistantMutedTools = "muted_tools"
	AssistantChannelOn                = "on"
	AssistantChannelOff               = "off"

	// PostPropAssistantCard marks a message someone posted from one of their cards (the card's ID).
	PostPropAssistantCard = "assistant_card"

	// A panel that has the assistant open says so at least every AssistantPresenceHeartbeatSeconds; it's taken as
	// closed AssistantPresenceTTLSeconds after its last word.
	AssistantPresenceTTLSeconds       = 90
	AssistantPresenceHeartbeatSeconds = 30

	AssistantMaxMutedTools     = 200
	AssistantResultMaxRunes    = 4000
	AssistantFeedMaxCards      = 50
	AssistantMaxPatterns       = 100
	AssistantMaxSeedEntities   = 2000
	AssistantMaxDictionaryKind = 32
	AssistantEntityMaxRunes    = 80
)

// AssistantSettings are the assistant's server settings (Config.AssistantSettings). It's off until a system admin turns
// it on; a community's admins may then change it for their community when AllowCommunityOverride is on.
type AssistantSettings struct {
	Enable *bool `access:"integrations_integration_management"`
	// Engine is "rules" or "model".
	Engine                 *string `access:"integrations_integration_management"`
	AllowCommunityOverride *bool   `access:"integrations_integration_management"`
	// ConnectionId and Model are the model engine's decision model: a server-wide connection's decisions model
	// (Model empty) or the model named.
	ConnectionId *string `access:"integrations_integration_management"`
	Model        *string `access:"integrations_integration_management"`
	// ThresholdPercent is the probability (in percent) the model engine needs to make a card.
	ThresholdPercent *int `access:"integrations_integration_management"`
	// The hourly budgets: the model engine's decisions per person and per channel, and the cards per person (both
	// engines).
	DecisionsPerUserHour    *int `access:"integrations_integration_management"`
	DecisionsPerChannelHour *int `access:"integrations_integration_management"`
	CardsPerUserHour        *int `access:"integrations_integration_management"`
	// DebounceSeconds of quiet in a channel make a window of its new messages.
	DebounceSeconds *int `access:"integrations_integration_management"`
	// OfferTimeoutSeconds is how long an action is offered to one person before it goes to the next.
	OfferTimeoutSeconds *int `access:"integrations_integration_management"`
	// AllowedTools names the tools the assistant may use, as agents' policies do ("*", "<namespace>.*", full names).
	AllowedTools []string `access:"integrations_integration_management"`
	// Patterns ([]AssistantPattern), SeedEntities ([]AssistantEntity) and DictionarySources
	// ([]AssistantDictionarySource) are the rules engine's data, as JSON; empty Patterns or DictionarySources use the
	// built-in ones.
	Patterns          json.RawMessage `access:"integrations_integration_management"`
	SeedEntities      json.RawMessage `access:"integrations_integration_management"`
	DictionarySources json.RawMessage `access:"integrations_integration_management"`
}

func (s *AssistantSettings) SetDefaults() {
	if s.Enable == nil {
		s.Enable = NewPointer(false)
	}
	if s.Engine == nil {
		s.Engine = NewPointer(AssistantEngineRules)
	}
	if s.AllowCommunityOverride == nil {
		s.AllowCommunityOverride = NewPointer(false)
	}
	if s.ConnectionId == nil {
		s.ConnectionId = NewPointer("")
	}
	if s.Model == nil {
		s.Model = NewPointer("")
	}
	if s.ThresholdPercent == nil {
		s.ThresholdPercent = NewPointer(70)
	}
	if s.DecisionsPerUserHour == nil {
		s.DecisionsPerUserHour = NewPointer(30)
	}
	if s.DecisionsPerChannelHour == nil {
		s.DecisionsPerChannelHour = NewPointer(60)
	}
	if s.CardsPerUserHour == nil {
		s.CardsPerUserHour = NewPointer(20)
	}
	if s.DebounceSeconds == nil {
		s.DebounceSeconds = NewPointer(4)
	}
	if s.OfferTimeoutSeconds == nil {
		s.OfferTimeoutSeconds = NewPointer(120)
	}
	if s.AllowedTools == nil {
		s.AllowedTools = []string{"*"}
	}
}

func (s *AssistantSettings) isValid() *AppError {
	invalid := func(field string) *AppError {
		return NewAppError("Config.IsValid", "model.config.is_valid.assistant.app_error", map[string]any{"Setting": field}, "", http.StatusBadRequest)
	}
	if s.Engine != nil && !IsValidAssistantEngine(*s.Engine) {
		return invalid("Engine")
	}
	if s.ConnectionId != nil && *s.ConnectionId != "" && !IsValidId(*s.ConnectionId) {
		return invalid("ConnectionId")
	}
	if s.Model != nil && len(*s.Model) > AIModelNameMaxLength {
		return invalid("Model")
	}
	for field, bounds := range map[string]struct {
		value    *int
		min, max int
	}{
		"ThresholdPercent":        {s.ThresholdPercent, 1, 100},
		"DecisionsPerUserHour":    {s.DecisionsPerUserHour, 0, 10000},
		"DecisionsPerChannelHour": {s.DecisionsPerChannelHour, 0, 10000},
		"CardsPerUserHour":        {s.CardsPerUserHour, 0, 10000},
		"DebounceSeconds":         {s.DebounceSeconds, 0, 60},
		"OfferTimeoutSeconds":     {s.OfferTimeoutSeconds, 10, 3600},
	} {
		if bounds.value != nil && (*bounds.value < bounds.min || *bounds.value > bounds.max) {
			return invalid(field)
		}
	}
	if len(s.AllowedTools) > AIToolChoicesMaxItems {
		return invalid("AllowedTools")
	}
	for _, pattern := range s.AllowedTools {
		if pattern != "*" && !IsValidAIAgentToolName(pattern) {
			return invalid("AllowedTools")
		}
	}
	if _, appErr := ParseAssistantPatterns(s.Patterns); appErr != nil {
		return invalid("Patterns")
	}
	if _, appErr := ParseAssistantSeedEntities(s.SeedEntities); appErr != nil {
		return invalid("SeedEntities")
	}
	if _, appErr := ParseAssistantDictionarySources(s.DictionarySources); appErr != nil {
		return invalid("DictionarySources")
	}
	return nil
}

// IsValidAssistantEngine tells whether an engine is one of the assistant's.
func IsValidAssistantEngine(engine string) bool {
	return engine == AssistantEngineRules || engine == AssistantEngineModel
}

// AssistantPattern is one of the rules engine's intent patterns. Its Match expressions are regular expressions over
// a message where the entities found are slots: "{host}" stands for a host, "{host|service}" for either. Named groups
// ((?P<metric>cpu|memory)) are words the arguments may use. Tools are patterns of tool names ("*" matches anything,
// dots included), tried in order: the first tool the person may use wins. An insight pattern only picks a tool that
// reads. Arguments are templates for the tool's arguments ("{host}", "{$metric}", or plain text); the arguments the
// templates don't give are filled from the entities by the property's name.
type AssistantPattern struct {
	Id        string            `json:"id"`
	Kind      string            `json:"kind"`
	Match     []string          `json:"match"`
	Tools     []string          `json:"tools"`
	Arguments map[string]string `json:"arguments,omitempty"`
}

// AssistantEntity is an entry of the rules engine's dictionary: a host, a service, a dashboard, a repository, a
// person..., with the other names it goes by.
type AssistantEntity struct {
	Type    string   `json:"type"`
	Value   string   `json:"value"`
	Aliases []string `json:"aliases,omitempty"`
}

// AssistantDictionarySource fills the dictionary from a read-only tool's listing: Tool (a pattern of tool names) runs
// with Arguments as the person, and every string under a key named Field in its JSON result is an entity of Type.
type AssistantDictionarySource struct {
	Tool      string          `json:"tool"`
	Type      string          `json:"type"`
	Field     string          `json:"field"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

var (
	assistantEntityType = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)
	assistantPatternID  = regexp.MustCompile(`^[a-z0-9][a-z0-9_.-]{0,63}$`)
	assistantSlot       = regexp.MustCompile(`\{([a-z][a-z0-9_]*(?:\|[a-z][a-z0-9_]*)*)\}`)
)

// AssistantEntityTokenOpen and AssistantEntityTokenClose wrap an entity's slot in a message the rules engine
// matches: ⟦host:0⟧ is the first entity found, a host.
const (
	AssistantEntityTokenOpen  = "⟦"
	AssistantEntityTokenClose = "⟧"
	// AssistantSlotGroupPrefix starts the names of the groups a pattern's slots become ("slot__0", "slot__1"...).
	AssistantSlotGroupPrefix = "slot__"
)

// CompileAssistantPatternExpression compiles one of a pattern's Match expressions: its slots become groups that
// capture the entity's index, named in the order of the slots in the result.
func CompileAssistantPatternExpression(expression string) (*regexp.Regexp, []string, error) {
	var slots []string
	expanded := assistantSlot.ReplaceAllStringFunc(expression, func(slot string) string {
		types := assistantSlot.FindStringSubmatch(slot)[1]
		name := AssistantSlotGroupPrefix + strconv.Itoa(len(slots))
		slots = append(slots, name)
		return AssistantEntityTokenOpen + "(?:" + types + "):(?P<" + name + ">[0-9]+)" + AssistantEntityTokenClose
	})
	re, err := regexp.Compile("(?i)" + expanded)
	if err != nil {
		return nil, nil, err
	}
	return re, slots, nil
}

// IsAssistantToolPattern tells whether a pattern of tool names is well formed.
func IsAssistantToolPattern(pattern string) bool {
	if pattern == "" || len(pattern) > AIAgentToolNameMaxLength {
		return false
	}
	return !strings.ContainsAny(pattern, " \t\r\n")
}

// IsValid checks a pattern.
func (p *AssistantPattern) IsValid() bool {
	if p == nil || !assistantPatternID.MatchString(p.Id) {
		return false
	}
	if p.Kind != AssistantCardKindInsight && p.Kind != AssistantCardKindAction {
		return false
	}
	if len(p.Match) == 0 || len(p.Match) > 16 || len(p.Tools) == 0 || len(p.Tools) > 16 || len(p.Arguments) > 32 {
		return false
	}
	for _, m := range p.Match {
		if m == "" || len(m) > 1024 {
			return false
		}
		if _, _, err := CompileAssistantPatternExpression(m); err != nil {
			return false
		}
	}
	for _, t := range p.Tools {
		if !IsAssistantToolPattern(t) {
			return false
		}
	}
	for name, template := range p.Arguments {
		if name == "" || len(name) > 128 || len(template) > 512 {
			return false
		}
	}
	return true
}

// ParseAssistantPatterns reads the patterns of the settings; nil for none (the built-in ones).
func ParseAssistantPatterns(raw json.RawMessage) ([]*AssistantPattern, *AppError) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var patterns []*AssistantPattern
	if err := json.Unmarshal(raw, &patterns); err != nil {
		return nil, NewAppError("ParseAssistantPatterns", "model.assistant.patterns.app_error", nil, "", http.StatusBadRequest).Wrap(err)
	}
	if len(patterns) > AssistantMaxPatterns {
		return nil, NewAppError("ParseAssistantPatterns", "model.assistant.patterns.app_error", nil, "too many", http.StatusBadRequest)
	}
	seen := map[string]bool{}
	for _, p := range patterns {
		if !p.IsValid() || seen[p.Id] {
			return nil, NewAppError("ParseAssistantPatterns", "model.assistant.patterns.app_error", nil, "", http.StatusBadRequest)
		}
		seen[p.Id] = true
	}
	return patterns, nil
}

// IsValidAssistantEntityType tells whether an entity type is well formed (lower case, as in "host").
func IsValidAssistantEntityType(t string) bool {
	return assistantEntityType.MatchString(t)
}

func isValidAssistantEntityName(name string) bool {
	trimmed := strings.TrimSpace(name)
	return trimmed != "" && utf8.RuneCountInString(trimmed) <= AssistantEntityMaxRunes && !strings.ContainsAny(trimmed, AssistantEntityTokenOpen+AssistantEntityTokenClose+"\n")
}

// ParseAssistantSeedEntities reads the seed entities of the settings.
func ParseAssistantSeedEntities(raw json.RawMessage) ([]AssistantEntity, *AppError) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var entities []AssistantEntity
	if err := json.Unmarshal(raw, &entities); err != nil {
		return nil, NewAppError("ParseAssistantSeedEntities", "model.assistant.entities.app_error", nil, "", http.StatusBadRequest).Wrap(err)
	}
	if len(entities) > AssistantMaxSeedEntities {
		return nil, NewAppError("ParseAssistantSeedEntities", "model.assistant.entities.app_error", nil, "too many", http.StatusBadRequest)
	}
	for _, e := range entities {
		if !IsValidAssistantEntityType(e.Type) || !isValidAssistantEntityName(e.Value) || len(e.Aliases) > 16 {
			return nil, NewAppError("ParseAssistantSeedEntities", "model.assistant.entities.app_error", nil, "", http.StatusBadRequest)
		}
		for _, alias := range e.Aliases {
			if !isValidAssistantEntityName(alias) {
				return nil, NewAppError("ParseAssistantSeedEntities", "model.assistant.entities.app_error", nil, "", http.StatusBadRequest)
			}
		}
	}
	return entities, nil
}

// ParseAssistantDictionarySources reads the dictionary's sources of the settings; nil for none (the built-in ones).
func ParseAssistantDictionarySources(raw json.RawMessage) ([]AssistantDictionarySource, *AppError) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var sources []AssistantDictionarySource
	if err := json.Unmarshal(raw, &sources); err != nil {
		return nil, NewAppError("ParseAssistantDictionarySources", "model.assistant.sources.app_error", nil, "", http.StatusBadRequest).Wrap(err)
	}
	if len(sources) > AssistantMaxDictionaryKind {
		return nil, NewAppError("ParseAssistantDictionarySources", "model.assistant.sources.app_error", nil, "too many", http.StatusBadRequest)
	}
	for _, s := range sources {
		if !IsAssistantToolPattern(s.Tool) || !IsValidAssistantEntityType(s.Type) || s.Field == "" || len(s.Field) > 64 || len(s.Arguments) > AIToolArgumentsMaxBytes {
			return nil, NewAppError("ParseAssistantDictionarySources", "model.assistant.sources.app_error", nil, "", http.StatusBadRequest)
		}
		if len(s.Arguments) > 0 {
			var object map[string]any
			if json.Unmarshal(s.Arguments, &object) != nil {
				return nil, NewAppError("ParseAssistantDictionarySources", "model.assistant.sources.app_error", nil, "arguments", http.StatusBadRequest)
			}
		}
	}
	return sources, nil
}

// AssistantCommunitySettings is what a community's admins changed for their community, when the server lets them
// (AllowCommunityOverride): nil Enabled and empty fields keep the server's. A community can't turn the assistant on
// when the server has it off.
type AssistantCommunitySettings struct {
	Enabled      *bool  `json:"enabled"`
	Engine       string `json:"engine"`
	ConnectionId string `json:"connection_id"`
	Model        string `json:"model"`
}

// IsValid checks a community's settings.
func (s *AssistantCommunitySettings) IsValid() *AppError {
	if s.Engine != "" && !IsValidAssistantEngine(s.Engine) {
		return NewAppError("AssistantCommunitySettings.IsValid", "model.assistant.community_settings.app_error", nil, "engine", http.StatusBadRequest)
	}
	if s.ConnectionId != "" && !IsValidId(s.ConnectionId) {
		return NewAppError("AssistantCommunitySettings.IsValid", "model.assistant.community_settings.app_error", nil, "connection_id", http.StatusBadRequest)
	}
	if len(s.Model) > AIModelNameMaxLength {
		return NewAppError("AssistantCommunitySettings.IsValid", "model.assistant.community_settings.app_error", nil, "model", http.StatusBadRequest)
	}
	return nil
}

// AssistantCommunitySettingsInfo is a community's settings as its admins see them, with whether the server lets
// them change anything.
type AssistantCommunitySettingsInfo struct {
	TeamId        string                     `json:"team_id"`
	ServerEnabled bool                       `json:"server_enabled"`
	MayOverride   bool                       `json:"may_override"`
	Settings      AssistantCommunitySettings `json:"settings"`
}

// AssistantEntityMatch is an entity found in a message: its type, its value in the dictionary, and the words.
type AssistantEntityMatch struct {
	Type  string `json:"type"`
	Value string `json:"value"`
	Text  string `json:"text"`
}

// AssistantWhy is why a card appeared: the engine, and the pattern and words that matched (rules) or the model's
// probabilities (model).
type AssistantWhy struct {
	Engine        string                 `json:"engine"`
	Pattern       string                 `json:"pattern,omitempty"`
	Words         []string               `json:"words,omitempty"`
	Entities      []AssistantEntityMatch `json:"entities,omitempty"`
	Probabilities map[string]float64     `json:"probabilities,omitempty"`
}

// AssistantCandidate is what an engine proposes: a tool call, with where it came from and why.
type AssistantCandidate struct {
	ChannelId string          `json:"channel_id"`
	PostIds   []string        `json:"post_ids"`
	AuthorId  string          `json:"author_id"`
	Kind      string          `json:"kind"`
	Tool      string          `json:"tool"`
	Arguments json.RawMessage `json:"arguments"`
	Why       AssistantWhy    `json:"why"`
	DedupeKey string          `json:"dedupe_key"`
}

// AssistantDedupeKey is the same for the same tool call in the same channel: a hash of the channel, the tool and the
// arguments, normalised (keys sorted, strings trimmed, in lower case, spaces collapsed).
func AssistantDedupeKey(channelID, tool string, arguments json.RawMessage) string {
	var value any
	normalised := []byte("{}")
	if len(arguments) > 0 && json.Unmarshal(arguments, &value) == nil {
		if out, err := json.Marshal(normaliseAssistantArgument(value)); err == nil {
			normalised = out
		}
	}
	sum := sha256.Sum256([]byte(channelID + "\n" + tool + "\n" + string(normalised)))
	return hex.EncodeToString(sum[:16])
}

func normaliseAssistantArgument(value any) any {
	switch v := value.(type) {
	case string:
		return strings.ToLower(strings.Join(strings.Fields(v), " "))
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, item := range v {
			out[key] = normaliseAssistantArgument(item)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = normaliseAssistantArgument(item)
		}
		return out
	}
	return value
}

// AssistantOfferView is where an action's offer stands, as one person sees it.
type AssistantOfferView struct {
	Id     string `json:"id"`
	Status string `json:"status"`
	// HolderId is the person it's offered to now; "" when it's offered to nobody (nobody who may run it is viewing).
	HolderId    string `json:"holder_id"`
	OfferedToMe bool   `json:"offered_to_me"`
	CanRun      bool   `json:"can_run"`
	// MovesAt is when it goes to the next person if the holder doesn't run or dismiss it (0: it doesn't move).
	MovesAt int64  `json:"moves_at"`
	DoneBy  string `json:"done_by,omitempty"`
	DoneAt  int64  `json:"done_at,omitempty"`
}

// AssistantCard is one card of a person's feed.
type AssistantCard struct {
	Id        string          `json:"id"`
	UserId    string          `json:"user_id"`
	ChannelId string          `json:"channel_id"`
	Kind      string          `json:"kind"`
	Tool      string          `json:"tool"`
	ToolTitle string          `json:"tool_title"`
	Arguments json.RawMessage `json:"arguments"`
	Why       AssistantWhy    `json:"why"`
	PostIds   []string        `json:"post_ids"`
	DedupeKey string          `json:"dedupe_key"`
	// Result is what the tool gave (insights only), shortened.
	Result string `json:"result,omitempty"`
	Status string `json:"status"`
	Pinned bool   `json:"pinned"`
	// OfferId and Offer are an action's offer (Offer is filled when the card is given out).
	OfferId  string              `json:"offer_id,omitempty"`
	Offer    *AssistantOfferView `json:"offer,omitempty"`
	CreateAt int64               `json:"create_at"`
	UpdateAt int64               `json:"update_at"`
}

// AssistantFeed is a person's feed in a channel.
type AssistantFeed struct {
	ChannelId string `json:"channel_id"`
	// Enabled tells whether the assistant works in this channel (the server's and the community's settings); On is
	// the person's own switch for it.
	Enabled bool             `json:"enabled"`
	On      bool             `json:"on"`
	Engine  string           `json:"engine"`
	Cards   []*AssistantCard `json:"cards"`
}

// AssistantPresenceRequest says that a person's panel has the assistant open on a channel, or closed it.
type AssistantPresenceRequest struct {
	Open bool `json:"open"`
}

// AssistantPresence answers it: whether the assistant now follows the conversation for them, and when to say it
// again.
type AssistantPresence struct {
	Following        bool `json:"following"`
	TTLSeconds       int  `json:"ttl_seconds"`
	HeartbeatSeconds int  `json:"heartbeat_seconds"`
}

// AssistantChannelSwitch is a person's switch for a channel.
type AssistantChannelSwitch struct {
	On bool `json:"on"`
}

// AssistantPinRequest pins or unpins a card.
type AssistantPinRequest struct {
	Pinned bool `json:"pinned"`
}

// AssistantMutedTools are the tools a person doesn't want suggested any more.
type AssistantMutedTools struct {
	Tools []string `json:"tools"`
}

// IsValid checks a list of muted tools.
func (m *AssistantMutedTools) IsValid() *AppError {
	if len(m.Tools) > AssistantMaxMutedTools {
		return NewAppError("AssistantMutedTools.IsValid", "model.assistant.muted_tools.app_error", nil, "too many", http.StatusBadRequest)
	}
	for _, t := range m.Tools {
		if t == "" || len(t) > AIAgentToolNameMaxLength || strings.ContainsAny(t, " \t\r\n") {
			return NewAppError("AssistantMutedTools.IsValid", "model.assistant.muted_tools.app_error", nil, "", http.StatusBadRequest)
		}
	}
	return nil
}

// ParseAssistantMutedTools reads a person's muted tools from their preference; a value that isn't valid counts as
// none.
func ParseAssistantMutedTools(value string) *AssistantMutedTools {
	muted := &AssistantMutedTools{Tools: []string{}}
	if value == "" {
		return muted
	}
	if err := json.Unmarshal([]byte(value), muted); err != nil || muted.IsValid() != nil {
		return &AssistantMutedTools{Tools: []string{}}
	}
	if muted.Tools == nil {
		muted.Tools = []string{}
	}
	return muted
}

// Mutes tells whether a tool is muted.
func (m *AssistantMutedTools) Mutes(tool string) bool {
	return m != nil && slices.Contains(m.Tools, tool)
}

// AssistantRunResult is what Run gives: the card, with its offer done, and whether the action ran (actions run from
// stage 3; until then the offer is only claimed and closed).
type AssistantRunResult struct {
	Card     *AssistantCard `json:"card"`
	Executed bool           `json:"executed"`
}

// isValidAssistantPreference checks a preference of the assistant's category.
func isValidAssistantPreference(p *Preference) bool {
	if p.Name == PreferenceNameAssistantMutedTools {
		var muted AssistantMutedTools
		return json.Unmarshal([]byte(p.Value), &muted) == nil && muted.IsValid() == nil
	}
	return IsValidId(p.Name) && (p.Value == AssistantChannelOn || p.Value == AssistantChannelOff)
}
