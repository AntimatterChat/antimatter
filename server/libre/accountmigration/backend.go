// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package accountmigration

import (
	"net/http"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/app"
)

// backend is the set of server operations used by the migrations.
type backend interface {
	UsersByAuthService(service string) ([]*model.User, error)
	UserByAuth(authData string, service string) (*model.User, error)
	UserByUsername(username string) (*model.User, error)
	LdapUsers(rctx request.CTX) ([]*model.User, *model.AppError)
	UpdateAuthData(userID string, service string, authData string) error
	UpdateUsername(rctx request.CTX, userID string, username string) error
}

type appBackend struct {
	a *app.App
}

func (b *appBackend) UsersByAuthService(service string) ([]*model.User, error) {
	return b.a.Srv().Store().User().GetAllUsingAuthService(service)
}

func (b *appBackend) UserByAuth(authData string, service string) (*model.User, error) {
	return b.a.Srv().Store().User().GetByAuth(&authData, service)
}

func (b *appBackend) UserByUsername(username string) (*model.User, error) {
	return b.a.Srv().Store().User().GetByUsername(username)
}

func (b *appBackend) LdapUsers(rctx request.CTX) ([]*model.User, *model.AppError) {
	ldapI := b.a.Ldap()
	if ldapI == nil {
		return nil, model.NewAppError("AccountMigration", "ent.ldap.disabled.app_error", nil, "", http.StatusNotImplemented)
	}
	return ldapI.GetAllLdapUsers(rctx)
}

func (b *appBackend) UpdateAuthData(userID string, service string, authData string) error {
	if _, err := b.a.Srv().Store().User().UpdateAuthData(userID, service, &authData, "", false); err != nil {
		return err
	}
	b.a.InvalidateCacheForUser(userID)
	return nil
}

func (b *appBackend) UpdateUsername(rctx request.CTX, userID string, username string) error {
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
