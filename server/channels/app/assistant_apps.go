// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package app

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

// Trusted apps' posts (the owner's decision of 2026-10-25): a post of an app the settings name (TriggeringApps, the
// Alertmanager app by default) starts the assistant like a person's message, so that an alert gives the people
// viewing a card about the host it names.
//
//   - It counts only when its author is a bot whose owner, on the bot's record read here (never a prop), is one of
//     those apps, and the app runs. The post came through the apps' API (from_plugin and from_bot, which only the
//     server sets), as a plain or attachment post, without an override of its name or icon. Incoming webhooks,
//     commands' responses, OAuth apps, other bots and system messages never count.
//   - The engines read its text and its attachments' pretext, title, text and fields, at most
//     AssistantAppTextMaxRunes. Only new posts count: an app updating its post (an alert that resolves) doesn't.
//   - An action from it has no author: it's offered to those who spoke after it in the window, then by who opened the
//     assistant first. Insights are each viewer's own, as for any message. A card's why names the app.
//   - Windows with an app's post are counted per channel and hour (AppWindowsPerChannelHour), so that an alert storm
//     can't flood the feeds: past the cap, the apps' posts are left out of the window (people's still count).

// assistantAppPostShape tells whether a message has the shape of an app's own post: through the apps' API by a bot
// account, a plain or attachment post, no webhook's, command's, OAuth app's, system or other server's message, no
// override of its author's name or icon, not posted from a card. Who the bot belongs to is checked on its record
// (assistantPostApp).
func assistantAppPostShape(post *model.Post) bool {
	if post == nil || (post.Type != model.PostTypeDefault && post.Type != model.PostTypeMessageAttachment) ||
		post.IsSystemMessage() || post.IsRemote() || post.DeleteAt != 0 {
		return false
	}
	if post.GetProp(model.PostPropsFromBot) != "true" || post.GetProp(model.PostPropsFromPlugin) != "true" {
		return false
	}
	for _, prop := range []string{
		model.PostPropsFromWebhook, model.PostPropsFromOAuthApp, PostPropAgentStreaming, model.PostPropAssistantCard,
		model.PostPropsOverrideUsername, model.PostPropsOverrideIconURL, model.PostPropsOverrideIconEmoji, model.PostPropsWebhookDisplayName,
	} {
		if post.GetProp(prop) != nil {
			return false
		}
	}
	return true
}

// assistantAppActive tells whether an app runs on this node.
func (a *App) assistantAppActive(appID string) bool {
	env := a.GetPluginsEnvironment()
	return env != nil && env.IsActive(appID)
}

// assistantPostApp gives the app a message may start the assistant as: one of apps, running, whose bot (by the bot's
// record) wrote it. "" for any other message.
func (a *App) assistantPostApp(rctx request.CTX, post *model.Post, author *model.User, apps []string) string {
	if len(apps) == 0 || author == nil || !author.IsBot || author.DeleteAt != 0 || author.Id != post.UserId || !assistantAppPostShape(post) {
		return ""
	}
	bot, appErr := a.GetBot(rctx, post.UserId, false)
	if appErr != nil || bot.DeleteAt != 0 || !slices.Contains(apps, bot.OwnerId) || !a.assistantAppActive(bot.OwnerId) {
		return ""
	}
	return bot.OwnerId
}

// assistantAppPostText is what the engines read of an app's post: its text, then each attachment's pretext, title,
// text and fields ("title: value"), a line each, at most AssistantAppTextMaxRunes.
func assistantAppPostText(post *model.Post) string {
	var lines []string
	add := func(text string) {
		if text = strings.TrimSpace(text); text != "" {
			lines = append(lines, text)
		}
	}
	add(post.Message)
	for _, attachment := range post.Attachments() {
		if attachment == nil {
			continue
		}
		add(attachment.Pretext)
		add(attachment.Title)
		add(attachment.Text)
		for _, field := range attachment.Fields {
			if field == nil {
				continue
			}
			value := ""
			switch v := field.Value.(type) {
			case nil:
			case string:
				value = v
			default:
				value = fmt.Sprint(v)
			}
			value = strings.TrimSpace(value)
			if value == "" {
				continue
			}
			if title := strings.TrimSpace(field.Title); title != "" {
				value = title + ": " + value
			}
			add(value)
		}
	}
	text := strings.Join(lines, "\n")
	if utf8.RuneCountInString(text) > model.AssistantAppTextMaxRunes {
		text = string([]rune(text)[:model.AssistantAppTextMaxRunes])
	}
	return text
}
