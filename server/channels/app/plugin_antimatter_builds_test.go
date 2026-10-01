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
	require.True(t, ch.marketplaceReplaceablePlugin("focalboard", false))
	require.False(t, ch.marketplaceReplaceablePlugin("focalboard", true))

	// A prepackaged bundle signed with the Antimatter key, installed or not.
	ch.antimatterPrepackagedPlugins.Store("some.other.plugin", struct{}{})
	require.False(t, ch.marketplaceReplaceablePlugin("some.other.plugin", false))

	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.PluginSettings.AllowMarketplaceToReplaceAntimatterPlugins = true
	})
	require.True(t, ch.marketplaceReplaceablePlugin("some.other.plugin", false))
	require.True(t, ch.marketplaceReplaceablePlugin("focalboard", true))
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
