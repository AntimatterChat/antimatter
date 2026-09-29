// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package libre

import (
	// Each import registers its implementation from init().
	_ "github.com/mattermost/mattermost/server/v8/libre/accesscontrol"
	_ "github.com/mattermost/mattermost/server/v8/libre/accountmigration"
	_ "github.com/mattermost/mattermost/server/v8/libre/autotranslation"
	_ "github.com/mattermost/mattermost/server/v8/libre/cluster"
	_ "github.com/mattermost/mattermost/server/v8/libre/compliance"
	_ "github.com/mattermost/mattermost/server/v8/libre/dataretention"
	_ "github.com/mattermost/mattermost/server/v8/libre/intune"
	_ "github.com/mattermost/mattermost/server/v8/libre/ldap"
	_ "github.com/mattermost/mattermost/server/v8/libre/messageexport"
	_ "github.com/mattermost/mattermost/server/v8/libre/metrics"
	_ "github.com/mattermost/mattermost/server/v8/libre/notification"
	_ "github.com/mattermost/mattermost/server/v8/libre/oauth/google"
	_ "github.com/mattermost/mattermost/server/v8/libre/oauth/office365"
	_ "github.com/mattermost/mattermost/server/v8/libre/oauth/openid"
	_ "github.com/mattermost/mattermost/server/v8/libre/outgoingoauth"
	_ "github.com/mattermost/mattermost/server/v8/libre/pushproxy"
	_ "github.com/mattermost/mattermost/server/v8/libre/saml"
	_ "github.com/mattermost/mattermost/server/v8/libre/search"
)
