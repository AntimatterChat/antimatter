// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package app

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

// The server's own tools (Antimatter), namespace "antimatter": they read and post as the person an agent acts for,
// checking that person's permissions on everything they touch (never the agent's: the agent's account is only the
// one that answers). A tool that can't find something the person may not read says it isn't there.

const (
	aiBuiltinSearchMax    = 20
	aiBuiltinChannelMax   = 50
	aiBuiltinThreadMax    = 50
	aiBuiltinMessageRunes = 1500
)

// aiToolCall is a tool's call for a person: who, through which agent, where, and the arguments.
type aiToolCall struct {
	callID    string
	agent     *model.AIAgent
	user      *model.User
	channel   *model.Channel
	teamID    string
	rootID    string
	arguments json.RawMessage
}

type aiBuiltinTool struct {
	tool *model.AITool
	run  func(a *App, rctx request.CTX, call *aiToolCall) (string, error)
}

func builtinAITool(name, title, description, kind, schema string, run func(a *App, rctx request.CTX, call *aiToolCall) (string, error)) *aiBuiltinTool {
	return &aiBuiltinTool{
		tool: &model.AITool{
			Name:        model.AIToolFullName(model.AIToolBuiltInNamespace, name),
			Namespace:   model.AIToolBuiltInNamespace,
			LocalName:   name,
			Source:      model.AIToolSourceBuiltIn,
			Title:       title,
			Description: description,
			InputSchema: json.RawMessage(schema),
			Kind:        kind,
		},
		run: run,
	}
}

// aiBuiltinTools are the server's own tools. They're set in init: the tools call into the app, which reaches back to
// this list, and a variable's initialiser may not.
var aiBuiltinTools []*aiBuiltinTool

func init() {
	aiBuiltinTools = builtinAITools()
}

func builtinAITools() []*aiBuiltinTool {
	return []*aiBuiltinTool{
		builtinAITool("search_messages", "Search messages",
			"Searches the messages the person can read, like the search box (terms, \"quoted phrases\", from:username, in:channel-name, before:/after:YYYY-MM-DD).",
			model.AIToolKindRead,
			`{"type":"object","properties":{"terms":{"type":"string","description":"What to search for."},"limit":{"type":"integer","minimum":1,"maximum":20}},"required":["terms"]}`,
			(*App).aiToolSearchMessages),
		builtinAITool("read_thread", "Read a thread",
			"Reads a thread (its first message and replies) the person can read, by the ID of any of its messages.",
			model.AIToolKindRead,
			`{"type":"object","properties":{"post_id":{"type":"string","description":"The ID of a message of the thread."}},"required":["post_id"]}`,
			(*App).aiToolReadThread),
		builtinAITool("read_channel", "Read a channel",
			"Reads the latest messages of a channel the person can read, by its ID or its name (in the current community).",
			model.AIToolKindRead,
			`{"type":"object","properties":{"channel_id":{"type":"string"},"channel_name":{"type":"string","description":"The channel's name, as in ~name."},"limit":{"type":"integer","minimum":1,"maximum":50}}}`,
			(*App).aiToolReadChannel),
		builtinAITool("lookup_user", "Look up a person",
			"Gives what the person may see of someone else: their name, position and whether they are a bot, by username.",
			model.AIToolKindRead,
			`{"type":"object","properties":{"username":{"type":"string"}},"required":["username"]}`,
			(*App).aiToolLookupUser),
		builtinAITool("post_message", "Post a message",
			"Posts a message as the person, in a channel they can post in (by ID or name, in the current community), optionally as a reply to a thread. The person confirms it first.",
			model.AIToolKindWrite,
			`{"type":"object","properties":{"channel_id":{"type":"string"},"channel_name":{"type":"string"},"message":{"type":"string"},"root_id":{"type":"string","description":"The first message of the thread to reply in."}},"required":["message"]}`,
			(*App).aiToolPostMessage),
	}
}

// findAIBuiltinTool gives one of the server's own tools by its full name.
func findAIBuiltinTool(name string) *aiBuiltinTool {
	for _, t := range aiBuiltinTools {
		if t.tool.Name == name {
			return t
		}
	}
	return nil
}

var errAIToolNotFound = errors.New("not found, or the person may not read it")

func aiToolArguments(raw json.RawMessage, out any) error {
	if len(raw) == 0 {
		raw = json.RawMessage(`{}`)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return errors.New("the arguments aren't valid: " + err.Error())
	}
	return nil
}

func clampAIToolLimit(limit, fallback, most int) int {
	if limit <= 0 {
		return fallback
	}
	return min(limit, most)
}

func shortenForAITool(text string, runes int) string {
	if utf8.RuneCountInString(text) <= runes {
		return text
	}
	return string([]rune(text)[:runes]) + "…"
}

// aiToolReadablePosts keeps the posts of channels the person may read, as an agent may read them.
func (a *App) aiToolReadablePosts(rctx request.CTX, user *model.User, posts []*model.Post) []*model.Post {
	readable := map[string]bool{}
	out := make([]*model.Post, 0, len(posts))
	for _, p := range posts {
		ok, seen := readable[p.ChannelId]
		if !seen {
			ok, _ = a.HasPermissionToChannel(rctx, user.Id, p.ChannelId, model.PermissionReadChannelContent)
			readable[p.ChannelId] = ok
		}
		if ok && postForAgentMaterial(p) {
			q := p.Clone()
			q.Message = shortenForAITool(q.Message, aiBuiltinMessageRunes)
			out = append(out, q)
		}
	}
	return out
}

func (a *App) aiToolSearchMessages(rctx request.CTX, call *aiToolCall) (string, error) {
	var args struct {
		Terms string `json:"terms"`
		Limit int    `json:"limit"`
	}
	if err := aiToolArguments(call.arguments, &args); err != nil {
		return "", err
	}
	terms := strings.TrimSpace(args.Terms)
	if terms == "" {
		return "", errors.New("terms are needed")
	}
	limit := clampAIToolLimit(args.Limit, 10, aiBuiltinSearchMax)
	results, _, appErr := a.SearchPostsForUser(rctx, terms, call.user.Id, call.teamID, false, false, 0, 0, limit)
	if appErr != nil {
		return "", errors.New("the search failed")
	}
	posts := []*model.Post{}
	if results != nil && results.PostList != nil {
		for _, id := range results.Order {
			if p, ok := results.Posts[id]; ok {
				posts = append(posts, p)
			}
		}
	}
	posts = a.aiToolReadablePosts(rctx, call.user, posts)
	if len(posts) == 0 {
		return "No messages found.", nil
	}
	channels := map[string]string{}
	var b strings.Builder
	for _, p := range posts {
		name, ok := channels[p.ChannelId]
		if !ok {
			if ch, appErr := a.GetChannel(rctx, p.ChannelId); appErr == nil {
				name = ch.Name
			}
			channels[p.ChannelId] = name
		}
		fmt.Fprintf(&b, "[message %s in ~%s]\n%s\n\n", p.Id, name, a.formatPostsForAgent(rctx, []*model.Post{p}))
	}
	return strings.TrimSpace(b.String()), nil
}

func (a *App) aiToolReadThread(rctx request.CTX, call *aiToolCall) (string, error) {
	var args struct {
		PostID string `json:"post_id"`
	}
	if err := aiToolArguments(call.arguments, &args); err != nil {
		return "", err
	}
	if !model.IsValidId(args.PostID) || !a.HasPermissionToChannelByPost(rctx, call.user.Id, args.PostID, model.PermissionReadChannelContent) {
		return "", errAIToolNotFound
	}
	post, appErr := a.GetSinglePost(rctx, args.PostID, false)
	if appErr != nil {
		return "", errAIToolNotFound
	}
	rootID := post.Id
	if post.RootId != "" {
		rootID = post.RootId
	}
	list, appErr := a.GetPostThread(rctx, rootID, model.GetPostsOptions{}, call.user.Id)
	if appErr != nil {
		return "", errAIToolNotFound
	}
	posts := []*model.Post{}
	for _, p := range list.Posts {
		if p.ChannelId == post.ChannelId {
			posts = append(posts, p)
		}
	}
	slices.SortFunc(posts, func(x, y *model.Post) int { return cmp.Compare(x.CreateAt, y.CreateAt) })
	if len(posts) > aiBuiltinThreadMax {
		posts = append(posts[:1], posts[len(posts)-aiBuiltinThreadMax+1:]...)
	}
	posts = a.aiToolReadablePosts(rctx, call.user, posts)
	if len(posts) == 0 {
		return "", errAIToolNotFound
	}
	return a.formatPostsForAgent(rctx, posts), nil
}

// aiToolChannel finds a channel by ID, or by name in the call's community; nil when the person may not have it with
// the permission.
func (a *App) aiToolChannel(rctx request.CTX, call *aiToolCall, channelID, channelName string, permission *model.Permission) *model.Channel {
	var channel *model.Channel
	switch {
	case channelID != "":
		if !model.IsValidId(channelID) {
			return nil
		}
		ch, appErr := a.GetChannel(rctx, channelID)
		if appErr != nil {
			return nil
		}
		channel = ch
	case channelName != "":
		name := strings.TrimPrefix(strings.TrimSpace(channelName), "~")
		if call.teamID == "" || !model.IsValidChannelIdentifier(name) {
			return nil
		}
		ch, appErr := a.GetChannelByName(rctx, name, call.teamID, false)
		if appErr != nil {
			return nil
		}
		channel = ch
	case call.channel != nil:
		channel = call.channel
	default:
		return nil
	}
	if channel.DeleteAt != 0 {
		return nil
	}
	if ok, _ := a.HasPermissionToChannel(rctx, call.user.Id, channel.Id, permission); !ok {
		return nil
	}
	return channel
}

func (a *App) aiToolReadChannel(rctx request.CTX, call *aiToolCall) (string, error) {
	var args struct {
		ChannelID   string `json:"channel_id"`
		ChannelName string `json:"channel_name"`
		Limit       int    `json:"limit"`
	}
	if err := aiToolArguments(call.arguments, &args); err != nil {
		return "", err
	}
	channel := a.aiToolChannel(rctx, call, args.ChannelID, args.ChannelName, model.PermissionReadChannelContent)
	if channel == nil {
		return "", errAIToolNotFound
	}
	limit := clampAIToolLimit(args.Limit, 30, aiBuiltinChannelMax)
	list, appErr := a.GetPostsPage(rctx, model.GetPostsOptions{UserId: call.user.Id, ChannelId: channel.Id, PerPage: limit, SkipFetchThreads: true})
	if appErr != nil {
		return "", errAIToolNotFound
	}
	posts := []*model.Post{}
	for _, p := range list.Posts {
		if p.ChannelId == channel.Id {
			posts = append(posts, p)
		}
	}
	slices.SortFunc(posts, func(x, y *model.Post) int { return cmp.Compare(x.CreateAt, y.CreateAt) })
	posts = a.aiToolReadablePosts(rctx, call.user, posts)
	header := fmt.Sprintf("Channel ~%s (%s)", channel.Name, channel.DisplayName)
	if len(posts) == 0 {
		return header + "\n(no messages)", nil
	}
	return header + "\n" + a.formatPostsForAgent(rctx, posts), nil
}

func (a *App) aiToolLookupUser(rctx request.CTX, call *aiToolCall) (string, error) {
	var args struct {
		Username string `json:"username"`
	}
	if err := aiToolArguments(call.arguments, &args); err != nil {
		return "", err
	}
	username := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(args.Username), "@"))
	if !model.IsValidUsername(username) {
		return "", errAIToolNotFound
	}
	other, appErr := a.GetUserByUsername(username)
	if appErr != nil || other.DeleteAt != 0 {
		return "", errAIToolNotFound
	}
	if ok, _ := a.UserCanSeeOtherUser(rctx, call.user.Id, other.Id); !ok {
		return "", errAIToolNotFound
	}
	lines := []string{"@" + other.Username}
	if model.SafeDereference(a.Config().PrivacySettings.ShowFullName) || other.Id == call.user.Id {
		if full := strings.TrimSpace(other.GetFullName()); full != "" {
			lines = append(lines, "Name: "+full)
		}
	}
	if other.Nickname != "" {
		lines = append(lines, "Nickname: "+other.Nickname)
	}
	if other.Position != "" {
		lines = append(lines, "Position: "+other.Position)
	}
	if other.IsBot {
		lines = append(lines, "A bot or an agent.")
	}
	return strings.Join(lines, "\n"), nil
}

func (a *App) aiToolPostMessage(rctx request.CTX, call *aiToolCall) (string, error) {
	if call.agent == nil {
		// Only an agent's confirmed call posts; the assistant never runs a write on its own.
		return "", errors.New("no agent posts this message")
	}
	var args struct {
		ChannelID   string `json:"channel_id"`
		ChannelName string `json:"channel_name"`
		Message     string `json:"message"`
		RootID      string `json:"root_id"`
	}
	if err := aiToolArguments(call.arguments, &args); err != nil {
		return "", err
	}
	message := strings.TrimSpace(args.Message)
	if message == "" || utf8.RuneCountInString(message) > model.PostMessageMaxRunesV2 {
		return "", errors.New("the message is empty or too long")
	}
	channel := a.aiToolChannel(rctx, call, args.ChannelID, args.ChannelName, model.PermissionCreatePost)
	if channel == nil {
		return "", errors.New("the person can't post in that channel")
	}
	if args.RootID != "" {
		root, appErr := a.GetSinglePost(rctx, args.RootID, false)
		if !model.IsValidId(args.RootID) || appErr != nil || root.ChannelId != channel.Id || root.DeleteAt != 0 {
			return "", errors.New("the thread isn't in that channel")
		}
		if root.RootId != "" {
			args.RootID = root.RootId
		}
	}
	post := &model.Post{UserId: call.user.Id, ChannelId: channel.Id, RootId: args.RootID, Message: message}
	post.AddProp(model.PostPropAgentPostedFor, call.agent.UserId)
	saved, _, appErr := a.CreatePostAsUser(rctx, post, "", false)
	if appErr != nil {
		return "", errors.New("the message couldn't be posted")
	}
	return fmt.Sprintf("Posted as @%s in ~%s: /pl/%s", call.user.Username, channel.Name, saved.Id), nil
}
