// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package app

import (
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/v8/platform/services/searchengine/elasticsearch"
	esjobs "github.com/mattermost/mattermost/server/v8/platform/services/searchengine/elasticsearch/jobs"
)

// elasticsearchEngine returns the Elasticsearch/OpenSearch engine of the
// server, if any (tests may register another implementation).
func (s *Server) elasticsearchEngine() *elasticsearch.Engine {
	broker := s.Platform().SearchEngine
	if broker == nil {
		return nil
	}
	e, _ := broker.ElasticsearchEngine.(*elasticsearch.Engine)
	return e
}

// newElasticsearchIndexerWorker creates the worker of the bulk indexing jobs.
func (s *Server) newElasticsearchIndexerWorker() model.Worker {
	return esjobs.NewIndexerWorker(s.Jobs, s.Store(), func() esjobs.BulkIndexer {
		if e := s.elasticsearchEngine(); e != nil {
			return e
		}
		return nil
	})
}

// newElasticsearchAggregatorWorker creates the worker of the post index
// aggregation jobs.
func (s *Server) newElasticsearchAggregatorWorker() model.Worker {
	return esjobs.NewAggregatorWorker(s.Jobs, func() esjobs.PostAggregator {
		if e := s.elasticsearchEngine(); e != nil {
			return e
		}
		return nil
	})
}
