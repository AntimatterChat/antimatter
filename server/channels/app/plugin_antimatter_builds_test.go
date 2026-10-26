// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/v8/channels/testlib"
	"github.com/mattermost/mattermost/server/v8/channels/utils/fileutils"
)

func TestMarketplaceReplaceablePlugin(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t)
	ch := th.App.ch

	require.True(t, ch.marketplaceReplaceablePlugin("some.other.plugin", false))
	require.True(t, ch.marketplaceReplaceablePlugin("some.other.plugin", true))

	// A plugin Antimatter ships its own build of: replaceable until it is installed.
	require.True(t, ch.marketplaceReplaceablePlugin("jira", false))
	require.False(t, ch.marketplaceReplaceablePlugin("jira", true))

	// Boards and Playbooks aren't bundled with the server any more, but Antimatter still ships
	// its own forks of both (installed from their own GitHub releases): the Marketplace's vanilla
	// builds still don't replace an installed copy, same as any other Antimatter build.
	require.True(t, ch.marketplaceReplaceablePlugin("focalboard", false))
	require.False(t, ch.marketplaceReplaceablePlugin("focalboard", true))
	require.True(t, ch.marketplaceReplaceablePlugin("playbooks", false))
	require.False(t, ch.marketplaceReplaceablePlugin("playbooks", true))

	// A prepackaged bundle signed with the Antimatter key, installed or not.
	ch.antimatterPrepackagedPlugins.Store("some.other.plugin", struct{}{})
	require.False(t, ch.marketplaceReplaceablePlugin("some.other.plugin", false))

	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.PluginSettings.AllowMarketplaceToReplaceAntimatterPlugins = true
	})
	require.True(t, ch.marketplaceReplaceablePlugin("some.other.plugin", false))
	require.True(t, ch.marketplaceReplaceablePlugin("focalboard", true))
	require.True(t, ch.marketplaceReplaceablePlugin("playbooks", true))
}

func TestAntimatterPluginIDsKeepsBoardsAndPlaybooks(t *testing.T) {
	mainHelper.Parallel(t)

	// Boards (focalboard) and Playbooks are no longer bundled with the server, but Antimatter
	// still builds and releases its own forks of both (installed from their own GitHub releases,
	// not prepackaged here): they stay in this list so the Marketplace's vanilla builds never
	// silently replace an installed copy.
	require.True(t, slices.Contains(antimatterPluginIDs, model.PluginIdFocalboard))
	require.True(t, slices.Contains(antimatterPluginIDs, model.PluginIdPlaybooks))
}

func TestGetMarketplacePluginsLeavesOutAntimatterBuilds(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t)

	remote := []*model.BaseMarketplacePlugin{
		{Manifest: &model.Manifest{Id: "antimatter.prepackaged", Name: "Prepackaged", Version: "9.0.0"}},
		{Manifest: &model.Manifest{Id: "other.plugin", Name: "Other", Version: "1.0.0"}},
	}
	marketplace := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewEncoder(w).Encode(remote))
	}))
	t.Cleanup(marketplace.Close)

	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.PluginSettings.Enable = true
		*cfg.PluginSettings.EnableMarketplace = true
		*cfg.PluginSettings.EnableRemoteMarketplace = true
		*cfg.PluginSettings.MarketplaceURL = marketplace.URL
	})
	th.App.ch.antimatterPrepackagedPlugins.Store("antimatter.prepackaged", struct{}{})

	listedIDs := func() []string {
		t.Helper()
		plugins, appErr := th.App.GetMarketplacePlugins(th.Context, &model.MarketplacePluginFilter{RemoteOnly: true})
		require.Nil(t, appErr)
		var ids []string
		for _, p := range plugins {
			ids = append(ids, p.Manifest.Id)
		}
		slices.Sort(ids)
		return ids
	}

	require.Equal(t, []string{"other.plugin"}, listedIDs())

	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.PluginSettings.AllowMarketplaceToReplaceAntimatterPlugins = true
	})
	require.Equal(t, []string{"antimatter.prepackaged", "other.plugin"}, listedIDs())
}

func TestPrepackagedPluginsSignedByOtherKeysAreNotAntimatterBuilds(t *testing.T) {
	mainHelper.Parallel(t)
	testsPath, found := fileutils.FindDir("tests")
	require.True(t, found)

	th := SetupConfig(t, func(cfg *model.Config) {
		cfg.PluginSettings.SignaturePublicKeyFiles = []string{filepath.Join(testsPath, "development-public-key.asc")}
	})
	prepackagedDir := filepath.Join(th.tempWorkspace, prepackagedPluginsDir)
	require.NoError(t, os.Mkdir(prepackagedDir, os.ModePerm))
	for _, name := range []string{"testplugin.tar.gz", "testplugin.tar.gz.sig"} {
		require.NoError(t, testlib.CopyFile(filepath.Join(testsPath, name), filepath.Join(prepackagedDir, name)))
	}

	require.Nil(t, th.App.ch.syncPlugins())
	require.NoError(t, th.App.ch.processPrepackagedPlugins(prepackagedDir))

	require.Len(t, th.App.GetPluginsEnvironment().PrepackagedPlugins(), 1)
	_, recorded := th.App.ch.antimatterPrepackagedPlugins.Load("testplugin")
	require.False(t, recorded, "testplugin is signed with the development key, not the Antimatter key")
}

// A configuration upgraded from Mattermost keeps EnableRemoteMarketplace=true, but SetDefaults blanks its
// mattermost.com MarketplaceURL. The listing used to fail with "Failed to create marketplace client" (500) and showed
// no app at all; it now skips the remote fetch and still lists the prepackaged and installed apps.
func TestGetMarketplacePluginsWithRemoteMarketplaceOnAndNoURL(t *testing.T) {
	mainHelper.Parallel(t)
	testsPath, found := fileutils.FindDir("tests")
	require.True(t, found)

	th := SetupConfig(t, func(cfg *model.Config) {
		cfg.PluginSettings.SignaturePublicKeyFiles = []string{filepath.Join(testsPath, "development-public-key.asc")}
	})
	prepackagedDir := filepath.Join(th.tempWorkspace, prepackagedPluginsDir)
	require.NoError(t, os.Mkdir(prepackagedDir, os.ModePerm))
	for _, name := range []string{"testplugin.tar.gz", "testplugin.tar.gz.sig"} {
		require.NoError(t, testlib.CopyFile(filepath.Join(testsPath, name), filepath.Join(prepackagedDir, name)))
	}

	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.PluginSettings.Enable = true
		*cfg.PluginSettings.EnableMarketplace = true
		*cfg.PluginSettings.EnableRemoteMarketplace = true
		*cfg.PluginSettings.MarketplaceURL = "https://api.integrations.mattermost.com"
	})
	require.True(t, *th.App.Config().PluginSettings.EnableRemoteMarketplace)
	require.Empty(t, *th.App.Config().PluginSettings.MarketplaceURL, "SetDefaults blanks a mattermost.com Marketplace URL")
	require.False(t, th.App.Config().PluginSettings.RemoteMarketplaceEnabled())

	require.Nil(t, th.App.ch.syncPlugins())
	require.NoError(t, th.App.ch.processPrepackagedPlugins(prepackagedDir))

	plugins, appErr := th.App.GetMarketplacePlugins(th.Context, &model.MarketplacePluginFilter{})
	require.Nil(t, appErr)
	var ids []string
	for _, p := range plugins {
		ids = append(ids, p.Manifest.Id)
		require.Empty(t, p.Labels, "no \"not listed in the marketplace\" label without a remote Marketplace")
	}
	require.Contains(t, ids, "testplugin")

	plugins, appErr = th.App.GetMarketplacePlugins(th.Context, &model.MarketplacePluginFilter{RemoteOnly: true})
	require.Nil(t, appErr)
	require.Empty(t, plugins)
}

func TestIsAntimatterBuild(t *testing.T) {
	require.True(t, isAntimatterBuild(&model.Manifest{HomepageURL: "https://github.com/antimatterchat/antimatter-plugin-agents"}))
	require.True(t, isAntimatterBuild(&model.Manifest{SupportURL: "https://github.com/AntimatterChat/antimatter-plugin-jira/issues"}))
	require.False(t, isAntimatterBuild(&model.Manifest{HomepageURL: "https://github.com/mattermost/mattermost-plugin-ai"}))
	require.False(t, isAntimatterBuild(&model.Manifest{}))
}

// Mattermost's builds of the plugins Antimatter forks often have higher version numbers (Mattermost prepackages
// Agents 2.9, Antimatter's fork is 2.8): Antimatter's prepackaged build still replaces an installed Mattermost
// build, but never an installed Antimatter build.
func TestInstallPrepackagedPluginReplacesUpstreamBuilds(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t)
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.PluginSettings.Enable = true })
	ch := th.App.ch
	logger := th.TestLogger

	const pluginID = "jira"
	require.Contains(t, antimatterPluginIDs, pluginID)

	bundleDir := func(t *testing.T, version, homepage string) (*model.Manifest, string) {
		t.Helper()
		manifest := &model.Manifest{Id: pluginID, Name: "Jira", Version: version, HomepageURL: homepage}
		dir := t.TempDir()
		data, err := json.Marshal(manifest)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(dir, "plugin.json"), data, 0600))
		return manifest, dir
	}
	install := func(t *testing.T, version, homepage string) {
		t.Helper()
		manifest, dir := bundleDir(t, version, homepage)
		_, appErr := ch.installExtractedPlugin(manifest, dir, installPluginLocallyAlways)
		require.Nil(t, appErr)
	}
	installedVersion := func(t *testing.T) string {
		t.Helper()
		manifest, err := ch.GetPluginsEnvironment().GetManifest(pluginID)
		require.NoError(t, err)
		return manifest.Version
	}
	const upstreamHome = "https://github.com/mattermost/mattermost-plugin-jira"
	const antimatterHome = "https://github.com/antimatterchat/antimatter-plugin-jira"

	t.Run("an Antimatter-signed bundle replaces a newer Mattermost build", func(t *testing.T) {
		install(t, "2.9.0", upstreamHome)
		// Mattermost's build came from the file store.
		_, appErr := th.App.Srv().writeFile(strings.NewReader("upstream bundle"), getBundleStorePath(pluginID))
		require.Nil(t, appErr)
		_, appErr = th.App.Srv().writeFile(strings.NewReader("upstream signature"), getSignatureStorePath(pluginID))
		require.Nil(t, appErr)

		ch.antimatterPrepackagedPlugins.Store(pluginID, struct{}{})
		t.Cleanup(func() { ch.antimatterPrepackagedPlugins.Delete(pluginID) })
		manifest, dir := bundleDir(t, "2.8.8", antimatterHome)
		require.Nil(t, ch.installPrepackagedPlugin(logger, manifest, dir))
		require.Equal(t, "2.8.8", installedVersion(t))

		exists, appErr := th.App.Srv().fileExists(getBundleStorePath(pluginID))
		require.Nil(t, appErr)
		require.False(t, exists, "Mattermost's build no longer comes back from the file store")
		exists, appErr = th.App.Srv().fileExists(getSignatureStorePath(pluginID))
		require.Nil(t, appErr)
		require.False(t, exists)
	})

	t.Run("an installed Antimatter build is never downgraded", func(t *testing.T) {
		install(t, "3.0.0", antimatterHome)
		ch.antimatterPrepackagedPlugins.Store(pluginID, struct{}{})
		t.Cleanup(func() { ch.antimatterPrepackagedPlugins.Delete(pluginID) })

		manifest, dir := bundleDir(t, "2.8.8", antimatterHome)
		appErr := ch.installPrepackagedPlugin(logger, manifest, dir)
		require.NotNil(t, appErr)
		require.Equal(t, "app.plugin.skip_installation.app_error", appErr.Id)
		require.Equal(t, "3.0.0", installedVersion(t))
	})

	t.Run("a bundle not signed with the Antimatter key follows the version rule", func(t *testing.T) {
		install(t, "2.9.0", upstreamHome)
		ch.antimatterPrepackagedPlugins.Delete(pluginID)

		manifest, dir := bundleDir(t, "2.8.8", antimatterHome)
		appErr := ch.installPrepackagedPlugin(logger, manifest, dir)
		require.NotNil(t, appErr)
		require.Equal(t, "2.9.0", installedVersion(t))
	})

	t.Run("not when Mattermost's builds may replace Antimatter's", func(t *testing.T) {
		install(t, "2.9.0", upstreamHome)
		ch.antimatterPrepackagedPlugins.Store(pluginID, struct{}{})
		t.Cleanup(func() { ch.antimatterPrepackagedPlugins.Delete(pluginID) })
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.PluginSettings.AllowMarketplaceToReplaceAntimatterPlugins = true })
		t.Cleanup(func() {
			th.App.UpdateConfig(func(cfg *model.Config) { *cfg.PluginSettings.AllowMarketplaceToReplaceAntimatterPlugins = false })
		})

		manifest, dir := bundleDir(t, "2.8.8", antimatterHome)
		require.NotNil(t, ch.installPrepackagedPlugin(logger, manifest, dir))
		require.Equal(t, "2.9.0", installedVersion(t))
	})
}
