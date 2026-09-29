// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"net/http"

	"github.com/mattermost/mattermost/server/public/model"
)

func (a *App) GetGlobalRetentionPolicy() (*model.GlobalRetentionPolicy, *model.AppError) {
	if a.DataRetention() == nil {
		return nil, newDataRetentionUnavailableError("GetGlobalRetentionPolicy")
	}
	return a.DataRetention().GetGlobalPolicy()
}

func (a *App) GetRetentionPolicies(offset, limit int) (*model.RetentionPolicyWithTeamAndChannelCountsList, *model.AppError) {
	if a.DataRetention() == nil {
		return nil, newDataRetentionUnavailableError("GetRetentionPolicies")
	}
	return a.DataRetention().GetPolicies(offset, limit)
}

func (a *App) GetRetentionPoliciesCount() (int64, *model.AppError) {
	if a.DataRetention() == nil {
		return 0, newDataRetentionUnavailableError("GetRetentionPoliciesCount")
	}
	return a.DataRetention().GetPoliciesCount()
}

func (a *App) GetRetentionPolicy(policyID string) (*model.RetentionPolicyWithTeamAndChannelCounts, *model.AppError) {
	if a.DataRetention() == nil {
		return nil, newDataRetentionUnavailableError("GetRetentionPolicy")
	}
	return a.DataRetention().GetPolicy(policyID)
}

func (a *App) CreateRetentionPolicy(policy *model.RetentionPolicyWithTeamAndChannelIDs) (*model.RetentionPolicyWithTeamAndChannelCounts, *model.AppError) {
	if a.DataRetention() == nil {
		return nil, newDataRetentionUnavailableError("CreateRetentionPolicy")
	}
	return a.DataRetention().CreatePolicy(policy)
}

func (a *App) PatchRetentionPolicy(patch *model.RetentionPolicyWithTeamAndChannelIDs) (*model.RetentionPolicyWithTeamAndChannelCounts, *model.AppError) {
	if a.DataRetention() == nil {
		return nil, newDataRetentionUnavailableError("PatchRetentionPolicy")
	}
	return a.DataRetention().PatchPolicy(patch)
}

func (a *App) DeleteRetentionPolicy(policyID string) *model.AppError {
	if a.DataRetention() == nil {
		return newDataRetentionUnavailableError("DeleteRetentionPolicy")
	}
	return a.DataRetention().DeletePolicy(policyID)
}

func (a *App) GetTeamsForRetentionPolicy(policyID string, offset, limit int) (*model.TeamsWithCount, *model.AppError) {
	if a.DataRetention() == nil {
		return nil, newDataRetentionUnavailableError("GetTeamsForRetentionPolicy")
	}
	return a.DataRetention().GetTeamsForPolicy(policyID, offset, limit)
}

func (a *App) AddTeamsToRetentionPolicy(policyID string, teamIDs []string) *model.AppError {
	if a.DataRetention() == nil {
		return newDataRetentionUnavailableError("AddTeamsToRetentionPolicy")
	}
	return a.DataRetention().AddTeamsToPolicy(policyID, teamIDs)
}

func (a *App) RemoveTeamsFromRetentionPolicy(policyID string, teamIDs []string) *model.AppError {
	if a.DataRetention() == nil {
		return newDataRetentionUnavailableError("RemoveTeamsFromRetentionPolicy")
	}
	return a.DataRetention().RemoveTeamsFromPolicy(policyID, teamIDs)
}

func (a *App) GetChannelsForRetentionPolicy(policyID string, offset, limit int) (*model.ChannelsWithCount, *model.AppError) {
	if a.DataRetention() == nil {
		return nil, newDataRetentionUnavailableError("GetChannelsForRetentionPolicy")
	}
	return a.DataRetention().GetChannelsForPolicy(policyID, offset, limit)
}

func (a *App) AddChannelsToRetentionPolicy(policyID string, channelIDs []string) *model.AppError {
	if a.DataRetention() == nil {
		return newDataRetentionUnavailableError("AddChannelsToRetentionPolicies")
	}
	return a.DataRetention().AddChannelsToPolicy(policyID, channelIDs)
}

func (a *App) RemoveChannelsFromRetentionPolicy(policyID string, channelIDs []string) *model.AppError {
	if a.DataRetention() == nil {
		return newDataRetentionUnavailableError("RemoveChannelsFromRetentionPolicy")
	}
	return a.DataRetention().RemoveChannelsFromPolicy(policyID, channelIDs)
}

func (a *App) GetTeamPoliciesForUser(userID string, offset, limit int) (*model.RetentionPolicyForTeamList, *model.AppError) {
	if a.DataRetention() == nil {
		return nil, newDataRetentionUnavailableError("GetTeamPoliciesForUser")
	}
	return a.DataRetention().GetTeamPoliciesForUser(userID, offset, limit)
}

func (a *App) GetChannelPoliciesForUser(userID string, offset, limit int) (*model.RetentionPolicyForChannelList, *model.AppError) {
	if a.DataRetention() == nil {
		return nil, newDataRetentionUnavailableError("GetChannelPoliciesForUser")
	}
	return a.DataRetention().GetChannelPoliciesForUser(userID, offset, limit)
}

func newDataRetentionUnavailableError(methodName string) *model.AppError {
	return model.NewAppError("App."+methodName, "ent.data_retention.generic.license.error",
		nil, "", http.StatusNotImplemented)
}
