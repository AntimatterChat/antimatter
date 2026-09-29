// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package engine

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

func toJSON(t *testing.T, v any) string {
	t.Helper()
	data, err := json.Marshal(v)
	require.NoError(t, err)
	return string(data)
}

func TestParseTerms(t *testing.T) {
	terms := parseTerms(`hello "big world" wild* !!! "" "phrase pre*" t-shirt`)
	assert.Equal(t, []searchTerm{
		{text: "big world", phrase: true},
		{text: "phrase pre", phrase: true, prefix: true},
		{text: "hello"},
		{text: "wild", prefix: true},
		{text: "t-shirt"},
	}, terms)
	assert.Empty(t, parseTerms("  *** ?? "))
}

func TestTextTermQuery(t *testing.T) {
	fields := []string{"message", "attachments"}
	assert.JSONEq(t, `{"multi_match":{"query":"Hello","type":"phrase","fields":["message","attachments"]}}`,
		toJSON(t, textTermQuery(searchTerm{text: "Hello"}, fields)))
	assert.JSONEq(t, `{"bool":{"should":[{"prefix":{"message":{"value":"wor"}}},{"prefix":{"attachments":{"value":"wor"}}}],"minimum_should_match":1}}`,
		toJSON(t, textTermQuery(searchTerm{text: "Wor", prefix: true}, fields)))
	// Multi token prefixes (here CJK) use a phrase prefix query.
	assert.JSONEq(t, `{"multi_match":{"query":"東京","type":"phrase_prefix","fields":["message"],"max_expansions":200}}`,
		toJSON(t, textTermQuery(searchTerm{text: "東京", prefix: true}, []string{"message"})))
	assert.JSONEq(t, `{"term":{"hashtags":"#foo"}}`, toJSON(t, hashtagTermQuery(searchTerm{text: "#Foo"})))
	assert.JSONEq(t, `{"prefix":{"hashtags":{"value":"#foo"}}}`, toJSON(t, hashtagTermQuery(searchTerm{text: "foo", prefix: true})))
}

func TestBuildPostSearch(t *testing.T) {
	channels := model.ChannelList{
		{Id: "c1", TeamId: "t1", Type: model.ChannelTypeOpen},
		{Id: "c2", TeamId: "t1", Type: model.ChannelTypePrivate},
		{Id: "d1", TeamId: "", Type: model.ChannelTypeDirect},
	}

	t.Run("and terms with filters", func(t *testing.T) {
		params := &model.SearchParams{
			Terms:            `apple "red car"`,
			ExcludedTerms:    "banana",
			InChannels:       []string{"c1"},
			ExcludedChannels: []string{"c2"},
			FromUsers:        []string{"u1"},
			ExcludedUsers:    []string{"u2"},
		}
		body := buildPostSearch(channels, []*model.SearchParams{params}, false, 2, 20)
		assert.Equal(t, 40, body["from"])
		assert.Equal(t, 20, body["size"])
		assert.JSONEq(t, `{"bool":{
			"filter":[{"terms":{"channel_id":["c1","c2","d1"]}}],
			"must_not":[{"prefix":{"type":"system_"}}],
			"should":[{"bool":{
				"filter":[{"terms":{"channel_id":["c1"]}},{"terms":{"user_id":["u1"]}}],
				"must":[
					{"multi_match":{"query":"red car","type":"phrase","fields":["message","attachments"]}},
					{"multi_match":{"query":"apple","type":"phrase","fields":["message","attachments"]}}
				],
				"must_not":[
					{"terms":{"channel_id":["c2"]}},
					{"terms":{"user_id":["u2"]}},
					{"multi_match":{"query":"banana","type":"phrase","fields":["message","attachments"]}}
				]
			}}],
			"minimum_should_match":1
		}}`, toJSON(t, body["query"]))
		assert.JSONEq(t, `[{"create_at":{"order":"desc"}},{"id":{"order":"desc"}}]`, toJSON(t, body["sort"]))
	})

	t.Run("or terms", func(t *testing.T) {
		body := buildPostSearch(channels, []*model.SearchParams{{Terms: "one two", OrTerms: true}}, false, 0, 10)
		q := toJSON(t, body["query"])
		assert.Contains(t, q, `"should":[{"bool":{"minimum_should_match":1,"should":[{"multi_match":{"fields":["message","attachments"],"query":"one","type":"phrase"}},{"multi_match":{"fields":["message","attachments"],"query":"two","type":"phrase"}}]}}]`)
	})

	t.Run("hashtags and plain terms are combined with OR", func(t *testing.T) {
		paramsList := model.ParseSearchParams("hello #world", 0)
		require.Len(t, paramsList, 2)
		body := buildPostSearch(channels, paramsList, false, 0, 10)
		q := toJSON(t, body["query"])
		assert.Contains(t, q, `{"term":{"hashtags":"#world"}}`)
		assert.Contains(t, q, `"query":"hello"`)
	})

	t.Run("dates", func(t *testing.T) {
		params := &model.SearchParams{AfterDate: "2024-01-01", BeforeDate: "2024-02-01", ExcludedDate: "2024-01-15"}
		body := buildPostSearch(channels, []*model.SearchParams{params}, false, 0, 10)
		q := toJSON(t, body["query"])
		assert.Contains(t, q, toJSON(t, map[string]any{"range": map[string]any{"create_at": map[string]any{"gte": params.GetAfterDateMillis()}}}))
		assert.Contains(t, q, toJSON(t, map[string]any{"range": map[string]any{"create_at": map[string]any{"lte": params.GetBeforeDateMillis()}}}))
		start, end := params.GetExcludedDateMillis()
		assert.Contains(t, q, toJSON(t, map[string]any{"range": map[string]any{"create_at": map[string]any{"gte": start, "lte": end}}}))

		on := &model.SearchParams{OnDate: "2024-01-15", AfterDate: "2020-01-01"}
		q = toJSON(t, buildPostSearch(channels, []*model.SearchParams{on}, false, 0, 10)["query"])
		start, end = on.GetOnDateMillis()
		assert.Contains(t, q, toJSON(t, map[string]any{"range": map[string]any{"create_at": map[string]any{"gte": start, "lte": end}}}))
		assert.NotContains(t, q, toJSON(t, on.GetAfterDateMillis()), "on: takes precedence")
	})

	t.Run("public channels without membership", func(t *testing.T) {
		body := buildPostSearch(channels, []*model.SearchParams{{Terms: "x"}}, true, 0, 10)
		q := body["query"].(map[string]any)["bool"].(map[string]any)["filter"].([]any)[0]
		assert.JSONEq(t, `{"bool":{"should":[
			{"terms":{"channel_id":["c1","c2","d1"]}},
			{"bool":{"filter":[{"term":{"channel_type":"O"}},{"terms":{"team_id":["t1"]}}]}}
		],"minimum_should_match":1}}`, toJSON(t, q))
	})

	t.Run("params without criteria match nothing", func(t *testing.T) {
		body := buildPostSearch(channels, []*model.SearchParams{{Terms: "!!!"}}, false, 0, 10)
		assert.Contains(t, toJSON(t, body["query"]), `"should":[{"match_none":{}}]`)
	})
}

func TestBuildFileSearch(t *testing.T) {
	params := &model.SearchParams{Terms: "report*", Extensions: []string{".PDF"}, ExcludedExtensions: []string{"txt"}}
	body := buildFileSearch(model.ChannelList{{Id: "c1"}}, []*model.SearchParams{params}, 0, 10)
	q := toJSON(t, body["query"])
	assert.Contains(t, q, `{"terms":{"extension":["pdf"]}}`)
	assert.Contains(t, q, `"must_not":[{"terms":{"extension":["txt"]}}]`)
	assert.Contains(t, q, `{"prefix":{"name":{"value":"report"}}}`)
	assert.Contains(t, q, `{"prefix":{"content":{"value":"report"}}}`)
	assert.Contains(t, q, `{"terms":{"channel_id":["c1"]}}`)
}

func TestBuildChannelSearch(t *testing.T) {
	t.Run("in team", func(t *testing.T) {
		body := buildChannelSearch("Town", channelSearchOptions{teamID: "t1", userID: "u1", limit: 50})
		assert.JSONEq(t, `{"bool":{
			"filter":[
				{"terms":{"type":["O","P"]}},
				{"term":{"team_id":"t1"}},
				{"term":{"delete_at":0}},
				{"bool":{"should":[{"term":{"type":"O"}},{"term":{"user_ids":"u1"}},{"term":{"discoverable":true}}],"minimum_should_match":1}}
			],
			"should":[
				{"constant_score":{"filter":{"wildcard":{"display_name":{"value":"*town*"}}},"boost":10}},
				{"constant_score":{"filter":{"wildcard":{"name":{"value":"*town*"}}},"boost":1}},
				{"constant_score":{"filter":{"multi_match":{"query":"Town","type":"bool_prefix","operator":"and","fields":["name.text","display_name.text","purpose"]}},"boost":1}}
			],
			"minimum_should_match":1
		}}`, toJSON(t, body["query"]))
		assert.Equal(t, 50, body["size"])
	})

	t.Run("guest in all teams including deleted", func(t *testing.T) {
		body := buildChannelSearch("", channelSearchOptions{userID: "u1", userTeamIDs: []string{"t1", "t2"}, userChannelIDs: []string{"c1"}, isGuest: true, includeDeleted: true, limit: 50})
		assert.JSONEq(t, `{"bool":{"filter":[
			{"terms":{"type":["O","P"]}},
			{"terms":{"team_id":["t1","t2"]}},
			{"terms":{"id":["c1"]}}
		]}}`, toJSON(t, body["query"]))
	})

	t.Run("wildcard characters are escaped", func(t *testing.T) {
		body := buildChannelSearch("a*b?", channelSearchOptions{teamID: "t1", userID: "u1", limit: 50})
		assert.Contains(t, toJSON(t, body["query"]), `"value":"*a\\*b\\?*"`)
	})
}

func TestBuildUserSearch(t *testing.T) {
	t.Run("names only, active users of a team", func(t *testing.T) {
		body := buildUserSearch("@jo", userFilter{teamID: "t1"}, &model.UserSearchOptions{Limit: 5})
		assert.JSONEq(t, `{"bool":{"filter":[
			{"term":{"team_ids":"t1"}},
			{"term":{"delete_at":0}},
			{"bool":{"should":[
				{"term":{"id":"jo"}},
				{"wildcard":{"username":{"value":"*jo*"}}},
				{"wildcard":{"nickname":{"value":"*jo*"}}}
			],"minimum_should_match":1}}
		]}}`, toJSON(t, body["query"]))
		assert.Equal(t, 5, body["size"])
	})

	t.Run("full names, emails, inactive, roles, restrictions", func(t *testing.T) {
		options := &model.UserSearchOptions{
			AllowFullNames:   true,
			AllowEmails:      true,
			AllowInactive:    true,
			Role:             "system_admin",
			Roles:            []string{model.SystemUserRoleId, model.SystemGuestRoleId},
			ViewRestrictions: &model.ViewUsersRestrictions{Teams: []string{"t1"}, Channels: []string{"c9"}},
		}
		body := buildUserSearch("john doe", userFilter{inChannel: "c1", restrictedToChannels: []string{"c1", "c2"}}, options)
		q := toJSON(t, body["query"])
		assert.NotContains(t, q, "delete_at")
		assert.Contains(t, q, `{"term":{"channel_ids":"c1"}}`)
		assert.Contains(t, q, `{"terms":{"channel_ids":["c1","c2"]}}`)
		assert.Contains(t, q, `{"term":{"roles":"system_admin"}}`)
		assert.Contains(t, q, `{"bool":{"minimum_should_match":1,"should":[{"term":{"roles_raw":"system_user"}},{"term":{"roles":"system_guest"}}]}}`)
		assert.Contains(t, q, `{"terms":{"team_ids":["t1"]}}`)
		assert.Contains(t, q, `{"terms":{"channel_ids":["c9"]}}`)
		assert.Contains(t, q, `{"wildcard":{"email":{"value":"*doe*"}}}`)
		assert.Contains(t, q, `{"wildcard":{"first_name":{"value":"*john*"}}}`)
		assert.Equal(t, model.UserSearchDefaultLimit, body["size"])
	})

	t.Run("no visible team or channel", func(t *testing.T) {
		body := buildUserSearch("", userFilter{notInChannel: "c1"}, &model.UserSearchOptions{ViewRestrictions: &model.ViewUsersRestrictions{Teams: []string{}, Channels: []string{}}})
		q := toJSON(t, body["query"])
		assert.Contains(t, q, `{"match_none":{}}`)
		assert.Contains(t, q, `"must_not":[{"term":{"channel_ids":"c1"}}]`)
	})
}

func TestSearchPosts(t *testing.T) {
	forBothBackends(t, func(t *testing.T, f *fakeCluster) {
		e := startedEngine(t, f, func(cfg *model.Config) {
			cfg.ElasticsearchSettings.GlobalSearchPrefix = model.NewPointer("mm")
		})
		var target string
		f.searchHits = func(tgt string, body map[string]any) []map[string]any {
			target = tgt
			return []map[string]any{
				{"_id": "p1", "highlight": map[string]any{"message": []string{"the <mm-libre-hl>Apple</mm-libre-hl> and <mm-libre-hl>apples</mm-libre-hl>, <mm-libre-hl>Apple</mm-libre-hl>"}}},
				{"_id": "p2"},
			}
		}
		ids, matches, appErr := e.SearchPosts(model.ChannelList{{Id: "c1", TeamId: "t1"}}, model.ParseSearchParams("apple*", 0), 0, 20)
		require.Nil(t, appErr)
		assert.Equal(t, "mm*posts_*", target)
		assert.Equal(t, []string{"p1", "p2"}, ids)
		assert.Equal(t, model.PostSearchMatches{"p1": {"Apple", "apples"}}, matches)

		// No channel: no request.
		before := len(f.requestsTo("/_search"))
		ids, _, appErr = e.SearchPosts(model.ChannelList{}, model.ParseSearchParams("apple", 0), 0, 20)
		require.Nil(t, appErr)
		assert.Empty(t, ids)
		assert.Len(t, f.requestsTo("/_search"), before)
	})
}

func TestSearchPostsPublicChannelsSetting(t *testing.T) {
	f := newFakeCluster(t, false)
	e := startedEngine(t, f, nil)
	channels := model.ChannelList{{Id: "c1", TeamId: "t1"}}
	params := model.ParseSearchParams("x", 0)

	_, _, appErr := e.SearchPosts(channels, params, 0, 10)
	require.Nil(t, appErr)
	reqs := f.requestsTo("/_search")
	assert.Contains(t, string(reqs[len(reqs)-1].Body), `"channel_type":"O"`, "enabled by default")

	cfg := f.config()
	cfg.ComplianceSettings.Enable = model.NewPointer(true)
	e.UpdateConfig(cfg)
	_, _, appErr = e.SearchPosts(channels, params, 0, 10)
	require.Nil(t, appErr)
	reqs = f.requestsTo("/_search")
	assert.NotContains(t, string(reqs[len(reqs)-1].Body), `"channel_type":"O"`, "disabled by compliance mode")
}

func TestSearchFiles(t *testing.T) {
	f := newFakeCluster(t, true)
	e := startedEngine(t, f, nil)
	var target string
	f.searchHits = func(tgt string, body map[string]any) []map[string]any {
		target = tgt
		return []map[string]any{{"_id": "f2"}, {"_id": "f1"}}
	}
	ids, appErr := e.SearchFiles(model.ChannelList{{Id: "c1"}}, model.ParseSearchParams("ext:pdf report", 0), 0, 10)
	require.Nil(t, appErr)
	assert.Equal(t, "mm_files", target)
	assert.Equal(t, []string{"f2", "f1"}, ids)
}

func TestSearchChannels(t *testing.T) {
	f := newFakeCluster(t, false)
	e := startedEngine(t, f, nil)
	var bodies []map[string]any
	f.searchHits = func(tgt string, body map[string]any) []map[string]any {
		assert.Equal(t, "mm_channels", tgt)
		bodies = append(bodies, body)
		return []map[string]any{{"_id": "c1"}}
	}

	ids, appErr := e.SearchChannels("t1", "u1", "town", false, false)
	require.Nil(t, appErr)
	assert.Equal(t, []string{"c1"}, ids)

	// All teams: the teams of the user come from the user index.
	_, appErr = e.SearchChannels("", "u1", "town", false, false)
	require.NotNil(t, appErr, "the user isn't indexed: the caller falls back to the database")

	require.Nil(t, e.IndexUser(nil, &model.User{Id: "u1", Username: "user1"}, []string{"t1", "t2"}, []string{"c1", "c5"}))
	ids, appErr = e.SearchChannels("", "u1", "town", false, false)
	require.Nil(t, appErr)
	assert.Equal(t, []string{"c1"}, ids)
	assert.Contains(t, toJSON(t, bodies[len(bodies)-1]), `{"terms":{"team_id":["t1","t2"]}}`)

	_, appErr = e.SearchChannels("t1", "u1", "", true, false)
	require.Nil(t, appErr)
	assert.Contains(t, toJSON(t, bodies[len(bodies)-1]), `{"terms":{"id":["c1","c5"]}}`)
}

func TestSearchUsers(t *testing.T) {
	f := newFakeCluster(t, false)
	e := startedEngine(t, f, nil)
	var bodies []string
	f.searchHits = func(tgt string, body map[string]any) []map[string]any {
		assert.Equal(t, "mm_users", tgt)
		bodies = append(bodies, toJSON(t, body))
		return []map[string]any{{"_id": "u" + string(rune('0'+len(bodies)))}}
	}

	in, out, appErr := e.SearchUsersInChannel("t1", "c1", nil, "jo", &model.UserSearchOptions{})
	require.Nil(t, appErr)
	assert.Equal(t, []string{"u1"}, in)
	assert.Equal(t, []string{"u2"}, out)
	assert.Contains(t, bodies[0], `{"term":{"channel_ids":"c1"}}`)
	assert.NotContains(t, bodies[0], "team_ids")
	assert.Contains(t, bodies[1], `"must_not":[{"term":{"channel_ids":"c1"}}]`)
	assert.Contains(t, bodies[1], `{"term":{"team_ids":"t1"}}`)

	ids, appErr := e.SearchUsersInTeam("t1", []string{"c1", "dm1"}, "jo", &model.UserSearchOptions{})
	require.Nil(t, appErr)
	assert.Equal(t, []string{"u3"}, ids)
	assert.Contains(t, bodies[2], `{"terms":{"channel_ids":["c1","dm1"]}}`)

	// An empty list of allowed channels can't match anyone.
	ids, appErr = e.SearchUsersInTeam("t1", []string{}, "jo", &model.UserSearchOptions{})
	require.Nil(t, appErr)
	assert.Empty(t, ids)
	assert.Len(t, bodies, 3)
}

func TestResolvePurgeKinds(t *testing.T) {
	names := indexNames{prefix: "mm_"}
	kinds, err := resolvePurgeKinds(names, []string{"channels,users", "mm_files", "mm_posts_2024_01_01", "posts"})
	require.NoError(t, err)
	assert.Equal(t, []string{"channels", "users", "files", "posts"}, kinds)
	_, err = resolvePurgeKinds(names, []string{"foo"})
	require.Error(t, err)
}

func TestIndexNames(t *testing.T) {
	names := indexNames{prefix: "mm_"}
	assert.Equal(t, "mm_posts_2024_12_31", names.postIndexForTime(1735689599000))
	start, end, aggregated, ok := names.parsePostIndex("mm_posts_2024_02_28")
	require.True(t, ok)
	assert.False(t, aggregated)
	assert.Equal(t, "2024-02-28", start.Format("2006-01-02"))
	assert.Equal(t, "2024-02-29", end.Format("2006-01-02"))
	start, end, aggregated, ok = names.parsePostIndex("mm_posts_agg_2024_02")
	require.True(t, ok)
	assert.True(t, aggregated)
	assert.Equal(t, "2024-02-01", start.Format("2006-01-02"))
	assert.Equal(t, "2024-03-01", end.Format("2006-01-02"))
	_, _, _, ok = names.parsePostIndex("other_posts_2024_02_28")
	assert.False(t, ok)
	assert.Equal(t, "posts", names.kindOf("mm_posts_agg_2024_02"))
	assert.Equal(t, "channels", names.kindOf("mm_channels"))
	assert.Equal(t, "", names.kindOf("mm_other"))

	assert.True(t, wildcardMatch(".security*", ".security-7"))
	assert.True(t, wildcardMatch("*posts*01", "mm_posts_2024_01_01"))
	assert.False(t, wildcardMatch("posts", "mm_posts"))
}

func TestAnalysisAndMappings(t *testing.T) {
	cfg := &model.Config{}
	cfg.SetDefaults()
	cfg.ElasticsearchSettings.EnableCJKAnalyzers = model.NewPointer(true)
	cfg.ElasticsearchSettings.PostIndexShards = model.NewPointer(3)
	tpl := toJSON(t, indexTemplate(indexPosts, indexNames{prefix: "x_"}, &cfg.ElasticsearchSettings))
	assert.Contains(t, tpl, `"cjk_bigram"`)
	assert.Contains(t, tpl, `"number_of_shards":3`)
	assert.Contains(t, tpl, `"index_patterns":["x_posts_*"]`)
	assert.Contains(t, tpl, `"dynamic":false`)
}
