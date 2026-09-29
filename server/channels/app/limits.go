// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"net/http"

	"github.com/mattermost/mattermost/server/public/model"
)

// GetServerLimits reports the server's limits. There are no user seat, single-channel guest or
// message history limits, so every limit field is always zero ("no limit"). The active user count
// is only computed when includeUserCounts is true, because the query is comparatively expensive and
// the count is only consumed by admin-gated UI.
func (a *App) GetServerLimits(includeUserCounts bool) (*model.ServerLimits, *model.AppError) {
	limits := &model.ServerLimits{}

	if !includeUserCounts {
		return limits, nil
	}

	activeUserCount, err := a.Srv().Store().User().Count(model.UserCountOptions{})
	if err != nil {
		return nil, model.NewAppError("GetServerLimits", "app.limits.get_app_limits.user_count.store_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	limits.ActiveUserCount = activeUserCount

	return limits, nil
}

// shouldTrackSingleChannelGuests reports whether the single-channel guest statistic is relevant,
// i.e. whether guest accounts are enabled.
func (a *App) shouldTrackSingleChannelGuests() bool {
	cfg := a.Config()
	if cfg == nil || cfg.GuestAccountsSettings.Enable == nil {
		return false
	}

	return *cfg.GuestAccountsSettings.Enable
}
