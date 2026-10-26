// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package app

import (
	"slices"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
)

// antimatterPluginIDs are the plugins Antimatter ships its own builds of, under the IDs of the
// upstream plugins they are forked from. Boards (focalboard) and Playbooks stay in this list even
// though the server no longer bundles them: Antimatter still builds and releases its own forks of
// both (installed from their own GitHub releases, not prepackaged here), and the Marketplace's
// vanilla builds must never silently replace them.
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

// antimatterBuildURLPrefix starts the homepage, support and release notes URLs of the plugins Antimatter builds.
const antimatterBuildURLPrefix = "https://github.com/antimatterchat/"

// isAntimatterBuild reports whether an installed plugin is one of Antimatter's builds, by the URLs of its manifest:
// once installed, a plugin's signature is gone, and Mattermost's builds of the same IDs point at Mattermost's
// repositories.
func isAntimatterBuild(manifest *model.Manifest) bool {
	for _, url := range []string{manifest.HomepageURL, manifest.SupportURL, manifest.ReleaseNotesURL} {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(url)), antimatterBuildURLPrefix) {
			return true
		}
	}
	return false
}

// prepackagedReplacesUpstreamBuild reports whether the prepackaged bundle (already verified and extracted) replaces
// the installed plugin of the same ID whatever their versions: the bundle is Antimatter's build, signed with the
// Antimatter key, of a plugin Antimatter ships its own build of (antimatterPluginIDs), and the installed plugin is
// Mattermost's build. Mattermost's builds often have higher version numbers than Antimatter's forks (Mattermost
// prepackages Agents 2.9 when Antimatter's fork is 2.8), so the usual "only if newer" rule would keep a server moved
// from Mattermost on Mattermost's build for good. An installed Antimatter build is never downgraded, and
// PluginSettings.AllowMarketplaceToReplaceAntimatterPlugins, which lets Mattermost's builds replace Antimatter's,
// turns this off.
func (ch *Channels) prepackagedReplacesUpstreamBuild(prepackaged *model.Manifest) bool {
	if *ch.cfgSvc.Config().PluginSettings.AllowMarketplaceToReplaceAntimatterPlugins {
		return false
	}
	if !slices.Contains(antimatterPluginIDs, prepackaged.Id) {
		return false
	}
	if _, ok := ch.antimatterPrepackagedPlugins.Load(prepackaged.Id); !ok {
		return false
	}
	pluginsEnvironment := ch.GetPluginsEnvironment()
	if pluginsEnvironment == nil {
		return false
	}
	installed, err := pluginsEnvironment.GetManifest(prepackaged.Id)
	if err != nil || installed == nil {
		return false
	}
	return !isAntimatterBuild(installed)
}

// installPrepackagedPlugin installs a prepackaged plugin when it is newer than the installed one, or when it is
// Antimatter's build replacing Mattermost's (prepackagedReplacesUpstreamBuild). Mattermost's build is then also
// removed from the file store, from which every server of the cluster installs it again at each start.
func (ch *Channels) installPrepackagedPlugin(logger *mlog.Logger, manifest *model.Manifest, pluginDir string) *model.AppError {
	if !ch.prepackagedReplacesUpstreamBuild(manifest) {
		_, appErr := ch.installExtractedPlugin(manifest, pluginDir, installPluginLocallyOnlyIfNewOrUpgrade)
		return appErr
	}

	upstreamVersion := ""
	if pluginsEnvironment := ch.GetPluginsEnvironment(); pluginsEnvironment != nil {
		if installed, err := pluginsEnvironment.GetManifest(manifest.Id); err == nil && installed != nil {
			upstreamVersion = installed.Version
		}
	}
	logger.Warn("Replacing Mattermost's build of the plugin with Antimatter's prepackaged build",
		mlog.String("mattermost_version", upstreamVersion), mlog.String("antimatter_version", manifest.Version))
	if _, appErr := ch.installExtractedPlugin(manifest, pluginDir, installPluginLocallyAlways); appErr != nil {
		return appErr
	}

	bundlePath := getBundleStorePath(manifest.Id)
	if exists, appErr := ch.srv.fileExists(bundlePath); appErr != nil {
		logger.Warn("Can't check the file store for Mattermost's build of the plugin", mlog.Err(appErr))
	} else if exists {
		if appErr := ch.srv.removeFile(bundlePath); appErr != nil {
			logger.Warn("Can't remove Mattermost's build of the plugin from the file store", mlog.Err(appErr))
		} else if appErr := ch.removeSignature(manifest.Id); appErr != nil {
			logger.Warn("Can't remove the signature of Mattermost's build of the plugin from the file store", mlog.Err(appErr))
		}
	}
	return nil
}
