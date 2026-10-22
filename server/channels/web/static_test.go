// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package web

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

func TestSelectWebUI(t *testing.T) {
	th := Setup(t).InitBasic(t)

	// Each web UI's root.html names it, so the response tells which one was served.
	require.NoError(t, os.WriteFile(filepath.Join(model.ClientDir, "root.html"), []byte(model.WebUIClassic), 0600))
	require.NoError(t, os.MkdirAll(model.FusionClientDir, 0700))
	t.Cleanup(func() { os.RemoveAll(model.FusionClientDir) })
	require.NoError(t, os.WriteFile(filepath.Join(model.FusionClientDir, "root.html"), []byte(model.WebUIFusion), 0600))

	session, appErr := th.App.CreateSession(th.Context, &model.Session{UserId: th.BasicUser.Id, Roles: th.BasicUser.GetRawRoles()})
	require.Nil(t, appErr)

	setConfig := func(defaultWebUI string, allowSelection bool) {
		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.ServiceSettings.DefaultWebUI = defaultWebUI
			*cfg.ServiceSettings.AllowUserWebUISelection = allowSelection
		})
	}

	setPreference := func(webUI string) {
		if webUI == "" {
			require.NoError(t, th.App.Srv().Store().Preference().Delete(th.BasicUser.Id, model.PreferenceCategoryDisplaySettings, model.PreferenceNameWebUI))
			return
		}
		appErr := th.App.UpdatePreferences(th.Context, th.BasicUser.Id, model.Preferences{{
			UserId:   th.BasicUser.Id,
			Category: model.PreferenceCategoryDisplaySettings,
			Name:     model.PreferenceNameWebUI,
			Value:    webUI,
		}})
		require.Nil(t, appErr)
	}

	type request struct {
		query    string
		cookie   string
		loggedIn bool
	}
	serve := func(req request) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodGet, "/"+req.query, nil)
		if req.cookie != "" {
			r.AddCookie(&http.Cookie{Name: model.WebUICookie, Value: req.cookie})
		}
		if req.loggedIn {
			r.AddCookie(&http.Cookie{Name: model.SessionCookieToken, Value: session.Token})
		}
		w := httptest.NewRecorder()
		th.Web.MainRouter.ServeHTTP(w, r)
		require.Equal(t, http.StatusOK, w.Code)
		return w
	}

	t.Run("serves the default web UI to users who didn't pick one", func(t *testing.T) {
		setConfig(model.WebUIFusion, true)
		setPreference("")
		assert.Equal(t, model.WebUIFusion, serve(request{}).Body.String())
		assert.Equal(t, model.WebUIFusion, serve(request{loggedIn: true}).Body.String())

		setConfig(model.WebUIClassic, true)
		assert.Equal(t, model.WebUIClassic, serve(request{}).Body.String())
		assert.Equal(t, model.WebUIClassic, serve(request{loggedIn: true}).Body.String())
	})

	t.Run("serves the web UI picked in this browser before signing in", func(t *testing.T) {
		setConfig(model.WebUIFusion, true)
		setPreference(model.WebUIFusion)
		assert.Equal(t, model.WebUIClassic, serve(request{cookie: model.WebUIClassic}).Body.String())
		assert.Equal(t, model.WebUIFusion, serve(request{cookie: "modern"}).Body.String())
	})

	t.Run("serves the web UI picked by the signed-in user over the browser's", func(t *testing.T) {
		setConfig(model.WebUIFusion, true)
		setPreference(model.WebUIClassic)
		assert.Equal(t, model.WebUIClassic, serve(request{loggedIn: true}).Body.String())
		assert.Equal(t, model.WebUIClassic, serve(request{loggedIn: true, cookie: model.WebUIFusion}).Body.String())

		setPreference(model.WebUIFusion)
		assert.Equal(t, model.WebUIFusion, serve(request{loggedIn: true, cookie: model.WebUIClassic}).Body.String())
	})

	t.Run("ignores an invalid preference", func(t *testing.T) {
		setConfig(model.WebUIFusion, true)
		setPreference("modern")
		assert.Equal(t, model.WebUIClassic, serve(request{loggedIn: true, cookie: model.WebUIClassic}).Body.String())
		assert.Equal(t, model.WebUIFusion, serve(request{loggedIn: true}).Body.String())
	})

	t.Run("serves the web UI in the query and saves it for this browser", func(t *testing.T) {
		setConfig(model.WebUIFusion, true)
		setPreference(model.WebUIFusion)
		w := serve(request{query: "?webui=classic", loggedIn: true, cookie: model.WebUIFusion})
		assert.Equal(t, model.WebUIClassic, w.Body.String())

		var cookie *http.Cookie
		for _, c := range w.Result().Cookies() {
			if c.Name == model.WebUICookie {
				cookie = c
			}
		}
		require.NotNil(t, cookie)
		assert.Equal(t, model.WebUIClassic, cookie.Value)
	})

	t.Run("serves the default web UI when users may not pick one", func(t *testing.T) {
		setConfig(model.WebUIClassic, false)
		setPreference(model.WebUIFusion)
		assert.Equal(t, model.WebUIClassic, serve(request{query: "?webui=fusion", loggedIn: true, cookie: model.WebUIFusion}).Body.String())

		setConfig(model.WebUIFusion, false)
		setPreference(model.WebUIClassic)
		assert.Equal(t, model.WebUIFusion, serve(request{query: "?webui=classic", loggedIn: true, cookie: model.WebUIClassic}).Body.String())
	})
}

func TestStaticMissingClientDirectory(t *testing.T) {
	// Without a client-fusion directory, /static/fusion/ must not fall back to the working directory, where
	// config.json and the logs live.
	_, err := os.Stat(model.FusionClientDir)
	require.True(t, os.IsNotExist(err), "the test needs no %s directory", model.FusionClientDir)
	th := Setup(t)

	for _, name := range []string{"go.mod", "config/config.json"} {
		_, err := os.Stat(name)
		require.NoError(t, err, "the test server's working directory should hold %s", name)
		req := httptest.NewRequest(http.MethodGet, "/static/fusion/"+name, nil)
		res := httptest.NewRecorder()
		th.Web.MainRouter.ServeHTTP(res, req)
		assert.Equal(t, http.StatusNotFound, res.Code, name)
	}
}

func TestStaticCompression(t *testing.T) {
	th := Setup(t)
	th.ConfigStore.SetReadOnlyFF(false)

	// The same compressible asset in each web UI's client directory: the classic web app's under /static/,
	// Fusion's under /static/fusion/.
	asset := []byte(strings.Repeat("console.log('Antimatter');\n", 200))
	classicAsset := filepath.Join(model.ClientDir, "compression_test.js")
	require.NoError(t, os.WriteFile(classicAsset, asset, 0600))
	t.Cleanup(func() { os.Remove(classicAsset) })
	require.NoError(t, os.MkdirAll(model.FusionClientDir, 0700))
	t.Cleanup(func() { os.RemoveAll(model.FusionClientDir) })
	require.NoError(t, os.WriteFile(filepath.Join(model.FusionClientDir, "root.html"), []byte(model.WebUIFusion), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(model.FusionClientDir, "compression_test.js"), asset, 0600))

	serve := func(t *testing.T, webserverMode string, brotli bool, path, acceptEncoding string) *httptest.ResponseRecorder {
		t.Helper()
		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.ServiceSettings.WebserverMode = webserverMode
			cfg.FeatureFlags.EnableBrotliCompression = brotli
		})
		// The static handlers read the config when they're set up, as they do when the server starts.
		w := &Web{srv: th.Server, MainRouter: mux.NewRouter()}
		w.InitStatic()

		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Accept-Encoding", acceptEncoding)
		res := httptest.NewRecorder()
		w.MainRouter.ServeHTTP(res, req)
		require.Equal(t, http.StatusOK, res.Code)
		return res
	}

	gunzip := func(t *testing.T, data []byte) []byte {
		t.Helper()
		r, err := gzip.NewReader(bytes.NewReader(data))
		require.NoError(t, err)
		decompressed, err := io.ReadAll(r)
		require.NoError(t, err)
		return decompressed
	}

	for name, path := range map[string]string{
		"classic": "/static/compression_test.js",
		"Fusion":  "/static/fusion/compression_test.js",
	} {
		t.Run(name, func(t *testing.T) {
			// Static files stay on gzip like upstream: Brotli only applies to API responses.
			for _, brotli := range []bool{false, true} {
				t.Run(fmt.Sprintf("serves gzip, not Brotli, with EnableBrotliCompression=%v", brotli), func(t *testing.T) {
					res := serve(t, "gzip", brotli, path, "br, gzip")
					assert.Equal(t, "gzip", res.Header().Get("Content-Encoding"))
					assert.Equal(t, asset, gunzip(t, res.Body.Bytes()))
				})
			}

			t.Run("doesn't compress when WebserverMode is uncompressed, even with EnableBrotliCompression on", func(t *testing.T) {
				res := serve(t, "uncompressed", true, path, "br, gzip")
				assert.Empty(t, res.Header().Get("Content-Encoding"))
				assert.Equal(t, asset, res.Body.Bytes())
			})
		})
	}
}
