// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package engine

import (
	"context"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
)

type searchResponse struct {
	Hits struct {
		Hits []struct {
			ID        string              `json:"_id"`
			Highlight map[string][]string `json:"highlight"`
		} `json:"hits"`
	} `json:"hits"`
}

func (c *client) search(ctx context.Context, target string, body map[string]any) (*searchResponse, error) {
	var res searchResponse
	if err := c.doJSON(ctx, http.MethodPost, "/"+target+"/_search", ignoreUnavailable(), body, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

func (r *searchResponse) ids() []string {
	ids := make([]string, 0, len(r.Hits.Hits))
	for _, hit := range r.Hits.Hits {
		ids = append(ids, hit.ID)
	}
	return ids
}

var highlightRegex = regexp.MustCompile(regexp.QuoteMeta(highlightPreTag) + `(.*?)` + regexp.QuoteMeta(highlightPostTag))

// extractMatches returns the distinct highlighted words of a hit.
func extractMatches(highlight map[string][]string) []string {
	var matches []string
	for _, fragments := range highlight {
		for _, fragment := range fragments {
			for _, m := range highlightRegex.FindAllStringSubmatch(fragment, -1) {
				if word := strings.TrimSpace(m[1]); word != "" && !slices.Contains(matches, word) {
					matches = append(matches, word)
				}
			}
		}
	}
	return matches
}

func (e *Engine) publicChannelsWithoutMembership(cfg *model.Config) bool {
	return model.SafeDereference(cfg.ElasticsearchSettings.EnableSearchPublicChannelsWithoutMembership) &&
		!model.SafeDereference(cfg.ComplianceSettings.Enable)
}

// SearchPosts returns the ids of the posts matching the search, most recent
// first, restricted to the given channels (the channels of the user).
func (e *Engine) SearchPosts(channels model.ChannelList, searchParams []*model.SearchParams, page, perPage int) ([]string, model.PostSearchMatches, *model.AppError) {
	c, cfg, appErr := e.activeClient("Engine.SearchPosts")
	if appErr != nil {
		return nil, nil, appErr
	}
	if !model.SafeDereference(cfg.ElasticsearchSettings.EnableSearching) {
		return nil, nil, model.NewAppError("Engine.SearchPosts", "ent.elasticsearch.search_posts.disabled", e.backendParams(), "", http.StatusInternalServerError)
	}
	if len(channels) == 0 || len(searchParams) == 0 || perPage <= 0 {
		return []string{}, model.PostSearchMatches{}, nil
	}

	body := buildPostSearch(channels, searchParams, e.publicChannelsWithoutMembership(cfg), page, perPage)
	ctx, cancel := e.requestContext()
	defer cancel()
	res, err := c.search(ctx, namesFor(cfg).postsSearchPattern(), body)
	if err != nil {
		return nil, nil, model.NewAppError("Engine.SearchPosts", "ent.elasticsearch.search_posts.search_failed", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	matches := model.PostSearchMatches{}
	for _, hit := range res.Hits.Hits {
		if m := extractMatches(hit.Highlight); len(m) > 0 {
			matches[hit.ID] = m
		}
	}
	return res.ids(), matches, nil
}

// SearchFiles returns the ids of the files matching the search, most recent
// first, restricted to the given channels.
func (e *Engine) SearchFiles(channels model.ChannelList, searchParams []*model.SearchParams, page, perPage int) ([]string, *model.AppError) {
	c, cfg, appErr := e.activeClient("Engine.SearchFiles")
	if appErr != nil {
		return nil, appErr
	}
	if !model.SafeDereference(cfg.ElasticsearchSettings.EnableSearching) {
		return nil, model.NewAppError("Engine.SearchFiles", "ent.elasticsearch.search_files.disabled", e.backendParams(), "", http.StatusInternalServerError)
	}
	if len(channels) == 0 || len(searchParams) == 0 || perPage <= 0 {
		return []string{}, nil
	}

	ctx, cancel := e.requestContext()
	defer cancel()
	res, err := c.search(ctx, namesFor(cfg).filesSearchPattern(), buildFileSearch(channels, searchParams, page, perPage))
	if err != nil {
		return nil, model.NewAppError("Engine.SearchFiles", "ent.elasticsearch.search_files.search_failed", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	return res.ids(), nil
}

// indexedUser returns the teams and channels of a user from the user index.
func (e *Engine) indexedUser(ctx context.Context, c *client, cfg *model.Config, userID string) (*userDocument, error) {
	var res struct {
		Found  bool          `json:"found"`
		Source *userDocument `json:"_source"`
	}
	err := c.doJSON(ctx, http.MethodGet, "/"+namesFor(cfg).users()+"/_doc/"+userID, url.Values{"_source_includes": {"team_ids,channel_ids"}}, nil, &res)
	if err != nil && !isNotFound(err) {
		return nil, err
	}
	if !res.Found || res.Source == nil {
		return nil, errUserNotIndexed
	}
	return res.Source, nil
}

// SearchChannels implements channel autocomplete, in a team or in all the
// teams of the user (teamId empty).
func (e *Engine) SearchChannels(teamId, userID, term string, isGuest, includeDeleted bool) ([]string, *model.AppError) {
	c, cfg, appErr := e.activeClient("Engine.SearchChannels")
	if appErr != nil {
		return nil, appErr
	}
	if !model.SafeDereference(cfg.ElasticsearchSettings.EnableAutocomplete) {
		return nil, model.NewAppError("Engine.SearchChannels", "ent.elasticsearch.search_channels.disabled", e.backendParams(), "", http.StatusInternalServerError)
	}

	ctx, cancel := e.requestContext()
	defer cancel()

	opts := channelSearchOptions{
		teamID:         teamId,
		userID:         userID,
		isGuest:        isGuest,
		includeDeleted: includeDeleted,
		limit:          model.ChannelSearchDefaultLimit,
	}
	if teamId == "" || isGuest {
		// Resolve the memberships of the user from the user index. When the
		// user isn't indexed yet, fail so that the caller falls back to the
		// database.
		doc, err := e.indexedUser(ctx, c, cfg, userID)
		if err != nil {
			return nil, model.NewAppError("Engine.SearchChannels", "ent.elasticsearch.search_channels.search_failed", nil, "", http.StatusInternalServerError).Wrap(err)
		}
		opts.userTeamIDs = doc.TeamIds
		opts.userChannelIDs = doc.ChannelIds
	}

	res, err := c.search(ctx, namesFor(cfg).channels(), buildChannelSearch(term, opts))
	if err != nil {
		return nil, model.NewAppError("Engine.SearchChannels", "ent.elasticsearch.search_channels.search_failed", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	return res.ids(), nil
}

func (e *Engine) searchUsers(where, term string, filter userFilter, options *model.UserSearchOptions) ([]string, *model.AppError) {
	c, cfg, appErr := e.activeClient(where)
	if appErr != nil {
		return nil, appErr
	}
	if filter.restrictedToChannels != nil && len(filter.restrictedToChannels) == 0 {
		return []string{}, nil
	}
	if options == nil {
		options = &model.UserSearchOptions{}
	}
	ctx, cancel := e.requestContext()
	defer cancel()
	res, err := c.search(ctx, namesFor(cfg).users(), buildUserSearch(term, filter, options))
	if err != nil {
		return nil, model.NewAppError(where, "ent.elasticsearch.search_users.search_failed", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	return res.ids(), nil
}

// SearchUsersInChannel returns the users of the channel, and the users of the
// team who aren't in the channel, matching the term. restrictedToChannels,
// when not nil, limits the results to users sharing one of those channels.
func (e *Engine) SearchUsersInChannel(teamId, channelId string, restrictedToChannels []string, term string, options *model.UserSearchOptions) ([]string, []string, *model.AppError) {
	inChannel, appErr := e.searchUsers("Engine.SearchUsersInChannel", term, userFilter{
		inChannel:            channelId,
		restrictedToChannels: restrictedToChannels,
	}, options)
	if appErr != nil {
		return nil, nil, appErr
	}
	outOfChannel, appErr := e.searchUsers("Engine.SearchUsersInChannel", term, userFilter{
		teamID:               teamId,
		notInChannel:         channelId,
		restrictedToChannels: restrictedToChannels,
	}, options)
	if appErr != nil {
		return nil, nil, appErr
	}
	return inChannel, outOfChannel, nil
}

// SearchUsersInTeam returns the users of the team (all users when teamId is
// empty) matching the term.
func (e *Engine) SearchUsersInTeam(teamId string, restrictedToChannels []string, term string, options *model.UserSearchOptions) ([]string, *model.AppError) {
	return e.searchUsers("Engine.SearchUsersInTeam", term, userFilter{
		teamID:               teamId,
		restrictedToChannels: restrictedToChannels,
	}, options)
}

type engineError string

func (e engineError) Error() string { return string(e) }

const errUserNotIndexed = engineError("the user is not indexed")
