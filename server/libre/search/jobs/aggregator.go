// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package jobs

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	mmjobs "github.com/mattermost/mattermost/server/v8/channels/jobs"
)

const aggregationTimeout = 6 * time.Hour

// PostAggregator is the subset of the search engine used by the aggregation
// job.
type PostAggregator interface {
	IsActive() bool
	AggregatePostIndexes(ctx context.Context, cutoff time.Time) (int, *model.AppError)
}

// AggregationCutoff returns the time before which daily post indexes are
// merged into monthly ones.
func AggregationCutoff(cfg *model.Config, now time.Time) time.Time {
	days := model.SafeDereference(cfg.ElasticsearchSettings.AggregatePostsAfterDays)
	if days < 1 {
		days = model.ElasticsearchSettingsDefaultAggregatePostsAfterDays
	}
	return now.UTC().AddDate(0, 0, -days)
}

// RunAggregation executes one aggregation job.
func RunAggregation(cfg *model.Config, aggregator PostAggregator, logger mlog.LoggerIFace, job *model.Job) error {
	if aggregator == nil || !aggregator.IsActive() {
		return model.NewAppError("AggregatorWorker", "ent.elasticsearch.not_started.error", map[string]any{"Backend": "Search engine"}, "", http.StatusInternalServerError)
	}
	ctx, cancel := context.WithTimeout(context.Background(), aggregationTimeout)
	defer cancel()

	cutoff := AggregationCutoff(cfg, time.Now())
	merged, appErr := aggregator.AggregatePostIndexes(ctx, cutoff)
	if job != nil {
		if job.Data == nil {
			job.Data = model.StringMap{}
		}
		job.Data["merged_indexes"] = strconv.Itoa(merged)
		job.Data["cutoff"] = cutoff.Format(time.DateOnly)
	}
	if appErr != nil {
		return appErr
	}
	logger.Info("Post index aggregation completed", mlog.Int("merged_indexes", merged), mlog.String("cutoff", cutoff.Format(time.DateOnly)))
	return nil
}

func aggregationEnabled(cfg *model.Config) bool {
	return cfg != nil && model.SafeDereference(cfg.ElasticsearchSettings.EnableIndexing)
}

// NewAggregatorWorker returns the worker of the post aggregation job.
func NewAggregatorWorker(jobServer *mmjobs.JobServer, getAggregator func() PostAggregator) model.Worker {
	execute := func(logger mlog.LoggerIFace, job *model.Job) error {
		return RunAggregation(jobServer.Config(), getAggregator(), logger, job)
	}
	return mmjobs.NewSimpleWorker("SearchPostAggregator", jobServer, execute, aggregationEnabled)
}

// NewAggregatorScheduler returns the daily scheduler of the post aggregation
// job, running at PostsAggregatorJobStartTime.
func NewAggregatorScheduler(jobServer *mmjobs.JobServer) *mmjobs.DailyScheduler {
	startTime := func(cfg *model.Config) *time.Time {
		t, err := parseStartTime(model.SafeDereference(cfg.ElasticsearchSettings.PostsAggregatorJobStartTime))
		if err != nil {
			jobServer.Logger().Warn("Invalid PostsAggregatorJobStartTime, using the default", mlog.Err(err))
			t, _ = parseStartTime(model.ElasticsearchSettingsDefaultPostsAggregatorJobStartTime)
		}
		return &t
	}
	return mmjobs.NewDailyScheduler(jobServer, model.JobTypeElasticsearchPostAggregation, startTime, aggregationEnabled)
}

func parseStartTime(value string) (time.Time, error) {
	t, err := time.Parse("15:04", value)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid time %q: %w", value, err)
	}
	return t, nil
}
