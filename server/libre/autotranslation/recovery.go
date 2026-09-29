// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package autotranslation

import (
	"errors"
	"sort"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/jobs"
	"github.com/mattermost/mattermost/server/v8/channels/store"
	ejobs "github.com/mattermost/mattermost/server/v8/einterfaces/jobs"
)

const (
	recoveryJobName = "AutoTranslationRecovery"
	// recoveryInterval is how often the sweep runs.
	recoveryInterval = 5 * time.Minute
	// recoveryMinAge is how long a translation must have been processing
	// before it is considered stuck.
	recoveryMinAge = 2 * time.Minute
	// recoveryGiveUpAge is the age after which a stuck translation is marked
	// unavailable instead of retried.
	recoveryGiveUpAge = time.Hour
	// recoveryBatchSize bounds the rows handled per sweep.
	recoveryBatchSize = 500
)

// MakeWorker returns the worker of the recovery sweep job.
func (s *Service) MakeWorker() model.Worker {
	js := s.h.JobServer()
	execute := func(logger mlog.LoggerIFace, job *model.Job) error {
		defer js.HandleJobPanic(logger, job)
		_, err := s.recoverStuck(request.EmptyContext(logger))
		return err
	}
	isEnabled := func(cfg *model.Config) bool {
		ok, _ := featureAvailable(cfg)
		return ok
	}
	return jobs.NewSimpleWorker(recoveryJobName, js, execute, isEnabled)
}

// MakeScheduler returns the periodic scheduler of the recovery sweep job.
func (s *Service) MakeScheduler() ejobs.Scheduler {
	isEnabled := func(cfg *model.Config) bool {
		ok, _ := featureAvailable(cfg)
		return ok
	}
	return jobs.NewPeriodicScheduler(s.h.JobServer(), model.JobTypeAutoTranslationRecovery, recoveryInterval, isEnabled)
}

// stuckMinAge is the processing age after which a translation is retried; it
// leaves room for a full pass over every configured language.
func (s *Service) stuckMinAge() time.Duration {
	if d := 4 * providerTimeout(s.h.Config()); d > recoveryMinAge {
		return d
	}
	return recoveryMinAge
}

// recoverStuck re-queues translations left in the processing state, and
// gives up on those that stayed stuck too long. It returns the number of
// stuck rows found.
func (s *Service) recoverStuck(rctx request.CTX) (int, error) {
	if ok, _ := featureAvailable(s.h.Config()); !ok {
		return 0, nil
	}

	now := time.Now()
	rows, err := s.h.Store().AutoTranslation().GetByStateOlderThan(
		model.TranslationStateProcessing,
		now.Add(-s.stuckMinAge()).UnixMilli(),
		recoveryBatchSize,
	)
	if err != nil {
		return 0, err
	}
	if m := s.h.Metrics(); m != nil && len(rows) > 0 {
		m.AddAutoTranslateRecoveryStuckFound(float64(len(rows)))
	}

	type objKey struct{ objectType, objectID string }
	grouped := make(map[objKey][]*model.Translation)
	var order []objKey
	for _, row := range rows {
		k := objKey{row.ObjectType, row.ObjectID}
		if _, ok := grouped[k]; !ok {
			order = append(order, k)
		}
		grouped[k] = append(grouped[k], row)
	}

	giveUpBefore := now.Add(-recoveryGiveUpAge).UnixMilli()
	for _, k := range order {
		if s.pool.isInflight(k.objectType, k.objectID) {
			continue
		}
		var retry, expired []*model.Translation
		for _, row := range grouped[k] {
			if row.UpdateAt < giveUpBefore {
				expired = append(expired, row)
			} else {
				retry = append(retry, row)
			}
		}
		s.recoverObject(rctx, k.objectType, k.objectID, retry, expired)
	}
	return len(rows), nil
}

// recoverObject re-queues the retryable rows of one object and marks the
// others unavailable.
func (s *Service) recoverObject(rctx request.CTX, objectType, objectID string, retry, expired []*model.Translation) {
	var (
		post    *model.Post
		content *sourceContent
		reason  = "translation timed out"
	)

	// Only posts can be reloaded: other object types were handed to
	// Translate by their owner and their content is not stored here.
	if objectType == model.TranslationObjectTypePost {
		p, err := s.h.Store().Post().GetSingle(rctx, objectID, false)
		var nfErr *store.ErrNotFound
		switch {
		case err == nil && p.DeleteAt == 0:
			if c, appErr := extractContent(p); appErr == nil {
				post, content = p, c
			} else {
				reason = appErr.Id
			}
		case err == nil || errors.As(err, &nfErr):
			reason = "post no longer exists"
		default:
			rctx.Logger().Warn("Auto-translation recovery failed to load post", mlog.String("post_id", objectID), mlog.Err(err))
			return
		}
	} else {
		reason = "content unavailable for recovery"
	}

	channelID := ""
	if post != nil {
		channelID = post.ChannelId
	}

	if content == nil {
		expired = append(expired, retry...)
		retry = nil
	}

	for _, row := range expired {
		row.ChannelID = channelID
		row.State = model.TranslationStateUnavailable
		if row.Provider == "" {
			row.Provider = s.providerName()
		}
		if row.Meta == nil {
			row.Meta = map[string]any{}
		}
		row.Meta[metaError] = reason
		if err := s.h.Store().AutoTranslation().Save(row); err != nil {
			rctx.Logger().Warn("Auto-translation recovery failed to update translation",
				mlog.String("object_id", objectID), mlog.String("lang", row.Lang), mlog.Err(err))
			continue
		}
		s.publish(&task{objectType: objectType, objectID: objectID, channelID: channelID, content: &sourceContent{}}, row)
	}

	if len(retry) == 0 {
		return
	}

	var langs []string
	for _, row := range retry {
		if row.NormHash != content.NormHash {
			// The post changed after the row was written: restart the
			// row from the current content.
			row.ChannelID = channelID
			row.NormHash = content.NormHash
			row.Type = content.Type
			if row.Provider == "" {
				row.Provider = s.providerName()
			}
			if err := s.h.Store().AutoTranslation().Save(row); err != nil {
				rctx.Logger().Warn("Auto-translation recovery failed to reset translation",
					mlog.String("object_id", objectID), mlog.String("lang", row.Lang), mlog.Err(err))
				continue
			}
		}
		langs = append(langs, row.Lang)
	}
	if len(langs) == 0 {
		return
	}
	sort.Strings(langs)

	_, locales, err := s.destinationLanguages(channelID)
	if err != nil {
		locales = nil
	}
	if err := s.pool.enqueue(&task{
		objectType: objectType,
		objectID:   objectID,
		channelID:  channelID,
		content:    content,
		langs:      langs,
		locales:    locales,
	}); err != nil {
		rctx.Logger().Warn("Auto-translation recovery failed to queue translation",
			mlog.String("object_id", objectID), mlog.Err(err))
	}
}
