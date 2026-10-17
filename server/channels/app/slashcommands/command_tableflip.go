// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package slashcommands

import (
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/i18n"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/app"
)

// TableflipProvider (/tableflip) and UnflipProvider (/unflip) are Antimatter's easter egg
// commands, built like /shrug: they append the table flip emoticon (or its
// "put the table back" answer) to an optional message.
type TableflipProvider struct {
}

type UnflipProvider struct {
}

const (
	CmdTableflip = "tableflip"
	CmdUnflip    = "unflip"

	// The emoticons contain no Markdown syntax (the lone "_" is surrounded
	// by spaces, so it can't start emphasis), so they are sent as they are.
	TableflipText = "(╯°□°)╯︵ ┻━┻"
	UnflipText    = "┬─┬ノ( º _ ºノ)"
)

func init() {
	app.RegisterCommandProvider(&TableflipProvider{})
	app.RegisterCommandProvider(&UnflipProvider{})
}

func appendEmoticon(message, emoticon string) string {
	if message == "" {
		return emoticon
	}
	return message + " " + emoticon
}

func (*TableflipProvider) GetTrigger() string {
	return CmdTableflip
}

func (*TableflipProvider) GetCommand(a *app.App, T i18n.TranslateFunc) *model.Command {
	return &model.Command{
		Trigger:          CmdTableflip,
		AutoComplete:     true,
		AutoCompleteDesc: T("api.command_tableflip.desc"),
		AutoCompleteHint: T("api.command_tableflip.hint"),
		DisplayName:      T("api.command_tableflip.name"),
	}
}

func (*TableflipProvider) DoCommand(a *app.App, rctx request.CTX, args *model.CommandArgs, message string) *model.CommandResponse {
	return &model.CommandResponse{ResponseType: model.CommandResponseTypeInChannel, Text: appendEmoticon(message, TableflipText)}
}

func (*UnflipProvider) GetTrigger() string {
	return CmdUnflip
}

func (*UnflipProvider) GetCommand(a *app.App, T i18n.TranslateFunc) *model.Command {
	return &model.Command{
		Trigger:          CmdUnflip,
		AutoComplete:     true,
		AutoCompleteDesc: T("api.command_unflip.desc"),
		AutoCompleteHint: T("api.command_unflip.hint"),
		DisplayName:      T("api.command_unflip.name"),
	}
}

func (*UnflipProvider) DoCommand(a *app.App, rctx request.CTX, args *model.CommandArgs, message string) *model.CommandResponse {
	return &model.CommandResponse{ResponseType: model.CommandResponseTypeInChannel, Text: appendEmoticon(message, UnflipText)}
}
