// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

const (
	PluginIdPlaybooks     = "playbooks"
	PluginIdFocalboard    = "focalboard"
	PluginIdApps          = "com.mattermost.apps"
	PluginIdCalls         = "com.mattermost.calls"
	PluginIdNPS           = "com.mattermost.nps"
	PluginIdChannelExport = "com.mattermost.plugin-channel-export"
	PluginIdAI            = "mattermost-ai"
)

// AntimatterPluginIds are the Antimatter plugins that Antimatter Next bundles. Like the bundled Mattermost plugins
// above, they're enabled by default, so that the server installs them from its prepackaged plugins.
var AntimatterPluginIds = []string{
	"com.antimatterchat.calendar",
	"com.antimatterchat.gifs",
	"com.antimatterchat.mail",
	"com.antimatterchat.notes",
	"com.antimatterchat.polls",
	"com.antimatterchat.voice-channels",
	"com.antimatterchat.whiteboard",
}
