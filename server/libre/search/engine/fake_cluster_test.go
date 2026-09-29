// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package engine

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

type recordedRequest struct {
	Method string
	Path   string
	Query  string
	Body   []byte
}

// fakeCluster emulates the subset of the Elasticsearch / OpenSearch REST API
// used by the engine. Documents are kept in memory; searches return the
// configured hits.
type fakeCluster struct {
	t          *testing.T
	server     *httptest.Server
	openSearch bool

	mu        sync.Mutex
	requests  []recordedRequest
	indexes   map[string]map[string]json.RawMessage
	aliases   map[string]string
	templates map[string]json.RawMessage
	settings  map[string][]json.RawMessage
	// searchHits returns the hits of a search request.
	searchHits func(target string, body map[string]any) []map[string]any
	// bulkStatus allows to force the status of a bulk item.
	bulkStatus func(op, index, id string) int
	failStatus int
	// foreign lists indexes created with mappings of another implementation.
	foreign map[string]bool
}

func newFakeCluster(t *testing.T, openSearch bool) *fakeCluster {
	f := &fakeCluster{
		t:          t,
		openSearch: openSearch,
		indexes:    map[string]map[string]json.RawMessage{},
		aliases:    map[string]string{},
		templates:  map[string]json.RawMessage{},
		settings:   map[string][]json.RawMessage{},
		foreign:    map[string]bool{},
	}
	f.server = httptest.NewServer(http.HandlerFunc(f.handle))
	t.Cleanup(f.server.Close)
	return f
}

func (f *fakeCluster) config() *model.Config {
	cfg := &model.Config{}
	cfg.SetDefaults()
	cfg.ElasticsearchSettings.ConnectionURL = model.NewPointer(f.server.URL)
	cfg.ElasticsearchSettings.EnableIndexing = model.NewPointer(true)
	cfg.ElasticsearchSettings.EnableSearching = model.NewPointer(true)
	cfg.ElasticsearchSettings.EnableAutocomplete = model.NewPointer(true)
	cfg.ElasticsearchSettings.Sniff = model.NewPointer(false)
	cfg.ElasticsearchSettings.IndexPrefix = model.NewPointer("mm_")
	cfg.ElasticsearchSettings.LiveIndexingBatchSize = model.NewPointer(1)
	if f.openSearch {
		cfg.ElasticsearchSettings.Backend = model.NewPointer(model.ElasticsearchSettingsOSBackend)
	}
	return cfg
}

func (f *fakeCluster) reply(w http.ResponseWriter, status int, body any) {
	if !f.openSearch {
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if body != nil {
		require.NoError(f.t, json.NewEncoder(w).Encode(body))
	}
}

func (f *fakeCluster) resolve(name string) string {
	if target, ok := f.aliases[name]; ok {
		return target
	}
	return name
}

func (f *fakeCluster) handle(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, recordedRequest{Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery, Body: body})

	if f.failStatus != 0 && r.URL.Path != "/" {
		f.reply(w, f.failStatus, map[string]any{"error": map[string]any{"type": "cluster_block_exception", "reason": "forced failure"}, "status": f.failStatus})
		return
	}

	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	switch {
	case r.URL.Path == "/" && r.Method == http.MethodGet:
		version := map[string]any{"number": "8.15.0"}
		if f.openSearch {
			version = map[string]any{"number": "2.17.1", "distribution": "opensearch"}
		}
		f.reply(w, http.StatusOK, map[string]any{"version": version, "tagline": "You Know, for Search"})
	case parts[0] == "_cat" && parts[1] == "plugins":
		f.reply(w, http.StatusOK, []map[string]any{{"name": "node1", "component": "analysis-icu"}, {"name": "node2", "component": "analysis-icu"}})
	case parts[0] == "_cat" && parts[1] == "indices":
		rows := []map[string]any{}
		pattern := "*"
		if len(parts) > 2 {
			pattern = parts[2]
		}
		for name := range f.indexes {
			if wildcardMatch(pattern, name) {
				rows = append(rows, map[string]any{"index": name})
			}
		}
		f.reply(w, http.StatusOK, rows)
	case parts[0] == "_index_template":
		f.templates[parts[1]] = body
		f.reply(w, http.StatusOK, map[string]any{"acknowledged": true})
	case parts[0] == "_bulk":
		f.handleBulk(w, body)
	case parts[0] == "_tasks":
		f.reply(w, http.StatusOK, map[string]any{"completed": true, "response": map[string]any{"total": 1, "deleted": 1, "failures": []any{}}})
	case parts[0] == "_reindex":
		var req struct {
			Source struct {
				Index []string `json:"index"`
			} `json:"source"`
			Dest struct {
				Index string `json:"index"`
			} `json:"dest"`
		}
		require.NoError(f.t, json.Unmarshal(body, &req))
		if f.indexes[req.Dest.Index] == nil {
			f.indexes[req.Dest.Index] = map[string]json.RawMessage{}
		}
		for _, src := range req.Source.Index {
			for id, doc := range f.indexes[src] {
				f.indexes[req.Dest.Index][id] = doc
			}
		}
		f.reply(w, http.StatusOK, map[string]any{"task": "node:1"})
	case parts[0] == "_aliases":
		var req struct {
			Actions []map[string]map[string]string `json:"actions"`
		}
		require.NoError(f.t, json.Unmarshal(body, &req))
		for _, action := range req.Actions {
			if a, ok := action["remove_index"]; ok {
				delete(f.indexes, a["index"])
			}
			if a, ok := action["add"]; ok {
				f.aliases[a["alias"]] = a["index"]
			}
		}
		f.reply(w, http.StatusOK, map[string]any{"acknowledged": true})
	case len(parts) == 2 && (parts[1] == "_delete_by_query" || parts[1] == "_update_by_query"):
		f.reply(w, http.StatusOK, map[string]any{"task": "node:1"})
	case len(parts) == 2 && parts[1] == "_search":
		var req map[string]any
		require.NoError(f.t, json.Unmarshal(body, &req))
		var hits []map[string]any
		if f.searchHits != nil {
			hits = f.searchHits(parts[0], req)
		}
		f.reply(w, http.StatusOK, map[string]any{"hits": map[string]any{"hits": hits}})
	case len(parts) == 2 && parts[1] == "_mapping":
		res := map[string]any{}
		for name := range strings.SplitSeq(parts[0], ",") {
			if _, ok := f.indexes[name]; !ok {
				continue
			}
			meta := map[string]any{"mattermost_libre_schema": schemaVersion}
			if f.foreign[name] {
				meta = map[string]any{}
			}
			res[name] = map[string]any{"mappings": map[string]any{"_meta": meta}}
		}
		f.reply(w, http.StatusOK, res)
	case len(parts) == 2 && parts[1] == "_refresh":
		f.reply(w, http.StatusOK, map[string]any{})
	case len(parts) == 2 && parts[1] == "_settings":
		f.settings[parts[0]] = append(f.settings[parts[0]], body)
		f.reply(w, http.StatusOK, map[string]any{"acknowledged": true})
	case len(parts) == 3 && parts[1] == "_doc" && r.Method == http.MethodGet:
		doc, ok := f.indexes[f.resolve(parts[0])][parts[2]]
		if !ok {
			f.reply(w, http.StatusNotFound, map[string]any{"found": false})
			return
		}
		f.reply(w, http.StatusOK, map[string]any{"found": true, "_id": parts[2], "_source": doc})
	case len(parts) == 1 && r.Method == http.MethodHead:
		if _, ok := f.indexes[parts[0]]; ok {
			f.reply(w, http.StatusOK, nil)
			return
		}
		if _, ok := f.aliases[parts[0]]; ok {
			f.reply(w, http.StatusOK, nil)
			return
		}
		f.reply(w, http.StatusNotFound, nil)
	case len(parts) == 1 && r.Method == http.MethodPut:
		if _, ok := f.indexes[parts[0]]; ok {
			f.reply(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"type": "resource_already_exists_exception", "reason": "exists"}})
			return
		}
		f.indexes[parts[0]] = map[string]json.RawMessage{}
		f.reply(w, http.StatusOK, map[string]any{"acknowledged": true})
	case len(parts) == 1 && r.Method == http.MethodDelete:
		for name := range strings.SplitSeq(parts[0], ",") {
			delete(f.indexes, name)
			delete(f.foreign, name)
			for alias, target := range f.aliases {
				if target == name {
					delete(f.aliases, alias)
				}
			}
		}
		f.reply(w, http.StatusOK, map[string]any{"acknowledged": true})
	default:
		f.t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		f.reply(w, http.StatusBadRequest, map[string]any{"error": "unexpected"})
	}
}

func (f *fakeCluster) handleBulk(w http.ResponseWriter, body []byte) {
	scanner := bufio.NewScanner(bytes.NewReader(body))
	scanner.Buffer(make([]byte, 1024*1024), 64*1024*1024)
	var items []map[string]any
	hasErrors := false
	for scanner.Scan() {
		var meta map[string]map[string]string
		require.NoError(f.t, json.Unmarshal(scanner.Bytes(), &meta))
		for op, m := range meta {
			index := f.resolve(m["_index"])
			id := m["_id"]
			status := http.StatusOK
			switch op {
			case opIndex:
				require.True(f.t, scanner.Scan())
				if f.indexes[index] == nil {
					f.indexes[index] = map[string]json.RawMessage{}
				}
				f.indexes[index][id] = append(json.RawMessage{}, scanner.Bytes()...)
				status = http.StatusCreated
			case opDelete:
				if _, ok := f.indexes[index][id]; !ok {
					status = http.StatusNotFound
				}
				delete(f.indexes[index], id)
			}
			if f.bulkStatus != nil {
				if s := f.bulkStatus(op, index, id); s != 0 {
					status = s
				}
			}
			item := map[string]any{"_index": index, "_id": id, "status": status}
			if status >= 400 && !(op == opDelete && status == http.StatusNotFound) {
				hasErrors = true
				item["error"] = map[string]any{"type": "mapper_parsing_exception", "reason": "bad document"}
			}
			items = append(items, map[string]any{op: item})
		}
	}
	f.reply(w, http.StatusOK, map[string]any{"errors": hasErrors, "items": items})
}

func (f *fakeCluster) requestsTo(suffix string) []recordedRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []recordedRequest
	for _, r := range f.requests {
		if strings.HasSuffix(r.Path, suffix) {
			out = append(out, r)
		}
	}
	return out
}

func (f *fakeCluster) doc(index, id string) map[string]any {
	f.mu.Lock()
	defer f.mu.Unlock()
	raw, ok := f.indexes[f.resolve(index)][id]
	if !ok {
		return nil
	}
	var doc map[string]any
	require.NoError(f.t, json.Unmarshal(raw, &doc))
	return doc
}

func (f *fakeCluster) indexNames() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var names []string
	for name := range f.indexes {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// startedEngine returns an engine connected to the fake cluster.
func startedEngine(t *testing.T, f *fakeCluster, mutate func(cfg *model.Config)) *Engine {
	cfg := f.config()
	if mutate != nil {
		mutate(cfg)
	}
	e := New(cfg, nil, nil)
	require.Nil(t, e.Start(t.Context()))
	t.Cleanup(func() { _ = e.Stop() })
	return e
}
