// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package libre

import (
	// Each import registers its implementation from init().
	_ "github.com/mattermost/mattermost/server/v8/channels/app/accesscontrol"
	_ "github.com/mattermost/mattermost/server/v8/channels/app/autotranslation"
	_ "github.com/mattermost/mattermost/server/v8/channels/app/pushproxy"
	_ "github.com/mattermost/mattermost/server/v8/libre/notification"
)
