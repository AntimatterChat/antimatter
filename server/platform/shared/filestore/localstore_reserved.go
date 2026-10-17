// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package filestore

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
)

// Antimatter keeps a file of its own at the root of the local file storage: the key that encrypts the model
// connections' API keys (AIKeyFileName), when no AM_AI_ENCRYPTION_KEY sets it. Nothing that goes through the file
// backend may read, list, copy, move, overwrite or delete it, nor the temporary files it is made with: apps reach the
// storage through the plugin API (ReadFile, ListDirectory...), and exports and archives walk it. To everything but the
// server's keyring, the file isn't there.

// AIKeyFileName is the AI encryption key file, at the root of the local file storage.
const AIKeyFileName = "ai-encryption.key"

// aiKeyTempPrefix starts the names of the temporary files the key file is made with.
const aiKeyTempPrefix = AIKeyFileName + ".tmp-"

// errReservedFile is what the reserved files give: they don't exist, as far as the callers can tell.
var errReservedFile = fmt.Errorf("%w: the file is reserved to the server", fs.ErrNotExist)

// IsReservedLocalPath tells whether a path of the local file storage is one of the server's own files.
func IsReservedLocalPath(path string) bool {
	clean := filepath.Clean(filepath.FromSlash(strings.TrimLeft(path, `/\`)))
	return clean == AIKeyFileName || strings.HasPrefix(clean, aiKeyTempPrefix)
}

// isRootPath tells whether a path names the root of the storage, which holds the reserved files.
func isRootPath(path string) bool {
	clean := filepath.Clean(filepath.FromSlash(strings.TrimLeft(path, `/\`)))
	return clean == "." || clean == ""
}

// relativeToRoot gives the path of a full path inside the storage's directory.
func (b *LocalFileBackend) relativeToRoot(full string) string {
	rel, err := filepath.Rel(filepath.Clean(b.directory), filepath.Clean(full))
	if err != nil {
		return full
	}
	return rel
}

// withoutReserved drops the server's own files from a listing.
func withoutReserved(paths []string) []string {
	return slices.DeleteFunc(paths, IsReservedLocalPath)
}
