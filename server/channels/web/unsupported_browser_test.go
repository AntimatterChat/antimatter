// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package web

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/platform/shared/templates"
)

const (
	oldWindowsEdgeUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/42.0.2311.135 Safari/537.36 Edge/12.10136"
	oldMacSafariUA   = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_9_3) AppleWebKit/537.75.14 (KHTML, like Gecko) Version/7.0.3 Safari/7046A194A"
)

func TestUnsupportedBrowserPageOffersAntimatterApp(t *testing.T) {
	rctx := request.TestContext(t)
	render := func(t *testing.T, ua string, downloadLink string) (map[string]any, string) {
		t.Helper()
		cfg := &model.Config{}
		cfg.SetDefaults()
		*cfg.NativeAppSettings.AppDownloadLink = downloadLink
		r := httptest.NewRequest("GET", "/", nil)
		r.Header.Set("User-Agent", ua)
		data := renderUnsupportedBrowser(rctx, cfg, r, "/")

		dir, ok := templates.GetTemplateDirectory()
		require.True(t, ok)
		container, err := templates.New(dir)
		require.NoError(t, err)
		page, err := container.RenderToString("unsupported_browser", data)
		require.NoError(t, err)
		return data.Props, page
	}

	for name, ua := range map[string]string{"windows": oldWindowsEdgeUA, "mac": oldMacSafariUA} {
		t.Run(name+": the configured download page and Antimatter's guide", func(t *testing.T) {
			props, page := render(t, ua, "https://downloads.example.com/chat")
			app, ok := props["App"].(MattermostApp)
			require.True(t, ok, "the desktop app is offered")
			assert.Equal(t, "https://downloads.example.com/chat", app.Link)
			assert.Equal(t, "https://antimatter.chat/docs/users/index.html#apps", app.InstallGuideLink)
			assert.Contains(t, page, "https://downloads.example.com/chat")
			assert.NotContains(t, page, "mattermost.com")
		})
	}

	t.Run("the default download page is Antimatter's", func(t *testing.T) {
		props, page := render(t, oldWindowsEdgeUA, model.NativeappSettingsDefaultAppDownloadLink)
		assert.Equal(t, model.NativeappSettingsDefaultAppDownloadLink, props["App"].(MattermostApp).Link)
		assert.Contains(t, page, "https://github.com/AntimatterChat/desktop-app/releases")
		assert.NotContains(t, page, "mattermost.com")
	})

	t.Run("no download page, no app box", func(t *testing.T) {
		props, page := render(t, oldWindowsEdgeUA, "")
		assert.NotContains(t, props, "App")
		assert.NotContains(t, page, "data-mattermost-hover='mattermost-app'")
		assert.NotContains(t, page, "mattermost.com")
	})
}
