// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"bytes"
	"io"
	"net/http"
	"path/filepath"
	"slices"

	"github.com/pkg/errors"
	"golang.org/x/crypto/openpgp"       //nolint:staticcheck
	"golang.org/x/crypto/openpgp/armor" //nolint:staticcheck

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"

	"github.com/mattermost/mattermost/server/v8/channels/utils"
)

// GetPublicKey will return the actual public key saved in the `name` file.
func (a *App) GetPublicKey(name string) ([]byte, *model.AppError) {
	return a.Srv().getPublicKey(name)
}

func (s *Server) getPublicKey(name string) ([]byte, *model.AppError) {
	data, err := s.platform.GetConfigFile(name)
	if err != nil {
		return nil, model.NewAppError("GetPublicKey", "app.plugin.get_public_key.get_file.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	return data, nil
}

// AddPublicKey will add plugin public key to the config. Overwrites the previous file
func (a *App) AddPublicKey(name string, key io.Reader) *model.AppError {
	if isSamlFile(&a.Config().SamlSettings, name) {
		return model.NewAppError("AddPublicKey", "app.plugin.modify_saml.app_error", nil, "", http.StatusInternalServerError)
	}
	data, err := io.ReadAll(key)
	if err != nil {
		return model.NewAppError("AddPublicKey", "app.plugin.write_file.read.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	err = a.Srv().platform.SetConfigFile(name, data)
	if err != nil {
		return model.NewAppError("AddPublicKey", "app.plugin.write_file.saving.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	a.UpdateConfig(func(cfg *model.Config) {
		if !slices.Contains(cfg.PluginSettings.SignaturePublicKeyFiles, name) {
			cfg.PluginSettings.SignaturePublicKeyFiles = append(cfg.PluginSettings.SignaturePublicKeyFiles, name)
		}
	})

	return nil
}

// DeletePublicKey will delete plugin public key from the config.
func (a *App) DeletePublicKey(name string) *model.AppError {
	if isSamlFile(&a.Config().SamlSettings, name) {
		return model.NewAppError("AddPublicKey", "app.plugin.modify_saml.app_error", nil, "", http.StatusInternalServerError)
	}
	filename := filepath.Base(name)
	if err := a.Srv().platform.RemoveConfigFile(filename); err != nil {
		return model.NewAppError("DeletePublicKey", "app.plugin.delete_public_key.delete.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	a.UpdateConfig(func(cfg *model.Config) {
		cfg.PluginSettings.SignaturePublicKeyFiles = utils.RemoveStringFromSlice(filename, cfg.PluginSettings.SignaturePublicKeyFiles)
	})

	return nil
}

func (ch *Channels) verifyPlugin(logger *mlog.Logger, plugin, signature io.ReadSeeker) *model.AppError {
	_, appErr := ch.verifyPluginSigner(logger, plugin, signature)
	return appErr
}

// pluginSigner identifies the key that verified a plugin signature: a built-in key, by name, or
// an admin-configured public key file.
type pluginSigner struct {
	name    string
	builtin bool
}

// isAntimatter reports whether the plugin was signed with the Antimatter plugin signing key.
func (s pluginSigner) isAntimatter() bool {
	return s.builtin && s.name == antimatterPluginPublicKeyName
}

// verifyPluginSigner verifies the plugin's signature like verifyPlugin and tells which key
// verified it.
func (ch *Channels) verifyPluginSigner(logger *mlog.Logger, plugin, signature io.ReadSeeker) (pluginSigner, *model.AppError) {
	// First try the hard-coded public keys (Antimatter's, then Mattermost's).
	if name, ok := verifyPluginWithKeys(logger, builtinPluginPublicKeys, plugin, signature); ok {
		logger.Debug("Plugin signature verified using hard-coded public key", mlog.String("public_key", name))
		return pluginSigner{name: name, builtin: true}, nil
	}

	// If that fails, try any of the admin-configured public keys.
	var configuredKeys []pluginPublicKey
	for _, pk := range ch.srv.Config().PluginSettings.SignaturePublicKeyFiles {
		pkBytes, appErr := ch.srv.getPublicKey(pk)
		if appErr != nil {
			logger.Warn("Unable to read configured signature public key file", mlog.String("public_key_path", pk))
			continue
		}
		configuredKeys = append(configuredKeys, pluginPublicKey{name: pk, key: pkBytes})
	}
	if name, ok := verifyPluginWithKeys(logger, configuredKeys, plugin, signature); ok {
		logger.Debug("Plugin signature verified using configured public key", mlog.String("public_key_path", name))
		return pluginSigner{name: name}, nil
	}

	return pluginSigner{}, model.NewAppError("VerifyPlugin", "api.plugin.verify_plugin.app_error", nil, "", http.StatusInternalServerError)
}

// verifyPluginWithKeys checks the plugin's detached signature against each key in turn,
// rewinding both readers before each attempt, and returns the name of the first key that
// verifies it.
func verifyPluginWithKeys(logger *mlog.Logger, keys []pluginPublicKey, plugin, signature io.ReadSeeker) (string, bool) {
	for _, pk := range keys {
		if _, err := plugin.Seek(0, io.SeekStart); err != nil {
			logger.Warn("Unable to seek in plugin for public key", mlog.String("public_key", pk.name), mlog.Err(err))
			continue
		}
		if _, err := signature.Seek(0, io.SeekStart); err != nil {
			logger.Warn("Unable to seek in signature for public key", mlog.String("public_key", pk.name), mlog.Err(err))
			continue
		}
		if err := verifySignature(bytes.NewReader(pk.key), plugin, signature); err == nil {
			return pk.name, true
		}
	}

	return "", false
}

func verifySignature(publicKey, message, signature io.Reader) error {
	pk, err := decodeIfArmored(publicKey)
	if err != nil {
		return errors.Wrap(err, "can't decode public key")
	}
	s, err := decodeIfArmored(signature)
	if err != nil {
		return errors.Wrap(err, "can't decode signature")
	}
	return verifyBinarySignature(pk, message, s)
}

func verifyBinarySignature(publicKey, signedFile, signature io.Reader) error {
	keyring, err := openpgp.ReadKeyRing(publicKey)
	if err != nil {
		return errors.Wrap(err, "can't read public key")
	}
	if _, err = openpgp.CheckDetachedSignature(keyring, signedFile, signature); err != nil {
		return errors.Wrap(err, "error while checking the signature")
	}
	return nil
}

func decodeIfArmored(reader io.Reader) (io.Reader, error) {
	readBytes, err := io.ReadAll(reader)
	if err != nil {
		return nil, errors.Wrap(err, "can't read the file")
	}
	block, err := armor.Decode(bytes.NewReader(readBytes))
	if err != nil {
		return bytes.NewReader(readBytes), nil
	}
	return block.Body, nil
}

// isSamlFile checks if filename is a SAML file.
func isSamlFile(saml *model.SamlSettings, filename string) bool {
	return filename == *saml.PublicCertificateFile || filename == *saml.PrivateKeyFile || filename == *saml.IdpCertificateFile
}
