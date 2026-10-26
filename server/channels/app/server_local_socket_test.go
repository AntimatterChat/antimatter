// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package app

import (
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/shared/mlog"
)

// Mattermost's mmctl --local connects to /var/tmp/mattermost_local.socket. A server using the Antimatter default
// socket also answers there, but never takes the path from another process.
func TestListenLegacyLocalModeSocket(t *testing.T) {
	logger := mlog.CreateConsoleTestLogger(t)
	// Kept short: a Unix socket path has at most 107 bytes, which t.TempDir() can exceed under a long TMPDIR.
	dir, err := os.MkdirTemp("/tmp", "amsock")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(dir) })

	accepts := func(t *testing.T, path string) {
		t.Helper()
		conn, err := net.Dial("unix", path)
		require.NoError(t, err)
		conn.Close()
	}

	t.Run("free path", func(t *testing.T) {
		path := filepath.Join(dir, "free.sock")
		listener := listenLegacyLocalModeSocket(logger, path)
		require.NotNil(t, listener)
		defer listener.Close()

		info, err := os.Stat(path)
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0600), info.Mode().Perm())
		accepts(t, path)
	})

	t.Run("stale socket of a stopped server", func(t *testing.T) {
		path := filepath.Join(dir, "stale.sock")
		stale, err := net.Listen("unix", path)
		require.NoError(t, err)
		stale.(*net.UnixListener).SetUnlinkOnClose(false)
		require.NoError(t, stale.Close())
		_, err = os.Lstat(path)
		require.NoError(t, err, "the stale socket file is left behind")

		listener := listenLegacyLocalModeSocket(logger, path)
		require.NotNil(t, listener)
		defer listener.Close()
		accepts(t, path)
	})

	t.Run("socket another process listens on", func(t *testing.T) {
		path := filepath.Join(dir, "live.sock")
		other, err := net.Listen("unix", path)
		require.NoError(t, err)
		defer other.Close()
		go func() {
			for {
				conn, acceptErr := other.Accept()
				if acceptErr != nil {
					return
				}
				conn.Close()
			}
		}()

		require.Nil(t, listenLegacyLocalModeSocket(logger, path))
		accepts(t, path)
	})

	t.Run("file that isn't a socket", func(t *testing.T) {
		path := filepath.Join(dir, "file.sock")
		require.NoError(t, os.WriteFile(path, []byte("keep me"), 0644))

		require.Nil(t, listenLegacyLocalModeSocket(logger, path))
		content, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Equal(t, "keep me", string(content))
	})
}
