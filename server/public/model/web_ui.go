// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

// The server can serve two web UIs: the classic web app from ClientDir, and the Fusion UI from
// FusionClientDir. ServiceSettings.DefaultWebUI picks the one served by default, and users may
// pick their own through the WebUICookie when ServiceSettings.AllowUserWebUISelection is enabled.
const (
	WebUIClassic = "classic"
	WebUIFusion  = "fusion"

	FusionClientDir = "client-fusion"

	// WebUICookie holds the web UI chosen by the user in this browser.
	WebUICookie = "AMWEBUI"

	// WebUIQueryParam switches the web UI of this browser when present on a page URL, e.g. /?webui=classic.
	WebUIQueryParam = "webui"
)

func IsValidWebUI(webUI string) bool {
	return webUI == WebUIClassic || webUI == WebUIFusion
}
