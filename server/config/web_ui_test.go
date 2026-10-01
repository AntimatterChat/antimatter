// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

func TestWebUISelection(t *testing.T) {
	cfg := &model.Config{}
	cfg.SetDefaults()
	require.Equal(t, model.WebUIFusion, *cfg.ServiceSettings.DefaultWebUI)
	require.True(t, *cfg.ServiceSettings.AllowUserWebUISelection)

	t.Chdir(t.TempDir())

	t.Run("without the Fusion UI deployed", func(t *testing.T) {
		*cfg.ServiceSettings.DefaultWebUI = model.WebUIFusion
		assert.False(t, FusionWebUIAvailable())
		assert.Equal(t, model.WebUIClassic, DefaultWebUI(cfg))
		assert.False(t, UserWebUISelectionAllowed(cfg))
	})

	require.NoError(t, os.Mkdir(model.FusionClientDir, 0o700))

	t.Run("with the Fusion UI deployed", func(t *testing.T) {
		*cfg.ServiceSettings.DefaultWebUI = model.WebUIFusion
		*cfg.ServiceSettings.AllowUserWebUISelection = true
		assert.True(t, FusionWebUIAvailable())
		assert.Equal(t, model.WebUIFusion, DefaultWebUI(cfg))
		assert.True(t, UserWebUISelectionAllowed(cfg))

		*cfg.ServiceSettings.DefaultWebUI = model.WebUIClassic
		*cfg.ServiceSettings.AllowUserWebUISelection = false
		assert.Equal(t, model.WebUIClassic, DefaultWebUI(cfg))
		assert.False(t, UserWebUISelectionAllowed(cfg))
	})

	t.Run("invalid default web UI", func(t *testing.T) {
		*cfg.ServiceSettings.DefaultWebUI = "modern"
		appErr := cfg.IsValid()
		require.NotNil(t, appErr)
		assert.Equal(t, "model.config.is_valid.default_web_ui.app_error", appErr.Id)
	})
}
