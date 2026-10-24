// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package app

import (
	"encoding/json"
	"slices"
	"sort"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

// Running the assistant: a person's new message in a channel where someone has the assistant open joins the
// channel's window; after a few seconds of quiet (at most three times that after its first message) the window runs,
// on the node that saved its messages, for each person viewing:
//
//   - only a person who may read the channel (a member, for private channels and direct and group messages), with
//     the assistant on there (the server, the community, their own switch), in a channel that isn't archived;
//   - with the tools they may use (the server's allowlist, minus what they turned off or muted), their dictionary and
//     their engine's candidates;
//   - actions first, offered to one person at a time (assistant_offer.go); then insights, each run as the person,
//     within their hourly budget of cards, and put in their own feed.

const assistantWindowMaxPosts = 20

// assistantCandidate is an engine's candidate with its tool, resolved for the person it was made for.
type assistantCandidate struct {
	model.AssistantCandidate
	tool *aiResolvedTool
}

// assistantWindow is a channel's new messages, gathered until the channel is quiet.
type assistantWindow struct {
	posts []string
	first time.Time
	timer *time.Timer
}

// queue adds a message to its channel's window: the window runs debounce after the last message, and at most three
// times that after the first.
func (st *assistantState) queue(channelID, postID string, debounce time.Duration, run func(channelID string, postIDs []string)) {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.stopped {
		return
	}
	if st.windows == nil {
		st.windows = map[string]*assistantWindow{}
	}
	if w, ok := st.windows[channelID]; ok {
		w.posts = append(w.posts, postID)
		if len(w.posts) > assistantWindowMaxPosts {
			w.posts = w.posts[len(w.posts)-assistantWindowMaxPosts:]
		}
		wait := debounce
		if remaining := 3*debounce - time.Since(w.first); remaining < wait {
			wait = max(remaining, 0)
		}
		w.timer.Reset(wait)
		return
	}
	w := &assistantWindow{posts: []string{postID}, first: time.Now()}
	w.timer = time.AfterFunc(debounce, func() {
		st.mu.Lock()
		if st.windows[channelID] != w || st.stopped {
			st.mu.Unlock()
			return
		}
		delete(st.windows, channelID)
		posts := slices.Clone(w.posts)
		st.mu.Unlock()
		run(channelID, posts)
	})
	st.windows[channelID] = w
}

// assistantPostCounts tells whether a message may start the assistant: a person's own message (not a system
// message, a webhook's, an app's, a bot's or another server's), not one posted from a card.
func assistantPostCounts(post *model.Post) bool {
	return postMayStartAgentTurn(post) && post.GetProp(model.PostPropAssistantCard) == nil
}

// maybeQueueAssistantPost puts a new message in its channel's window, when someone has the assistant open there.
func (a *App) maybeQueueAssistantPost(post *model.Post, channel *model.Channel) {
	s := a.Config().AISettings.Assistant
	if s == nil || !model.SafeDereference(s.Enable) || channel == nil || channel.DeleteAt != 0 || !assistantPostCounts(post) {
		return
	}
	if len(a.assistantViewers(channel.Id, true)) == 0 {
		return
	}
	debounce := time.Duration(model.SafeDereference(s.DebounceSeconds)) * time.Second
	a.assistant().queue(channel.Id, post.Id, debounce, func(channelID string, postIDs []string) {
		a.Srv().Go(func() {
			a.runAssistantWindow(request.EmptyContext(a.Log()), channelID, postIDs)
		})
	})
}

// assistantWindowPosts gives a window's messages that still count, oldest first, and their authors' usernames.
func (a *App) assistantWindowPosts(rctx request.CTX, channel *model.Channel, postIDs []string) ([]*model.Post, map[string]string) {
	var posts []*model.Post
	names := map[string]string{}
	for _, id := range postIDs {
		post, appErr := a.GetSinglePost(rctx, id, false)
		if appErr != nil || post.ChannelId != channel.Id || post.DeleteAt != 0 || !assistantPostCounts(post) {
			continue
		}
		if _, known := names[post.UserId]; !known {
			author, appErr := a.GetUser(rctx, post.UserId)
			if appErr != nil || author.IsBot || author.DeleteAt != 0 {
				continue
			}
			names[post.UserId] = author.Username
		}
		posts = append(posts, post)
	}
	sort.SliceStable(posts, func(i, j int) bool { return posts[i].CreateAt < posts[j].CreateAt })
	return posts, names
}

// assistantViewerFor gives a person viewing a channel when the assistant may work for them there.
func (a *App) assistantViewerFor(rctx request.CTX, userID string, channel *model.Channel) *model.User {
	user, _, appErr := a.assistantChannelFor(rctx, userID, channel.Id)
	if appErr != nil || !a.assistantChannelOn(userID, channel.Id) {
		return nil
	}
	return user
}

// assistantToolsFor gives the tools the assistant may use for a person in a channel: those of the registry they may
// use there (the server's, the running apps', the MCP servers available in the channel's community they may use and
// signed in to), that the server's allowlist names, minus what they turned off for agents or muted for the
// assistant. A community's MCP servers count in its own channels only.
func (a *App) assistantToolsFor(rctx request.CTX, user *model.User, channel *model.Channel, cfg *assistantConfig) []*aiResolvedTool {
	scope := model.AIAgentScopeServer
	if channel.TeamId != "" {
		scope = channel.TeamId
	}
	where := &model.AIAgent{Scope: scope}
	choices := a.aiToolChoicesFor(user.Id)
	muted := a.assistantMutedTools(user.Id)
	sources := a.aiToolSourcesFor(rctx, where, user, channel.TeamId, func(namespace, key string) bool {
		return aiPolicyMayAllowNamespace(cfg.tools, namespace) && !slices.Contains(choices.OffSources, key)
	})
	var tools []*aiResolvedTool
	for _, source := range sources {
		for _, rt := range source.tools {
			if cfg.tools.Allows(rt.tool) && choices.Allows(rt.tool) && !muted.Mutes(rt.tool.Name) {
				tools = append(tools, rt)
			}
		}
	}
	sort.Slice(tools, func(i, j int) bool { return tools[i].tool.Name < tools[j].tool.Name })
	return tools
}

// runAssistantTool runs a tool that reads, as the person, with their permissions and their own sign-in, and audits
// it. The assistant never runs any other tool on its own.
func (a *App) runAssistantTool(rctx request.CTX, user *model.User, channel *model.Channel, rt *aiResolvedTool, arguments json.RawMessage) (string, error) {
	if !rt.tool.IsRead() {
		return "", errAssistantNotARead
	}
	tc := &aiToolCall{callID: model.NewId(), user: user, channel: channel, teamID: channel.TeamId, arguments: arguments}
	started := time.Now()
	text, err := a.runResolvedAITool(rctx, tc, rt)
	a.auditAIToolCall(rctx, tc, rt.tool, aiToolDecisionAssistant, time.Since(started), err)
	return text, err
}

// newAssistantCard makes a person's card of a candidate.
func newAssistantCard(userID string, c *assistantCandidate) *model.AssistantCard {
	now := model.GetMillis()
	title := c.tool.tool.Title
	if title == "" {
		title = c.tool.tool.Name
	}
	return &model.AssistantCard{
		Id: model.NewId(), UserId: userID, ChannelId: c.ChannelId, Kind: c.Kind, Tool: c.Tool, ToolTitle: title,
		Arguments: c.Arguments, Why: c.Why, PostIds: c.PostIds, DedupeKey: c.DedupeKey,
		Status: model.AssistantCardStatusNew, CreateAt: now, UpdateAt: now,
	}
}

// makeAssistantInsight runs an insight's tool as the person and puts its result in their feed, unless they have the
// same card from the last ten minutes or their hourly budget of cards is spent.
func (a *App) makeAssistantInsight(rctx request.CTX, channel *model.Channel, user *model.User, c *assistantCandidate, cfg *assistantConfig) {
	if a.assistantFeed(user.Id, channel.Id).has(c.DedupeKey, "", model.GetMillis()) {
		return
	}
	if !a.assistantTakeBudget(rctx, "cards_user", user.Id, cfg.cardsPerUser) {
		return
	}
	text, err := a.runAssistantTool(rctx, user, channel, c.tool, c.Arguments)
	if err != nil {
		rctx.Logger().Debug("The assistant's tool failed", mlog.String("tool", c.Tool), mlog.String("user_id", user.Id), mlog.Err(err))
		return
	}
	card := newAssistantCard(user.Id, c)
	card.Result = shortenForAITool(text, model.AssistantResultMaxRunes)
	a.addAssistantCard(rctx, card)
}

// assistantViewerRun is one person's candidates in a window.
type assistantViewerRun struct {
	user       *model.User
	candidates []*assistantCandidate
}

// runAssistantWindow runs a channel's window for everyone viewing it.
func (a *App) runAssistantWindow(rctx request.CTX, channelID string, postIDs []string) {
	channel, appErr := a.GetChannel(rctx, channelID)
	if appErr != nil || channel.DeleteAt != 0 {
		return
	}
	cfg := a.assistantConfigFor(channel.TeamId)
	if !cfg.enabled {
		return
	}
	posts, names := a.assistantWindowPosts(rctx, channel, postIDs)
	if len(posts) == 0 {
		return
	}
	cache := assistantDecisionCache{}
	var runs []assistantViewerRun
	for _, v := range a.assistantViewers(channel.Id, false) {
		user := a.assistantViewerFor(rctx, v.userID, channel)
		if user == nil {
			continue
		}
		tools := a.assistantToolsFor(rctx, user, channel, cfg)
		rctx.Logger().Debug("The assistant looks at a window for someone", mlog.String("channel_id", channel.Id), mlog.String("user_id", user.Id), mlog.Int("tools", len(tools)))
		if len(tools) == 0 {
			continue
		}
		dict := a.assistantDictionaryFor(rctx, user, channel, tools, cfg)
		var candidates []*assistantCandidate
		if cfg.engine == model.AssistantEngineModel {
			candidates = a.assistantModelCandidates(rctx, user, channel, posts, names, tools, dict, cfg, cache)
		} else {
			seen := map[string]bool{}
			for _, post := range posts {
				for _, c := range assistantRuleCandidates(post, dict, cfg.patterns, tools) {
					if !seen[c.DedupeKey] {
						seen[c.DedupeKey] = true
						candidates = append(candidates, c)
					}
				}
			}
		}
		if len(candidates) > 0 {
			runs = append(runs, assistantViewerRun{user: user, candidates: candidates})
		}
	}

	offers := []string{}
	for _, run := range runs {
		for _, c := range run.candidates {
			if c.Kind != model.AssistantCardKindAction {
				continue
			}
			if id := a.offerAssistantAction(rctx, channel, run.user, c, posts, cfg); id != "" && !slices.Contains(offers, id) {
				offers = append(offers, id)
			}
		}
	}
	rctx.Logger().Debug("The assistant ran a window", mlog.String("channel_id", channel.Id), mlog.Int("posts", len(posts)), mlog.Int("people", len(runs)), mlog.Int("offers", len(offers)))
	for _, id := range offers {
		a.assignAssistantOffer(rctx, id)
	}
	for _, run := range runs {
		for _, c := range run.candidates {
			if c.Kind == model.AssistantCardKindInsight {
				a.makeAssistantInsight(rctx, channel, run.user, c, cfg)
			}
		}
	}
}
