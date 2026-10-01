// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

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
