// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package aiservice

import (
	"crypto/rand"
	"encoding/base64"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func randomKey(t *testing.T) []byte {
	t.Helper()
	raw := make([]byte, keySize)
	_, err := rand.Read(raw)
	require.NoError(t, err)
	return raw
}

func TestKeyringRoundTrip(t *testing.T) {
	kr, err := NewKeyring(randomKey(t))
	require.NoError(t, err)

	sealed, err := kr.Seal("sk-secret-value", "aiconnection:abc:api_key")
	require.NoError(t, err)
	assert.NotContains(t, sealed, "sk-secret-value")

	data, err := base64.StdEncoding.DecodeString(sealed)
	require.NoError(t, err)
	assert.Equal(t, sealVersion, data[0], "a version byte first")

	again, err := kr.Seal("sk-secret-value", "aiconnection:abc:api_key")
	require.NoError(t, err)
	assert.NotEqual(t, sealed, again, "a fresh nonce each time")

	plain, err := kr.Open(sealed, "aiconnection:abc:api_key")
	require.NoError(t, err)
	assert.Equal(t, "sk-secret-value", plain)

	empty, err := kr.Seal("", "x")
	require.NoError(t, err)
	assert.Empty(t, empty)
	plain, err = kr.Open("", "x")
	require.NoError(t, err)
	assert.Empty(t, plain)
}

func TestKeyringWrongKeyFails(t *testing.T) {
	kr, err := NewKeyring(randomKey(t))
	require.NoError(t, err)
	other, err := NewKeyring(randomKey(t))
	require.NoError(t, err)

	sealed, err := kr.Seal("secret", "aad")
	require.NoError(t, err)
	_, err = other.Open(sealed, "aad")
	assert.ErrorIs(t, err, ErrWrongKey)
}

func TestKeyringTamperedFails(t *testing.T) {
	kr, err := NewKeyring(randomKey(t))
	require.NoError(t, err)
	sealed, err := kr.Seal("secret", "aiconnection:abc:api_key")
	require.NoError(t, err)

	data, err := base64.StdEncoding.DecodeString(sealed)
	require.NoError(t, err)
	data[len(data)-1] ^= 0x01
	_, err = kr.Open(base64.StdEncoding.EncodeToString(data), "aiconnection:abc:api_key")
	assert.ErrorIs(t, err, ErrTampered, "a changed ciphertext")

	_, err = kr.Open(sealed, "aiconnection:other:api_key")
	assert.ErrorIs(t, err, ErrTampered, "a ciphertext moved to another connection")

	_, err = kr.Open("not base64!", "aad")
	assert.ErrorIs(t, err, ErrTampered)

	data, _ = base64.StdEncoding.DecodeString(sealed)
	data[0] = 9
	_, err = kr.Open(base64.StdEncoding.EncodeToString(data), "aiconnection:abc:api_key")
	assert.ErrorIs(t, err, ErrUnknownVersion)
}

func TestKeyringRotation(t *testing.T) {
	oldKey, newKey := randomKey(t), randomKey(t)
	old, err := NewKeyring(oldKey)
	require.NoError(t, err)
	sealed, err := old.Seal("secret", "aad")
	require.NoError(t, err)

	rotated, err := NewKeyring(newKey, oldKey)
	require.NoError(t, err)
	plain, err := rotated.Open(sealed, "aad")
	require.NoError(t, err)
	assert.Equal(t, "secret", plain)
	assert.True(t, rotated.NeedsRotation(sealed))

	resealed, err := rotated.Seal(plain, "aad")
	require.NoError(t, err)
	assert.False(t, rotated.NeedsRotation(resealed))
}

func TestLoadKeyring(t *testing.T) {
	envKey := base64.StdEncoding.EncodeToString(randomKey(t))

	t.Run("from the environment", func(t *testing.T) {
		dir := t.TempDir()
		kr, source, _, err := LoadKeyring(LoadKeyringOptions{
			Lookup:        func(name string) (string, bool) { return envKey, name == EncryptionKeyEnv },
			DataDir:       dir,
			MayCreateFile: true,
		})
		require.NoError(t, err)
		assert.Equal(t, KeySourceEnvironment, source)
		assert.NotNil(t, kr)
		_, err = os.Stat(filepath.Join(dir, keyFileName))
		assert.True(t, os.IsNotExist(err), "no key file when the variable sets the key")
	})

	// The compose files pass AM_AI_ENCRYPTION_KEY: ${AM_AI_ENCRYPTION_KEY:-}, so the variable is set but empty
	// when .env has no key: the server must then use its key file, not fail.
	t.Run("an empty variable falls back to the key file", func(t *testing.T) {
		dir := t.TempDir()
		empty := func(name string) (string, bool) {
			return " ", name == EncryptionKeyEnv || name == PreviousKeysEnv
		}
		kr, source, path, err := LoadKeyring(LoadKeyringOptions{Lookup: empty, DataDir: dir, MayCreateFile: true})
		require.NoError(t, err)
		assert.Equal(t, KeySourceFile, source)
		assert.Equal(t, filepath.Join(dir, keyFileName), path)
		sealed, err := kr.Seal("secret", "aad")
		require.NoError(t, err)

		again, source, _, err := LoadKeyring(LoadKeyringOptions{Lookup: empty, DataDir: dir, MayCreateFile: true})
		require.NoError(t, err)
		assert.Equal(t, KeySourceFile, source)
		plain, err := again.Open(sealed, "aad")
		require.NoError(t, err)
		assert.Equal(t, "secret", plain, "the same key file on the next start")
	})

	t.Run("a bad variable is an error, not a fallback", func(t *testing.T) {
		_, _, _, err := LoadKeyring(LoadKeyringOptions{
			Lookup:        func(name string) (string, bool) { return "c2hvcnQ=", name == EncryptionKeyEnv },
			DataDir:       t.TempDir(),
			MayCreateFile: true,
		})
		assert.Error(t, err)
	})

	t.Run("a key file is created once, 0600, and reused", func(t *testing.T) {
		dir := t.TempDir()
		kr, source, path, err := LoadKeyring(LoadKeyringOptions{DataDir: dir, MayCreateFile: true})
		require.NoError(t, err)
		assert.Equal(t, KeySourceFile, source)
		info, err := os.Stat(path)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())

		sealed, err := kr.Seal("secret", "aad")
		require.NoError(t, err)
		again, _, _, err := LoadKeyring(LoadKeyringOptions{DataDir: dir, MayCreateFile: true})
		require.NoError(t, err)
		plain, err := again.Open(sealed, "aad")
		require.NoError(t, err)
		assert.Equal(t, "secret", plain)

		leftovers, err := filepath.Glob(filepath.Join(dir, keyFileName+".tmp-*"))
		require.NoError(t, err)
		assert.Empty(t, leftovers)
	})

	t.Run("servers starting together share one key", func(t *testing.T) {
		dir := t.TempDir()
		var wg sync.WaitGroup
		keyrings := make([]*Keyring, 8)
		for i := range keyrings {
			wg.Go(func() {
				kr, _, _, err := LoadKeyring(LoadKeyringOptions{DataDir: dir, MayCreateFile: true})
				assert.NoError(t, err)
				keyrings[i] = kr
			})
		}
		wg.Wait()
		for _, kr := range keyrings[1:] {
			require.NotNil(t, kr)
			assert.Equal(t, keyrings[0].current.fingerprint, kr.current.fingerprint)
		}
	})

	t.Run("no file may be created", func(t *testing.T) {
		_, _, _, err := LoadKeyring(LoadKeyringOptions{DataDir: t.TempDir(), MayCreateFile: false})
		assert.ErrorIs(t, err, ErrNoKey)
		_, _, _, err = LoadKeyring(LoadKeyringOptions{})
		assert.ErrorIs(t, err, ErrNoKey)
	})
}
