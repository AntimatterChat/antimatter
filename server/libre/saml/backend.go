// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package saml

import (
	"errors"
	"net/http"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/app"
	"github.com/mattermost/mattermost/server/v8/channels/store"
	"github.com/mattermost/mattermost/server/v8/einterfaces"
)

// backend is the narrow set of server operations the SAML service relies on.
// It is satisfied by appBackend (a thin wrapper around *app.App) in production
// and by an in-memory fake in unit tests.
type backend interface {
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

type appBackend struct {
	a *app.App
}

func (b *appBackend) Config() *model.Config           { return b.a.Config() }
func (b *appBackend) GetSiteURL() string              { return b.a.GetSiteURL() }
func (b *appBackend) Ldap() einterfaces.LdapInterface { return b.a.Ldap() }
func (b *appBackend) GetConfigFile(n string) ([]byte, error) {
	return b.a.Srv().Platform().GetConfigFile(n)
}

func (b *appBackend) GetUserByAuth(authData string) (*model.User, *model.AppError) {
	return b.a.GetUserByAuth(&authData, model.UserAuthServiceSaml)
}

func (b *appBackend) GetUserByEmail(email string) (*model.User, *model.AppError) {
	return b.a.GetUserByEmail(email)
}

func (b *appBackend) GetUserByUsername(username string) (*model.User, *model.AppError) {
	return b.a.GetUserByUsername(username)
}

func (b *appBackend) CreateUser(rctx request.CTX, user *model.User, guest bool) (*model.User, *model.AppError) {
	if guest {
		return b.a.CreateGuest(rctx, user)
	}
	return b.a.CreateUser(rctx, user)
}

func (b *appBackend) SaveUser(rctx request.CTX, user *model.User) (*model.User, *model.AppError) {
	result, err := b.a.Srv().Store().User().Update(rctx, user, false)
	if err != nil {
		var appErr *model.AppError
		var invErr *store.ErrInvalidInput
		var conErr *store.ErrConflict
		switch {
		case errors.As(err, &appErr):
			return nil, appErr
		case errors.As(err, &conErr):
			if conErr.Resource == "Username" {
				return nil, model.NewAppError("SaveUser", "app.user.save.username_exists.app_error", nil, "", http.StatusBadRequest).Wrap(err)
			}
			return nil, model.NewAppError("SaveUser", "app.user.save.email_exists.app_error", nil, "", http.StatusBadRequest).Wrap(err)
		case errors.As(err, &invErr):
			return nil, model.NewAppError("SaveUser", "app.user.update.find.app_error", nil, "", http.StatusBadRequest).Wrap(err)
		default:
			return nil, model.NewAppError("SaveUser", "app.user.update.finding.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
		}
	}
	b.a.InvalidateCacheForUser(user.Id)

	updated := result.New
	if result.Old != nil && updated.Username != result.Old.Username && updated.LastPictureUpdate <= 0 {
		if appErr := b.a.UpdateDefaultProfileImage(rctx, updated); appErr != nil {
			rctx.Logger().Warn("Failed to update default profile image after SAML username change", mlog.Err(appErr))
		}
	}
	return updated, nil
}

func (b *appBackend) UpdateUserRoles(rctx request.CTX, user *model.User, roles string) (*model.User, *model.AppError) {
	return b.a.UpdateUserRolesWithUser(rctx, user, roles, true)
}

func (b *appBackend) DemoteUserToGuest(rctx request.CTX, user *model.User) *model.AppError {
	return b.a.DemoteUserToGuest(rctx, user)
}

func (b *appBackend) UpdateAuthData(userID string, authData string, email string, resetMfa bool) *model.AppError {
	if _, err := b.a.Srv().Store().User().UpdateAuthData(userID, model.UserAuthServiceSaml, &authData, email, resetMfa); err != nil {
		var invErr *store.ErrInvalidInput
		if errors.As(err, &invErr) {
			return model.NewAppError("UpdateAuthData", "app.user.update_auth_data.email_exists.app_error", nil, "", http.StatusBadRequest).Wrap(err)
		}
		return model.NewAppError("UpdateAuthData", "app.user.update_auth_data.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	b.a.InvalidateCacheForUser(userID)
	return nil
}

func (b *appBackend) GetSamlEmailToken(token string) (*model.Token, *model.AppError) {
	return b.a.GetSamlEmailToken(token)
}

func (b *appBackend) DeleteToken(token string) error {
	return b.a.Srv().Store().Token().Delete(token)
}

func (b *appBackend) GetPropertyGroup(rctx request.CTX, name string) (*model.PropertyGroup, *model.AppError) {
	return b.a.GetPropertyGroup(rctx, name)
}

func (b *appBackend) SearchPropertyFields(rctx request.CTX, groupID string, opts model.PropertyFieldSearchOpts) ([]*model.PropertyField, *model.AppError) {
	return b.a.SearchPropertyFields(rctx, groupID, opts)
}

func (b *appBackend) SearchPropertyValues(rctx request.CTX, groupID string, opts model.PropertyValueSearchOpts) ([]*model.PropertyValue, *model.AppError) {
	return b.a.SearchPropertyValues(rctx, groupID, opts)
}

func (b *appBackend) UpsertPropertyValues(rctx request.CTX, values []*model.PropertyValue, objectType, targetID string) ([]*model.PropertyValue, *model.AppError) {
	return b.a.UpsertPropertyValues(rctx, values, objectType, targetID, "")
}

func (b *appBackend) DeletePropertyValue(rctx request.CTX, groupID, valueID string) *model.AppError {
	return b.a.DeletePropertyValue(rctx, groupID, valueID)
}
