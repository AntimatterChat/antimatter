// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package web

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"html"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/klauspost/compress/gzhttp"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/v8/channels/app"
	"github.com/mattermost/mattermost/server/v8/channels/utils"
	"github.com/mattermost/mattermost/server/v8/channels/utils/fileutils"
	"github.com/mattermost/mattermost/server/v8/config"
	"github.com/mattermost/mattermost/server/v8/platform/shared/templates"
)

var robotsTxt = []byte("User-agent: *\nDisallow: /\n")

func (w *Web) InitStatic() {
	if *w.srv.Config().ServiceSettings.WebserverMode != "disabled" {
		if err := utils.UpdateAssetsSubpathFromConfig(w.srv.Config()); err != nil {
			mlog.Error("Failed to update assets subpath from config", mlog.Err(err))
		}
		if config.FusionWebUIAvailable() {
			if err := utils.UpdateFusionAssetsSubpathFromConfig(w.srv.Config()); err != nil {
				mlog.Error("Failed to update Fusion UI assets subpath from config", mlog.Err(err))
			}
		}

		staticDir, _ := fileutils.FindDir(model.ClientDir)
		mlog.Debug("Using client directory", mlog.String("client_dir", staticDir))

		subpath, _ := utils.GetSubpathFromConfig(w.srv.Config())

		staticHandler := staticFilesHandler(http.StripPrefix(path.Join(subpath, "static"), http.FileServer(http.Dir(staticDir))))
		pluginHandler := staticFilesHandler(http.StripPrefix(path.Join(subpath, "static", "plugins"), http.FileServer(http.Dir(*w.srv.Config().PluginSettings.ClientDirectory))))

		// The Fusion UI is optional: without its directory, its assets are simply not found.
		fusionDir, _ := fileutils.FindDir(model.FusionClientDir)
		fusionHandler := staticFilesHandler(http.StripPrefix(path.Join(subpath, "static", "fusion"), http.FileServer(http.Dir(fusionDir))))

		if *w.srv.Config().ServiceSettings.WebserverMode == "gzip" {
			staticHandler = gzhttp.GzipHandler(staticHandler)
			pluginHandler = gzhttp.GzipHandler(pluginHandler)
			fusionHandler = gzhttp.GzipHandler(fusionHandler)
		}

		w.MainRouter.PathPrefix("/static/plugins/").Handler(pluginHandler)
		w.MainRouter.PathPrefix("/static/fusion/").Handler(fusionHandler)
		w.MainRouter.PathPrefix("/static/").Handler(staticHandler)
		w.MainRouter.Handle("/robots.txt", http.HandlerFunc(robotsHandler))
		w.MainRouter.Handle("/unsupported_browser.js", http.HandlerFunc(unsupportedBrowserScriptHandler))
		w.MainRouter.Handle("/{anything:.*}", w.NewStaticHandler(root)).Methods(http.MethodGet, http.MethodHead)

		// When a subpath is defined, it's necessary to handle redirects without a
		// trailing slash. We don't want to use StrictSlash on the w.MainRouter and affect
		// all routes, just /subpath -> /subpath/.
		w.MainRouter.HandleFunc("", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.URL.Path += "/"
			http.Redirect(w, r, r.URL.String(), http.StatusFound)
		}))
	}
}

func root(c *Context, w http.ResponseWriter, r *http.Request) {
	if !CheckClientCompatibility(r.UserAgent()) {
		w.Header().Set("Cache-Control", "no-store")
		subpath, _ := utils.GetSubpathFromConfig(c.App.Srv().Config())
		data := renderUnsupportedBrowser(c.AppContext, r, subpath)

		err := c.App.Srv().TemplatesContainer().Render(w, "unsupported_browser", data)
		if err != nil {
			c.Logger.Error("Failed to render template", mlog.Err(err))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		return
	}

	if !CheckDesktopAppCompatibility(r.UserAgent(), c.App.Srv().Config().ServiceSettings.MinimumDesktopAppVersion) {
		w.Header().Set("Cache-Control", "no-store")

		currentVersion, ok := app.GetDesktopAppVersion(r.UserAgent())
		if !ok {
			currentVersion = "unknown"
		}
		cfg := c.App.Srv().Config()
		subpath, _ := utils.GetSubpathFromConfig(cfg)

		data := renderUnsupportedDesktopApp(c.AppContext, cfg, currentVersion, subpath)
		err := c.App.Srv().TemplatesContainer().Render(w, "unsupported_desktop_app", data)
		if err != nil {
			c.Logger.Error("Failed to render template", mlog.Err(err))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		return
	}

	if IsAPICall(c.App, r) {
		Handle404(c.App, w, r)
		return
	}

	w.Header().Set("Cache-Control", "no-cache, max-age=31556926, public")
	// The page served depends on the web UI cookie and on the session cookie.
	w.Header().Add("Vary", "Cookie")

	clientDir := model.ClientDir
	if selectWebUI(c, w, r) == model.WebUIFusion {
		clientDir = model.FusionClientDir
	}

	staticDir, _ := fileutils.FindDir(clientDir)
	contents, err := os.ReadFile(filepath.Join(staticDir, "root.html"))
	if err != nil {
		c.Logger.Warn("Failed to read content from file",
			mlog.String("file_path", filepath.Join(staticDir, "root.html")),
			mlog.Err(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	titleTemplate := "<title>%s</title>"
	originalHTML := fmt.Sprintf(titleTemplate, html.EscapeString(model.TeamSettingsDefaultSiteName))
	modifiedHTML := getOpenGraphMetaTags(c)
	if originalHTML != modifiedHTML {
		contents = bytes.ReplaceAll(contents, []byte(originalHTML), []byte(modifiedHTML))
	}

	w.Header().Set("Content-Type", "text/html")
	if _, err = w.Write(contents); err != nil {
		c.Logger.Warn("Failed to write content to HTTP reply", mlog.Err(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// selectWebUI returns the web UI to serve for this request. When users may pick their own, that's the one
// in a valid ?webui= query parameter (which is also saved for this browser), then the one the signed-in
// user picked (their web UI preference, which follows them across browsers), then the one picked in
// this browser (the cookie, which also covers the pages shown before signing in), and otherwise the
// default one.
func selectWebUI(c *Context, w http.ResponseWriter, r *http.Request) string {
	cfg := c.App.Srv().Config()
	if !config.UserWebUISelectionAllowed(cfg) {
		return config.DefaultWebUI(cfg)
	}

	if requested := r.URL.Query().Get(model.WebUIQueryParam); model.IsValidWebUI(requested) {
		subpath, _ := utils.GetSubpathFromConfig(cfg)
		http.SetCookie(w, &http.Cookie{
			Name:     model.WebUICookie,
			Value:    requested,
			Path:     subpath,
			MaxAge:   int((365 * 24 * time.Hour).Seconds()),
			Secure:   app.GetProtocol(r) == "https",
			SameSite: http.SameSiteLaxMode,
		})
		return requested
	}

	if webUI := userWebUI(c); webUI != "" {
		return webUI
	}

	if cookie, err := r.Cookie(model.WebUICookie); err == nil && model.IsValidWebUI(cookie.Value) {
		return cookie.Value
	}

	return config.DefaultWebUI(cfg)
}

// userWebUI returns the web UI picked by the user signed in with this request, or "" if there's
// no such user or they didn't pick one.
func userWebUI(c *Context) string {
	userID := c.AppContext.Session().UserId
	if userID == "" {
		return ""
	}

	pref, err := c.App.Srv().Store().Preference().Get(userID, model.PreferenceCategoryDisplaySettings, model.PreferenceNameWebUI)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			c.Logger.Warn("Failed to get the user's web UI preference", mlog.String("user_id", userID), mlog.Err(err))
		}
		return ""
	}

	if !model.IsValidWebUI(pref.Value) {
		return ""
	}
	return pref.Value
}

func staticFilesHandler(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//wrap our ResponseWriter with our no-cache 404-handler
		w = &notFoundNoCacheResponseWriter{ResponseWriter: w}

		if path.Base(r.URL.Path) == "remote_entry.js" {
			w.Header().Set("Cache-Control", "no-cache, max-age=31556926, public")
		} else {
			w.Header().Set("Cache-Control", "max-age=31556926, public")
		}

		// Hardcoded sensible default values for these security headers. Feel free to override in proxy or ingress
		w.Header().Set("Permissions-Policy", "")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")

		if strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r)
			return
		}

		handler.ServeHTTP(w, r)
	})
}

type notFoundNoCacheResponseWriter struct {
	http.ResponseWriter
}

func (w *notFoundNoCacheResponseWriter) WriteHeader(statusCode int) {
	if statusCode == http.StatusNotFound {
		// we have a 404, update our cache header first then fall through
		w.Header().Set("Cache-Control", "no-cache, public")
	}
	w.ResponseWriter.WriteHeader(statusCode)
}

func robotsHandler(w http.ResponseWriter, r *http.Request) {
	if strings.HasSuffix(r.URL.Path, "/") {
		http.NotFound(w, r)
		return
	}
	if _, err := w.Write(robotsTxt); err != nil {
		mlog.Warn("Failed to write robots.txt", mlog.Err(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func unsupportedBrowserScriptHandler(w http.ResponseWriter, r *http.Request) {
	if strings.HasSuffix(r.URL.Path, "/") {
		http.NotFound(w, r)
		return
	}

	templatesDir, _ := templates.GetTemplateDirectory()
	http.ServeFile(w, r, filepath.Join(templatesDir, "unsupported_browser.js"))
}

func ensureTrailingSlash(s string) string {
	if s == "" {
		return "/"
	}
	if !strings.HasSuffix(s, "/") {
		return s + "/"
	}
	return s
}

func getOpenGraphMetaTags(c *Context) string {
	siteName := model.TeamSettingsDefaultSiteName
	customSiteName := c.App.Srv().Config().TeamSettings.SiteName
	if customSiteName != nil && *customSiteName != "" {
		siteName = *customSiteName
	}

	siteDescription := model.TeamSettingsDefaultCustomDescriptionText
	customSiteDescription := c.App.Srv().Config().TeamSettings.CustomDescriptionText
	if customSiteDescription != nil && *customSiteDescription != "" {
		siteDescription = *customSiteDescription
	}

	titleTemplate := "<title>%s</title>"
	titleHTML := fmt.Sprintf(titleTemplate, html.EscapeString(siteName))
	descriptionHTML := ""
	if siteDescription != "" {
		descriptionTemplate := "<meta property=\"og:description\" content=\"%s\" />"
		descriptionHTML = fmt.Sprintf(descriptionTemplate, html.EscapeString(siteDescription))
	}

	return titleHTML + descriptionHTML
}
