// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package saml

import (
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/einterfaces"
)

// Backend is the narrow set of server operations the SAML service relies on.
// The app layer implements it (channels/app/saml_backend.go); the unit tests
// use an in-memory fake.
type Backend interface {
	Config() *model.Config
	GetSiteURL() string
	GetConfigFile(name string) ([]byte, error)
	Ldap() einterfaces.LdapInterface

	GetUserByAuth(authData string) (*model.User, *model.AppError)
	GetUserByEmail(email string) (*model.User, *model.AppError)
	GetUserByUsername(username string) (*model.User, *model.AppError)
	CreateUser(rctx request.CTX, user *model.User, guest bool) (*model.User, *model.AppError)
	// SaveUser persists profile fields (not roles) of an existing user.
	SaveUser(rctx request.CTX, user *model.User) (*model.User, *model.AppError)
	UpdateUserRoles(rctx request.CTX, user *model.User, roles string) (*model.User, *model.AppError)
	DemoteUserToGuest(rctx request.CTX, user *model.User) *model.AppError
	UpdateAuthData(userID string, authData string, email string, resetMfa bool) *model.AppError

	GetSamlEmailToken(token string) (*model.Token, *model.AppError)
	DeleteToken(token string) error

	GetPropertyGroup(rctx request.CTX, name string) (*model.PropertyGroup, *model.AppError)
	SearchPropertyFields(rctx request.CTX, groupID string, opts model.PropertyFieldSearchOpts) ([]*model.PropertyField, *model.AppError)
	SearchPropertyValues(rctx request.CTX, groupID string, opts model.PropertyValueSearchOpts) ([]*model.PropertyValue, *model.AppError)
	UpsertPropertyValues(rctx request.CTX, values []*model.PropertyValue, objectType, targetID string) ([]*model.PropertyValue, *model.AppError)
	DeletePropertyValue(rctx request.CTX, groupID, valueID string) *model.AppError
}

// The ids of the app layer's errors for a missing account
// (app.MissingAuthAccountError and app.MissingAccountError), which the backend
// returns when no user matches.
const (
	MissingAuthAccountErrorID = "app.user.get_by_auth.missing_account.app_error"
	MissingAccountErrorID     = "app.user.missing_account.const"
)
