// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package filestore

import (
	"archive/zip"
	"bytes"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The AI encryption key at the root of the local storage is invisible and untouchable through the file backend,
// which apps (the plugin API), exports and archives use.
func TestLocalBackendHidesTheAIKeyFile(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, AIKeyFileName)
	require.NoError(t, os.WriteFile(keyPath, []byte("c2VjcmV0LWtleQ==\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, AIKeyFileName+".tmp-123"), []byte("half"), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "teams"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "teams", "file.txt"), []byte("ok"), 0o600))

	backend, err := NewFileBackend(FileBackendSettings{DriverName: driverLocal, Directory: dir})
	require.NoError(t, err)

	for _, path := range []string{AIKeyFileName, "/" + AIKeyFileName, "./" + AIKeyFileName, "teams/../" + AIKeyFileName, AIKeyFileName + ".tmp-123"} {
		_, err = backend.ReadFile(path)
		assert.ErrorIs(t, err, fs.ErrNotExist, path)
		_, err = backend.Reader(path)
		assert.ErrorIs(t, err, fs.ErrNotExist, path)
		exists, existsErr := backend.FileExists(path)
		require.NoError(t, existsErr)
		assert.False(t, exists, path)
		_, err = backend.FileSize(path)
		assert.Error(t, err, path)
		_, err = backend.FileModTime(path)
		assert.Error(t, err, path)
		_, err = backend.ZipReader(path, false)
		assert.Error(t, err, path)
		_, err = backend.WriteFile(strings.NewReader("overwritten"), path)
		assert.Error(t, err, path)
		_, err = backend.AppendFile(strings.NewReader("more"), path)
		assert.Error(t, err, path)
		assert.Error(t, backend.RemoveFile(path), path)
		assert.Error(t, backend.RemoveDirectory(path), path)
		assert.Error(t, backend.CopyFile(path, "teams/copy"), path)
		assert.Error(t, backend.CopyFile("teams/file.txt", path), path)
		assert.Error(t, backend.MoveFile(path, "teams/moved"), path)
		assert.Error(t, backend.MoveFile("teams/file.txt", path), path)
	}
	assert.Error(t, backend.RemoveDirectory(""), "removing the root would delete the key")
	assert.Error(t, backend.MoveFile("", "elsewhere"), "moving the root would move the key")

	listing, err := backend.ListDirectory("")
	require.NoError(t, err)
	assert.Equal(t, []string{"teams"}, listing)
	recursive, err := backend.ListDirectoryRecursively("")
	require.NoError(t, err)
	assert.Equal(t, []string{filepath.Join("teams", "file.txt")}, recursive)

	archive, err := backend.ZipReader("", false)
	require.NoError(t, err)
	data, err := io.ReadAll(archive)
	require.NoError(t, err)
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	require.NoError(t, err)
	for _, f := range reader.File {
		assert.NotContains(t, f.Name, AIKeyFileName, "an archive of the storage leaves the key out")
	}

	content, err := os.ReadFile(keyPath)
	require.NoError(t, err)
	assert.Equal(t, "c2VjcmV0LWtleQ==\n", string(content), "the key is untouched")

	// Other files work as before.
	got, err := backend.ReadFile("teams/file.txt")
	require.NoError(t, err)
	assert.Equal(t, "ok", string(got))
}
