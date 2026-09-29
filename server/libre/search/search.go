// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

// Package search registers the Elasticsearch / OpenSearch search engine and
// its jobs (bulk indexing, post index aggregation) with the server.
package search

import (
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/v8/channels/app"
	"github.com/mattermost/mattermost/server/v8/channels/app/platform"
	ejobs "github.com/mattermost/mattermost/server/v8/einterfaces/jobs"
	"github.com/mattermost/mattermost/server/v8/libre/search/engine"
	"github.com/mattermost/mattermost/server/v8/libre/search/jobs"
	"github.com/mattermost/mattermost/server/v8/platform/services/searchengine"
)

func init() {
	platform.RegisterElasticsearchInterface(func(ps *platform.PlatformService) searchengine.SearchEngineInterface {
		return engine.New(ps.Config(), ps.Log, ps.GetConfigFile)
	})
	app.RegisterJobsElasticsearchIndexerInterface(func(s *app.Server) ejobs.IndexerJobInterface {
		return &indexerJob{server: s}
	})
	app.RegisterJobsElasticsearchAggregatorInterface(func(s *app.Server) ejobs.ElasticsearchAggregatorInterface {
		return &aggregatorJob{server: s}
	})
}

// searchEngine returns the running engine of the server, if any.
func searchEngine(s *app.Server) *engine.Engine {
	broker := s.Platform().SearchEngine
	if broker == nil {
		return nil
	}
	e, _ := broker.ElasticsearchEngine.(*engine.Engine)
	return e
}

type indexerJob struct {
	server *app.Server
}

func (j *indexerJob) MakeWorker() model.Worker {
	return jobs.NewIndexerWorker(j.server.Jobs, j.server.Store(), func() jobs.BulkIndexer {
		if e := searchEngine(j.server); e != nil {
			return e
		}
		return nil
	})
}

type aggregatorJob struct {
	server *app.Server
}

func (j *aggregatorJob) MakeWorker() model.Worker {
	return jobs.NewAggregatorWorker(j.server.Jobs, func() jobs.PostAggregator {
		if e := searchEngine(j.server); e != nil {
			return e
		}
		return nil
	})
}

func (j *aggregatorJob) MakeScheduler() ejobs.Scheduler {
	return jobs.NewAggregatorScheduler(j.server.Jobs)
}
