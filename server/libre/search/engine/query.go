// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package engine

import (
	"regexp"
	"slices"
	"strings"
	"unicode"

	"github.com/mattermost/mattermost/server/public/model"
)

// searchTerm is one element of a search expression.
type searchTerm struct {
	text   string
	phrase bool // quoted: all the words, in that order
	prefix bool // trailing wildcard
}

var quotedRegex = regexp.MustCompile(`"[^"]*"`)

// parseTerms splits a term string into words and quoted phrases. Terms
// without any letter or digit are dropped, as they can't match anything.
func parseTerms(s string) []searchTerm {
	var terms []searchTerm
	add := func(text string, phrase bool) {
		text = strings.TrimSpace(text)
		prefix := strings.HasSuffix(text, "*")
		text = strings.TrimSpace(strings.ReplaceAll(text, "*", ""))
		if !hasAlphaNum(text) {
			return
		}
		terms = append(terms, searchTerm{text: text, phrase: phrase, prefix: prefix})
	}

	rest := quotedRegex.ReplaceAllStringFunc(s, func(match string) string {
		add(strings.Trim(match, `"`), true)
		return " "
	})
	for word := range strings.FieldsSeq(strings.ReplaceAll(rest, `"`, " ")) {
		add(word, false)
	}
	return terms
}

func hasAlphaNum(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

// isSingleToken reports whether the text is analyzed into a single token by
// the standard tokenizer, in which case prefix queries can be used.
func isSingleToken(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) || unicode.Is(unicode.Hangul, r) {
			return false
		}
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && !unicode.IsMark(r) && r != '_' {
			return false
		}
	}
	return true
}

// textTermQuery matches a term in any of the analyzed fields.
func textTermQuery(t searchTerm, fields []string) map[string]any {
	if t.prefix && isSingleToken(t.text) {
		should := make([]any, 0, len(fields))
		for _, f := range fields {
			should = append(should, map[string]any{"prefix": map[string]any{f: map[string]any{"value": strings.ToLower(t.text)}}})
		}
		if len(should) == 1 {
			return should[0].(map[string]any)
		}
		return map[string]any{"bool": map[string]any{"should": should, "minimum_should_match": 1}}
	}

	matchType := "phrase"
	if t.prefix {
		matchType = "phrase_prefix"
	}
	mm := map[string]any{
		"query":  t.text,
		"type":   matchType,
		"fields": fields,
	}
	if t.prefix {
		mm["max_expansions"] = 200
	}
	return map[string]any{"multi_match": mm}
}

func hashtagTermQuery(t searchTerm) map[string]any {
	tag := strings.ToLower(t.text)
	if !strings.HasPrefix(tag, "#") {
		tag = "#" + tag
	}
	if t.prefix {
		return map[string]any{"prefix": map[string]any{"hashtags": map[string]any{"value": tag}}}
	}
	return termQuery("hashtags", tag)
}

type boolQuery struct {
	filter  []any
	must    []any
	should  []any
	mustNot []any
	msm     int
}

func (b *boolQuery) build() map[string]any {
	q := map[string]any{}
	if len(b.filter) > 0 {
		q["filter"] = b.filter
	}
	if len(b.must) > 0 {
		q["must"] = b.must
	}
	if len(b.should) > 0 {
		q["should"] = b.should
		q["minimum_should_match"] = max(b.msm, 1)
	}
	if len(b.mustNot) > 0 {
		q["must_not"] = b.mustNot
	}
	return map[string]any{"bool": q}
}

func matchNone() map[string]any {
	return map[string]any{"match_none": map[string]any{}}
}

// dateFilters applies the on:, before:, after: filters (and their
// exclusions) on the create_at field, with the database search semantics.
func dateFilters(b *boolQuery, p *model.SearchParams) bool {
	used := false
	rng := func(op string, v int64) map[string]any {
		return map[string]any{"range": map[string]any{"create_at": map[string]any{op: v}}}
	}
	if p.OnDate != "" {
		start, end := p.GetOnDateMillis()
		b.filter = append(b.filter, map[string]any{"range": map[string]any{"create_at": map[string]any{"gte": start, "lte": end}}})
		return true
	}
	if p.ExcludedDate != "" {
		start, end := p.GetExcludedDateMillis()
		b.mustNot = append(b.mustNot, map[string]any{"range": map[string]any{"create_at": map[string]any{"gte": start, "lte": end}}})
		used = true
	}
	if p.AfterDate != "" {
		b.filter = append(b.filter, rng("gte", p.GetAfterDateMillis()))
		used = true
	}
	if p.BeforeDate != "" {
		b.filter = append(b.filter, rng("lte", p.GetBeforeDateMillis()))
		used = true
	}
	if p.ExcludedAfterDate != "" {
		b.filter = append(b.filter, rng("lt", p.GetExcludedAfterDateMillis()))
		used = true
	}
	if p.ExcludedBeforeDate != "" {
		b.filter = append(b.filter, rng("gt", p.GetExcludedBeforeDateMillis()))
		used = true
	}
	return used
}

// commonFilters applies the in:, from: filters and dates. It reports whether
// any criterion was applied.
func commonFilters(b *boolQuery, p *model.SearchParams, userField string) bool {
	used := false
	if len(p.InChannels) > 0 {
		b.filter = append(b.filter, termsQuery("channel_id", p.InChannels))
		used = true
	}
	if len(p.ExcludedChannels) > 0 {
		b.mustNot = append(b.mustNot, termsQuery("channel_id", p.ExcludedChannels))
		used = true
	}
	if len(p.FromUsers) > 0 {
		b.filter = append(b.filter, termsQuery(userField, p.FromUsers))
		used = true
	}
	if len(p.ExcludedUsers) > 0 {
		b.mustNot = append(b.mustNot, termsQuery(userField, p.ExcludedUsers))
		used = true
	}
	if dateFilters(b, p) {
		used = true
	}
	return used
}

// termClauses adds the included and excluded terms to the query.
func termClauses(b *boolQuery, p *model.SearchParams, toQuery func(searchTerm) map[string]any) bool {
	used := false
	included := parseTerms(p.Terms)
	for _, t := range included {
		if p.OrTerms {
			b.should = append(b.should, toQuery(t))
		} else {
			b.must = append(b.must, toQuery(t))
		}
		used = true
	}
	for _, t := range parseTerms(p.ExcludedTerms) {
		b.mustNot = append(b.mustNot, toQuery(t))
		used = true
	}
	return used
}

var postTextFields = []string{"message", "attachments"}

// postParamsQuery builds the query of one SearchParams.
func postParamsQuery(p *model.SearchParams) map[string]any {
	b := &boolQuery{}
	used := commonFilters(b, p, "user_id")
	toQuery := func(t searchTerm) map[string]any { return textTermQuery(t, postTextFields) }
	if p.IsHashtag {
		toQuery = hashtagTermQuery
	}
	if termClauses(b, p, toQuery) {
		used = true
	}
	if !used {
		return matchNone()
	}
	return b.build()
}

// postChannelScope restricts a post search to the channels the user belongs
// to, and optionally to the public channels of the teams of these channels.
func postChannelScope(channels model.ChannelList, publicWithoutMembership bool) map[string]any {
	ids := make([]string, 0, len(channels))
	var teams []string
	for _, ch := range channels {
		if ch == nil {
			continue
		}
		ids = append(ids, ch.Id)
		if ch.TeamId != "" && !slices.Contains(teams, ch.TeamId) {
			teams = append(teams, ch.TeamId)
		}
	}
	membership := termsQuery("channel_id", ids)
	if !publicWithoutMembership || len(teams) == 0 {
		return membership
	}
	public := (&boolQuery{filter: []any{
		termQuery("channel_type", string(model.ChannelTypeOpen)),
		termsQuery("team_id", teams),
	}}).build()
	return (&boolQuery{should: []any{membership, public}}).build()
}

const (
	highlightPreTag  = "<mm-libre-hl>"
	highlightPostTag = "</mm-libre-hl>"
)

// buildPostSearch builds the body of a post search request.
func buildPostSearch(channels model.ChannelList, paramsList []*model.SearchParams, publicWithoutMembership bool, page, perPage int) map[string]any {
	root := &boolQuery{
		filter:  []any{postChannelScope(channels, publicWithoutMembership)},
		mustNot: []any{map[string]any{"prefix": map[string]any{"type": model.PostSystemMessagePrefix}}},
	}
	for _, p := range paramsList {
		root.should = append(root.should, postParamsQuery(p))
	}
	if len(root.should) == 0 {
		root.must = append(root.must, matchNone())
	}

	return map[string]any{
		"query":            root.build(),
		"from":             max(page, 0) * perPage,
		"size":             perPage,
		"_source":          false,
		"track_total_hits": false,
		"sort": []any{
			map[string]any{"create_at": map[string]any{"order": "desc"}},
			map[string]any{"id": map[string]any{"order": "desc"}},
		},
		"highlight": map[string]any{
			"pre_tags":            []string{highlightPreTag},
			"post_tags":           []string{highlightPostTag},
			"require_field_match": false,
			"fields": map[string]any{
				"message":     map[string]any{"number_of_fragments": 0},
				"attachments": map[string]any{"number_of_fragments": 0},
				"hashtags":    map[string]any{"number_of_fragments": 0},
			},
		},
	}
}

var fileTextFields = []string{"name", "name.text", "content"}

func fileParamsQuery(p *model.SearchParams) map[string]any {
	b := &boolQuery{}
	used := commonFilters(b, p, "user_id")
	if len(p.Extensions) > 0 {
		b.filter = append(b.filter, termsQuery("extension", normalizeExtensions(p.Extensions)))
		used = true
	}
	if len(p.ExcludedExtensions) > 0 {
		b.mustNot = append(b.mustNot, termsQuery("extension", normalizeExtensions(p.ExcludedExtensions)))
		used = true
	}
	if termClauses(b, p, func(t searchTerm) map[string]any { return textTermQuery(t, fileTextFields) }) {
		used = true
	}
	if !used {
		return matchNone()
	}
	return b.build()
}

func normalizeExtensions(exts []string) []string {
	out := make([]string, 0, len(exts))
	for _, ext := range exts {
		out = append(out, strings.ToLower(strings.TrimPrefix(ext, ".")))
	}
	return out
}

// buildFileSearch builds the body of a file search request.
func buildFileSearch(channels model.ChannelList, paramsList []*model.SearchParams, page, perPage int) map[string]any {
	ids := make([]string, 0, len(channels))
	for _, ch := range channels {
		if ch != nil {
			ids = append(ids, ch.Id)
		}
	}
	root := &boolQuery{filter: []any{termsQuery("channel_id", ids)}}
	for _, p := range paramsList {
		root.should = append(root.should, fileParamsQuery(p))
	}
	if len(root.should) == 0 {
		root.must = append(root.must, matchNone())
	}
	return map[string]any{
		"query":            root.build(),
		"from":             max(page, 0) * perPage,
		"size":             perPage,
		"_source":          false,
		"track_total_hits": false,
		"sort": []any{
			map[string]any{"create_at": map[string]any{"order": "desc"}},
			map[string]any{"id": map[string]any{"order": "desc"}},
		},
	}
}

// escapeWildcard escapes the characters with a special meaning in wildcard
// queries.
func escapeWildcard(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `*`, `\*`, `?`, `\?`)
	return r.Replace(s)
}

func containsQuery(field, term string) map[string]any {
	return map[string]any{"wildcard": map[string]any{field: map[string]any{"value": "*" + escapeWildcard(strings.ToLower(term)) + "*"}}}
}

func constantScore(q map[string]any, boost float64) map[string]any {
	return map[string]any{"constant_score": map[string]any{"filter": q, "boost": boost}}
}

// channelSearchOptions are the visibility rules of a channel autocomplete.
type channelSearchOptions struct {
	teamID         string
	userID         string
	userTeamIDs    []string // teams of the user, used when teamID is empty
	userChannelIDs []string // channels of the user, used for guests
	isGuest        bool
	includeDeleted bool
	limit          int
}

// buildChannelSearch mirrors the database channel autocomplete: substring
// match on name and display name, word prefix match on name, display name and
// purpose, channels whose display name matches first.
func buildChannelSearch(term string, opts channelSearchOptions) map[string]any {
	b := &boolQuery{filter: []any{termsQuery("type", []string{string(model.ChannelTypeOpen), string(model.ChannelTypePrivate)})}}

	if opts.teamID != "" {
		b.filter = append(b.filter, termQuery("team_id", opts.teamID))
	} else {
		b.filter = append(b.filter, termsQuery("team_id", nonNil(opts.userTeamIDs)))
	}
	if !opts.includeDeleted {
		b.filter = append(b.filter, termQuery("delete_at", 0))
	}
	if opts.isGuest {
		b.filter = append(b.filter, termsQuery("id", nonNil(opts.userChannelIDs)))
	} else {
		visible := &boolQuery{should: []any{
			termQuery("type", string(model.ChannelTypeOpen)),
			termQuery("user_ids", opts.userID),
			termQuery("discoverable", true),
		}}
		b.filter = append(b.filter, visible.build())
	}

	term = strings.TrimSpace(term)
	if term != "" {
		b.should = []any{
			constantScore(containsQuery("display_name", term), 10),
			constantScore(containsQuery("name", term), 1),
			constantScore(map[string]any{"multi_match": map[string]any{
				"query":    term,
				"type":     "bool_prefix",
				"operator": "and",
				"fields":   []string{"name.text", "display_name.text", "purpose"},
			}}, 1),
		}
	}

	return map[string]any{
		"query":            b.build(),
		"size":             opts.limit,
		"_source":          false,
		"track_total_hits": false,
		"sort": []any{
			map[string]any{"_score": map[string]any{"order": "desc"}},
			map[string]any{"display_name": map[string]any{"order": "asc"}},
			map[string]any{"id": map[string]any{"order": "asc"}},
		},
	}
}

// userSearchFields returns the fields searched according to the privacy
// options, as the database search does.
func userSearchFields(options *model.UserSearchOptions) []string {
	fields := []string{"username", "nickname"}
	if options.AllowFullNames {
		fields = append(fields, "first_name", "last_name")
	}
	if options.AllowEmails {
		fields = append(fields, "email")
	}
	return fields
}

// userFilter describes the membership part of a user search.
type userFilter struct {
	teamID               string
	inChannel            string
	notInChannel         string
	restrictedToChannels []string // nil means no restriction
}

func buildUserSearch(term string, f userFilter, options *model.UserSearchOptions) map[string]any {
	b := &boolQuery{}
	if f.teamID != "" {
		b.filter = append(b.filter, termQuery("team_ids", f.teamID))
	}
	if f.inChannel != "" {
		b.filter = append(b.filter, termQuery("channel_ids", f.inChannel))
	}
	if f.notInChannel != "" {
		b.mustNot = append(b.mustNot, termQuery("channel_ids", f.notInChannel))
	}
	if f.restrictedToChannels != nil {
		b.filter = append(b.filter, termsQuery("channel_ids", f.restrictedToChannels))
	}
	if !options.AllowInactive {
		b.filter = append(b.filter, termQuery("delete_at", 0))
	}
	if options.Role != "" {
		b.filter = append(b.filter, termQuery("roles", options.Role))
	}
	if len(options.Roles) > 0 && options.Roles[0] != "" {
		roles := &boolQuery{}
		for _, role := range options.Roles {
			if role == model.SystemUserRoleId {
				// Only users having no other system role.
				roles.should = append(roles.should, termQuery("roles_raw", role))
			} else {
				roles.should = append(roles.should, termQuery("roles", role))
			}
		}
		b.filter = append(b.filter, roles.build())
	}
	if vr := options.ViewRestrictions; vr != nil {
		if vr.Teams != nil && len(vr.Teams) == 0 && vr.Channels != nil && len(vr.Channels) == 0 {
			b.filter = append(b.filter, matchNone())
		}
		if len(vr.Teams) > 0 {
			b.filter = append(b.filter, termsQuery("team_ids", vr.Teams))
		}
		if len(vr.Channels) > 0 {
			b.filter = append(b.filter, termsQuery("channel_ids", vr.Channels))
		}
	}

	fields := userSearchFields(options)
	for word := range strings.FieldsSeq(strings.TrimLeft(term, "@")) {
		word = strings.TrimLeft(word, "@")
		if word == "" {
			continue
		}
		anyField := &boolQuery{should: []any{termQuery("id", word)}}
		for _, field := range fields {
			anyField.should = append(anyField.should, containsQuery(field, word))
		}
		b.filter = append(b.filter, anyField.build())
	}

	limit := options.Limit
	if limit <= 0 {
		limit = model.UserSearchDefaultLimit
	}
	return map[string]any{
		"query":            b.build(),
		"size":             min(limit, model.UserSearchMaxLimit),
		"_source":          false,
		"track_total_hits": false,
		"sort": []any{
			map[string]any{"username": map[string]any{"order": "asc"}},
		},
	}
}
