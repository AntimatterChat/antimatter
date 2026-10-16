// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package libre

import (
	// Each import registers its implementation from init().
	_ "github.com/mattermost/mattermost/server/v8/channels/app/accesscontrol"
	_ "github.com/mattermost/mattermost/server/v8/channels/app/accountmigration"
	_ "github.com/mattermost/mattermost/server/v8/channels/app/autotranslation"
	_ "github.com/mattermost/mattermost/server/v8/channels/app/intune"
	_ "github.com/mattermost/mattermost/server/v8/channels/app/oauthproviders/google"
	_ "github.com/mattermost/mattermost/server/v8/channels/app/oauthproviders/office365"
	_ "github.com/mattermost/mattermost/server/v8/channels/app/oauthproviders/openid"
	_ "github.com/mattermost/mattermost/server/v8/channels/app/outgoingoauth"
	_ "github.com/mattermost/mattermost/server/v8/channels/app/pushproxy"
	_ "github.com/mattermost/mattermost/server/v8/channels/app/saml"
	_ "github.com/mattermost/mattermost/server/v8/channels/jobs/compliance"
	_ "github.com/mattermost/mattermost/server/v8/channels/jobs/dataretention"
	_ "github.com/mattermost/mattermost/server/v8/channels/jobs/messageexport"
	_ "github.com/mattermost/mattermost/server/v8/libre/notification"
	_ "github.com/mattermost/mattermost/server/v8/libre/search"
)
