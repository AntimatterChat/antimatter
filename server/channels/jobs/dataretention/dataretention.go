// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

// Package dataretention implements einterfaces.DataRetentionInterface (the
// global policy view and the CRUD of granular team/channel retention
// policies) and the data retention job which permanently deletes messages
// and files according to the global and granular policies.
package dataretention

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/v8/channels/app"
	"github.com/mattermost/mattermost/server/v8/channels/store"
	"github.com/mattermost/mattermost/server/v8/einterfaces"
	ejobs "github.com/mattermost/mattermost/server/v8/einterfaces/jobs"
)

const (
	// PostDurationForever is the post duration (in days) of a policy which
	// keeps messages forever.
	PostDurationForever = int64(-1)

	maxPolicyDisplayNameLength = 64

	msPerHour = int64(60 * 60 * 1000)
)

func init() {
	app.RegisterDataRetentionInterface(func(a *app.App) einterfaces.DataRetentionInterface {
		return New(func() store.Store { return a.Srv().Store() }, a.Config)
	})

	app.RegisterJobsDataRetentionJobInterface(func(s *app.Server) ejobs.DataRetentionJobInterface {
		return NewJob(serverJobDeps(s))
	})
}

// DataRetention implements einterfaces.DataRetentionInterface.
type DataRetention struct {
	store  func() store.Store
	config func() *model.Config
	now    func() int64
}

var _ einterfaces.DataRetentionInterface = (*DataRetention)(nil)

// New creates a DataRetention backed by the given store and configuration.
func New(storeFn func() store.Store, config func() *model.Config) *DataRetention {
	return &DataRetention{store: storeFn, config: config, now: model.GetMillis}
}

func internalError(where string, err error) *model.AppError {
	return model.NewAppError("DataRetention."+where, "ent.data_retention.policies.internal_error", nil, "", http.StatusInternalServerError).Wrap(err)
}

func invalidPolicyError(where, details string) *model.AppError {
	return model.NewAppError("DataRetention."+where, "ent.data_retention.policies.invalid_policy", nil, details, http.StatusBadRequest)
}

func notFoundError(where, policyID string) *model.AppError {
	return model.NewAppError("DataRetention."+where, "ent.data_retention.policies.not_found", map[string]any{"PolicyId": policyID}, "policy_id="+policyID, http.StatusNotFound)
}

// storeError converts a store error into an AppError.
func storeError(where string, err error) *model.AppError {
	var appErr *model.AppError
	var nfErr *store.ErrNotFound
	var invErr *store.ErrInvalidInput
	switch {
	case errors.As(err, &appErr):
		return appErr
	case errors.As(err, &nfErr):
		// The store reports missing teams/channels this way.
		return invalidPolicyError(where, nfErr.Error()).Wrap(err)
	case errors.As(err, &invErr):
		return invalidPolicyError(where, invErr.Error()).Wrap(err)
	default:
		return internalError(where, err)
	}
}

// GetGlobalPolicy returns the global retention policy as configured in
// DataRetentionSettings.
func (d *DataRetention) GetGlobalPolicy() (*model.GlobalRetentionPolicy, *model.AppError) {
	settings := d.config().DataRetentionSettings
	now := d.now()
	policy := &model.GlobalRetentionPolicy{
		MessageDeletionEnabled: model.SafeDereference(settings.EnableMessageDeletion),
		FileDeletionEnabled:    model.SafeDereference(settings.EnableFileDeletion),
	}
	if policy.MessageDeletionEnabled {
		policy.MessageRetentionCutoff = now - int64(settings.GetMessageRetentionHours())*msPerHour
	}
	if policy.FileDeletionEnabled {
		policy.FileRetentionCutoff = now - int64(settings.GetFileRetentionHours())*msPerHour
	}
	return policy, nil
}

func (d *DataRetention) GetPolicies(offset, limit int) (*model.RetentionPolicyWithTeamAndChannelCountsList, *model.AppError) {
	policies, err := d.store().RetentionPolicy().GetAll(offset, limit)
	if err != nil {
		return nil, internalError("GetPolicies", err)
	}
	count, err := d.store().RetentionPolicy().GetCount()
	if err != nil {
		return nil, internalError("GetPolicies", err)
	}
	if policies == nil {
		policies = []*model.RetentionPolicyWithTeamAndChannelCounts{}
	}
	return &model.RetentionPolicyWithTeamAndChannelCountsList{Policies: policies, TotalCount: count}, nil
}

func (d *DataRetention) GetPoliciesCount() (int64, *model.AppError) {
	count, err := d.store().RetentionPolicy().GetCount()
	if err != nil {
		return 0, internalError("GetPoliciesCount", err)
	}
	return count, nil
}

func (d *DataRetention) GetPolicy(policyID string) (*model.RetentionPolicyWithTeamAndChannelCounts, *model.AppError) {
	if !model.IsValidId(policyID) {
		return nil, notFoundError("GetPolicy", policyID)
	}
	policy, err := d.store().RetentionPolicy().Get(policyID)
	if err != nil {
		var nfErr *store.ErrNotFound
		if errors.Is(err, sql.ErrNoRows) || errors.As(err, &nfErr) {
			return nil, notFoundError("GetPolicy", policyID)
		}
		return nil, internalError("GetPolicy", err)
	}
	return policy, nil
}

func validateDisplayName(where, name string) *model.AppError {
	if strings.TrimSpace(name) == "" {
		return invalidPolicyError(where, "display_name is required")
	}
	if utf8.RuneCountInString(name) > maxPolicyDisplayNameLength {
		return invalidPolicyError(where, "display_name is too long")
	}
	return nil
}

func validatePostDuration(where string, duration *int64) *model.AppError {
	if duration == nil {
		return invalidPolicyError(where, "post_duration is required")
	}
	if *duration != PostDurationForever && *duration < 1 {
		return invalidPolicyError(where, "post_duration must be -1 (forever) or a positive number of days")
	}
	return nil
}

func validateIDs(where, field string, ids []string) *model.AppError {
	for _, id := range ids {
		if !model.IsValidId(id) {
			return invalidPolicyError(where, "invalid id in "+field+": "+id)
		}
	}
	return nil
}

func dedupe(ids []string) []string {
	if ids == nil {
		return nil
	}
	seen := make(map[string]struct{}, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func (d *DataRetention) CreatePolicy(policy *model.RetentionPolicyWithTeamAndChannelIDs) (*model.RetentionPolicyWithTeamAndChannelCounts, *model.AppError) {
	const where = "CreatePolicy"
	if policy == nil {
		return nil, invalidPolicyError(where, "policy is required")
	}
	if appErr := validateDisplayName(where, policy.DisplayName); appErr != nil {
		return nil, appErr
	}
	if appErr := validatePostDuration(where, policy.PostDurationDays); appErr != nil {
		return nil, appErr
	}
	policy.TeamIDs = dedupe(policy.TeamIDs)
	policy.ChannelIDs = dedupe(policy.ChannelIDs)
	if appErr := validateIDs(where, "team_ids", policy.TeamIDs); appErr != nil {
		return nil, appErr
	}
	if appErr := validateIDs(where, "channel_ids", policy.ChannelIDs); appErr != nil {
		return nil, appErr
	}

	// The ID is always generated by the store.
	policy.ID = ""
	newPolicy, err := d.store().RetentionPolicy().Save(policy)
	if err != nil {
		return nil, storeError(where, err)
	}
	return newPolicy, nil
}

func (d *DataRetention) PatchPolicy(patch *model.RetentionPolicyWithTeamAndChannelIDs) (*model.RetentionPolicyWithTeamAndChannelCounts, *model.AppError) {
	const where = "PatchPolicy"
	if patch == nil {
		return nil, invalidPolicyError(where, "policy is required")
	}
	if _, appErr := d.GetPolicy(patch.ID); appErr != nil {
		return nil, appErr
	}
	if patch.DisplayName != "" {
		if appErr := validateDisplayName(where, patch.DisplayName); appErr != nil {
			return nil, appErr
		}
	}
	if patch.PostDurationDays != nil {
		if appErr := validatePostDuration(where, patch.PostDurationDays); appErr != nil {
			return nil, appErr
		}
	}
	patch.TeamIDs = dedupe(patch.TeamIDs)
	patch.ChannelIDs = dedupe(patch.ChannelIDs)
	if appErr := validateIDs(where, "team_ids", patch.TeamIDs); appErr != nil {
		return nil, appErr
	}
	if appErr := validateIDs(where, "channel_ids", patch.ChannelIDs); appErr != nil {
		return nil, appErr
	}

	policy, err := d.store().RetentionPolicy().Patch(patch)
	if err != nil {
		return nil, storeError(where, err)
	}
	return policy, nil
}

func (d *DataRetention) DeletePolicy(policyID string) *model.AppError {
	if _, appErr := d.GetPolicy(policyID); appErr != nil {
		return appErr
	}
	if err := d.store().RetentionPolicy().Delete(policyID); err != nil {
		return internalError("DeletePolicy", err)
	}
	return nil
}

func (d *DataRetention) GetTeamsForPolicy(policyID string, offset, limit int) (*model.TeamsWithCount, *model.AppError) {
	if _, appErr := d.GetPolicy(policyID); appErr != nil {
		return nil, appErr
	}
	teams, err := d.store().RetentionPolicy().GetTeams(policyID, offset, limit)
	if err != nil {
		return nil, internalError("GetTeamsForPolicy", err)
	}
	count, err := d.store().RetentionPolicy().GetTeamsCount(policyID)
	if err != nil {
		return nil, internalError("GetTeamsForPolicy", err)
	}
	if teams == nil {
		teams = []*model.Team{}
	}
	return &model.TeamsWithCount{Teams: teams, TotalCount: count}, nil
}

func (d *DataRetention) AddTeamsToPolicy(policyID string, teamIDs []string) *model.AppError {
	const where = "AddTeamsToPolicy"
	if _, appErr := d.GetPolicy(policyID); appErr != nil {
		return appErr
	}
	teamIDs = dedupe(teamIDs)
	if appErr := validateIDs(where, "team_ids", teamIDs); appErr != nil {
		return appErr
	}
	if err := d.store().RetentionPolicy().AddTeams(policyID, teamIDs); err != nil {
		return storeError(where, err)
	}
	return nil
}

func (d *DataRetention) RemoveTeamsFromPolicy(policyID string, teamIDs []string) *model.AppError {
	const where = "RemoveTeamsFromPolicy"
	if _, appErr := d.GetPolicy(policyID); appErr != nil {
		return appErr
	}
	if appErr := validateIDs(where, "team_ids", teamIDs); appErr != nil {
		return appErr
	}
	if err := d.store().RetentionPolicy().RemoveTeams(policyID, dedupe(teamIDs)); err != nil {
		return storeError(where, err)
	}
	return nil
}

func (d *DataRetention) GetChannelsForPolicy(policyID string, offset, limit int) (*model.ChannelsWithCount, *model.AppError) {
	if _, appErr := d.GetPolicy(policyID); appErr != nil {
		return nil, appErr
	}
	channels, err := d.store().RetentionPolicy().GetChannels(policyID, offset, limit)
	if err != nil {
		return nil, internalError("GetChannelsForPolicy", err)
	}
	count, err := d.store().RetentionPolicy().GetChannelsCount(policyID)
	if err != nil {
		return nil, internalError("GetChannelsForPolicy", err)
	}
	if channels == nil {
		channels = model.ChannelListWithTeamData{}
	}
	return &model.ChannelsWithCount{Channels: channels, TotalCount: count}, nil
}

func (d *DataRetention) AddChannelsToPolicy(policyID string, channelIDs []string) *model.AppError {
	const where = "AddChannelsToPolicy"
	if _, appErr := d.GetPolicy(policyID); appErr != nil {
		return appErr
	}
	channelIDs = dedupe(channelIDs)
	if appErr := validateIDs(where, "channel_ids", channelIDs); appErr != nil {
		return appErr
	}
	if err := d.store().RetentionPolicy().AddChannels(policyID, channelIDs); err != nil {
		return storeError(where, err)
	}
	return nil
}

func (d *DataRetention) RemoveChannelsFromPolicy(policyID string, channelIDs []string) *model.AppError {
	const where = "RemoveChannelsFromPolicy"
	if _, appErr := d.GetPolicy(policyID); appErr != nil {
		return appErr
	}
	if appErr := validateIDs(where, "channel_ids", channelIDs); appErr != nil {
		return appErr
	}
	if err := d.store().RetentionPolicy().RemoveChannels(policyID, dedupe(channelIDs)); err != nil {
		return storeError(where, err)
	}
	return nil
}

func (d *DataRetention) GetTeamPoliciesForUser(userID string, offset, limit int) (*model.RetentionPolicyForTeamList, *model.AppError) {
	policies, err := d.store().RetentionPolicy().GetTeamPoliciesForUser(userID, offset, limit)
	if err != nil {
		return nil, internalError("GetTeamPoliciesForUser", err)
	}
	count, err := d.store().RetentionPolicy().GetTeamPoliciesCountForUser(userID)
	if err != nil {
		return nil, internalError("GetTeamPoliciesForUser", err)
	}
	if policies == nil {
		policies = []*model.RetentionPolicyForTeam{}
	}
	return &model.RetentionPolicyForTeamList{Policies: policies, TotalCount: count}, nil
}

func (d *DataRetention) GetChannelPoliciesForUser(userID string, offset, limit int) (*model.RetentionPolicyForChannelList, *model.AppError) {
	policies, err := d.store().RetentionPolicy().GetChannelPoliciesForUser(userID, offset, limit)
	if err != nil {
		return nil, internalError("GetChannelPoliciesForUser", err)
	}
	count, err := d.store().RetentionPolicy().GetChannelPoliciesCountForUser(userID)
	if err != nil {
		return nil, internalError("GetChannelPoliciesForUser", err)
	}
	if policies == nil {
		policies = []*model.RetentionPolicyForChannel{}
	}
	return &model.RetentionPolicyForChannelList{Policies: policies, TotalCount: count}, nil
}
