// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

// Antimatter: a team's autolinks turn patterns of its messages into links as they're posted, as the community
// Autolink plugin did (github.com/mattermost-community/mattermost-plugin-autolink, Apache License 2.0), whose
// message processing this follows.

import (
	"sync"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/markdown"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

type compiledTeamAutolinks struct {
	raw    string
	onEdit bool
	links  []*model.CompiledAutolink
}

// autolinkCache keeps the compiled autolinks of each team, until the team's change.
var autolinkCache sync.Map

func compileTeamAutolinks(raw string) *compiledTeamAutolinks {
	compiled := &compiledTeamAutolinks{raw: raw}
	links, err := model.ParseTeamAutolinks(raw)
	if err != nil {
		return compiled
	}
	compiled.onEdit = links.OnEdit
	for _, l := range links.Links {
		if l == nil {
			continue
		}
		if c, err := l.Compile(); err == nil && c != nil {
			compiled.links = append(compiled.links, c)
		}
	}
	return compiled
}

func teamAutolinks(team *model.Team) *compiledTeamAutolinks {
	if team.Autolinks == "" {
		return nil
	}
	if cached, ok := autolinkCache.Load(team.Id); ok && cached.(*compiledTeamAutolinks).raw == team.Autolinks {
		return cached.(*compiledTeamAutolinks)
	}
	compiled := compileTeamAutolinks(team.Autolinks)
	autolinkCache.Store(team.Id, compiled)
	return compiled
}

// ApplyAutolinks rewrites a message with a set of autolinks, for a channel: only its text, never inside code,
// links or images. isBot leaves the message to the rules that apply to bots.
func ApplyAutolinks(message string, links []*model.CompiledAutolink, channelID string, isBot bool) string {
	if len(links) == 0 || message == "" {
		return message
	}
	offset := 0
	result := message
	markdown.Inspect(message, func(node any) bool {
		if node == nil {
			return false
		}
		start, end := 0, 0
		switch node := node.(type) {
		// Never inside the text of a link or an image.
		case *markdown.InlineLink, *markdown.InlineImage, *markdown.ReferenceLink, *markdown.ReferenceImage:
			return false
		case *markdown.Autolink:
			start, end = node.RawDestination.Position, node.RawDestination.End
			if raw := message[start:end]; markdown.Unescape(raw) != raw {
				return true
			}
		case *markdown.Text:
			start, end = node.Range.Position, node.Range.End
			if node.Text != message[start:end] {
				// The parsed text doesn't match the message (escapes...): leave it.
				return true
			}
		default:
			return true
		}
		original := message[start:end]
		processed := original
		for _, link := range links {
			if !link.AppliesTo(channelID) || (isBot && !link.Rule.ProcessBotPosts) {
				continue
			}
			processed = link.Replace(processed)
		}
		if processed != original {
			result = result[:start+offset] + processed + result[end+offset:]
			offset += len(processed) - len(original)
		}
		return true
	})
	return result
}

// applyTeamAutolinks rewrites a post with its team's autolinks, when the team has some (and, for an edit, applies
// them to edits).
func (a *App) applyTeamAutolinks(rctx request.CTX, post *model.Post, channel *model.Channel, edit bool) {
	if post.Message == "" || post.Type == model.PostTypeBurnOnRead || post.IsSystemMessage() {
		return
	}
	if channel == nil {
		var appErr *model.AppError
		if channel, appErr = a.GetChannel(rctx, post.ChannelId); appErr != nil {
			return
		}
	}
	if channel.TeamId == "" {
		return
	}
	team, appErr := a.GetTeam(channel.TeamId)
	if appErr != nil {
		return
	}
	links := teamAutolinks(team)
	if links == nil || len(links.links) == 0 || (edit && !links.onEdit) {
		return
	}
	isBot := false
	if user, appErr := a.GetUser(rctx, post.UserId); appErr == nil {
		isBot = user.IsBot
	} else {
		rctx.Logger().Debug("Autolinks: could not tell whether the author is a bot", mlog.Err(appErr))
	}
	message := ApplyAutolinks(post.Message, links.links, channel.Id, isBot)
	if message != post.Message {
		post.Message = message
		post.Hashtags, _ = model.ParseHashtags(message)
	}
}

// PreviewTeamAutolinks shows what a set of autolinks makes of a text, for the team settings.
func PreviewTeamAutolinks(links *model.TeamAutolinks, text, channelID string) string {
	var compiled []*model.CompiledAutolink
	for _, l := range links.Links {
		if l == nil {
			continue
		}
		if c, err := l.Compile(); err == nil && c != nil {
			compiled = append(compiled, c)
		}
	}
	return ApplyAutolinks(text, compiled, channelID, false)
}
