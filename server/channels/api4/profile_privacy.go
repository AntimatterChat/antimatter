// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"github.com/mattermost/mattermost/server/public/model"
)

// hideProfileDetails hides from the session's user the details of users they may not see (Antimatter's profile
// privacy, see app/profile_privacy.go): system admins see everything. It gives the ids of the users of whom something
// was hidden.
func hideProfileDetails(c *Context, users ...*model.User) map[string]bool {
	return c.App.HideProfileDetails(c.AppContext.Session().UserId, c.IsSystemAdmin(), users...)
}

// profileEtag is the etag of a profile as the session's user gets it: the same profile reads differently to people
// of different communities.
func profileEtag(c *Context, user *model.User) string {
	etag := user.Etag(*c.App.Config().PrivacySettings.ShowFullName, *c.App.Config().PrivacySettings.ShowEmailAddress)
	if c.AppContext.Session().UserId == user.Id {
		return etag
	}
	if key := model.ProfileFieldsKey(c.App.ProfileDetailsHiddenFrom(c.AppContext.Session().UserId, c.IsSystemAdmin(), user)); key != "" {
		etag += "." + key
	}
	return etag
}
