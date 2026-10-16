// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package app

import (
	"errors"
	"fmt"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/mattermost/mattermost/server/v8/channels/jobs/dataretention"
	"github.com/mattermost/mattermost/server/v8/channels/jobs/messageexport"
	"github.com/mattermost/mattermost/server/v8/platform/services/searchengine"
)

// dataRetentionJobDeps returns what the data retention job needs from the
// server.
func (s *Server) dataRetentionJobDeps() dataretention.JobDeps {
	return dataretention.JobDeps{
		JobServer:   s.Jobs,
		Store:       s.Store,
		Config:      s.Config,
		FileBackend: s.FileBackend,
		SearchEngines: func() []searchengine.SearchEngineInterface {
			if s.Platform() == nil || s.Platform().SearchEngine == nil {
				return nil
			}
			return s.Platform().SearchEngine.GetActiveEngines()
		},
		RunPluginsDataRetention: func(nowMillis, batchSize int64) (int64, error) {
			ch := s.Channels()
			if ch == nil {
				return 0, nil
			}
			var total int64
			var errs []error
			ch.RunMultiHook(func(hooks plugin.Hooks, manifest *model.Manifest) bool {
				deleted, err := hooks.RunDataRetention(nowMillis, batchSize)
				if err != nil {
					errs = append(errs, fmt.Errorf("plugin %s: %w", manifest.Id, err))
				}
				total += deleted
				return true
			}, plugin.RunDataRetentionID)
			return total, errors.Join(errs...)
		},
	}
}

// messageExportDeps returns what the message export job and service need from
// the server.
func (s *Server) messageExportDeps() messageexport.Deps {
	return messageexport.Deps{
		JobServer:         s.Jobs,
		Store:             s.Store,
		Config:            s.Config,
		FileBackend:       s.FileBackend,
		ExportFileBackend: s.ExportFileBackend,
	}
}
