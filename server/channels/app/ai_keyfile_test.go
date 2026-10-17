// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package app

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/v8/platform/shared/filestore"
)

// The key file the server creates in its data directory is out of reach of apps, archives and support packets.
func TestAIKeyFileStaysOnTheServer(t *testing.T) {
	th := Setup(t).InitBasic(t)
	_, appErr := th.App.Srv().aiKeyring()
	require.Nil(t, appErr)
	keyPath := filepath.Join(*th.App.Config().FileSettings.Directory, filestore.AIKeyFileName)
	key, err := os.ReadFile(keyPath)
	require.NoError(t, err, "the server made its key file")
	keyText := strings.TrimSpace(string(key))
	require.NotEmpty(t, keyText)

	t.Run("apps can't read it", func(t *testing.T) {
		api := th.SetupPluginAPI()
		_, appErr := api.ReadFile(filestore.AIKeyFileName)
		require.NotNil(t, appErr)
		_, appErr = api.ReadFile("teams/../" + filestore.AIKeyFileName)
		require.NotNil(t, appErr)
	})

	t.Run("the server's file helpers don't see it", func(t *testing.T) {
		exists, appErr := th.App.FileExists(filestore.AIKeyFileName)
		require.Nil(t, appErr)
		assert.False(t, exists)
		listing, appErr := th.App.ListDirectory("")
		require.Nil(t, appErr)
		assert.NotContains(t, listing, filestore.AIKeyFileName)
		_, appErr = th.App.WriteFile(strings.NewReader("replaced"), filestore.AIKeyFileName)
		require.NotNil(t, appErr)
		require.NotNil(t, th.App.MoveFile(filestore.AIKeyFileName, "elsewhere"))
		require.NotNil(t, th.App.RemoveFile(filestore.AIKeyFileName))

		archive, appErr := th.App.ZipReader("", false)
		require.Nil(t, appErr)
		data, err := io.ReadAll(archive)
		require.NoError(t, err)
		assert.False(t, bytes.Contains(data, []byte(keyText)), "an archive of the data directory leaves the key out")

		again, err := os.ReadFile(keyPath)
		require.NoError(t, err)
		assert.Equal(t, key, again, "the key is untouched")
	})

	t.Run("the support packet doesn't carry it", func(t *testing.T) {
		noProfile := time.Duration(0)
		for _, f := range th.App.GenerateSupportPacket(th.Context, &model.SupportPacketOptions{IncludeLogs: true, CPUProfileDuration: &noProfile}) {
			assert.False(t, bytes.Contains(f.Body, []byte(keyText)), "%s carries the AI encryption key", f.Filename)
		}
	})
}
