// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package accountmigration

import (
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

// Backend is the set of server operations used by the migrations. The app
// layer implements it (channels/app/account_migration_backend.go).
type Backend interface {
	UsersByAuthService(service string) ([]*model.User, error)
	UserByAuth(authData string, service string) (*model.User, error)
	UserByUsername(username string) (*model.User, error)
	LdapUsers(rctx request.CTX) ([]*model.User, *model.AppError)
	UpdateAuthData(userID string, service string, authData string) error
	UpdateUsername(rctx request.CTX, userID string, username string) error
}
