// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package app

import (
	"errors"
	"net/http"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/app/saml"
	"github.com/mattermost/mattermost/server/v8/channels/store"
	"github.com/mattermost/mattermost/server/v8/einterfaces"
)

// samlBackend implements saml.Backend on top of the application layer.
type samlBackend struct {
	a *App
}

var _ saml.Backend = (*samlBackend)(nil)

func (b *samlBackend) Config() *model.Config           { return b.a.Config() }
func (b *samlBackend) GetSiteURL() string              { return b.a.GetSiteURL() }
func (b *samlBackend) Ldap() einterfaces.LdapInterface { return b.a.Ldap() }
func (b *samlBackend) GetConfigFile(n string) ([]byte, error) {
	return b.a.Srv().Platform().GetConfigFile(n)
}

func (b *samlBackend) GetUserByAuth(authData string) (*model.User, *model.AppError) {
	return b.a.GetUserByAuth(&authData, model.UserAuthServiceSaml)
}

func (b *samlBackend) GetUserByEmail(email string) (*model.User, *model.AppError) {
	return b.a.GetUserByEmail(email)
}

func (b *samlBackend) GetUserByUsername(username string) (*model.User, *model.AppError) {
	return b.a.GetUserByUsername(username)
}

func (b *samlBackend) CreateUser(rctx request.CTX, user *model.User, guest bool) (*model.User, *model.AppError) {
	if guest {
		return b.a.CreateGuest(rctx, user)
	}
	return b.a.CreateUser(rctx, user)
}

func (b *samlBackend) SaveUser(rctx request.CTX, user *model.User) (*model.User, *model.AppError) {
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

func (b *samlBackend) UpdateUserRoles(rctx request.CTX, user *model.User, roles string) (*model.User, *model.AppError) {
	return b.a.UpdateUserRolesWithUser(rctx, user, roles, true)
}

func (b *samlBackend) DemoteUserToGuest(rctx request.CTX, user *model.User) *model.AppError {
	return b.a.DemoteUserToGuest(rctx, user)
}

func (b *samlBackend) UpdateAuthData(userID string, authData string, email string, resetMfa bool) *model.AppError {
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

func (b *samlBackend) GetSamlEmailToken(token string) (*model.Token, *model.AppError) {
	return b.a.GetSamlEmailToken(token)
}

func (b *samlBackend) DeleteToken(token string) error {
	return b.a.Srv().Store().Token().Delete(token)
}

func (b *samlBackend) GetPropertyGroup(rctx request.CTX, name string) (*model.PropertyGroup, *model.AppError) {
	return b.a.GetPropertyGroup(rctx, name)
}

func (b *samlBackend) SearchPropertyFields(rctx request.CTX, groupID string, opts model.PropertyFieldSearchOpts) ([]*model.PropertyField, *model.AppError) {
	return b.a.SearchPropertyFields(rctx, groupID, opts)
}

func (b *samlBackend) SearchPropertyValues(rctx request.CTX, groupID string, opts model.PropertyValueSearchOpts) ([]*model.PropertyValue, *model.AppError) {
	return b.a.SearchPropertyValues(rctx, groupID, opts)
}

func (b *samlBackend) UpsertPropertyValues(rctx request.CTX, values []*model.PropertyValue, objectType, targetID string) ([]*model.PropertyValue, *model.AppError) {
	return b.a.UpsertPropertyValues(rctx, values, objectType, targetID, "")
}

func (b *samlBackend) DeletePropertyValue(rctx request.CTX, groupID, valueID string) *model.AppError {
	return b.a.DeletePropertyValue(rctx, groupID, valueID)
}
