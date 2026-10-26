// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package messageexport

import (
	"archive/zip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A panic while filling an archive fails that export, not the server, and leaves no partial file.
func TestWriteZip_RecoversPanicInFill(t *testing.T) {
	backend := newLocalBackend(t)
	err := writeZip(backend, "export/broken.zip", func(zw *zip.Writer) error {
		w, err := createZipEntry(zw, "messages.csv", time.Now())
		require.NoError(t, err)
		_, err = w.Write([]byte("a,b,c\n"))
		require.NoError(t, err)
		var props map[string]any
		_ = props["missing"].(string) // a bad value hitting an unguarded assertion
		return nil
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "panic")
	exists, existsErr := backend.FileExists("export/broken.zip")
	require.NoError(t, existsErr)
	assert.False(t, exists, "the partial archive is removed")

	require.NoError(t, writeZip(backend, "export/fine.zip", func(zw *zip.Writer) error {
		_, err := createZipEntry(zw, "messages.csv", time.Now())
		return err
	}), "the next archive is written as usual")
}
