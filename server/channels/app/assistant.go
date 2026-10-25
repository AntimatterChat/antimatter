// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package app

import (
	"encoding/json"
	"math"
	"net/http"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

// The assistant (Antimatter, stage 1 of the ambient assistant, AI_ASSISTANT_DESIGN.md): a personal feed of cards for
// the conversation a person has the assistant open on.
//
//   - A panel says the assistant is open on a channel (presence, with a heartbeat). New messages in a channel where
//     someone has it open are gathered for a few seconds (assistant_run.go), then each person viewing gets their own
//     candidates from the engine (rules, assistant_rules.go, or model, assistant_model.go), with the tools they may
//     use: the server's allowlist, minus what they turned off or muted.
//   - An insight (a tool that only reads) runs as that person, with their permissions and their own sign-in to other
//     services, never as a bot or the message's author, and becomes a card in their feed only.
//   - An action (a tool that changes something) never runs on its own: it's offered to one person at a time, first
//     the author of the message, and claimed server-side (assistant_offer.go).
//   - Nothing is posted and nothing notifies: cards reach their owner's clients over the websocket only.
//
// Everything is kept in the key-value store, in the namespace antimatter:assistant: the feeds (per person and
// channel, for a week), who has the panel open, the offers and their claims (compare-and-set, so two nodes can't both
// hand one out), the hourly budgets and the communities' settings. An app's ID can't have a colon, so no installed
// app can read or write this namespace (people's cards carry what their tools gave them).

const (
	assistantKVNamespace = "antimatter:assistant"

	assistantFeedPrefix      = "feed_"
	assistantPresencePrefix  = "presence_"
	assistantOfferPrefix     = "offer_"
	assistantOfferKeyPrefix  = "offerkey_"
	assistantBudgetPrefix    = "budget_"
	assistantCommunityPrefix = "community_"

	assistantFeedTTL        = 7 * 24 * time.Hour
	assistantDedupeWindow   = 10 * time.Minute
	assistantOfferTTL       = 24 * time.Hour
	assistantPresenceCached = 2 * time.Second
	assistantCASRetries     = 8
)

// assistantState is the assistant's state on a node: the channels' windows being gathered, a short cache of who has
// the panel open, the dictionaries, the compiled patterns and the offers' timers.
type assistantState struct {
	mu       sync.Mutex
	stopped  bool
	windows  map[string]*assistantWindow
	presence map[string]assistantPresenceCache
	dicts    map[string]*assistantDictionaryCache
	patterns map[string][]*assistantCompiledPattern
	timers   map[string]*time.Timer

	// publishHook sees every event the assistant sends (tests).
	publishHook func(*model.WebSocketEvent)
}

type assistantPresenceCache struct {
	viewers []assistantViewer
	fetched time.Time
}

func (a *App) assistant() *assistantState {
	return &a.Srv().ai.assistant
}

// stop drops the windows and timers of a node that shuts down.
func (s *assistantState) stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopped = true
	for _, w := range s.windows {
		w.timer.Stop()
	}
	for _, t := range s.timers {
		t.Stop()
	}
	s.windows, s.timers = nil, nil
}

// assistantConfig is the assistant's settings for a community ("" for direct and group messages): the server's,
// with what the community's admins changed when the server lets them.
type assistantConfig struct {
	enabled             bool
	engine              string
	connectionID        string
	model               string
	threshold           float64
	decisionsPerUser    int
	decisionsPerChannel int
	cardsPerUser        int
	appWindows          int
	triggeringApps      []string
	debounce            time.Duration
	offerTimeout        time.Duration
	tools               *model.AIAgentToolPolicy
	patterns            []*assistantCompiledPattern
	seeds               []model.AssistantEntity
	sources             []model.AssistantDictionarySource
}

func (a *App) assistantConfigFor(teamID string) *assistantConfig {
	s := &a.Config().AssistantSettings
	cfg := &assistantConfig{
		enabled:             model.SafeDereference(s.Enable),
		engine:              model.SafeDereference(s.Engine),
		connectionID:        model.SafeDereference(s.ConnectionId),
		model:               model.SafeDereference(s.Model),
		threshold:           float64(model.SafeDereference(s.ThresholdPercent)) / 100,
		decisionsPerUser:    model.SafeDereference(s.DecisionsPerUserHour),
		decisionsPerChannel: model.SafeDereference(s.DecisionsPerChannelHour),
		cardsPerUser:        model.SafeDereference(s.CardsPerUserHour),
		appWindows:          model.SafeDereference(s.AppWindowsPerChannelHour),
		triggeringApps:      slices.Clone(s.TriggeringApps),
		debounce:            time.Duration(model.SafeDereference(s.DebounceSeconds)) * time.Second,
		offerTimeout:        time.Duration(model.SafeDereference(s.OfferTimeoutSeconds)) * time.Second,
		tools:               &model.AIAgentToolPolicy{Enabled: true, Allowed: slices.Clone(s.AllowedTools)},
	}
	if cfg.offerTimeout <= 0 {
		cfg.offerTimeout = 2 * time.Minute
	}
	if cfg.enabled && teamID != "" && model.SafeDereference(s.AllowCommunityOverride) {
		if cs := a.assistantCommunitySettings(teamID); cs != nil {
			if cs.Enabled != nil && !*cs.Enabled {
				cfg.enabled = false
			}
			if cs.Engine != "" {
				cfg.engine = cs.Engine
			}
			if cs.ConnectionId != "" {
				cfg.connectionID, cfg.model = cs.ConnectionId, cs.Model
			}
		}
	}
	cfg.patterns = a.assistantPatterns(s.Patterns)
	cfg.seeds, _ = model.ParseAssistantSeedEntities(s.SeedEntities)
	cfg.sources, _ = model.ParseAssistantDictionarySources(s.DictionarySources)
	if cfg.sources == nil {
		cfg.sources = assistantDefaultDictionarySources
	}
	return cfg
}

// assistantPatterns gives the compiled patterns of the settings (the built-in ones when they have none), compiled
// once per node.
func (a *App) assistantPatterns(raw json.RawMessage) []*assistantCompiledPattern {
	key := string(raw)
	st := a.assistant()
	st.mu.Lock()
	defer st.mu.Unlock()
	if compiled, ok := st.patterns[key]; ok {
		return compiled
	}
	patterns, appErr := model.ParseAssistantPatterns(raw)
	if appErr != nil || patterns == nil {
		patterns = assistantDefaultPatterns()
	}
	compiled := compileAssistantPatterns(patterns)
	if st.patterns == nil {
		st.patterns = map[string][]*assistantCompiledPattern{}
	}
	st.patterns[key] = compiled
	return compiled
}

// assistantKVUpdate changes a key with compare-and-set, retrying when someone else changed it meanwhile. update
// gives the new value (nil deletes it) and whether to write at all.
func (a *App) assistantKVUpdate(rctx request.CTX, key string, ttl time.Duration, update func(old []byte) ([]byte, bool)) (bool, *model.AppError) {
	for range assistantCASRetries {
		old, appErr := a.GetPluginKey(assistantKVNamespace, key)
		if appErr != nil {
			return false, appErr
		}
		value, write := update(old)
		if !write {
			return false, nil
		}
		var ok bool
		switch {
		case value == nil && old == nil:
			return true, nil
		case value == nil:
			ok, appErr = a.CompareAndDeletePluginKey(rctx, assistantKVNamespace, key, old)
		default:
			ok, appErr = a.SetPluginKeyWithOptions(assistantKVNamespace, key, value, model.PluginKVSetOptions{Atomic: true, OldValue: old, ExpireInSeconds: int64(ttl / time.Second)})
		}
		if appErr != nil {
			return false, appErr
		}
		if ok {
			return true, nil
		}
	}
	return false, model.NewAppError("assistantKVUpdate", "app.assistant.busy.app_error", nil, "key="+key, http.StatusConflict)
}

// assistantKVInsert stores a key only if it isn't there: it tells whether this call stored it.
func (a *App) assistantKVInsert(key string, value []byte, ttl time.Duration) (bool, *model.AppError) {
	return a.SetPluginKeyWithOptions(assistantKVNamespace, key, value, model.PluginKVSetOptions{Atomic: true, ExpireInSeconds: int64(ttl / time.Second)})
}

// assistantTakeBudget counts one more use of an hourly budget, unless it's spent: it tells whether there was some
// left. The count is kept for the whole cluster.
func (a *App) assistantTakeBudget(rctx request.CTX, kind, id string, limit int) bool {
	if limit <= 0 {
		return false
	}
	key := assistantBudgetPrefix + kind + "_" + id + "_" + time.Now().UTC().Format("2006010215")
	taken := false
	_, appErr := a.assistantKVUpdate(rctx, key, 2*time.Hour, func(old []byte) ([]byte, bool) {
		count := 0
		if old != nil {
			_ = json.Unmarshal(old, &count)
		}
		if count >= limit {
			taken = false
			return nil, false
		}
		taken = true
		out, _ := json.Marshal(count + 1)
		return out, true
	})
	if appErr != nil {
		rctx.Logger().Debug("The assistant couldn't count its budget", mlog.String("kind", kind), mlog.Err(appErr))
		return false
	}
	return taken
}

// assistantBudgetLeft tells whether an hourly budget has some left, without using it.
func (a *App) assistantBudgetLeft(kind, id string, limit int) bool {
	if limit <= 0 {
		return false
	}
	raw, _ := a.GetPluginKey(assistantKVNamespace, assistantBudgetPrefix+kind+"_"+id+"_"+time.Now().UTC().Format("2006010215"))
	count := 0
	if raw != nil {
		_ = json.Unmarshal(raw, &count)
	}
	return count < limit
}

// assistantCommunitySettings gives what a community's admins changed, nil for nothing.
func (a *App) assistantCommunitySettings(teamID string) *model.AssistantCommunitySettings {
	raw, appErr := a.GetPluginKey(assistantKVNamespace, assistantCommunityPrefix+teamID)
	if appErr != nil || raw == nil {
		return nil
	}
	var s model.AssistantCommunitySettings
	if json.Unmarshal(raw, &s) != nil || s.IsValid() != nil {
		return nil
	}
	return &s
}

// GetAssistantCommunitySettings gives a community's settings, for its admins.
func (a *App) GetAssistantCommunitySettings(teamID string) *model.AssistantCommunitySettingsInfo {
	s := &a.Config().AssistantSettings
	info := &model.AssistantCommunitySettingsInfo{
		TeamId:        teamID,
		ServerEnabled: model.SafeDereference(s.Enable),
		MayOverride:   model.SafeDereference(s.AllowCommunityOverride),
	}
	if cs := a.assistantCommunitySettings(teamID); cs != nil {
		info.Settings = *cs
	}
	return info
}

// SetAssistantCommunitySettings changes a community's settings, when the server lets its admins: its decision model
// must be one the community may use (its own connection, or a server-wide one shared with it).
func (a *App) SetAssistantCommunitySettings(rctx request.CTX, teamID string, settings *model.AssistantCommunitySettings) (*model.AssistantCommunitySettingsInfo, *model.AppError) {
	const where = "SetAssistantCommunitySettings"
	s := &a.Config().AssistantSettings
	if !model.SafeDereference(s.AllowCommunityOverride) {
		return nil, model.NewAppError(where, "app.assistant.community_override.app_error", nil, "", http.StatusForbidden)
	}
	if appErr := settings.IsValid(); appErr != nil {
		return nil, appErr
	}
	if settings.ConnectionId != "" {
		c, appErr := a.GetAIConnection(rctx, settings.ConnectionId)
		if appErr != nil || !(c.Scope == teamID || c.IsServerScoped() && c.SharedWithTeam(teamID)) {
			return nil, model.NewAppError(where, "app.assistant.community_connection.app_error", nil, "connection_id="+settings.ConnectionId, http.StatusBadRequest)
		}
	}
	data, err := json.Marshal(settings)
	if err != nil {
		return nil, model.NewAppError(where, "app.assistant.save.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	if appErr := a.SetPluginKey(assistantKVNamespace, assistantCommunityPrefix+teamID, data); appErr != nil {
		return nil, appErr
	}
	return a.GetAssistantCommunitySettings(teamID), nil
}

// assistantChannelFor checks that a person may have the assistant in a channel: an active person (not a bot) who
// may read it, and a member of it when it's private, a direct or a group message. Their own switch isn't looked at.
func (a *App) assistantChannelFor(rctx request.CTX, userID, channelID string) (*model.User, *model.Channel, *model.AppError) {
	const where = "assistantChannelFor"
	forbidden := func(detail string) *model.AppError {
		return model.NewAppError(where, "app.assistant.access.app_error", nil, detail, http.StatusForbidden)
	}
	user, appErr := a.GetUser(rctx, userID)
	if appErr != nil || user.DeleteAt != 0 || user.IsBot {
		return nil, nil, forbidden("user")
	}
	channel, appErr := a.GetChannel(rctx, channelID)
	if appErr != nil {
		return nil, nil, forbidden("channel")
	}
	if channel.Type != model.ChannelTypeOpen {
		if member, appErr := a.GetChannelMember(rctx, channel.Id, user.Id); appErr != nil || member == nil {
			return nil, nil, forbidden("not a member")
		}
	}
	if ok, _ := a.HasPermissionToChannel(rctx, user.Id, channel.Id, model.PermissionReadChannelContent); !ok {
		return nil, nil, forbidden("no read access")
	}
	return user, channel, nil
}

// assistantChannelOn tells whether a person keeps the assistant on in a channel (on unless they turned it off).
func (a *App) assistantChannelOn(userID, channelID string) bool {
	pref, err := a.Srv().Store().Preference().Get(userID, model.PreferenceCategoryAssistant, channelID)
	return err != nil || pref.Value != model.AssistantChannelOff
}

// assistantMutedTools gives the tools a person doesn't want suggested.
func (a *App) assistantMutedTools(userID string) *model.AssistantMutedTools {
	pref, err := a.Srv().Store().Preference().Get(userID, model.PreferenceCategoryAssistant, model.PreferenceNameAssistantMutedTools)
	if err != nil {
		return &model.AssistantMutedTools{Tools: []string{}}
	}
	return model.ParseAssistantMutedTools(pref.Value)
}

// GetAssistantMutedTools gives a person's muted tools.
func (a *App) GetAssistantMutedTools(userID string) *model.AssistantMutedTools {
	return a.assistantMutedTools(userID)
}

// SetAssistantMutedTools replaces a person's muted tools.
func (a *App) SetAssistantMutedTools(rctx request.CTX, userID string, muted *model.AssistantMutedTools) (*model.AssistantMutedTools, *model.AppError) {
	if muted.Tools == nil {
		muted.Tools = []string{}
	}
	slices.Sort(muted.Tools)
	muted.Tools = slices.Compact(muted.Tools)
	if appErr := muted.IsValid(); appErr != nil {
		return nil, appErr
	}
	data, _ := json.Marshal(muted)
	if appErr := a.UpdatePreferences(rctx, userID, model.Preferences{{UserId: userID, Category: model.PreferenceCategoryAssistant, Name: model.PreferenceNameAssistantMutedTools, Value: string(data)}}); appErr != nil {
		return nil, appErr
	}
	return muted, nil
}

// --- Presence: who has the assistant open on a channel.

type assistantPresenceEntry struct {
	OpenedAt int64 `json:"opened_at"`
	SeenAt   int64 `json:"seen_at"`
}

// assistantViewer is a person with the assistant open on a channel, and since when.
type assistantViewer struct {
	userID   string
	openedAt int64
}

func assistantPresenceKey(channelID string) string {
	return assistantPresencePrefix + channelID
}

func assistantAlivePresence(raw []byte, now int64) map[string]assistantPresenceEntry {
	entries := map[string]assistantPresenceEntry{}
	if raw != nil {
		_ = json.Unmarshal(raw, &entries)
	}
	for userID, e := range entries {
		if now-e.SeenAt > model.AssistantPresenceTTLSeconds*1000 {
			delete(entries, userID)
		}
	}
	return entries
}

func assistantViewersOf(entries map[string]assistantPresenceEntry) []assistantViewer {
	viewers := make([]assistantViewer, 0, len(entries))
	for userID, e := range entries {
		viewers = append(viewers, assistantViewer{userID: userID, openedAt: e.OpenedAt})
	}
	sort.Slice(viewers, func(i, j int) bool {
		if viewers[i].openedAt != viewers[j].openedAt {
			return viewers[i].openedAt < viewers[j].openedAt
		}
		return viewers[i].userID < viewers[j].userID
	})
	return viewers
}

// assistantViewers gives who has the assistant open on a channel, by who opened it first. cached takes the node's
// copy of the last two seconds (for the check after every post).
func (a *App) assistantViewers(channelID string, cached bool) []assistantViewer {
	st := a.assistant()
	if cached {
		st.mu.Lock()
		entry, ok := st.presence[channelID]
		st.mu.Unlock()
		if ok && time.Since(entry.fetched) < assistantPresenceCached {
			return entry.viewers
		}
	}
	raw, _ := a.GetPluginKey(assistantKVNamespace, assistantPresenceKey(channelID))
	viewers := assistantViewersOf(assistantAlivePresence(raw, model.GetMillis()))
	st.mu.Lock()
	if st.presence == nil {
		st.presence = map[string]assistantPresenceCache{}
	}
	st.presence[channelID] = assistantPresenceCache{viewers: viewers, fetched: time.Now()}
	st.mu.Unlock()
	return viewers
}

func (a *App) setAssistantPresenceEntry(rctx request.CTX, userID, channelID string, open bool) *model.AppError {
	now := model.GetMillis()
	_, appErr := a.assistantKVUpdate(rctx, assistantPresenceKey(channelID), 2*model.AssistantPresenceTTLSeconds*time.Second, func(old []byte) ([]byte, bool) {
		entries := assistantAlivePresence(old, now)
		if open {
			e, ok := entries[userID]
			if !ok {
				e.OpenedAt = now
			}
			e.SeenAt = now
			entries[userID] = e
		} else {
			if _, ok := entries[userID]; !ok && old == nil {
				return nil, false
			}
			delete(entries, userID)
		}
		if len(entries) == 0 {
			return nil, true
		}
		out, _ := json.Marshal(entries)
		return out, true
	})
	st := a.assistant()
	st.mu.Lock()
	delete(st.presence, channelID)
	st.mu.Unlock()
	return appErr
}

// SetAssistantPresence is a panel saying the assistant is open on a channel (again, as a heartbeat) or closed. It
// follows the conversation only where it's on (the server, the community, the person's switch) and the channel isn't
// archived.
func (a *App) SetAssistantPresence(rctx request.CTX, userID, channelID string, open bool) (*model.AssistantPresence, *model.AppError) {
	_, channel, appErr := a.assistantChannelFor(rctx, userID, channelID)
	if appErr != nil {
		return nil, appErr
	}
	answer := &model.AssistantPresence{TTLSeconds: model.AssistantPresenceTTLSeconds, HeartbeatSeconds: model.AssistantPresenceHeartbeatSeconds}
	cfg := a.assistantConfigFor(channel.TeamId)
	follows := open && cfg.enabled && channel.DeleteAt == 0 && a.assistantChannelOn(userID, channelID)
	if appErr := a.setAssistantPresenceEntry(rctx, userID, channelID, follows); appErr != nil {
		return nil, appErr
	}
	answer.Following = follows
	return answer, nil
}

// SetAssistantChannelOn is a person's switch for a channel. Turning it off forgets their feed there.
func (a *App) SetAssistantChannelOn(rctx request.CTX, userID, channelID string, on bool) *model.AppError {
	if _, _, appErr := a.assistantChannelFor(rctx, userID, channelID); appErr != nil {
		return appErr
	}
	value := model.AssistantChannelOn
	if !on {
		value = model.AssistantChannelOff
	}
	if appErr := a.UpdatePreferences(rctx, userID, model.Preferences{{UserId: userID, Category: model.PreferenceCategoryAssistant, Name: channelID, Value: value}}); appErr != nil {
		return appErr
	}
	if on {
		return nil
	}
	if appErr := a.setAssistantPresenceEntry(rctx, userID, channelID, false); appErr != nil {
		return appErr
	}
	return a.DeletePluginKey(assistantKVNamespace, assistantFeedKey(userID, channelID))
}

// --- Feeds: a person's cards in a channel.

type assistantStoredFeed struct {
	Cards []*model.AssistantCard `json:"cards"`
}

func assistantFeedKey(userID, channelID string) string {
	return assistantFeedPrefix + userID + "_" + channelID
}

func (a *App) assistantFeed(userID, channelID string) *assistantStoredFeed {
	feed := &assistantStoredFeed{}
	raw, appErr := a.GetPluginKey(assistantKVNamespace, assistantFeedKey(userID, channelID))
	if appErr == nil && raw != nil {
		_ = json.Unmarshal(raw, feed)
	}
	return feed
}

// prune drops the dismissed cards once they no longer stop the same card coming back, and the oldest cards past the
// most a feed keeps (pinned cards stay).
func (f *assistantStoredFeed) prune(now int64) {
	f.Cards = slices.DeleteFunc(f.Cards, func(c *model.AssistantCard) bool {
		return c.Status == model.AssistantCardStatusDismissed && !c.Pinned && now-c.UpdateAt > assistantDedupeWindow.Milliseconds()
	})
	excess := len(f.Cards) - model.AssistantFeedMaxCards
	for i := 0; i < len(f.Cards) && excess > 0; {
		if !f.Cards[i].Pinned {
			f.Cards = slices.Delete(f.Cards, i, i+1)
			excess--
			continue
		}
		i++
	}
}

// updateAssistantFeed changes a person's feed in a channel; change tells whether it changed anything.
func (a *App) updateAssistantFeed(rctx request.CTX, userID, channelID string, change func(f *assistantStoredFeed) bool) *model.AppError {
	_, appErr := a.assistantKVUpdate(rctx, assistantFeedKey(userID, channelID), assistantFeedTTL, func(old []byte) ([]byte, bool) {
		feed := &assistantStoredFeed{}
		if old != nil {
			_ = json.Unmarshal(old, feed)
		}
		if !change(feed) {
			return nil, false
		}
		feed.prune(model.GetMillis())
		out, err := json.Marshal(feed)
		if err != nil {
			return nil, false
		}
		return out, true
	})
	return appErr
}

// assistantFeedHas tells whether a feed already has a card like this one lately (or, for an action, of its offer).
func (f *assistantStoredFeed) has(dedupeKey, offerID string, now int64) bool {
	return slices.ContainsFunc(f.Cards, func(c *model.AssistantCard) bool {
		if offerID != "" && c.OfferId == offerID {
			return true
		}
		return c.DedupeKey == dedupeKey && now-c.CreateAt < assistantDedupeWindow.Milliseconds()
	})
}

// addAssistantCard puts a card in its owner's feed, unless one like it is there already, and tells its owner's
// clients. It tells whether the card was added.
func (a *App) addAssistantCard(rctx request.CTX, card *model.AssistantCard) bool {
	added := false
	appErr := a.updateAssistantFeed(rctx, card.UserId, card.ChannelId, func(f *assistantStoredFeed) bool {
		if f.has(card.DedupeKey, card.OfferId, card.CreateAt) {
			added = false
			return false
		}
		f.Cards = append(f.Cards, card)
		added = true
		return true
	})
	if appErr != nil {
		rctx.Logger().Warn("The assistant couldn't save a card", mlog.String("user_id", card.UserId), mlog.Err(appErr))
		return false
	}
	if added {
		a.publishAssistantCard(rctx, model.WebsocketEventAssistantCardAdded, card)
	}
	return added
}

// publishAssistant sends an event to one person's clients: never to a channel or a community.
func (a *App) publishAssistant(event model.WebsocketEventType, userID string, data map[string]any) {
	message := model.NewWebSocketEvent(event, "", "", userID, nil, "")
	for key, value := range data {
		message.Add(key, value)
	}
	message.GetBroadcast().ReliableClusterSend = true
	st := a.assistant()
	st.mu.Lock()
	hook := st.publishHook
	st.mu.Unlock()
	if hook != nil {
		hook(message)
	}
	a.Publish(message)
}

// publishAssistantCard tells a card's owner that it was added or changed, with its offer as they see it.
func (a *App) publishAssistantCard(rctx request.CTX, event model.WebsocketEventType, card *model.AssistantCard) {
	out := *card
	if out.OfferId != "" {
		out.Offer = a.assistantOfferView(out.OfferId, out.UserId)
	}
	a.publishAssistant(event, card.UserId, map[string]any{"channel_id": card.ChannelId, "card": &out})
}

// GetAssistantFeed gives a person's own feed in a channel: their cards (pinned first, then the newest), with each
// action's offer as they see it. Nobody can read another person's feed.
func (a *App) GetAssistantFeed(rctx request.CTX, userID, channelID string) (*model.AssistantFeed, *model.AppError) {
	_, channel, appErr := a.assistantChannelFor(rctx, userID, channelID)
	if appErr != nil {
		return nil, appErr
	}
	cfg := a.assistantConfigFor(channel.TeamId)
	feed := &model.AssistantFeed{
		ChannelId: channelID,
		Enabled:   cfg.enabled && channel.DeleteAt == 0,
		On:        a.assistantChannelOn(userID, channelID),
		Engine:    cfg.engine,
		Cards:     []*model.AssistantCard{},
	}
	stored := a.assistantFeed(userID, channelID)
	for _, card := range slices.Backward(stored.Cards) {
		c := *card
		if c.Status == model.AssistantCardStatusDismissed {
			continue
		}
		if c.OfferId != "" {
			a.refreshAssistantOffer(rctx, c.OfferId)
			c.Offer = a.assistantOfferView(c.OfferId, userID)
		}
		feed.Cards = append(feed.Cards, &c)
	}
	sort.SliceStable(feed.Cards, func(i, j int) bool { return feed.Cards[i].Pinned && !feed.Cards[j].Pinned })
	return feed, nil
}

// changeAssistantCard changes one of a person's own cards; someone else's is not found.
func (a *App) changeAssistantCard(rctx request.CTX, userID, channelID, cardID string, change func(c *model.AssistantCard) *model.AppError) (*model.AssistantCard, *model.AppError) {
	const where = "changeAssistantCard"
	if !model.IsValidId(cardID) {
		return nil, model.NewAppError(where, "app.assistant.card_not_found.app_error", nil, "", http.StatusNotFound)
	}
	if _, _, appErr := a.assistantChannelFor(rctx, userID, channelID); appErr != nil {
		return nil, appErr
	}
	var changed *model.AssistantCard
	var changeErr *model.AppError
	appErr := a.updateAssistantFeed(rctx, userID, channelID, func(f *assistantStoredFeed) bool {
		changed, changeErr = nil, nil
		for _, c := range f.Cards {
			if c.Id != cardID {
				continue
			}
			if changeErr = change(c); changeErr != nil {
				return false
			}
			c.UpdateAt = model.GetMillis()
			copied := *c
			changed = &copied
			return true
		}
		changeErr = model.NewAppError(where, "app.assistant.card_not_found.app_error", nil, "", http.StatusNotFound)
		return false
	})
	if changeErr != nil {
		return nil, changeErr
	}
	if appErr != nil {
		return nil, appErr
	}
	a.publishAssistantCard(rctx, model.WebsocketEventAssistantCardUpdated, changed)
	if changed.OfferId != "" {
		changed.Offer = a.assistantOfferView(changed.OfferId, userID)
	}
	return changed, nil
}

// findAssistantCard gives one of a person's own cards.
func (a *App) findAssistantCard(userID, channelID, cardID string) *model.AssistantCard {
	for _, c := range a.assistantFeed(userID, channelID).Cards {
		if c.Id == cardID {
			copied := *c
			return &copied
		}
	}
	return nil
}

// DismissAssistantCard hides one of a person's cards. Dismissing an action offered to them hands it to the next
// person.
func (a *App) DismissAssistantCard(rctx request.CTX, userID, channelID, cardID string) (*model.AssistantCard, *model.AppError) {
	card, appErr := a.changeAssistantCard(rctx, userID, channelID, cardID, func(c *model.AssistantCard) *model.AppError {
		c.Status = model.AssistantCardStatusDismissed
		c.Pinned = false
		return nil
	})
	if appErr != nil {
		return nil, appErr
	}
	if card.OfferId != "" {
		a.passAssistantOffer(rctx, card.OfferId, userID)
		card.Offer = a.assistantOfferView(card.OfferId, userID)
	}
	return card, nil
}

// PinAssistantCard pins or unpins one of a person's cards.
func (a *App) PinAssistantCard(rctx request.CTX, userID, channelID, cardID string, pinned bool) (*model.AssistantCard, *model.AppError) {
	return a.changeAssistantCard(rctx, userID, channelID, cardID, func(c *model.AssistantCard) *model.AppError {
		if c.Status == model.AssistantCardStatusDismissed {
			return model.NewAppError("PinAssistantCard", "app.assistant.card_not_found.app_error", nil, "dismissed", http.StatusNotFound)
		}
		c.Pinned = pinned
		return nil
	})
}

// MuteAssistantCardTool is "don't suggest this kind again": the card's tool is muted for the person (everywhere),
// and the card dismissed.
func (a *App) MuteAssistantCardTool(rctx request.CTX, userID, channelID, cardID string) (*model.AssistantCard, *model.AppError) {
	if _, _, appErr := a.assistantChannelFor(rctx, userID, channelID); appErr != nil {
		return nil, appErr
	}
	card := a.findAssistantCard(userID, channelID, cardID)
	if card == nil {
		return nil, model.NewAppError("MuteAssistantCardTool", "app.assistant.card_not_found.app_error", nil, "", http.StatusNotFound)
	}
	muted := a.assistantMutedTools(userID)
	if !muted.Mutes(card.Tool) {
		muted.Tools = append(muted.Tools, card.Tool)
		if _, appErr := a.SetAssistantMutedTools(rctx, userID, muted); appErr != nil {
			return nil, appErr
		}
	}
	return a.DismissAssistantCard(rctx, userID, channelID, cardID)
}

// PostAssistantCard posts an insight card's content to its channel as the person, if they may post there.
func (a *App) PostAssistantCard(rctx request.CTX, userID, channelID, cardID string) (*model.Post, *model.AppError) {
	const where = "PostAssistantCard"
	user, channel, appErr := a.assistantChannelFor(rctx, userID, channelID)
	if appErr != nil {
		return nil, appErr
	}
	card := a.findAssistantCard(userID, channelID, cardID)
	if card == nil || card.Status == model.AssistantCardStatusDismissed {
		return nil, model.NewAppError(where, "app.assistant.card_not_found.app_error", nil, "", http.StatusNotFound)
	}
	if card.Kind != model.AssistantCardKindInsight || strings.TrimSpace(card.Result) == "" {
		return nil, model.NewAppError(where, "app.assistant.post_insight_only.app_error", nil, "", http.StatusBadRequest)
	}
	if ok, _ := a.HasPermissionToChannel(rctx, user.Id, channel.Id, model.PermissionCreatePost); !ok {
		return nil, model.NewAppError(where, "api.context.permissions.app_error", nil, "create_post", http.StatusForbidden)
	}

	post := &model.Post{UserId: user.Id, ChannelId: channel.Id}
	if attachments, summary, ok := assistantResultAttachments(card.Result); ok {
		// ASSISTANT_DISPLAY.md #4: a card whose result is the attachments format posts as a formatted message,
		// exactly like a bot's (props.attachments), sanitised: no actions, http(s) links only, text/fields/chart
		// capped, at most assistantPostAttachmentsMax attachments.
		post.Message = truncateRunes(summary, model.PostMessageMaxRunesV2)
		model.ParseMessageAttachment(post, attachments)
	} else {
		title := card.ToolTitle
		if title == "" {
			title = card.Tool
		}
		post.Message = truncateRunes("**"+title+"**\n"+card.Result, model.PostMessageMaxRunesV2)
	}
	post.AddProp(model.PostPropAssistantCard, card.Id)
	saved, _, appErr := a.CreatePostAsUser(rctx, post, rctx.Session().Id, false)
	if appErr != nil {
		return nil, appErr
	}
	if _, appErr := a.changeAssistantCard(rctx, userID, channelID, cardID, func(c *model.AssistantCard) *model.AppError {
		c.Status = model.AssistantCardStatusPosted
		return nil
	}); appErr != nil {
		rctx.Logger().Debug("The assistant couldn't mark a card as posted", mlog.Err(appErr))
	}
	return saved, nil
}

// The caps PostAssistantCard applies to the attachments format (ASSISTANT_DISPLAY.md #1/#4): a tool's result is a
// JSON payload an app made up, not something the person typed, so it gets no benefit of the doubt before it becomes
// a post everyone in the channel sees.
const (
	assistantPostAttachmentsMax    = 10
	assistantPostAttachmentTextMax = 2000
	assistantPostFieldsMax         = 20
	assistantPostFieldValueMax     = 300
	assistantPostChartValuesMax    = 60
)

// assistantCardResult is the JSON shape a tool's result may take (ASSISTANT_DISPLAY.md #1): the same
// message-attachment format integrations already post, plus the post's one-line message.
type assistantCardResult struct {
	Attachments []*model.MessageAttachment `json:"attachments"`
	Summary     string                     `json:"summary"`
}

// assistantResultAttachments reads a tool's result as the attachments format, sanitised for posting. ok is false
// for anything else (plain text, or JSON without attachments), which PostAssistantCard posts as it always has.
func assistantResultAttachments(result string) (attachments []*model.MessageAttachment, summary string, ok bool) {
	var parsed assistantCardResult
	if err := json.Unmarshal([]byte(result), &parsed); err != nil || len(parsed.Attachments) == 0 {
		return nil, "", false
	}
	shown := parsed.Attachments
	if len(shown) > assistantPostAttachmentsMax {
		shown = shown[:assistantPostAttachmentsMax]
	}
	sanitized := make([]*model.MessageAttachment, 0, len(shown))
	for _, attachment := range shown {
		if attachment == nil {
			continue
		}
		sanitized = append(sanitized, sanitizeAssistantAttachment(attachment))
	}
	return sanitized, parsed.Summary, true
}

// sanitizeAssistantAttachment drops what a formatted post never carries from the assistant (actions), keeps links
// to http(s) only, and caps text, fields and chart values.
func sanitizeAssistantAttachment(attachment *model.MessageAttachment) *model.MessageAttachment {
	clean := *attachment
	clean.Actions = nil

	if !model.IsValidHTTPURL(clean.TitleLink) {
		clean.TitleLink = ""
	}
	if !model.IsValidHTTPURL(clean.AuthorLink) {
		clean.AuthorLink = ""
	}
	if !model.IsValidHTTPURL(clean.ImageURL) {
		clean.ImageURL = ""
	}
	if !model.IsValidHTTPURL(clean.AuthorIcon) {
		clean.AuthorIcon = ""
	}
	if !model.IsValidHTTPURL(clean.FooterIcon) {
		clean.FooterIcon = ""
	}

	clean.Text = truncateRunes(clean.Text, assistantPostAttachmentTextMax)

	fields := clean.Fields
	if len(fields) > assistantPostFieldsMax {
		fields = fields[:assistantPostFieldsMax]
	}
	cleanFields := make([]*model.MessageAttachmentField, 0, len(fields))
	for _, field := range fields {
		if field == nil {
			continue
		}
		cleanField := *field
		if value, isString := cleanField.Value.(string); isString {
			cleanField.Value = truncateRunes(value, assistantPostFieldValueMax)
		}
		cleanFields = append(cleanFields, &cleanField)
	}
	clean.Fields = cleanFields

	clean.Chart = sanitizeAssistantChart(clean.Chart)

	return &clean
}

// sanitizeAssistantChart keeps at most assistantPostChartValuesMax finite values, the most recent ones (oldest
// first, as ASSISTANT_DISPLAY.md #1 specifies), and drops a non-finite threshold.
func sanitizeAssistantChart(chart *model.MessageAttachmentChart) *model.MessageAttachmentChart {
	if chart == nil {
		return nil
	}
	values := make([]float64, 0, len(chart.Values))
	for _, v := range chart.Values {
		if !math.IsNaN(v) && !math.IsInf(v, 0) {
			values = append(values, v)
		}
	}
	if len(values) > assistantPostChartValuesMax {
		values = values[len(values)-assistantPostChartValuesMax:]
	}
	clean := &model.MessageAttachmentChart{Values: values, Label: chart.Label}
	if chart.Threshold != nil && !math.IsNaN(*chart.Threshold) && !math.IsInf(*chart.Threshold, 0) {
		threshold := *chart.Threshold
		clean.Threshold = &threshold
	}
	return clean
}

// truncateRunes shortens s to at most maxRunes runes, as the rest of the assistant does for text going into a post.
func truncateRunes(s string, maxRunes int) string {
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	return string([]rune(s)[:maxRunes])
}
