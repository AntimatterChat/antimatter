// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package app

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/utils"
	"github.com/mattermost/mattermost/server/v8/channels/app/aiservice"
)

// aiServerState is the AI service's state on a server node: the keyring that seals the connections' keys, and the
// requests-per-minute limiter.
type aiServerState struct {
	mu      sync.Mutex
	keyring *aiservice.Keyring
	limiter *aiservice.Limiter
}

// aiKeyring gives the keyring of the connections' keys, loaded the first time it's needed.
//
// The key encryption key comes from AM_AI_ENCRYPTION_KEY, else from the key file ai-encryption.key in the server's
// data directory (FileSettings.Directory), which the server creates (0600) the first time. The data directory is
// where the server keeps what must outlive it but not sit in the database: a database dump or backup alone then
// doesn't reveal the keys; the config may live in the database too, so it isn't the place. The local file storage
// of a cluster has to be shared between its nodes, so they share the file too; a cluster storing its files
// elsewhere (S3) has no shared directory, and must set the variable: there the server doesn't create the file.
func (s *Server) aiKeyring() (*aiservice.Keyring, *model.AppError) {
	s.ai.mu.Lock()
	defer s.ai.mu.Unlock()
	if s.ai.keyring != nil {
		return s.ai.keyring, nil
	}
	cfg := s.Config()
	clustered := cfg.ClusterSettings.Enable != nil && *cfg.ClusterSettings.Enable
	sharedDir := cfg.FileSettings.DriverName != nil && *cfg.FileSettings.DriverName == model.ImageDriverLocal
	keyring, source, path, err := aiservice.LoadKeyring(aiservice.LoadKeyringOptions{
		Lookup:        utils.LookupServerEnv,
		DataDir:       model.SafeDereference(cfg.FileSettings.Directory),
		MayCreateFile: !clustered || sharedDir,
	})
	if err != nil {
		s.Log().Error("The AI encryption key isn't available: model connections can't keep keys", mlog.String("key_file", path), mlog.Err(err))
		return nil, model.NewAppError("aiKeyring", "app.ai.encryption_key.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	s.Log().Info("Loaded the AI encryption key", mlog.String("source", string(source)), mlog.String("key_file", path))
	s.ai.keyring = keyring
	return keyring, nil
}

// aiLimiter gives the node's requests-per-minute limiter.
func (s *Server) aiLimiter() *aiservice.Limiter {
	s.ai.mu.Lock()
	defer s.ai.mu.Unlock()
	if s.ai.limiter == nil {
		s.ai.limiter = aiservice.NewLimiter()
	}
	return s.ai.limiter
}

// sealAIConnectionSecrets encrypts a connection's key and headers, bound to its ID, and keeps their visible parts:
// the key's last four characters and the headers' names.
func (a *App) sealAIConnectionSecrets(c *model.AIConnection, apiKey *string, headers *map[string]string) *model.AppError {
	if apiKey == nil && headers == nil {
		return nil
	}
	keyring, appErr := a.Srv().aiKeyring()
	if appErr != nil {
		return appErr
	}
	if apiKey != nil {
		sealed, err := keyring.Seal(*apiKey, model.AIConnectionAAD(c.Id, model.AIConnectionFieldAPIKey))
		if err != nil {
			return model.NewAppError("sealAIConnectionSecrets", "app.ai.encryption_key.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
		}
		c.APIKeySealed = sealed
		c.APIKeyLast4 = model.AIAPIKeyLast4(*apiKey)
	}
	if headers != nil {
		sealed := ""
		if len(*headers) > 0 {
			data, err := json.Marshal(*headers)
			if err != nil {
				return model.NewAppError("sealAIConnectionSecrets", "app.ai.encryption_key.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
			}
			sealed, err = keyring.Seal(string(data), model.AIConnectionAAD(c.Id, model.AIConnectionFieldExtraHeaders))
			if err != nil {
				return model.NewAppError("sealAIConnectionSecrets", "app.ai.encryption_key.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
			}
		}
		c.ExtraHeadersSealed = sealed
		c.ExtraHeaderNames = model.AIHeaderNames(*headers)
	}
	return nil
}

// openAIConnectionSecrets decrypts a connection's key and headers, for a call only.
func (a *App) openAIConnectionSecrets(c *model.AIConnection) (string, map[string]string, *model.AppError) {
	if c.APIKeySealed == "" && c.ExtraHeadersSealed == "" {
		return "", nil, nil
	}
	keyring, appErr := a.Srv().aiKeyring()
	if appErr != nil {
		return "", nil, appErr
	}
	apiKey, err := keyring.Open(c.APIKeySealed, model.AIConnectionAAD(c.Id, model.AIConnectionFieldAPIKey))
	if err != nil {
		return "", nil, model.NewAppError("openAIConnectionSecrets", "app.ai.connection.key_unreadable.app_error", nil, "connection_id="+c.Id, http.StatusInternalServerError).Wrap(err)
	}
	var headers map[string]string
	if c.ExtraHeadersSealed != "" {
		raw, err := keyring.Open(c.ExtraHeadersSealed, model.AIConnectionAAD(c.Id, model.AIConnectionFieldExtraHeaders))
		if err != nil {
			return "", nil, model.NewAppError("openAIConnectionSecrets", "app.ai.connection.key_unreadable.app_error", nil, "connection_id="+c.Id, http.StatusInternalServerError).Wrap(err)
		}
		if err := json.Unmarshal([]byte(raw), &headers); err != nil {
			return "", nil, model.NewAppError("openAIConnectionSecrets", "app.ai.connection.key_unreadable.app_error", nil, "connection_id="+c.Id, http.StatusInternalServerError).Wrap(err)
		}
	}
	return apiKey, headers, nil
}
