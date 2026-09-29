// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package ldap

import (
	"github.com/mattermost/mattermost/server/v8/channels/app"
	"github.com/mattermost/mattermost/server/v8/channels/app/platform"
	"github.com/mattermost/mattermost/server/v8/einterfaces"
	ejobs "github.com/mattermost/mattermost/server/v8/einterfaces/jobs"
)

func init() {
	app.RegisterLdapInterface(func(a *app.App) einterfaces.LdapInterface {
		return newLdap(&appBackend{a: a}, nil)
	})
	app.RegisterJobsLdapSyncInterface(func(a *app.App) ejobs.LdapSyncInterface {
		return &syncJob{a: a}
	})
	platform.RegisterLdapDiagnosticInterface(func(ps *platform.PlatformService) einterfaces.LdapDiagnosticInterface {
		return newDiagnostic(ps, nil)
	})
}
