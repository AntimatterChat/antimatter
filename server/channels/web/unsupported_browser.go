// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package web

import (
	"net/http"

	"github.com/avct/uasurfer"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/platform/shared/templates"
)

// appInstallGuideLink is the user guide's section on the desktop and mobile apps.
const appInstallGuideLink = "https://antimatter.chat/docs/users/index.html#apps"

// MattermostApp describes downloads for the Antimatter desktop app (the type keeps its upstream name)
type MattermostApp struct {
	LogoSrc                string
	Title                  string
	SupportedVersionString string
	Label                  string
	Link                   string
	InstallGuide           string
	InstallGuideLink       string
}

// Browser describes a browser with a download link
type Browser struct {
	LogoSrc                string
	Title                  string
	SupportedVersionString string
	Src                    string
	GetLatestString        string
}

// SystemBrowser describes a browser but includes 2 links: one to open the local browser, and one to make it default
type SystemBrowser struct {
	LogoSrc                string
	Title                  string
	SupportedVersionString string
	LabelOpen              string
	LinkOpen               string
	LinkMakeDefault        string
	OrString               string
	MakeDefaultString      string
}

func renderUnsupportedBrowser(rctx request.CTX, cfg *model.Config, r *http.Request, subpath string) templates.Data {
	data := templates.Data{
		Props: map[string]any{
			"Subpath":                           ensureTrailingSlash(subpath),
			"DownloadAppOrUpgradeBrowserString": rctx.T("web.error.unsupported_browser.download_app_or_upgrade_browser"),
		},
	}

	// User Agent info
	ua := uasurfer.Parse(r.UserAgent())
	isWindows := ua.OS.Platform.String() == "PlatformWindows"
	isWindows10 := isWindows && ua.OS.Version.Major == 10
	isMacOSX := ua.OS.Name.String() == "OSMacOSX" && ua.OS.Version.Major == 10
	isSafari := ua.Browser.Name.String() == "BrowserSafari"

	// Basic heading translations
	if isSafari {
		data.Props["NoLongerSupportString"] = rctx.T("web.error.unsupported_browser.no_longer_support_version")
	} else {
		data.Props["NoLongerSupportString"] = rctx.T("web.error.unsupported_browser.no_longer_support")
	}

	// The desktop app, from the download page the admin set (the Antimatter releases by default); none without one.
	downloadLink := ""
	if cfg.NativeAppSettings.AppDownloadLink != nil {
		downloadLink = *cfg.NativeAppSettings.AppDownloadLink
	}
	if downloadLink != "" {
		if isWindows {
			data.Props["App"] = renderMattermostAppWindows(rctx, downloadLink)
		} else if isMacOSX {
			data.Props["App"] = renderMattermostAppMac(rctx, downloadLink)
		}
	}

	// Browsers to download
	// Show a link to Safari if you're using safari and it's outdated
	// Can't show on Mac all the time because there's no way to open it via URI
	browsers := []Browser{renderBrowserChrome(rctx), renderBrowserFirefox(rctx)}
	if isSafari {
		browsers = append(browsers, renderBrowserSafari(rctx))
	}
	data.Props["Browsers"] = browsers

	// If on Windows 10, show link to Edge
	if isWindows10 {
		data.Props["SystemBrowser"] = renderSystemBrowserEdge(rctx, r)
	}

	return data
}

func renderMattermostAppMac(rctx request.CTX, downloadLink string) MattermostApp {
	return MattermostApp{
		"mac.png",
		rctx.T("web.error.unsupported_browser.download_the_app"),
		rctx.T("web.error.unsupported_browser.min_os_version.mac"),
		rctx.T("web.error.unsupported_browser.download"),
		downloadLink,
		rctx.T("web.error.unsupported_browser.install_guide.mac"),
		appInstallGuideLink,
	}
}

func renderMattermostAppWindows(rctx request.CTX, downloadLink string) MattermostApp {
	return MattermostApp{
		"windows.svg",
		rctx.T("web.error.unsupported_browser.download_the_app"),
		rctx.T("web.error.unsupported_browser.min_os_version.windows"),
		rctx.T("web.error.unsupported_browser.download"),
		downloadLink,
		rctx.T("web.error.unsupported_browser.install_guide.windows"),
		appInstallGuideLink,
	}
}

func renderBrowserChrome(rctx request.CTX) Browser {
	return Browser{
		"chrome.svg",
		rctx.T("web.error.unsupported_browser.browser_title.chrome"),
		rctx.T("web.error.unsupported_browser.min_browser_version.chrome"),
		"http://www.google.com/chrome",
		rctx.T("web.error.unsupported_browser.browser_get_latest.chrome"),
	}
}

func renderBrowserFirefox(rctx request.CTX) Browser {
	return Browser{
		"firefox.svg",
		rctx.T("web.error.unsupported_browser.browser_title.firefox"),
		rctx.T("web.error.unsupported_browser.min_browser_version.firefox"),
		"https://www.mozilla.org/firefox/new/",
		rctx.T("web.error.unsupported_browser.browser_get_latest.firefox"),
	}
}

func renderBrowserSafari(rctx request.CTX) Browser {
	return Browser{
		"safari.svg",
		rctx.T("web.error.unsupported_browser.browser_title.safari"),
		rctx.T("web.error.unsupported_browser.min_browser_version.safari"),
		"macappstore://showUpdatesPage",
		rctx.T("web.error.unsupported_browser.browser_get_latest.safari"),
	}
}

func renderSystemBrowserEdge(rctx request.CTX, r *http.Request) SystemBrowser {
	return SystemBrowser{
		"edge.svg",
		rctx.T("web.error.unsupported_browser.browser_title.edge"),
		rctx.T("web.error.unsupported_browser.min_browser_version.edge"),
		rctx.T("web.error.unsupported_browser.open_system_browser.edge"),
		"microsoft-edge:http://" + r.Host + r.RequestURI, //TODO: Can we get HTTP or HTTPS? If someone's server doesn't have a redirect this won't work
		"ms-settings:defaultapps",
		rctx.T("web.error.unsupported_browser.system_browser_or"),
		rctx.T("web.error.unsupported_browser.system_browser_make_default"),
	}
}
