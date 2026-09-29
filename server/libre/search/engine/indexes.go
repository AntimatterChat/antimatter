// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package engine

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
)

// Index layout
//
// Posts are stored in one index per UTC day, named <prefix>posts_YYYY_MM_DD.
// The aggregation job later merges old daily indexes into one index per
// month, <prefix>posts_agg_YYYY_MM, and replaces every merged daily index by
// an alias of the same name pointing to the monthly index. Thanks to that, a
// post can always be addressed through the daily name derived from its
// CreateAt, whether the day was aggregated or not.
//
// Channels, users and files each live in a single index.
const (
	indexPosts    = "posts"
	indexChannels = "channels"
	indexUsers    = "users"
	indexFiles    = "files"

	dailyIndexDateLayout   = "2006_01_02"
	monthlyIndexDateLayout = "2006_01"
	aggregatedPostsInfix   = "posts_agg_"

	// Version of the mappings below. Stored in the index metadata so that
	// outdated indexes can be detected.
	schemaVersion = 1

	templatePriority = 200
)

var (
	dailyPostsIndexRegex   = regexp.MustCompile(`^posts_(\d{4}_\d{2}_\d{2})$`)
	monthlyPostsIndexRegex = regexp.MustCompile(`^posts_agg_(\d{4}_\d{2})$`)
)

type indexNames struct {
	prefix       string
	globalPrefix string
}

func (n indexNames) channels() string { return n.prefix + indexChannels }
func (n indexNames) users() string    { return n.prefix + indexUsers }
func (n indexNames) files() string    { return n.prefix + indexFiles }

// postsPattern matches every post index (and alias) of this server.
func (n indexNames) postsPattern() string { return n.prefix + indexPosts + "_*" }

// postsSearchPattern is the pattern used when searching posts: when a global
// search prefix is configured, the posts of every server sharing that prefix
// are searched.
func (n indexNames) postsSearchPattern() string {
	if n.globalPrefix != "" {
		return n.globalPrefix + "*" + indexPosts + "_*"
	}
	return n.postsPattern()
}

func (n indexNames) filesSearchPattern() string {
	if n.globalPrefix != "" {
		return n.globalPrefix + "*" + indexFiles
	}
	return n.files()
}

// postIndexForTime returns the daily index (or alias) for a post creation
// timestamp in milliseconds.
func (n indexNames) postIndexForTime(createAt int64) string {
	return n.prefix + indexPosts + "_" + time.UnixMilli(createAt).UTC().Format(dailyIndexDateLayout)
}

func (n indexNames) monthlyPostIndex(day time.Time) string {
	return n.prefix + aggregatedPostsInfix + day.UTC().Format(monthlyIndexDateLayout)
}

// parsePostIndex returns the time range covered by one of our post indexes.
// ok is false when the name is not a post index of this server.
func (n indexNames) parsePostIndex(name string) (start, end time.Time, aggregated, ok bool) {
	if !strings.HasPrefix(name, n.prefix) {
		return
	}
	rest := strings.TrimPrefix(name, n.prefix)
	if m := dailyPostsIndexRegex.FindStringSubmatch(rest); m != nil {
		day, err := time.ParseInLocation(dailyIndexDateLayout, m[1], time.UTC)
		if err != nil {
			return
		}
		return day, day.AddDate(0, 0, 1), false, true
	}
	if m := monthlyPostsIndexRegex.FindStringSubmatch(rest); m != nil {
		month, err := time.ParseInLocation(monthlyIndexDateLayout, m[1], time.UTC)
		if err != nil {
			return
		}
		return month, month.AddDate(0, 1, 0), true, true
	}
	return
}

// kindOf returns the logical index ("posts", "channels", ...) of a concrete
// index name, or "" if it isn't one of ours.
func (n indexNames) kindOf(name string) string {
	if _, _, _, ok := n.parsePostIndex(name); ok {
		return indexPosts
	}
	switch name {
	case n.channels():
		return indexChannels
	case n.users():
		return indexUsers
	case n.files():
		return indexFiles
	}
	return ""
}

func (n indexNames) templateName(kind string) string {
	return n.prefix + "mattermost_libre_" + kind
}

func (n indexNames) templatePattern(kind string) string {
	if kind == indexPosts {
		return n.postsPattern()
	}
	return n.prefix + kind
}

// analysisSettings returns the analyzers shared by all our indexes.
func analysisSettings(cjk bool) map[string]any {
	textFilters := []string{"lowercase", "mm_ascii_folding"}
	if cjk {
		textFilters = []string{"cjk_width", "lowercase", "mm_ascii_folding", "cjk_bigram"}
	}
	return map[string]any{
		"filter": map[string]any{
			"mm_ascii_folding": map[string]any{
				"type":              "asciifolding",
				"preserve_original": true,
			},
		},
		"tokenizer": map[string]any{
			"mm_filename_tokenizer": map[string]any{
				"type":    "pattern",
				"pattern": `[^\p{L}\p{N}]+`,
			},
		},
		"analyzer": map[string]any{
			"mm_text": map[string]any{
				"type":      "custom",
				"tokenizer": "standard",
				"filter":    textFilters,
			},
			"mm_filename": map[string]any{
				"type":      "custom",
				"tokenizer": "mm_filename_tokenizer",
				"filter":    []string{"lowercase", "mm_ascii_folding"},
			},
		},
		"normalizer": map[string]any{
			"mm_lowercase": map[string]any{
				"type":   "custom",
				"filter": []string{"lowercase"},
			},
		},
	}
}

var (
	keywordField   = map[string]any{"type": "keyword"}
	lowercaseField = map[string]any{"type": "keyword", "normalizer": "mm_lowercase", "ignore_above": 1024}
	longField      = map[string]any{"type": "long"}
	textField      = map[string]any{"type": "text", "analyzer": "mm_text"}
)

func mappingFor(kind string) map[string]any {
	var properties map[string]any
	switch kind {
	case indexPosts:
		properties = map[string]any{
			"id":           keywordField,
			"team_id":      keywordField,
			"channel_id":   keywordField,
			"channel_type": keywordField,
			"user_id":      keywordField,
			"root_id":      keywordField,
			"type":         keywordField,
			"create_at":    longField,
			"message":      textField,
			"attachments":  textField,
			"hashtags":     lowercaseField,
		}
	case indexChannels:
		properties = map[string]any{
			"id":        keywordField,
			"team_id":   keywordField,
			"type":      keywordField,
			"delete_at": longField,
			"create_at": longField,
			"name": map[string]any{
				"type": "keyword", "normalizer": "mm_lowercase", "ignore_above": 1024,
				"fields": map[string]any{"text": textField},
			},
			"display_name": map[string]any{
				"type": "keyword", "normalizer": "mm_lowercase", "ignore_above": 1024,
				"fields": map[string]any{"text": textField},
			},
			"purpose":      textField,
			"user_ids":     keywordField,
			"discoverable": map[string]any{"type": "boolean"},
		}
	case indexUsers:
		properties = map[string]any{
			"id":          keywordField,
			"username":    lowercaseField,
			"nickname":    lowercaseField,
			"first_name":  lowercaseField,
			"last_name":   lowercaseField,
			"email":       lowercaseField,
			"roles":       keywordField,
			"roles_raw":   keywordField,
			"delete_at":   longField,
			"create_at":   longField,
			"team_ids":    keywordField,
			"channel_ids": keywordField,
		}
	case indexFiles:
		properties = map[string]any{
			"id":         keywordField,
			"channel_id": keywordField,
			"post_id":    keywordField,
			"user_id":    keywordField,
			"create_at":  longField,
			"extension":  lowercaseField,
			"name": map[string]any{
				"type": "text", "analyzer": "mm_filename",
				"fields": map[string]any{"text": textField},
			},
			"content": textField,
		}
	}
	return map[string]any{
		"dynamic":    false,
		"_meta":      map[string]any{"mattermost_libre_schema": schemaVersion},
		"properties": properties,
	}
}

func shardSettings(kind string, s *model.ElasticsearchSettings) (shards, replicas int) {
	switch kind {
	case indexPosts, indexFiles:
		return model.SafeDereference(s.PostIndexShards), model.SafeDereference(s.PostIndexReplicas)
	case indexChannels:
		return model.SafeDereference(s.ChannelIndexShards), model.SafeDereference(s.ChannelIndexReplicas)
	default:
		return model.SafeDereference(s.UserIndexShards), model.SafeDereference(s.UserIndexReplicas)
	}
}

func indexTemplate(kind string, names indexNames, s *model.ElasticsearchSettings) map[string]any {
	shards, replicas := shardSettings(kind, s)
	return map[string]any{
		"index_patterns": []string{names.templatePattern(kind)},
		"priority":       templatePriority,
		"template": map[string]any{
			"settings": map[string]any{
				"index": map[string]any{
					"number_of_shards":   max(shards, 1),
					"number_of_replicas": max(replicas, 0),
				},
				"analysis": analysisSettings(model.SafeDereference(s.EnableCJKAnalyzers)),
			},
			"mappings": mappingFor(kind),
		},
		"_meta": map[string]any{"managed_by": "mattermost-libre", "schema": schemaVersion},
	}
}

var allKinds = []string{indexPosts, indexChannels, indexUsers, indexFiles}

func templateErrorID(kind string) string {
	switch kind {
	case indexPosts:
		return "ent.elasticsearch.create_template_posts_if_not_exists.template_create_failed"
	case indexChannels:
		return "ent.elasticsearch.create_template_channels_if_not_exists.template_create_failed"
	case indexUsers:
		return "ent.elasticsearch.create_template_users_if_not_exists.template_create_failed"
	default:
		return "ent.elasticsearch.create_template_file_info_if_not_exists.template_create_failed"
	}
}

// putTemplates installs (or updates) the index templates, so that any index
// created later, explicitly or implicitly by a write, gets our mappings.
func (e *Engine) putTemplates(ctx context.Context, c *client, names indexNames, s *model.ElasticsearchSettings) *model.AppError {
	for _, kind := range allKinds {
		if err := c.doJSON(ctx, http.MethodPut, "/_index_template/"+names.templateName(kind), nil, indexTemplate(kind, names, s), nil); err != nil {
			return model.NewAppError("Engine.putTemplates", templateErrorID(kind), e.backendParams(), "", http.StatusInternalServerError).Wrap(err)
		}
	}
	return nil
}

// ensureIndex makes sure an index (or alias) exists, creating it from the
// templates when missing. Known names are cached.
func (e *Engine) ensureIndex(ctx context.Context, c *client, name string) error {
	if _, ok := e.knownIndexes.Load(name); ok {
		return nil
	}
	_, err := c.do(ctx, http.MethodHead, "/"+name, nil, nil)
	if err == nil {
		e.knownIndexes.Store(name, true)
		return nil
	}
	if !isNotFound(err) {
		return err
	}
	_, err = c.do(ctx, http.MethodPut, "/"+name, nil, nil)
	if err != nil && !isErrorType(err, "resource_already_exists_exception") {
		return err
	}
	e.knownIndexes.Store(name, true)
	return nil
}

func (e *Engine) forgetIndexes() {
	e.knownIndexes.Clear()
}

// checkSchemas records which of the channel, user and file indexes were not
// created with the current mappings (for instance by another implementation
// sharing the cluster). Such indexes must be purged and rebuilt; until then
// autocompletion is not served from them.
func (e *Engine) checkSchemas(ctx context.Context, c *client, names indexNames) {
	targets := []string{names.channels(), names.users(), names.files()}
	var res map[string]struct {
		Mappings struct {
			Meta map[string]any `json:"_meta"`
		} `json:"mappings"`
	}
	outdated := map[string]bool{}
	if err := c.doJSON(ctx, http.MethodGet, "/"+strings.Join(targets, ",")+"/_mapping", ignoreUnavailable(), nil, &res); err != nil {
		e.logger().Warn("Failed to check the mappings of the search indexes", mlog.Err(err))
		e.outdated.Store(&outdated)
		return
	}
	for _, name := range targets {
		entry, ok := res[name]
		if !ok {
			continue
		}
		version, _ := entry.Mappings.Meta["mattermost_libre_schema"].(float64)
		if int(version) != schemaVersion {
			outdated[names.kindOf(name)] = true
			e.logger().Warn("The search index was not created by this server version and must be rebuilt: purge the indexes and run a bulk indexing job",
				mlog.String("index", name))
		}
	}
	e.outdated.Store(&outdated)
}

func (e *Engine) isOutdated(kind string) bool {
	if m := e.outdated.Load(); m != nil {
		return (*m)[kind]
	}
	return false
}

type catIndex struct {
	Index string `json:"index"`
}

// listIndexes returns the concrete indexes matching the pattern.
func (c *client) listIndexes(ctx context.Context, pattern string) ([]string, error) {
	var rows []catIndex
	query := url.Values{"format": {"json"}, "h": {"index"}, "expand_wildcards": {"open,closed"}}
	if err := c.doJSON(ctx, http.MethodGet, "/_cat/indices/"+pattern, query, nil, &rows); err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	names := make([]string, 0, len(rows))
	for _, row := range rows {
		names = append(names, row.Index)
	}
	slices.Sort(names)
	return names, nil
}

// deleteIndexes deletes the given concrete indexes, in chunks to keep the
// URL length reasonable.
func (c *client) deleteIndexes(ctx context.Context, names []string) error {
	const chunk = 50
	for i := 0; i < len(names); i += chunk {
		part := names[i:min(i+chunk, len(names))]
		_, err := c.do(ctx, http.MethodDelete, "/"+strings.Join(part, ","), url.Values{"ignore_unavailable": {"true"}}, nil)
		if err != nil && !isNotFound(err) {
			return err
		}
	}
	return nil
}

// wildcardMatch implements the simple "*" glob used for the ignored purge
// index setting.
func wildcardMatch(pattern, name string) bool {
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return pattern == name
	}
	if !strings.HasPrefix(name, parts[0]) {
		return false
	}
	name = name[len(parts[0]):]
	for i := 1; i < len(parts)-1; i++ {
		idx := strings.Index(name, parts[i])
		if idx < 0 {
			return false
		}
		name = name[idx+len(parts[i]):]
	}
	return strings.HasSuffix(name, parts[len(parts)-1])
}

func ignoredIndexPatterns(s *model.ElasticsearchSettings) []string {
	var patterns []string
	for p := range strings.SplitSeq(model.SafeDereference(s.IgnoredPurgeIndexes), ",") {
		if p = strings.TrimSpace(p); p != "" {
			patterns = append(patterns, p)
		}
	}
	return patterns
}

func isIgnored(name string, names indexNames, patterns []string) bool {
	for _, p := range patterns {
		if wildcardMatch(p, name) || wildcardMatch(p, strings.TrimPrefix(name, names.prefix)) {
			return true
		}
	}
	return false
}

// purge deletes the indexes of the given kinds managed by this server, except
// the ones listed in IgnoredPurgeIndexes, and recreates empty ones.
func (e *Engine) purge(ctx context.Context, kinds []string) *model.AppError {
	c, cfg, appErr := e.activeClient("Engine.purge")
	if appErr != nil {
		return appErr
	}
	names := namesFor(cfg)
	if e.processor() != nil {
		e.processor().flush()
	}

	candidates, err := c.listIndexes(ctx, names.prefix+"*")
	if err != nil {
		return model.NewAppError("Engine.purge", "ent.elasticsearch.purge_index.delete_failed", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	ignored := ignoredIndexPatterns(&cfg.ElasticsearchSettings)
	var toDelete []string
	for _, name := range candidates {
		kind := names.kindOf(name)
		if kind == "" || !slices.Contains(kinds, kind) || isIgnored(name, names, ignored) {
			continue
		}
		toDelete = append(toDelete, name)
	}

	if err := c.deleteIndexes(ctx, toDelete); err != nil {
		return model.NewAppError("Engine.purge", "ent.elasticsearch.purge_index.delete_failed", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	e.forgetIndexes()

	for _, kind := range kinds {
		if kind == indexPosts {
			continue
		}
		name := names.prefix + kind
		if isIgnored(name, names, ignored) {
			continue
		}
		if err := e.ensureIndex(ctx, c, name); err != nil {
			return model.NewAppError("Engine.purge", "ent.elasticsearch.purge_index.delete_failed", nil, "failed to recreate index "+name, http.StatusInternalServerError).Wrap(err)
		}
	}
	e.checkSchemas(ctx, c, names)
	return nil
}

// resolvePurgeKinds maps the index names sent by the admin console (e.g.
// "channels", optionally prefixed, possibly comma separated) to index kinds.
func resolvePurgeKinds(names indexNames, requested []string) ([]string, error) {
	var kinds []string
	for _, entry := range requested {
		for name := range strings.SplitSeq(entry, ",") {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			short := strings.TrimPrefix(name, names.prefix)
			kind := ""
			switch {
			case slices.Contains(allKinds, short):
				kind = short
			case names.kindOf(name) != "":
				kind = names.kindOf(name)
			}
			if kind == "" {
				return nil, fmt.Errorf("unknown index %q", name)
			}
			if !slices.Contains(kinds, kind) {
				kinds = append(kinds, kind)
			}
		}
	}
	return kinds, nil
}
