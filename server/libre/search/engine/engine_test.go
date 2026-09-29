// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package engine

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

func forBothBackends(t *testing.T, fn func(t *testing.T, f *fakeCluster)) {
	for _, openSearch := range []bool{false, true} {
		name := "elasticsearch"
		if openSearch {
			name = "opensearch"
		}
		t.Run(name, func(t *testing.T) {
			fn(t, newFakeCluster(t, openSearch))
		})
	}
}

func TestStartStop(t *testing.T) {
	forBothBackends(t, func(t *testing.T, f *fakeCluster) {
		e := New(f.config(), nil, nil)
		assert.False(t, e.IsActive())
		assert.False(t, e.IsHealthy())
		assert.True(t, e.IsEnabled())

		require.Nil(t, e.Start(t.Context()))
		assert.True(t, e.IsActive())
		assert.True(t, e.IsHealthy())
		assert.True(t, e.IsIndexingEnabled())
		assert.True(t, e.IsSearchEnabled())
		assert.True(t, e.IsAutocompletionEnabled())
		assert.False(t, e.IsIndexingSync())
		assert.Equal(t, []string{"analysis-icu"}, e.GetPlugins())
		if f.openSearch {
			assert.Equal(t, "opensearch", e.GetName())
			assert.Equal(t, "2.17.1", e.GetFullVersion())
			assert.Equal(t, 2, e.GetVersion())
		} else {
			assert.Equal(t, "elasticsearch", e.GetName())
			assert.Equal(t, "8.15.0", e.GetFullVersion())
			assert.Equal(t, 8, e.GetVersion())
		}

		// Templates for every index kind, and the fixed indexes.
		assert.Len(t, f.templates, 4)
		var tpl map[string]any
		require.NoError(t, json.Unmarshal(f.templates["mm_mattermost_libre_posts"], &tpl))
		assert.Equal(t, []any{"mm_posts_*"}, tpl["index_patterns"])
		assert.Equal(t, []string{"mm_channels", "mm_files", "mm_users"}, f.indexNames())

		require.Nil(t, e.HealthCheck(request.TestContext(t)))

		// Starting twice is a no-op.
		require.Nil(t, e.Start(t.Context()))

		require.Nil(t, e.Stop())
		assert.False(t, e.IsActive())
		assert.False(t, e.IsHealthy())
		assert.NotNil(t, e.HealthCheck(request.TestContext(t)))
		// Version information is kept for the support packet.
		assert.NotEmpty(t, e.GetFullVersion())
		require.Nil(t, e.Stop())
	})
}

func TestStartDisabled(t *testing.T) {
	f := newFakeCluster(t, false)
	cfg := f.config()
	cfg.ElasticsearchSettings.EnableIndexing = model.NewPointer(false)
	e := New(cfg, nil, nil)
	require.Nil(t, e.Start(t.Context()))
	assert.False(t, e.IsActive())
	assert.False(t, e.IsEnabled())
}

func TestStartBackendMismatch(t *testing.T) {
	t.Run("opensearch server with elasticsearch backend", func(t *testing.T) {
		f := newFakeCluster(t, true)
		cfg := f.config()
		cfg.ElasticsearchSettings.Backend = model.NewPointer(model.ElasticsearchSettingsESBackend)
		e := New(cfg, nil, nil)
		// The Elasticsearch client refuses servers which are not Elasticsearch.
		appErr := e.Start(t.Context())
		require.NotNil(t, appErr)
		assert.Equal(t, "ent.elasticsearch.start.get_server_version.app_error", appErr.Id)
		assert.False(t, e.IsActive())
	})
	t.Run("elasticsearch server with opensearch backend", func(t *testing.T) {
		f := newFakeCluster(t, false)
		cfg := f.config()
		cfg.ElasticsearchSettings.Backend = model.NewPointer(model.ElasticsearchSettingsOSBackend)
		e := New(cfg, nil, nil)
		appErr := e.Start(t.Context())
		require.NotNil(t, appErr)
		assert.False(t, e.IsActive())
	})
}

func TestStartUnreachable(t *testing.T) {
	cfg := &model.Config{}
	cfg.SetDefaults()
	cfg.ElasticsearchSettings.ConnectionURL = model.NewPointer("http://127.0.0.1:1")
	cfg.ElasticsearchSettings.EnableIndexing = model.NewPointer(true)
	cfg.ElasticsearchSettings.Sniff = model.NewPointer(false)
	cfg.ElasticsearchSettings.RequestTimeoutSeconds = model.NewPointer(1)
	e := New(cfg, nil, nil)
	appErr := e.Start(t.Context())
	require.NotNil(t, appErr)
	assert.False(t, e.IsActive())
}

func TestTestConfig(t *testing.T) {
	forBothBackends(t, func(t *testing.T, f *fakeCluster) {
		e := New(nil, nil, nil)
		require.Nil(t, e.TestConfig(request.TestContext(t), f.config()))
		assert.Len(t, f.templates, 4)

		cfg := f.config()
		cfg.ElasticsearchSettings.EnableIndexing = model.NewPointer(false)
		appErr := e.TestConfig(request.TestContext(t), cfg)
		require.NotNil(t, appErr)
		assert.Equal(t, "ent.elasticsearch.test_config.indexing_disabled.error", appErr.Id)

		cfg = f.config()
		cfg.ElasticsearchSettings.CA = model.NewPointer("/nonexistent/ca.pem")
		appErr = e.TestConfig(request.TestContext(t), cfg)
		require.NotNil(t, appErr)
		assert.Equal(t, "ent.elasticsearch.create_client.ca_cert_missing", appErr.Id)
	})
}

func TestSetHealthy(t *testing.T) {
	f := newFakeCluster(t, false)
	e := startedEngine(t, f, nil)
	e.SetHealthy(false)
	assert.False(t, e.IsHealthy())
	e.SetHealthy(true)
	assert.True(t, e.IsHealthy())
	require.Nil(t, e.Stop())
	e.SetHealthy(true)
	assert.False(t, e.IsHealthy(), "a stopped engine can't become healthy")
}

func TestFlagsFollowConfig(t *testing.T) {
	f := newFakeCluster(t, false)
	e := startedEngine(t, f, nil)

	cfg := f.config()
	cfg.ElasticsearchSettings.EnableSearching = model.NewPointer(false)
	cfg.ElasticsearchSettings.EnableAutocomplete = model.NewPointer(false)
	e.UpdateConfig(cfg)
	assert.True(t, e.IsIndexingEnabled())
	assert.False(t, e.IsSearchEnabled())
	assert.False(t, e.IsAutocompletionEnabled())

	_, _, appErr := e.SearchPosts(model.ChannelList{{Id: "c1"}}, []*model.SearchParams{{Terms: "x"}}, 0, 10)
	require.NotNil(t, appErr)
	assert.Equal(t, "ent.elasticsearch.search_posts.disabled", appErr.Id)
	_, appErr = e.SearchChannels("t", "u", "x", false, false)
	require.NotNil(t, appErr)
}

func TestUpdateConfigReinstallsTemplates(t *testing.T) {
	f := newFakeCluster(t, false)
	e := startedEngine(t, f, nil)

	cfg := f.config()
	cfg.ElasticsearchSettings.IndexPrefix = model.NewPointer("other_")
	e.UpdateConfig(cfg)

	require.Eventually(t, func() bool {
		f.mu.Lock()
		defer f.mu.Unlock()
		_, ok := f.templates["other_mattermost_libre_posts"]
		_, idx := f.indexes["other_users"]
		return ok && idx
	}, 5*time.Second, 20*time.Millisecond)
}

func TestIndexPostAndDelete(t *testing.T) {
	forBothBackends(t, func(t *testing.T, f *fakeCluster) {
		e := startedEngine(t, f, nil)
		createAt := time.Date(2024, 3, 15, 23, 30, 0, 0, time.UTC).UnixMilli()
		post := &model.Post{
			Id:        model.NewId(),
			ChannelId: "channel1",
			UserId:    "user1",
			CreateAt:  createAt,
			Message:   "Hello #World",
			Hashtags:  "#World",
		}
		post.AddProp(model.PostPropsAttachments, []*model.MessageAttachment{{Title: "Build", Text: "failed", Fields: []*model.MessageAttachmentField{{Title: "Branch", Value: "main"}}}})

		require.Nil(t, e.IndexPost(post, "team1", "O"))
		doc := f.doc("mm_posts_2024_03_15", post.Id)
		require.NotNil(t, doc)
		assert.Equal(t, "Hello #World", doc["message"])
		assert.Equal(t, "team1", doc["team_id"])
		assert.Equal(t, "O", doc["channel_type"])
		assert.Equal(t, []any{"#world"}, doc["hashtags"])
		assert.Equal(t, "Build\nfailed\nBranch\nmain", doc["attachments"])

		// System messages are never indexed.
		sys := &model.Post{Id: model.NewId(), ChannelId: "channel1", CreateAt: createAt, Type: model.PostTypeJoinChannel}
		require.Nil(t, e.IndexPost(sys, "team1", "O"))
		assert.Nil(t, f.doc("mm_posts_2024_03_15", sys.Id))

		// A deleted post is removed from the index.
		deleted := post.Clone()
		deleted.DeleteAt = model.GetMillis()
		require.Nil(t, e.IndexPost(deleted, "team1", "O"))
		assert.Nil(t, f.doc("mm_posts_2024_03_15", post.Id))

		require.Nil(t, e.IndexPost(post, "team1", "O"))
		require.Nil(t, e.DeletePost(post))
		assert.Nil(t, f.doc("mm_posts_2024_03_15", post.Id))

		// Deleting a missing document is not an error.
		require.Nil(t, e.DeletePost(post))
	})
}

func TestLiveIndexingBatching(t *testing.T) {
	f := newFakeCluster(t, false)
	e := startedEngine(t, f, func(cfg *model.Config) {
		cfg.ElasticsearchSettings.LiveIndexingBatchSize = model.NewPointer(3)
	})

	users := []*model.User{{Id: "u1", Username: "a"}, {Id: "u2", Username: "b"}, {Id: "u3", Username: "c"}}
	require.Nil(t, e.IndexUser(request.TestContext(t), users[0], nil, nil))
	require.Nil(t, e.IndexUser(request.TestContext(t), users[1], nil, nil))
	assert.Nil(t, f.doc("mm_users", "u1"), "writes are buffered")
	require.Nil(t, e.IndexUser(request.TestContext(t), users[2], nil, nil))

	require.Eventually(t, func() bool {
		return f.doc("mm_users", "u1") != nil && f.doc("mm_users", "u3") != nil
	}, 5*time.Second, 20*time.Millisecond)
	assert.Len(t, f.requestsTo("/_bulk"), 1)

	// Stop flushes what is pending.
	require.Nil(t, e.IndexUser(request.TestContext(t), &model.User{Id: "u4", Username: "d"}, nil, nil))
	require.Nil(t, e.Stop())
	assert.NotNil(t, f.doc("mm_users", "u4"))
}

func TestBulkFailures(t *testing.T) {
	f := newFakeCluster(t, false)
	e := startedEngine(t, f, nil)
	f.bulkStatus = func(op, index, id string) int {
		if id == "bad" {
			return http.StatusBadRequest
		}
		return 0
	}
	appErr := e.IndexUser(request.TestContext(t), &model.User{Id: "bad"}, nil, nil)
	require.NotNil(t, appErr)
	assert.Equal(t, "ent.elasticsearch.index_user.error", appErr.Id)

	failures, appErr := e.BulkIndex(t.Context(), []BulkAction{
		e.UserAction(&model.User{Id: "good"}, nil, nil),
		e.UserAction(&model.User{Id: "bad"}, nil, nil),
	})
	require.Nil(t, appErr)
	require.Len(t, failures, 1)
	assert.Equal(t, "bad", failures[0].ID)
	assert.NotNil(t, f.doc("mm_users", "good"))
}

func TestBulkRetriesOnTooManyRequests(t *testing.T) {
	f := newFakeCluster(t, false)
	e := startedEngine(t, f, nil)
	attempts := 0
	f.bulkStatus = func(op, index, id string) int {
		if id == "busy" {
			attempts++
			if attempts < 2 {
				return http.StatusTooManyRequests
			}
		}
		return 0
	}
	failures, appErr := e.BulkIndex(t.Context(), []BulkAction{e.UserAction(&model.User{Id: "busy"}, nil, nil)})
	require.Nil(t, appErr)
	assert.Empty(t, failures)
	assert.Equal(t, 2, attempts)
}

func TestIndexChannel(t *testing.T) {
	f := newFakeCluster(t, true)
	e := startedEngine(t, f, nil)
	rctx := request.TestContext(t)

	private := &model.Channel{Id: "p1", TeamId: "t1", Type: model.ChannelTypePrivate, Name: "secret", DisplayName: "Secret", Discoverable: true}
	require.Nil(t, e.IndexChannel(rctx, private, []string{"u1", "u2"}, []string{"u1", "u2", "u3"}))
	doc := f.doc("mm_channels", "p1")
	require.NotNil(t, doc)
	assert.Equal(t, []any{"u1", "u2"}, doc["user_ids"])
	assert.Equal(t, true, doc["discoverable"])
	assert.NotContains(t, doc, "team_member_ids")

	dm := &model.Channel{Id: "d1", Type: model.ChannelTypeDirect, Name: "a__b"}
	require.Nil(t, e.IndexChannel(rctx, dm, nil, nil))
	assert.Nil(t, f.doc("mm_channels", "d1"))

	public := &model.Channel{Id: "o1", TeamId: "t1", Type: model.ChannelTypeOpen, Name: "town-square", DisplayName: "Town Square"}
	require.Nil(t, e.SyncBulkIndexChannels(rctx, []*model.Channel{public, private, dm}, func(ch *model.Channel) ([]string, error) {
		return []string{"u9"}, nil
	}, nil))
	assert.NotNil(t, f.doc("mm_channels", "o1"))
	assert.Equal(t, []any{"u9"}, f.doc("mm_channels", "p1")["user_ids"])
	assert.Equal(t, []any{}, f.doc("mm_channels", "o1")["user_ids"])

	require.Nil(t, e.DeleteChannel(public))
	assert.Nil(t, f.doc("mm_channels", "o1"))
}

func TestIndexFile(t *testing.T) {
	f := newFakeCluster(t, false)
	e := startedEngine(t, f, nil)

	file := &model.FileInfo{Id: "f1", CreatorId: "u1", PostId: "p1", Name: "Report-2024.PDF", Extension: "PDF", Content: "quarterly results", CreateAt: 1}
	require.Nil(t, e.IndexFile(file, "c1"))
	doc := f.doc("mm_files", "f1")
	require.NotNil(t, doc)
	assert.Equal(t, "pdf", doc["extension"])
	assert.Equal(t, "c1", doc["channel_id"])
	assert.Equal(t, "quarterly results", doc["content"])

	// Files not attached to a post are removed.
	orphan := &model.FileInfo{Id: "f1", CreatorId: "u1"}
	require.Nil(t, e.IndexFile(orphan, "c1"))
	assert.Nil(t, f.doc("mm_files", "f1"))

	require.Nil(t, e.IndexFile(file, "c1"))
	require.Nil(t, e.DeleteFile("f1"))
	assert.Nil(t, f.doc("mm_files", "f1"))
}

func TestByQueryOperations(t *testing.T) {
	f := newFakeCluster(t, false)
	e := startedEngine(t, f, nil)
	rctx := request.TestContext(t)

	require.Nil(t, e.DeleteChannelPosts(rctx, "c1"))
	require.Nil(t, e.DeleteUserPosts(rctx, "u1"))
	require.Nil(t, e.DeletePostFiles(rctx, "p1"))
	require.Nil(t, e.DeleteUserFiles(rctx, "u1"))
	require.Nil(t, e.DeleteFilesBatch(rctx, 1000, 50))

	deletes := f.requestsTo("/_delete_by_query")
	require.Len(t, deletes, 5)
	assert.Equal(t, "/mm_posts_*/_delete_by_query", deletes[0].Path)
	assert.JSONEq(t, `{"query":{"term":{"channel_id":"c1"}}}`, string(deletes[0].Body))
	assert.JSONEq(t, `{"query":{"term":{"user_id":"u1"}}}`, string(deletes[1].Body))
	assert.Equal(t, "/mm_files/_delete_by_query", deletes[2].Path)
	assert.JSONEq(t, `{"query":{"term":{"post_id":"p1"}}}`, string(deletes[2].Body))
	assert.JSONEq(t, `{"query":{"range":{"create_at":{"lt":1000}}}}`, string(deletes[4].Body))
	assert.Contains(t, deletes[4].Query, "max_docs=50")
	assert.Contains(t, deletes[4].Query, "wait_for_completion=false")
	assert.NotEmpty(t, f.requestsTo("/_tasks/node:1"))

	require.Nil(t, e.UpdatePostsChannelTypeByChannelId(rctx, "c1", "P"))
	require.Nil(t, e.BackfillPostsChannelType(rctx, []string{"c1", "c2"}, "O"))
	updates := f.requestsTo("/_update_by_query")
	require.Len(t, updates, 2)
	var body map[string]any
	require.NoError(t, json.Unmarshal(updates[1].Body, &body))
	assert.Equal(t, map[string]any{"channel_type": "O"}, body["script"].(map[string]any)["params"])
	assert.Contains(t, updates[1].Query, "requests_per_second=")
	assert.NotContains(t, updates[0].Query, "requests_per_second=")

	require.Nil(t, e.DeletePost(&model.Post{Id: "p-no-date"}))
	deletes = f.requestsTo("/_delete_by_query")
	assert.JSONEq(t, `{"query":{"ids":{"values":["p-no-date"]}}}`, string(deletes[len(deletes)-1].Body))
}

func TestNotStartedErrors(t *testing.T) {
	e := New(nil, nil, nil)
	rctx := request.TestContext(t)
	appErr := e.IndexPost(&model.Post{Id: "p"}, "", "O")
	require.NotNil(t, appErr)
	assert.Equal(t, "ent.elasticsearch.not_started.error", appErr.Id)
	assert.NotNil(t, e.DeleteChannelPosts(rctx, "c"))
	_, _, appErr = e.SearchPosts(nil, nil, 0, 10)
	assert.NotNil(t, appErr)
	assert.NotNil(t, e.PurgeIndexes(rctx))
	assert.NotNil(t, e.RefreshIndexes(rctx))
}

func TestPurge(t *testing.T) {
	f := newFakeCluster(t, false)
	e := startedEngine(t, f, func(cfg *model.Config) {
		cfg.ElasticsearchSettings.IgnoredPurgeIndexes = model.NewPointer("mm_posts_2024_01_*")
	})
	rctx := request.TestContext(t)

	f.mu.Lock()
	for _, name := range []string{"mm_posts_2024_01_01", "mm_posts_2024_02_01", "mm_posts_agg_2023_05", "mm_other", "unrelated"} {
		f.indexes[name] = map[string]json.RawMessage{}
	}
	f.indexes["mm_users"]["u1"] = json.RawMessage(`{}`)
	f.mu.Unlock()

	appErr := e.PurgeIndexList(rctx, []string{"unknown"})
	require.NotNil(t, appErr)
	assert.Equal(t, "ent.elasticsearch.purge_indexes.unknown_index", appErr.Id)

	require.Nil(t, e.PurgeIndexList(rctx, []string{"channels"}))
	assert.Contains(t, f.indexNames(), "mm_channels", "the channels index is recreated")
	assert.NotNil(t, f.doc("mm_users", "u1"))

	require.Nil(t, e.PurgeIndexes(rctx))
	assert.Equal(t, []string{"mm_channels", "mm_files", "mm_other", "mm_posts_2024_01_01", "mm_users", "unrelated"}, f.indexNames())
	assert.Nil(t, f.doc("mm_users", "u1"))
}

func TestRefreshIndexes(t *testing.T) {
	f := newFakeCluster(t, false)
	e := startedEngine(t, f, nil)
	require.Nil(t, e.RefreshIndexes(request.TestContext(t)))
	refreshes := f.requestsTo("/_refresh")
	require.Len(t, refreshes, 1)
	assert.Equal(t, "/mm_posts_*,mm_channels,mm_users,mm_files/_refresh", refreshes[0].Path)
}

func TestAggregatePostIndexes(t *testing.T) {
	f := newFakeCluster(t, false)
	e := startedEngine(t, f, nil)

	f.mu.Lock()
	f.indexes["mm_posts_2023_01_30"] = map[string]json.RawMessage{"a": json.RawMessage(`{"id":"a"}`)}
	f.indexes["mm_posts_2023_01_31"] = map[string]json.RawMessage{"b": json.RawMessage(`{"id":"b"}`)}
	f.indexes["mm_posts_2023_02_01"] = map[string]json.RawMessage{"c": json.RawMessage(`{"id":"c"}`)}
	f.indexes["mm_posts_2023_02_10"] = map[string]json.RawMessage{"d": json.RawMessage(`{"id":"d"}`)}
	f.mu.Unlock()

	cutoff := time.Date(2023, 2, 10, 12, 0, 0, 0, time.UTC)
	merged, appErr := e.AggregatePostIndexes(t.Context(), cutoff)
	require.Nil(t, appErr)
	assert.Equal(t, 3, merged)

	assert.Equal(t, []string{"mm_channels", "mm_files", "mm_posts_2023_02_10", "mm_posts_agg_2023_01", "mm_posts_agg_2023_02", "mm_users"}, f.indexNames())
	assert.Equal(t, "mm_posts_agg_2023_01", f.aliases["mm_posts_2023_01_30"])
	assert.Equal(t, "mm_posts_agg_2023_02", f.aliases["mm_posts_2023_02_01"])
	assert.NotNil(t, f.doc("mm_posts_agg_2023_01", "b"))
	assert.Len(t, f.settings["mm_posts_2023_01_30"], 1, "writes are blocked before copying")

	// Posts of aggregated days are still addressed through their daily name.
	post := &model.Post{Id: "e", ChannelId: "c", CreateAt: time.Date(2023, 1, 30, 10, 0, 0, 0, time.UTC).UnixMilli(), Message: "late edit"}
	require.Nil(t, e.IndexPost(post, "t", "O"))
	assert.NotNil(t, f.doc("mm_posts_agg_2023_01", "e"))
	assert.NotContains(t, f.indexNames(), "mm_posts_2023_01_30")

	// Nothing left to aggregate.
	merged, appErr = e.AggregatePostIndexes(t.Context(), cutoff)
	require.Nil(t, appErr)
	assert.Equal(t, 0, merged)
}

func TestDataRetentionDeleteIndexes(t *testing.T) {
	f := newFakeCluster(t, false)
	e := startedEngine(t, f, nil)
	f.mu.Lock()
	for _, name := range []string{"mm_posts_2023_01_30", "mm_posts_2023_02_01", "mm_posts_agg_2022_12", "mm_posts_agg_2023_01"} {
		f.indexes[name] = map[string]json.RawMessage{}
	}
	f.mu.Unlock()

	require.Nil(t, e.DataRetentionDeleteIndexes(request.TestContext(t), time.Date(2023, 2, 1, 0, 0, 0, 0, time.UTC)))
	assert.Equal(t, []string{"mm_channels", "mm_files", "mm_posts_2023_02_01", "mm_users"}, f.indexNames())
}

func TestBackendErrorIsReported(t *testing.T) {
	f := newFakeCluster(t, false)
	e := startedEngine(t, f, nil)
	f.failStatus = http.StatusForbidden
	appErr := e.IndexUser(request.TestContext(t), &model.User{Id: "u"}, nil, nil)
	require.NotNil(t, appErr)
	assert.Contains(t, appErr.Error(), "cluster_block_exception")
}

func TestOutdatedIndexesDisableAutocomplete(t *testing.T) {
	f := newFakeCluster(t, false)
	f.indexes["mm_channels"] = map[string]json.RawMessage{}
	f.foreign["mm_channels"] = true

	e := startedEngine(t, f, nil)
	assert.True(t, e.IsSearchEnabled())
	assert.False(t, e.IsAutocompletionEnabled(), "the channel index must be rebuilt first")

	require.Nil(t, e.PurgeIndexList(request.TestContext(t), []string{"channels"}))
	assert.True(t, e.IsAutocompletionEnabled())
}

func TestUpdateConfigRestartsOnClientSettingsChange(t *testing.T) {
	f := newFakeCluster(t, false)
	e := startedEngine(t, f, nil)
	before := len(f.requestsTo("/_index_template/mm_mattermost_libre_posts"))

	cfg := f.config()
	cfg.ElasticsearchSettings.RequestTimeoutSeconds = model.NewPointer(12)
	e.UpdateConfig(cfg)

	require.Eventually(t, func() bool {
		return len(f.requestsTo("/_index_template/mm_mattermost_libre_posts")) > before && e.IsActive()
	}, 5*time.Second, 20*time.Millisecond)
	assert.Equal(t, 12*time.Second, e.state.Load().client.timeout)
}
