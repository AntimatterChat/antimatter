// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package app

import (
	"net/http"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/app/accountmigration"
)

// accountMigrationBackend implements accountmigration.Backend on top of the
// application layer.
type accountMigrationBackend struct {
	a *App
}

var _ accountmigration.Backend = (*accountMigrationBackend)(nil)

func (b *accountMigrationBackend) UsersByAuthService(service string) ([]*model.User, error) {
	return b.a.Srv().Store().User().GetAllUsingAuthService(service)
}

func (b *accountMigrationBackend) UserByAuth(authData string, service string) (*model.User, error) {
	return b.a.Srv().Store().User().GetByAuth(&authData, service)
}

func (b *accountMigrationBackend) UserByUsername(username string) (*model.User, error) {
	return b.a.Srv().Store().User().GetByUsername(username)
}

func (b *accountMigrationBackend) LdapUsers(rctx request.CTX) ([]*model.User, *model.AppError) {
	ldapI := b.a.Ldap()
	if ldapI == nil {
		return nil, model.NewAppError("AccountMigration", "ent.ldap.disabled.app_error", nil, "", http.StatusNotImplemented)
	}
	return ldapI.GetAllLdapUsers(rctx)
}

func (b *accountMigrationBackend) UpdateAuthData(userID string, service string, authData string) error {
	if _, err := b.a.Srv().Store().User().UpdateAuthData(userID, service, &authData, "", false); err != nil {
		return err
	}
	b.a.InvalidateCacheForUser(userID)
	return nil
}

func (b *accountMigrationBackend) UpdateUsername(rctx request.CTX, userID string, username string) error {
	user, appErr := b.a.GetUser(rctx, userID)
	if appErr != nil {
		return appErr
	}
	user.Username = username
	if _, appErr := b.a.UpdateUser(rctx, user, false); appErr != nil {
		return appErr
	}
	return nil
}
