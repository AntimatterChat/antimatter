// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package app

import "slices"

// antimatterPluginIDs are the plugins Antimatter ships its own builds of, under the IDs of the
// upstream plugins they are forked from.
var antimatterPluginIDs = []string{
	"com.github.manland.mattermost-plugin-gitlab",
	"com.mattermost.calls",
	"com.mattermost.mattermost-plugin-metrics",
	"focalboard",
	"github",
	"jira",
	"mattermost-ai",
	"playbooks",
}

// marketplaceReplaceablePlugin reports whether the remote Marketplace may list and install its
// build of the plugin. Antimatter's plugins share their IDs with the Marketplace's (Mattermost's)
// builds, which a Marketplace "update" would install over them. So unless
// PluginSettings.AllowMarketplaceToReplaceAntimatterPlugins is set, the Marketplace's build is
// left out when Antimatter's is available here: a prepackaged bundle signed with the Antimatter
// key, or an installed plugin Antimatter ships its own build of.
func (ch *Channels) marketplaceReplaceablePlugin(pluginID string, installed bool) bool {
	if *ch.cfgSvc.Config().PluginSettings.AllowMarketplaceToReplaceAntimatterPlugins {
		return true
	}
	if _, ok := ch.antimatterPrepackagedPlugins.Load(pluginID); ok {
		return false
	}
	return !installed || !slices.Contains(antimatterPluginIDs, pluginID)
}

// isPluginInstalled reports whether the plugin is installed, enabled or not.
func (ch *Channels) isPluginInstalled(pluginID string) bool {
	pluginsEnvironment := ch.GetPluginsEnvironment()
	if pluginsEnvironment == nil {
		return false
	}
	manifest, err := pluginsEnvironment.GetManifest(pluginID)
	return err == nil && manifest != nil
}
