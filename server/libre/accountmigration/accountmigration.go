// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

// Package accountmigration implements the bulk migration of user accounts
// from one authentication service to AD/LDAP or SAML.
package accountmigration

import (
	"net/http"
	"sort"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/app"
	"github.com/mattermost/mattermost/server/v8/einterfaces"
)

func init() {
	app.RegisterAccountMigrationInterface(func(a *app.App) einterfaces.AccountMigrationInterface {
		return &AccountMigration{b: &appBackend{a: a}}
	})
}

// AccountMigration implements einterfaces.AccountMigrationInterface.
type AccountMigration struct {
	b backend
}

var _ einterfaces.AccountMigrationInterface = (*AccountMigration)(nil)

const (
	matchFieldEmail    = "email"
	matchFieldUsername = "username"
)

// usersToMigrate returns the human users using the given authentication
// service ("" for email/password accounts).
func (m *AccountMigration) usersToMigrate(fromAuthService string) ([]*model.User, *model.AppError) {
	users, err := m.b.UsersByAuthService(fromAuthService)
	if err != nil {
		return nil, model.NewAppError("AccountMigration", "ent.account_migration.get_all_failed", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	res := make([]*model.User, 0, len(users))
	for _, u := range users {
		if u.IsBot || u.IsRemote() {
			continue
		}
		res = append(res, u)
	}
	return res, nil
}

// MigrateToLdap binds the accounts using fromAuthService to their AD/LDAP
// account, matched on the email address or the username. Users that cannot be
// matched are left untouched. Unless force is set, the migration is refused
// when several AD/LDAP accounts share the same value of the matching field;
// with force, the ambiguous accounts are skipped.
func (m *AccountMigration) MigrateToLdap(rctx request.CTX, fromAuthService string, foreignUserFieldNameToMatch string, force bool, dryRun bool) *model.AppError {
	matchField := strings.ToLower(strings.TrimSpace(foreignUserFieldNameToMatch))
	if matchField != matchFieldEmail && matchField != matchFieldUsername {
		return model.NewAppError("MigrateToLdap", "api.context.invalid_param.app_error", map[string]any{"Name": "match_field"}, "", http.StatusBadRequest)
	}
	if fromAuthService == model.UserAuthServiceLdap {
		return model.NewAppError("MigrateToLdap", "api.context.invalid_param.app_error", map[string]any{"Name": "from"}, "", http.StatusBadRequest)
	}

	key := func(u *model.User) string {
		if matchField == matchFieldEmail {
			return strings.ToLower(strings.TrimSpace(u.Email))
		}
		return strings.ToLower(strings.TrimSpace(u.Username))
	}

	ldapUsers, appErr := m.b.LdapUsers(rctx)
	if appErr != nil {
		return appErr
	}

	byKey := make(map[string]*model.User, len(ldapUsers))
	duplicates := map[string]bool{}
	for _, lu := range ldapUsers {
		if lu.AuthData == nil || *lu.AuthData == "" {
			continue
		}
		k := key(lu)
		if k == "" {
			continue
		}
		if _, ok := byKey[k]; ok {
			duplicates[k] = true
			continue
		}
		byKey[k] = lu
	}

	users, appErr := m.usersToMigrate(fromAuthService)
	if appErr != nil {
		return appErr
	}

	if len(duplicates) > 0 {
		dups := make([]string, 0, len(duplicates))
		for k := range duplicates {
			dups = append(dups, k)
		}
		sort.Strings(dups)
		if !force {
			return model.NewAppError("MigrateToLdap", "ent.migration.migratetoldap.duplicate_field", nil, "duplicates: "+strings.Join(dups, ", "), http.StatusBadRequest)
		}
		rctx.Logger().Warn("Skipping AD/LDAP accounts with duplicate values of the matching field", mlog.String("match_field", matchField), mlog.Array("values", dups))
	}

	// Plan the migration before changing anything.
	type change struct {
		user     *model.User
		authData string
	}
	var changes []change
	claimed := map[string]string{}
	for _, user := range users {
		k := key(user)
		if duplicates[k] {
			continue
		}
		lu, ok := byKey[k]
		if !ok {
			rctx.Logger().Warn("Unable to find user on AD/LDAP server, skipping", mlog.String("user_id", user.Id), mlog.String("match_field", matchField))
			continue
		}
		authData := *lu.AuthData
		if other, ok := claimed[authData]; ok && other != user.Id {
			rctx.Logger().Warn("AD/LDAP account matched by several users, skipping", mlog.String("user_id", user.Id))
			continue
		}
		existing, err := m.b.UserByAuth(authData, model.UserAuthServiceLdap)
		if err == nil && existing != nil && existing.Id != user.Id {
			rctx.Logger().Warn("AD/LDAP account already bound to another user, skipping", mlog.String("user_id", user.Id), mlog.String("other_user_id", existing.Id))
			continue
		}
		claimed[authData] = user.Id
		changes = append(changes, change{user: user, authData: authData})
	}

	for _, c := range changes {
		if dryRun {
			rctx.Logger().Info("Dry run: would migrate user to AD/LDAP", mlog.String("user_id", c.user.Id))
			continue
		}
		if err := m.b.UpdateAuthData(c.user.Id, model.UserAuthServiceLdap, c.authData); err != nil {
			return model.NewAppError("MigrateToLdap", "app.user.update_auth_data.app_error", nil, "user_id="+c.user.Id, http.StatusInternalServerError).Wrap(err)
		}
		rctx.Logger().Info("Migrated user to AD/LDAP", mlog.String("user_id", c.user.Id))
	}
	return nil
}

// MigrateToSaml binds the accounts using fromAuthService to SAML. With auto,
// every account is migrated assuming its username and email are identical in
// the SAML identity provider. Otherwise usersMap maps the email of the
// accounts to migrate to their SAML username.
func (m *AccountMigration) MigrateToSaml(rctx request.CTX, fromAuthService string, usersMap map[string]string, auto bool, dryRun bool) *model.AppError {
	if fromAuthService == model.UserAuthServiceSaml {
		return model.NewAppError("MigrateToSaml", "api.context.invalid_param.app_error", map[string]any{"Name": "from"}, "", http.StatusBadRequest)
	}

	users, appErr := m.usersToMigrate(fromAuthService)
	if appErr != nil {
		return appErr
	}

	samlUsers, err := m.b.UsersByAuthService(model.UserAuthServiceSaml)
	if err != nil {
		return model.NewAppError("MigrateToSaml", "ent.account_migration.get_saml_users_failed", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	samlAuthData := make(map[string]string, len(samlUsers))
	for _, u := range samlUsers {
		if u.AuthData != nil {
			samlAuthData[strings.ToLower(*u.AuthData)] = u.Id
		}
	}

	mapping := make(map[string]string, len(usersMap))
	for email, username := range usersMap {
		mapping[strings.ToLower(strings.TrimSpace(email))] = strings.TrimSpace(username)
	}

	type change struct {
		user     *model.User
		username string
		email    string
	}
	var changes []change
	for _, user := range users {
		email := strings.ToLower(user.Email)
		username := user.Username
		if !auto {
			mapped, ok := mapping[email]
			if !ok {
				rctx.Logger().Debug("User not found in the users file, skipping", mlog.String("user_id", user.Id))
				continue
			}
			if mapped != "" {
				username = strings.ToLower(mapped)
			}
		}

		if other, ok := samlAuthData[email]; ok && other != user.Id {
			rctx.Logger().Warn("Email already used by another SAML user, skipping", mlog.String("user_id", user.Id), mlog.String("other_user_id", other))
			continue
		}

		if username != user.Username {
			if !model.IsValidUsername(username) {
				rctx.Logger().Warn("Invalid SAML username, skipping", mlog.String("user_id", user.Id))
				continue
			}
			if other, err := m.b.UserByUsername(username); err == nil && other != nil && other.Id != user.Id {
				rctx.Logger().Warn("Username already used by another Mattermost user, skipping", mlog.String("user_id", user.Id), mlog.String("other_user_id", other.Id))
				continue
			}
		}

		samlAuthData[email] = user.Id
		changes = append(changes, change{user: user, username: username, email: email})
	}

	for _, c := range changes {
		if dryRun {
			rctx.Logger().Info("Dry run: would migrate user to SAML", mlog.String("user_id", c.user.Id))
			continue
		}
		if err := m.b.UpdateAuthData(c.user.Id, model.UserAuthServiceSaml, c.email); err != nil {
			return model.NewAppError("MigrateToSaml", "app.user.update_auth_data.app_error", nil, "user_id="+c.user.Id, http.StatusInternalServerError).Wrap(err)
		}
		if c.username != c.user.Username {
			if err := m.b.UpdateUsername(rctx, c.user.Id, c.username); err != nil {
				return model.NewAppError("MigrateToSaml", "app.user.update.finding.app_error", nil, "user_id="+c.user.Id, http.StatusInternalServerError).Wrap(err)
			}
		}
		rctx.Logger().Info("Migrated user to SAML", mlog.String("user_id", c.user.Id))
	}
	return nil
}
