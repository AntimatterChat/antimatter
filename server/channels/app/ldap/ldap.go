// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

// Package ldap implements AD/LDAP authentication, synchronization, group
// synchronization and diagnostics for Mattermost.
package ldap

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/mattermost/ldap"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/einterfaces"
)

// userPropAdminFilter marks the users that were promoted to system admin by
// the AD/LDAP admin filter, so that they can be demoted when they stop
// matching it without touching manually promoted admins.
const userPropAdminFilter = "ldap_admin_filter"

// Ldap implements einterfaces.LdapInterface.
type Ldap struct {
	b    backend
	dial Dialer

	// pictures remembers the hash of the last profile picture copied from the
	// directory for each user, to avoid re-processing it at each login.
	pictures sync.Map

	// syncMutex prevents concurrent synchronizations on this node.
	syncMutex sync.Mutex
}

var _ einterfaces.LdapInterface = (*Ldap)(nil)

func newLdap(b backend, dial Dialer) *Ldap {
	if dial == nil {
		dial = dialLDAP
	}
	return &Ldap{b: b, dial: dial}
}

func (l *Ldap) settings() *settings {
	return newSettings(&l.b.Config().LdapSettings)
}

func (l *Ldap) open(s *settings) (*session, *model.AppError) {
	return openSession(l.dial, s, l.b.GetConfigFile)
}

// checkPassword verifies the credentials of a user by binding with them on a
// dedicated connection.
func (l *Ldap) checkPassword(s *settings, dn, password string) *model.AppError {
	if password == "" || dn == "" {
		return model.NewAppError("Ldap.checkPassword", "ent.ldap.do_login.invalid_password.app_error", nil, "", http.StatusUnauthorized)
	}
	conn, appErr := l.dial(s, l.b.GetConfigFile)
	if appErr != nil {
		return appErr
	}
	defer conn.Close()

	if err := conn.Bind(dn, password); err != nil {
		var ldapErr *ldap.Error
		if errors.As(err, &ldapErr) && ldapErr.ResultCode == ldap.ErrorNetwork {
			return model.NewAppError("Ldap.checkPassword", "ent.ldap.do_login.unable_to_connect.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
		}
		return model.NewAppError("Ldap.checkPassword", "ent.ldap.do_login.invalid_password.app_error", nil, "", http.StatusUnauthorized).Wrap(err)
	}
	return nil
}

// findUser searches for a single user entry, matching attr=value, restricted
// by the user filter.
func (l *Ldap) findUser(ss *session, attr, value string, extraAttributes ...string) (*ldapUser, *model.AppError) {
	s := ss.s
	if attr == "" || strings.TrimSpace(value) == "" {
		return nil, model.NewAppError("Ldap.findUser", "ent.ldap.do_login.invalid_id", nil, "", http.StatusBadRequest)
	}

	attrs := s.userAttributes(extraAttributes...)
	entries, err := ss.searchLimit(andFilters(s.userFilter(), equalityFilter(attr, value)), attrs, 2)
	if errors.Is(err, errSizeLimitExceeded) {
		return nil, model.NewAppError("Ldap.findUser", "ent.ldap.do_login.matched_to_many_users.app_error", nil, "", http.StatusBadRequest)
	}
	if err != nil {
		return nil, searchError("Ldap.findUser", err)
	}

	switch len(entries) {
	case 1:
		return newLdapUser(s, entries[0]), nil
	case 0:
		// Distinguish users rejected by the user filter from unknown users.
		if s.UserFilter != "" {
			others, err := ss.searchLimit(equalityFilter(attr, value), []string{"1.1"}, 1)
			if errors.Is(err, errSizeLimitExceeded) || (err == nil && len(others) > 0) {
				return nil, model.NewAppError("Ldap.findUser", "ent.ldap.do_login.user_filtered.app_error", nil, "", http.StatusUnauthorized)
			}
		}
		return nil, model.NewAppError("Ldap.findUser", "ent.ldap.do_login.user_not_registered.app_error", nil, "", http.StatusNotFound)
	default:
		return nil, model.NewAppError("Ldap.findUser", "ent.ldap.do_login.matched_to_many_users.app_error", nil, "", http.StatusBadRequest)
	}
}

// findUserForMMUser finds the directory entry of a Mattermost user, which may
// be an AD/LDAP user or a SAML user synchronized with AD/LDAP.
func (l *Ldap) findUserForMMUser(ss *session, user *model.User, extraAttributes ...string) (*ldapUser, *model.AppError) {
	s := ss.s
	cfg := l.b.Config()

	switch {
	case user.IsLDAPUser():
		if user.AuthData == nil || *user.AuthData == "" {
			return nil, model.NewAppError("Ldap.findUserForMMUser", "ent.ldap.do_login.invalid_id", nil, "", http.StatusBadRequest)
		}
		return l.findUser(ss, s.IdAttribute, *user.AuthData, extraAttributes...)
	case user.IsSAMLUser():
		if samlBindsWithID(cfg) && user.AuthData != nil && *user.AuthData != "" && *user.AuthData != user.Email {
			if lu, appErr := l.findUser(ss, s.IdAttribute, *user.AuthData, extraAttributes...); appErr == nil {
				return lu, nil
			}
		}
		return l.findUser(ss, s.EmailAttribute, user.Email, extraAttributes...)
	default:
		return l.findUser(ss, s.EmailAttribute, user.Email, extraAttributes...)
	}
}

// samlBindsWithID reports whether SAML users are bound to their AD/LDAP ID
// attribute (as opposed to their email address).
func samlBindsWithID(cfg *model.Config) bool {
	return model.SafeDereference(cfg.SamlSettings.EnableSyncWithLdapIncludeAuth) &&
		model.SafeDereference(cfg.SamlSettings.IdAttribute) != ""
}

// matchesFilter reports whether the entry with the given DN matches filter.
func matchesFilter(ss *session, dn, filter string) (bool, error) {
	entry, err := ss.readEntry(dn, ensureParens(filter), []string{"1.1"})
	if err != nil {
		return false, err
	}
	return entry != nil, nil
}

// DoLogin authenticates the user whose ID attribute value is id. Following
// the expectations of the application layer, the Mattermost account is
// created before the password is verified so that failed attempts of first
// time users can be counted.
func (l *Ldap) DoLogin(rctx request.CTX, id string, password string) (*model.User, *model.AppError) {
	s := l.settings()
	ss, appErr := l.open(s)
	if appErr != nil {
		return nil, appErr
	}
	defer ss.Close()

	cpa, err := l.loadCPAMapping(rctx)
	if err != nil {
		rctx.Logger().Debug("Unable to load user attribute fields synchronized with AD/LDAP", mlog.Err(err))
		cpa = nil
	}

	lu, appErr := l.findUser(ss, s.IdAttribute, id, cpa.attributes()...)
	if appErr != nil {
		return nil, appErr
	}

	user, err := l.b.GetUserByAuth(id, model.UserAuthServiceLdap)
	if err != nil && lu.ID != id {
		user, err = l.b.GetUserByAuth(lu.ID, model.UserAuthServiceLdap)
	}
	isNew := false
	if err != nil {
		if !isNotFound(err) {
			return nil, model.NewAppError("Ldap.DoLogin", "ent.ldap.get_user_by_auth.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
		}
		user, appErr = l.createUser(rctx, ss, lu)
		if appErr != nil {
			return nil, appErr
		}
		isNew = true
	}

	if appErr = l.checkPassword(s, lu.DN, password); appErr != nil {
		return nil, appErr
	}

	firstLogin := isNew || user.LastLogin == 0

	if !isNew {
		if refreshed, appErr := l.refreshUser(rctx, s, user, lu); appErr != nil {
			rctx.Logger().Warn("Failed to update user profile from AD/LDAP", mlog.String("user_id", user.Id), mlog.Err(appErr))
		} else {
			user = refreshed
		}
	}

	user = l.applyLoginRoles(rctx, ss, user, lu)

	if err := l.syncCPAWithMapping(rctx, cpa, user.Id, lu.Entry); err != nil {
		rctx.Logger().Warn("Failed to synchronize user attributes from AD/LDAP", mlog.String("user_id", user.Id), mlog.Err(err))
	}

	if firstLogin {
		if appErr := l.firstLoginSync(rctx, ss, user, lu); appErr != nil {
			rctx.Logger().Warn("Failed to synchronize group memberships of new AD/LDAP user", mlog.String("user_id", user.Id), mlog.Err(appErr))
		}
	}

	return user, nil
}

// createUser creates the Mattermost account of a directory user.
func (l *Ldap) createUser(rctx request.CTX, ss *session, lu *ldapUser) (*model.User, *model.AppError) {
	s := ss.s
	if lu.ID == "" {
		return nil, model.NewAppError("Ldap.createUser", "ent.ldap.do_login.invalid_id", nil, "", http.StatusBadRequest)
	}
	user := userFromEntry(s, lu)
	if user.Email == "" {
		return nil, model.NewAppError("Ldap.createUser", "ent.ldap.create_fail", nil, "missing email attribute", http.StatusBadRequest)
	}
	if user.Username == "" {
		user.Username = model.CleanUsername(rctx.Logger(), attributeValue(lu.Entry, s.UsernameAttribute))
	}

	guest := false
	if guestFilterEnabled(l.b.Config(), s) {
		matchesAdmin := false
		if adminFilterEnabled(s) {
			matchesAdmin, _ = matchesFilter(ss, lu.DN, s.AdminFilter)
		}
		if !matchesAdmin {
			isGuest, err := matchesFilter(ss, lu.DN, s.GuestFilter)
			if err != nil {
				rctx.Logger().Warn("Failed to evaluate the AD/LDAP guest filter", mlog.Err(err))
			}
			guest = isGuest
		}
	}

	created, appErr := l.b.CreateUser(rctx, user, guest)
	if appErr != nil {
		switch appErr.Id {
		case "app.user.save.email_exists.app_error":
			return nil, model.NewAppError("Ldap.createUser", "ent.ldap.save_user.email_exists.ldap_app_error", nil, "", http.StatusBadRequest).Wrap(appErr)
		case "app.user.save.username_exists.app_error":
			return nil, model.NewAppError("Ldap.createUser", "ent.ldap.save_user.username_exists.ldap_app_error", nil, "", http.StatusBadRequest).Wrap(appErr)
		}
		return nil, model.NewAppError("Ldap.createUser", "ent.ldap.create_fail", nil, "", appErr.StatusCode).Wrap(appErr)
	}
	rctx.Logger().LogM(mlog.MlvlLDAPInfo, "Created Mattermost account for AD/LDAP user", mlog.String("user_id", created.Id))
	return created, nil
}

// refreshUser copies the profile managed by the directory into the user.
func (l *Ldap) refreshUser(rctx request.CTX, s *settings, user *model.User, lu *ldapUser) (*model.User, *model.AppError) {
	updated := user.DeepCopy()
	if !applyProfile(updated, profileFromEntry(s, lu.Entry), true) {
		return user, nil
	}
	saved, err := l.b.SaveUser(rctx, updated)
	if err != nil {
		return nil, model.NewAppError("Ldap.refreshUser", "app.user.update.finding.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	return saved, nil
}

// applyLoginRoles applies the admin and guest filters. Failures are logged
// and do not prevent the login.
func (l *Ldap) applyLoginRoles(rctx request.CTX, ss *session, user *model.User, lu *ldapUser) *model.User {
	s := ss.s
	isAdmin := false

	if adminFilterEnabled(s) {
		matches, err := matchesFilter(ss, lu.DN, s.AdminFilter)
		if err != nil {
			rctx.Logger().Warn("Failed to evaluate the AD/LDAP admin filter", mlog.Err(err))
			return user
		}
		isAdmin = matches
		switch {
		case matches && !user.IsSystemAdmin():
			user = l.setSystemAdmin(rctx, user, true)
		case !matches && user.IsSystemAdmin() && user.Props[userPropAdminFilter] == "true":
			user = l.setSystemAdmin(rctx, user, false)
		}
	}

	if !isAdmin && guestFilterEnabled(l.b.Config(), s) && !user.IsGuest() && !user.IsSystemAdmin() {
		matches, err := matchesFilter(ss, lu.DN, s.GuestFilter)
		if err != nil {
			rctx.Logger().Warn("Failed to evaluate the AD/LDAP guest filter", mlog.Err(err))
			return user
		}
		if matches {
			if err := l.b.DemoteUserToGuest(rctx, user); err != nil {
				rctx.Logger().Warn("Failed to demote AD/LDAP user to guest", mlog.String("user_id", user.Id), mlog.Err(err))
			} else if refreshed, err := l.b.GetUser(rctx, user.Id); err == nil {
				user = refreshed
			}
		}
	}

	return user
}

// setSystemAdmin grants or removes the system admin role, remembering that
// the change was made by the admin filter.
func (l *Ldap) setSystemAdmin(rctx request.CTX, user *model.User, admin bool) *model.User {
	logger := rctx.Logger().With(mlog.String("user_id", user.Id))

	if admin && user.IsGuest() {
		if err := l.b.PromoteGuestToUser(rctx, user); err != nil {
			logger.Warn("Failed to promote guest matching the AD/LDAP admin filter", mlog.Err(err))
			return user
		}
		if refreshed, err := l.b.GetUser(rctx, user.Id); err == nil {
			user = refreshed
		}
	}

	roles := slices.DeleteFunc(user.GetRoles(), func(r string) bool {
		return r == model.SystemAdminRoleId || r == model.SystemUserRoleId || r == ""
	})
	roles = append([]string{model.SystemUserRoleId}, roles...)
	if admin {
		roles = append(roles, model.SystemAdminRoleId)
	}
	if err := l.b.UpdateUserRoles(rctx, user.Id, strings.Join(roles, " ")); err != nil {
		logger.Warn("Failed to update roles from the AD/LDAP admin filter", mlog.Err(err))
		return user
	}

	refreshed, err := l.b.GetUser(rctx, user.Id)
	if err != nil {
		logger.Warn("Failed to reload user after role update", mlog.Err(err))
		return user
	}
	if refreshed.Props == nil {
		refreshed.Props = model.StringMap{}
	}
	if admin {
		refreshed.Props[userPropAdminFilter] = "true"
	} else {
		delete(refreshed.Props, userPropAdminFilter)
	}
	saved, err := l.b.SaveUser(rctx, refreshed)
	if err != nil {
		logger.Warn("Failed to save user after role update", mlog.Err(err))
		return refreshed
	}
	logger.LogM(mlog.MlvlLDAPInfo, "Updated system admin role from the AD/LDAP admin filter", mlog.Bool("system_admin", admin))
	return saved
}

// GetUser returns the directory user whose login ID attribute is loginID. The
// returned user is not persisted.
func (l *Ldap) GetUser(rctx request.CTX, loginID string) (*model.User, *model.AppError) {
	s := l.settings()
	ss, appErr := l.open(s)
	if appErr != nil {
		return nil, appErr
	}
	defer ss.Close()

	lu, appErr := l.findUser(ss, s.LoginIdAttribute, loginID)
	if appErr != nil {
		return nil, appErr
	}
	if lu.ID == "" {
		return nil, model.NewAppError("Ldap.GetUser", "ent.ldap.do_login.invalid_id", nil, "", http.StatusBadRequest)
	}
	return userFromEntry(s, lu), nil
}

// GetLDAPUserForMMUser returns the directory user matching a Mattermost user
// (an AD/LDAP user, or a SAML user synchronized with AD/LDAP), along with the
// distinguished name of its entry.
func (l *Ldap) GetLDAPUserForMMUser(rctx request.CTX, mmUser *model.User) (*model.User, string, *model.AppError) {
	if mmUser == nil {
		return nil, "", model.NewAppError("Ldap.GetLDAPUserForMMUser", "ent.ldap.do_login.invalid_id", nil, "", http.StatusBadRequest)
	}
	s := l.settings()
	ss, appErr := l.open(s)
	if appErr != nil {
		return nil, "", appErr
	}
	defer ss.Close()

	lu, appErr := l.findUserForMMUser(ss, mmUser)
	if appErr != nil {
		return nil, "", appErr
	}
	return userFromEntry(s, lu), lu.DN, nil
}

// GetUserAttributes returns the requested attributes of the directory user
// whose ID attribute value is id.
func (l *Ldap) GetUserAttributes(rctx request.CTX, id string, attributes []string) (map[string]string, *model.AppError) {
	s := l.settings()
	ss, appErr := l.open(s)
	if appErr != nil {
		return nil, appErr
	}
	defer ss.Close()

	lu, appErr := l.findUser(ss, s.IdAttribute, id, attributes...)
	if appErr != nil {
		return nil, appErr
	}

	res := make(map[string]string, len(attributes))
	for _, attr := range attributes {
		res[attr] = attributeValue(lu.Entry, attr)
	}
	return res, nil
}

// CheckProviderAttributes returns the name of the first field of the patch
// that would override a value managed by the directory, or "".
func (l *Ldap) CheckProviderAttributes(rctx request.CTX, ls *model.LdapSettings, ouser *model.User, patch *model.UserPatch) string {
	if ls == nil || ouser == nil || patch == nil {
		return ""
	}
	s := newSettings(ls)
	changing := func(attr string, current string, value *string) bool {
		return attr != "" && value != nil && *value != current
	}

	switch {
	case changing(s.UsernameAttribute, ouser.Username, patch.Username):
		return "username"
	case changing(s.EmailAttribute, ouser.Email, patch.Email):
		return "email"
	case changing(s.FirstNameAttribute, ouser.FirstName, patch.FirstName):
		return "first name"
	case changing(s.LastNameAttribute, ouser.LastName, patch.LastName):
		return "last name"
	case changing(s.NicknameAttribute, ouser.Nickname, patch.Nickname):
		return "nickname"
	case changing(s.PositionAttribute, ouser.Position, patch.Position):
		return "position"
	}
	return ""
}

// SwitchToLdap converts an email/password account into an AD/LDAP account
// after verifying the AD/LDAP credentials.
func (l *Ldap) SwitchToLdap(rctx request.CTX, userID, ldapID, ldapPassword string) *model.AppError {
	user, err := l.b.GetUser(rctx, userID)
	if err != nil {
		return model.NewAppError("Ldap.SwitchToLdap", "ent.ldap.get_user_by_auth.app_error", nil, "", http.StatusBadRequest).Wrap(err)
	}

	s := l.settings()
	ss, appErr := l.open(s)
	if appErr != nil {
		return appErr
	}
	defer ss.Close()

	lu, appErr := l.findUser(ss, s.LoginIdAttribute, ldapID)
	if appErr != nil {
		return appErr
	}
	if lu.ID == "" {
		return model.NewAppError("Ldap.SwitchToLdap", "ent.ldap.do_login.invalid_id", nil, "", http.StatusBadRequest)
	}

	if existing, err := l.b.GetUserByAuth(lu.ID, model.UserAuthServiceLdap); err == nil && existing.Id != user.Id {
		return model.NewAppError("Ldap.SwitchToLdap", "ent.ldap.switch_to_ldap.already_attached.app_error", nil, "", http.StatusBadRequest)
	} else if err != nil && !isNotFound(err) {
		return model.NewAppError("Ldap.SwitchToLdap", "ent.ldap.get_user_by_auth.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	if appErr := l.checkPassword(s, lu.DN, ldapPassword); appErr != nil {
		return appErr
	}

	if err := l.b.UpdateAuthData(user.Id, model.UserAuthServiceLdap, lu.ID); err != nil {
		return model.NewAppError("Ldap.SwitchToLdap", "app.user.update_auth_data.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	return nil
}

// GetAllLdapUsers returns all the users selected by the user filter. The
// returned users are not persisted.
func (l *Ldap) GetAllLdapUsers(rctx request.CTX) ([]*model.User, *model.AppError) {
	s := l.settings()
	ss, appErr := l.open(s)
	if appErr != nil {
		return nil, appErr
	}
	defer ss.Close()

	entries, err := ss.search(s.userFilter(), s.userAttributes())
	if err != nil {
		return nil, searchError("Ldap.GetAllLdapUsers", err)
	}
	users := make([]*model.User, 0, len(entries))
	for _, e := range entries {
		lu := newLdapUser(s, e)
		if lu.ID == "" {
			continue
		}
		users = append(users, userFromEntry(s, lu))
	}
	return users, nil
}

// MigrateIDAttribute rewrites the AuthData of all the AD/LDAP users with the
// value of another attribute, so that the ID attribute can be changed
// afterwards without losing accounts.
func (l *Ldap) MigrateIDAttribute(rctx request.CTX, toAttribute string) error {
	toAttribute = strings.TrimSpace(toAttribute)
	if toAttribute == "" {
		return model.NewAppError("Ldap.MigrateIDAttribute", "ent.ldap_id_migrate.app_error", nil, "empty attribute", http.StatusBadRequest)
	}

	s := l.settings()
	if strings.EqualFold(toAttribute, s.IdAttribute) {
		return nil
	}

	ss, appErr := l.open(s)
	if appErr != nil {
		return appErr
	}
	defer ss.Close()

	entries, err := ss.search("("+s.IdAttribute+"=*)", []string{s.IdAttribute, toAttribute})
	if err != nil {
		return searchError("Ldap.MigrateIDAttribute", err)
	}

	newIDs := make(map[string]string, len(entries))
	seen := make(map[string]string, len(entries))
	for _, e := range entries {
		oldID := attributeValue(e, s.IdAttribute)
		newID := attributeValue(e, toAttribute)
		if oldID == "" || newID == "" {
			continue
		}
		if other, ok := seen[newID]; ok && other != oldID {
			return model.NewAppError("Ldap.MigrateIDAttribute", "ent.ldap_id_migrate.app_error", nil, "duplicate value for attribute "+toAttribute, http.StatusBadRequest)
		}
		seen[newID] = oldID
		newIDs[oldID] = newID
	}

	users, err := l.b.GetUsersByAuthService(model.UserAuthServiceLdap)
	if err != nil {
		return model.NewAppError("Ldap.MigrateIDAttribute", "ent.ldap.syncronize.get_all.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	for _, user := range users {
		if user.AuthData == nil {
			continue
		}
		newID, ok := newIDs[*user.AuthData]
		if !ok {
			rctx.Logger().Warn("AD/LDAP user not found while migrating the ID attribute", mlog.String("user_id", user.Id))
			continue
		}
		if newID == *user.AuthData {
			continue
		}
		if err := l.b.UpdateAuthData(user.Id, model.UserAuthServiceLdap, newID); err != nil {
			return model.NewAppError("Ldap.MigrateIDAttribute", "ent.ldap_id_migrate.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
		}
	}
	return nil
}

// FirstLoginSync synchronizes the group memberships of a user and adds them
// to the teams and channels of their groups.
func (l *Ldap) FirstLoginSync(rctx request.CTX, user *model.User) *model.AppError {
	if user == nil {
		return model.NewAppError("Ldap.FirstLoginSync", "ent.ldap.do_login.invalid_id", nil, "", http.StatusBadRequest)
	}
	s := l.settings()
	ss, appErr := l.open(s)
	if appErr != nil {
		return appErr
	}
	defer ss.Close()

	lu, appErr := l.findUserForMMUser(ss, user)
	if appErr != nil {
		return appErr
	}
	return l.firstLoginSync(rctx, ss, user, lu)
}

func (l *Ldap) firstLoginSync(rctx request.CTX, ss *session, user *model.User, lu *ldapUser) *model.AppError {
	if ss.s.GroupIdAttribute == "" {
		return nil
	}
	if appErr := l.syncUserGroups(rctx, ss, user.Id, lu.DN); appErr != nil {
		return appErr
	}
	params := model.CreateDefaultMembershipParams{
		Since:               0,
		ReAddRemovedMembers: ss.s.ReAddRemovedMembers,
		ScopedUserID:        &user.Id,
	}
	if err := l.b.CreateDefaultMemberships(rctx, params); err != nil {
		return model.NewAppError("Ldap.FirstLoginSync", "ent.ldap.syncronize.populate_syncables", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	return nil
}

// UpdateProfilePictureIfNecessary copies the profile picture stored in the
// directory into Mattermost, or resets the picture when it was removed from
// the directory.
func (l *Ldap) UpdateProfilePictureIfNecessary(rctx request.CTX, user model.User, _ model.Session) {
	s := l.settings()
	if s.PictureAttribute == "" {
		return
	}
	cfg := l.b.Config()
	if !user.IsLDAPUser() && !(user.IsSAMLUser() && model.SafeDereference(cfg.SamlSettings.EnableSyncWithLdap)) {
		return
	}

	logger := rctx.Logger().With(mlog.String("user_id", user.Id))

	ss, appErr := l.open(s)
	if appErr != nil {
		logger.Warn("Unable to connect to AD/LDAP to update the profile picture", mlog.Err(appErr))
		return
	}
	defer ss.Close()

	lu, appErr := l.findUserForMMUser(ss, &user, s.PictureAttribute)
	if appErr != nil {
		logger.Debug("Unable to find AD/LDAP user to update the profile picture", mlog.Err(appErr))
		return
	}

	raw := rawAttributeValue(lu.Entry, s.PictureAttribute)
	if len(raw) == 0 {
		l.pictures.Delete(user.Id)
		if user.LastPictureUpdate > 0 {
			if err := l.b.SetDefaultProfileImage(rctx, &user); err != nil {
				logger.Warn("Failed to reset the profile picture removed from AD/LDAP", mlog.Err(err))
			}
		}
		return
	}

	sum := sha256.Sum256(raw)
	hash := hex.EncodeToString(sum[:])
	if previous, ok := l.pictures.Load(user.Id); ok && previous == hash && user.LastPictureUpdate > 0 {
		return
	}
	if err := l.b.SetProfileImage(rctx, user.Id, raw); err != nil {
		logger.Warn("Failed to update the profile picture from AD/LDAP", mlog.Err(err))
		return
	}
	l.pictures.Store(user.Id, hash)
}

// StartSynchronizeJob schedules a synchronization job, optionally waiting for
// it to finish.
func (l *Ldap) StartSynchronizeJob(rctx request.CTX, waitForJobToFinish bool) (*model.Job, *model.AppError) {
	job, appErr := l.b.CreateSyncJob(rctx, nil)
	if appErr != nil {
		return nil, appErr
	}
	if !waitForJobToFinish {
		return job, nil
	}

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-rctx.Context().Done():
			return job, nil
		case <-ticker.C:
		}
		current, appErr := l.b.GetJob(rctx, job.Id)
		if appErr != nil {
			return nil, appErr
		}
		switch current.Status {
		case model.JobStatusSuccess, model.JobStatusError, model.JobStatusCanceled, model.JobStatusWarning:
			return current, nil
		}
	}
}
