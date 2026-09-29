// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"net/http"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/utils"
	"github.com/mattermost/mattermost/server/v8/platform/services/ipfiltering"
)

// ClientIPAddress returns the address of the client that sent r, honouring the
// configured trusted proxy headers.
func (s *Server) ClientIPAddress(r *http.Request) string {
	return utils.GetIPAddress(r, s.platform.Config().ServiceSettings.TrustedProxyIPHeader)
}

// updateIPFilter puts the IP filtering rules of cfg in force.
func (s *Server) updateIPFilter(cfg *model.Config) error {
	rules, err := ipfiltering.Compile(cfg.IPFilteringSettings.Rules)
	if err != nil {
		return err
	}
	s.ipFilter.Set(rules)
	return nil
}

// GetIPFilters returns the IP filtering rules.
func (a *App) GetIPFilters() model.AllowedIPRanges {
	rules := a.Config().IPFilteringSettings.Rules
	if rules == nil {
		return model.AllowedIPRanges{}
	}
	return rules
}

// ApplyIPFilters replaces the IP filtering rules. It refuses rules that would lock
// out requesterIP, the address the change is made from, since the requester could
// not undo them.
func (a *App) ApplyIPFilters(rctx request.CTX, ranges model.AllowedIPRanges, requesterIP string) (model.AllowedIPRanges, *model.AppError) {
	normalized := make(model.AllowedIPRanges, len(ranges))
	for i, r := range ranges {
		r.CIDRBlock = strings.TrimSpace(r.CIDRBlock)
		if r.Action == "" {
			r.Action = model.IPFilterActionAllow
		}
		if r.OwnerID == "" {
			r.OwnerID = rctx.Session().UserId
		}
		normalized[i] = r
	}

	if appErr := normalized.IsValid(); appErr != nil {
		return nil, appErr
	}

	rules, err := ipfiltering.Compile(normalized)
	if err != nil {
		return nil, model.NewAppError("ApplyIPFilters", "model.ip_filtering.is_valid.cidr_block.app_error", nil, "", http.StatusBadRequest).Wrap(err)
	}
	if !rules.AllowsString(requesterIP) {
		return nil, model.NewAppError("ApplyIPFilters", "app.ip_filtering.apply.lockout.app_error", map[string]any{"IP": requesterIP}, "", http.StatusBadRequest)
	}

	cfg := a.Config().Clone()
	cfg.IPFilteringSettings.Rules = normalized
	if _, _, appErr := a.SaveConfig(cfg, true); appErr != nil {
		return nil, appErr
	}

	return a.GetIPFilters(), nil
}

// SendIPFiltersChangedEmail tells every system admin that userID changed the IP
// filtering rules.
func (a *App) SendIPFiltersChangedEmail(rctx request.CTX, userID string) error {
	initiatingUser, appErr := a.GetUser(rctx, userID)
	if appErr != nil {
		return appErr
	}

	users, err := a.Srv().Store().User().GetSystemAdminProfiles()
	if err != nil {
		return err
	}

	siteURL := *a.Config().ServiceSettings.SiteURL
	for _, user := range users {
		if err = a.Srv().EmailService.SendIPFiltersChangedEmail(user.Email, initiatingUser, siteURL, user.Locale); err != nil {
			rctx.Logger().Error("Error while sending IP filters changed email", mlog.String("user_id", user.Id), mlog.Err(err))
		}
	}

	return nil
}
