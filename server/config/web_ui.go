// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package config

import (
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/v8/channels/utils/fileutils"
)

// FusionWebUIAvailable reports whether the Fusion UI was deployed next to the classic web app.
func FusionWebUIAvailable() bool {
	_, found := fileutils.FindDir(model.FusionClientDir)
	return found
}

// DefaultWebUI returns the web UI served to users who didn't pick one, falling back to the classic
// web app when the configured default isn't deployed.
func DefaultWebUI(c *model.Config) string {
	if *c.ServiceSettings.DefaultWebUI == model.WebUIFusion && FusionWebUIAvailable() {
		return model.WebUIFusion
	}
	return model.WebUIClassic
}

// UserWebUISelectionAllowed reports whether users can pick the web UI they're served.
func UserWebUISelectionAllowed(c *model.Config) bool {
	return *c.ServiceSettings.AllowUserWebUISelection && FusionWebUIAvailable()
}
