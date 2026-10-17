// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package aiservice

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/mattermost/mattermost/server/v8/platform/shared/filestore"
)

// The keys of model connections (and their extra headers) are encrypted at rest with AES-256-GCM.
//
// The key encryption key comes from AM_AI_ENCRYPTION_KEY (base64 of 32 bytes; MM_AI_ENCRYPTION_KEY is the older
// name), else from a key file the server creates, readable by it alone (0600), in its data directory. The file
// backend hides that file (filestore.AIKeyFileName) from everything else: apps, exports, archives. Older keys
// can still decrypt during a rotation through AM_AI_ENCRYPTION_PREVIOUS_KEYS (comma-separated, base64); what's
// encrypted again is encrypted with the current key.
//
// A sealed value is base64 of: a format version byte (1), the key's 4-byte fingerprint (the start of its SHA-256,
// so that a rotation knows which key to use), a 12-byte random nonce, then the AES-GCM ciphertext and its tag. The
// associated data binds a value to its row and field (e.g. "aiconnection:<id>:api_key"): a ciphertext copied to
// another connection doesn't decrypt.

const (
	sealVersion      byte = 1
	keySize               = 32
	fingerprintSize       = 4
	nonceSize             = 12
	keyFileName           = filestore.AIKeyFileName
	EncryptionKeyEnv      = "AI_ENCRYPTION_KEY"
	PreviousKeysEnv       = "AI_ENCRYPTION_PREVIOUS_KEYS"
)

var (
	ErrWrongKey       = errors.New("the value was encrypted with another key")
	ErrTampered       = errors.New("the encrypted value is damaged or was changed")
	ErrUnknownVersion = errors.New("the encrypted value has an unknown format")
)

// Keyring encrypts and decrypts with the current key, and decrypts with the previous ones too.
type Keyring struct {
	current  key
	previous []key
}

type key struct {
	aead        cipher.AEAD
	fingerprint [fingerprintSize]byte
}

func newKey(raw []byte) (key, error) {
	if len(raw) != keySize {
		return key{}, fmt.Errorf("an AI encryption key must be %d bytes, this one has %d", keySize, len(raw))
	}
	block, err := aes.NewCipher(raw)
	if err != nil {
		return key{}, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return key{}, err
	}
	sum := sha256.Sum256(raw)
	k := key{aead: aead}
	copy(k.fingerprint[:], sum[:fingerprintSize])
	return k, nil
}

// NewKeyring builds a keyring from raw 32-byte keys: the current one first, then older ones that may still decrypt.
func NewKeyring(current []byte, previous ...[]byte) (*Keyring, error) {
	k, err := newKey(current)
	if err != nil {
		return nil, err
	}
	kr := &Keyring{current: k}
	for _, raw := range previous {
		pk, err := newKey(raw)
		if err != nil {
			return nil, fmt.Errorf("a previous AI encryption key: %w", err)
		}
		kr.previous = append(kr.previous, pk)
	}
	return kr, nil
}

// DecodeKey reads a base64 key (standard or URL alphabet, padded or not).
func DecodeKey(encoded string) ([]byte, error) {
	encoded = strings.TrimSpace(encoded)
	for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		if raw, err := enc.DecodeString(encoded); err == nil {
			if len(raw) != keySize {
				return nil, fmt.Errorf("an AI encryption key must be %d bytes, this one has %d", keySize, len(raw))
			}
			return raw, nil
		}
	}
	return nil, errors.New("the AI encryption key isn't valid base64")
}

// Seal encrypts plaintext for the field named by aad. An empty plaintext stays empty.
func (kr *Keyring) Seal(plaintext, aad string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	out := make([]byte, 0, 1+fingerprintSize+nonceSize+len(plaintext)+kr.current.aead.Overhead())
	out = append(out, sealVersion)
	out = append(out, kr.current.fingerprint[:]...)
	nonce := make([]byte, nonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	out = append(out, nonce...)
	out = kr.current.aead.Seal(out, nonce, []byte(plaintext), []byte(aad))
	return base64.StdEncoding.EncodeToString(out), nil
}

// Open decrypts a value Seal made for the same aad. An empty value is an empty plaintext.
func (kr *Keyring) Open(sealed, aad string) (string, error) {
	if sealed == "" {
		return "", nil
	}
	data, err := base64.StdEncoding.DecodeString(sealed)
	if err != nil {
		return "", ErrTampered
	}
	if len(data) < 1+fingerprintSize+nonceSize {
		return "", ErrTampered
	}
	if data[0] != sealVersion {
		return "", ErrUnknownVersion
	}
	var fp [fingerprintSize]byte
	copy(fp[:], data[1:1+fingerprintSize])
	k, ok := kr.find(fp)
	if !ok {
		return "", ErrWrongKey
	}
	nonce := data[1+fingerprintSize : 1+fingerprintSize+nonceSize]
	plaintext, err := k.aead.Open(nil, nonce, data[1+fingerprintSize+nonceSize:], []byte(aad))
	if err != nil {
		return "", ErrTampered
	}
	return string(plaintext), nil
}

// NeedsRotation tells whether a sealed value was made with an older key, so it should be sealed again.
func (kr *Keyring) NeedsRotation(sealed string) bool {
	data, err := base64.StdEncoding.DecodeString(sealed)
	if err != nil || len(data) < 1+fingerprintSize {
		return false
	}
	var fp [fingerprintSize]byte
	copy(fp[:], data[1:1+fingerprintSize])
	return fp != kr.current.fingerprint
}

func (kr *Keyring) find(fp [fingerprintSize]byte) (key, bool) {
	if fp == kr.current.fingerprint {
		return kr.current, true
	}
	for _, k := range kr.previous {
		if fp == k.fingerprint {
			return k, true
		}
	}
	return key{}, false
}

// KeySource says where the current key came from, for the logs (never the key itself).
type KeySource string

const (
	KeySourceEnvironment KeySource = "environment"
	KeySourceFile        KeySource = "file"
)

// LoadKeyringOptions says where to look for the key.
type LoadKeyringOptions struct {
	// Lookup reads a server environment variable by its name without prefix (utils.LookupServerEnv: AM_, then MM_).
	Lookup func(name string) (string, bool)
	// DataDir is the server's data directory, where the key file lives when no variable sets the key.
	DataDir string
	// MayCreateFile allows creating the key file. A cluster whose nodes don't share the data directory must not:
	// each node would make its own key. Then the variable is required.
	MayCreateFile bool
}

// ErrNoKey says that no key is set and none may be created.
var ErrNoKey = errors.New("no AI encryption key: set AM_AI_ENCRYPTION_KEY (base64 of 32 random bytes), which a cluster whose nodes don't share the data directory needs")

// LoadKeyring finds the key: the environment variable first, else the key file in the data directory, created
// (0600, in a 0700 directory if it has to make it) when missing and allowed. Two servers starting together on a
// shared directory end up with the same key: the file is created exclusively, and the loser reads the winner's.
func LoadKeyring(opts LoadKeyringOptions) (*Keyring, KeySource, string, error) {
	var previous [][]byte
	if opts.Lookup != nil {
		if list, ok := opts.Lookup(PreviousKeysEnv); ok && strings.TrimSpace(list) != "" {
			for encoded := range strings.SplitSeq(list, ",") {
				if strings.TrimSpace(encoded) == "" {
					continue
				}
				raw, err := DecodeKey(encoded)
				if err != nil {
					return nil, "", "", fmt.Errorf("AM_%s: %w", PreviousKeysEnv, err)
				}
				previous = append(previous, raw)
			}
		}
		if encoded, ok := opts.Lookup(EncryptionKeyEnv); ok && strings.TrimSpace(encoded) != "" {
			raw, err := DecodeKey(encoded)
			if err != nil {
				return nil, "", "", fmt.Errorf("AM_%s: %w", EncryptionKeyEnv, err)
			}
			kr, err := NewKeyring(raw, previous...)
			return kr, KeySourceEnvironment, "", err
		}
	}

	if opts.DataDir == "" {
		return nil, "", "", ErrNoKey
	}
	path := filepath.Join(opts.DataDir, keyFileName)
	raw, err := readKeyFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		if !opts.MayCreateFile {
			return nil, "", path, ErrNoKey
		}
		raw, err = createKeyFile(path)
	}
	if err != nil {
		return nil, "", path, err
	}
	kr, err := NewKeyring(raw, previous...)
	return kr, KeySourceFile, path, err
}

func readKeyFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	raw, err := DecodeKey(string(data))
	if err != nil {
		return nil, fmt.Errorf("the AI encryption key file %s: %w", path, err)
	}
	return raw, nil
}

// createKeyFile writes a new random key to a temporary file and links it into place, which fails when the file
// already exists: a node that loses the race reads the winner's complete file, never a half-written one.
func createKeyFile(path string) ([]byte, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("can't create the directory of the AI encryption key file: %w", err)
	}
	raw := make([]byte, keySize)
	if _, err := rand.Read(raw); err != nil {
		return nil, err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), keyFileName+".tmp-*")
	if err != nil {
		return nil, fmt.Errorf("can't create the AI encryption key file: %w", err)
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return nil, err
	}
	if _, err := tmp.WriteString(base64.StdEncoding.EncodeToString(raw) + "\n"); err != nil {
		tmp.Close()
		return nil, fmt.Errorf("can't write the AI encryption key file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return nil, err
	}
	if err := tmp.Close(); err != nil {
		return nil, err
	}
	if err := os.Link(tmp.Name(), path); err != nil {
		if errors.Is(err, fs.ErrExist) {
			// Another node made it first.
			return readKeyFile(path)
		}
		return nil, fmt.Errorf("can't create the AI encryption key file: %w", err)
	}
	return raw, nil
}
