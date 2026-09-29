// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

func (a *App) SendIPFiltersChangedEmail(rctx request.CTX, userID string) error {
	initiatingUser, err := a.Srv().Store().User().GetProfileByIds(rctx, []string{userID}, nil, true)
	if err != nil {
		rctx.Logger().Error("Failed to get initiating user", mlog.Err(err))
	}

	users, err := a.Srv().Store().User().GetSystemAdminProfiles()
	if err != nil {
		rctx.Logger().Error("Failed to get system admins", mlog.Err(err))
	}

	for _, user := range users {
		if err = a.Srv().EmailService.SendIPFiltersChangedEmail(user.Email, initiatingUser[0], *a.Config().ServiceSettings.SiteURL, *a.Config().CloudSettings.CWSURL, user.Locale, false); err != nil {
			rctx.Logger().Error("Error while sending IP filters changed email", mlog.Err(err))
		}
	}

	return nil
}
