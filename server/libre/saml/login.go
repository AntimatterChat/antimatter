// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package saml

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	saml2 "github.com/mattermost/gosaml2"
	dsig "github.com/russellhaering/goxmldsig"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/app"
)

// samlProfile is the user information extracted from a validated assertion.
type samlProfile struct {
	// idValue is the value of the Id attribute, when configured.
	idValue string

	email    string
	username string

	firstName, lastName, nickname, position, locale                string
	hasFirstName, hasLastName, hasNickname, hasPosition, hasLocale bool

	isGuest bool
	// isAdmin is only meaningful when adminManaged is true.
	isAdmin      bool
	adminManaged bool
}

// DoLogin validates the SAMLResponse posted by the Identity Provider and
// returns the matching Mattermost user, creating or updating it as needed.
// relayState holds the (already verified) relay state properties set by the
// web layer when the login started.
func (s *Service) DoLogin(rctx request.CTX, encodedXML string, relayState map[string]string) (*model.User, *saml2.AssertionInfo, *model.AppError) {
	encodedXML = strings.TrimSpace(encodedXML)
	if encodedXML == "" {
		return nil, nil, model.NewAppError("DoLogin", "ent.saml.do_login.empty_response.app_error", nil, "", http.StatusBadRequest)
	}
	if _, err := base64.StdEncoding.DecodeString(encodedXML); err != nil {
		return nil, nil, model.NewAppError("DoLogin", "ent.saml.do_login.parse.app_error", nil, "SAMLResponse is not valid base64", http.StatusBadRequest).Wrap(err)
	}

	sp, appErr := s.serviceProvider(rctx, false)
	if appErr != nil {
		return nil, nil, appErr
	}

	info, err := sp.RetrieveAssertionInfo(encodedXML)
	if err != nil {
		return nil, nil, classifyAssertionError(err)
	}

	if info.WarningInfo != nil {
		if info.WarningInfo.InvalidTime {
			return nil, nil, model.NewAppError("DoLogin", "ent.saml.do_login.invalid_time.app_error", nil, "assertion is outside of its validity period", http.StatusBadRequest)
		}
		if info.WarningInfo.NotInAudience {
			return nil, nil, model.NewAppError("DoLogin", "ent.saml.do_login.parse.app_error", nil,
				fmt.Sprintf("assertion audience does not include the service provider identifier %q", sp.AudienceURI), http.StatusBadRequest)
		}
	}

	if len(info.Assertions) > 0 {
		assertion := info.Assertions[0]
		now := time.Now()
		expiry := now.Add(replayFallbackTTL)
		if assertion.Conditions != nil {
			if t, perr := time.Parse(time.RFC3339, assertion.Conditions.NotOnOrAfter); perr == nil {
				expiry = t
			}
		}
		if s.replay.markUsed(assertion.ID, expiry, now) {
			return nil, nil, model.NewAppError("DoLogin", "ent.saml.do_login.parse.app_error", nil, "assertion has already been used", http.StatusBadRequest)
		}
	}

	settings := s.b.Config().SamlSettings
	profile, appErr := extractProfile(rctx, &settings, info)
	if appErr != nil {
		return nil, nil, appErr
	}

	user, appErr := s.loginUser(rctx, &settings, profile, info, relayState)
	if appErr != nil {
		return nil, nil, appErr
	}

	return user, info, nil
}

// classifyAssertionError maps gosaml2/goxmldsig validation errors to the error
// ids that the web layer displays.
func classifyAssertionError(err error) *model.AppError {
	cause := err
	var verr saml2.ErrVerification
	if errors.As(err, &verr) && verr.Cause != nil {
		cause = verr.Cause
	}

	var invalidValue saml2.ErrInvalidValue
	if errors.As(cause, &invalidValue) && invalidValue.Reason == saml2.ReasonExpired {
		return model.NewAppError("DoLogin", "ent.saml.do_login.invalid_time.app_error", nil, "", http.StatusBadRequest).Wrap(err)
	}

	msg := strings.ToLower(cause.Error())
	switch {
	case strings.Contains(msg, "assertions are not encrypted"):
		return model.NewAppError("DoLogin", "ent.saml.configure.not_encrypted_response.app_error", nil, "", http.StatusBadRequest).Wrap(err)
	case errors.Is(cause, dsig.ErrMissingSignature),
		strings.Contains(msg, "signature"),
		strings.Contains(msg, "must be signed"),
		strings.Contains(msg, "cert"),
		strings.Contains(msg, "digest"):
		return model.NewAppError("DoLogin", "ent.saml.do_login.invalid_signature.app_error", nil, "", http.StatusBadRequest).Wrap(err)
	case strings.Contains(msg, "decrypt"):
		return model.NewAppError("DoLogin", "ent.saml.configure.load_private_key.app_error", nil, "", http.StatusBadRequest).Wrap(err)
	}
	return model.NewAppError("DoLogin", "ent.saml.do_login.parse.app_error", nil, "", http.StatusBadRequest).Wrap(err)
}

// getAttribute returns the first value of the named attribute. The name is
// matched against the attribute Name first, then against its FriendlyName.
func getAttribute(info *saml2.AssertionInfo, name string) (string, bool) {
	values, ok := getAttributeValues(info, name)
	if !ok {
		return "", false
	}
	if len(values) == 0 {
		return "", true
	}
	return strings.TrimSpace(values[0]), true
}

func getAttributeValues(info *saml2.AssertionInfo, name string) ([]string, bool) {
	name = strings.TrimSpace(name)
	if info == nil || name == "" {
		return nil, false
	}
	if attr, ok := info.Values[name]; ok {
		return info.Values.GetAll(attr.Name), true
	}
	for key, attr := range info.Values {
		if attr.FriendlyName != "" && attr.FriendlyName == name {
			return info.Values.GetAll(key), true
		}
	}
	return nil, false
}

// matchesAttributeFilter evaluates a "attribute=value" filter (GuestAttribute,
// AdminAttribute) against the assertion. The filter matches when any value of
// the attribute equals the expected value.
func matchesAttributeFilter(info *saml2.AssertionInfo, filter string) bool {
	name, expected, ok := strings.Cut(filter, "=")
	if !ok {
		return false
	}
	name = strings.TrimSpace(name)
	expected = strings.TrimSpace(expected)
	values, found := getAttributeValues(info, name)
	if !found {
		return false
	}
	return slices.ContainsFunc(values, func(v string) bool { return strings.TrimSpace(v) == expected })
}

func truncateRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}

func attributeError(detail string) *model.AppError {
	return model.NewAppError("DoLogin", "ent.saml.attribute.app_error", nil, detail, http.StatusBadRequest)
}

// extractProfile maps the assertion attributes to a samlProfile according to
// the SamlSettings attribute mappings.
func extractProfile(rctx request.CTX, ss *model.SamlSettings, info *saml2.AssertionInfo) (*samlProfile, *model.AppError) {
	p := &samlProfile{}

	emailAttr := model.SafeDereference(ss.EmailAttribute)
	email, ok := getAttribute(info, emailAttr)
	email = model.NormalizeEmail(email)
	if !ok || email == "" {
		return nil, attributeError(fmt.Sprintf("email attribute %q is missing from the SAML assertion", emailAttr))
	}
	if !model.IsValidEmail(email) || len(email) > model.UserEmailMaxLength {
		return nil, attributeError(fmt.Sprintf("email attribute %q does not contain a valid email address", emailAttr))
	}
	p.email = email

	usernameAttr := model.SafeDereference(ss.UsernameAttribute)
	username, ok := getAttribute(info, usernameAttr)
	if !ok || username == "" {
		return nil, attributeError(fmt.Sprintf("username attribute %q is missing from the SAML assertion", usernameAttr))
	}
	normalized := model.NormalizeUsername(username)
	if !model.IsValidUsername(normalized) {
		normalized = model.CleanUsername(rctx.Logger(), username)
	}
	p.username = normalized

	if idAttr := model.SafeDereference(ss.IdAttribute); idAttr != "" {
		id, ok := getAttribute(info, idAttr)
		if !ok || id == "" {
			return nil, attributeError(fmt.Sprintf("id attribute %q is missing from the SAML assertion", idAttr))
		}
		if len(id) > model.UserAuthDataMaxLength {
			return nil, attributeError(fmt.Sprintf("id attribute %q is longer than %d characters", idAttr, model.UserAuthDataMaxLength))
		}
		p.idValue = id
	}

	optional := func(attr *string, maxRunes int) (string, bool) {
		name := model.SafeDereference(attr)
		if name == "" {
			return "", false
		}
		v, ok := getAttribute(info, name)
		if !ok {
			return "", false
		}
		return truncateRunes(v, maxRunes), true
	}
	p.firstName, p.hasFirstName = optional(ss.FirstNameAttribute, model.UserFirstNameMaxRunes)
	p.lastName, p.hasLastName = optional(ss.LastNameAttribute, model.UserLastNameMaxRunes)
	p.nickname, p.hasNickname = optional(ss.NicknameAttribute, model.UserNicknameMaxRunes)
	p.position, p.hasPosition = optional(ss.PositionAttribute, model.UserPositionMaxRunes)

	if locale, ok := optional(ss.LocaleAttribute, 64); ok {
		locale = strings.ReplaceAll(locale, "_", "-")
		if locale != "" && model.IsValidLocale(locale) {
			p.locale, p.hasLocale = locale, true
		} else if locale != "" {
			rctx.Logger().Debug("Ignoring invalid locale from SAML assertion", mlog.String("locale", locale))
		}
	}

	if guestFilter := model.SafeDereference(ss.GuestAttribute); guestFilter != "" {
		p.isGuest = matchesAttributeFilter(info, guestFilter)
	}
	if model.SafeDereference(ss.EnableAdminAttribute) {
		if adminFilter := model.SafeDereference(ss.AdminAttribute); adminFilter != "" {
			p.adminManaged = true
			p.isAdmin = matchesAttributeFilter(info, adminFilter)
		}
	}

	return p, nil
}

// primaryAuthData is the value stored in User.AuthData for SAML users: the Id
// attribute when configured, the email address otherwise.
func (p *samlProfile) primaryAuthData() string {
	if p.idValue != "" {
		return p.idValue
	}
	return p.email
}

// loginUser finds (or creates) and updates the Mattermost user for the profile.
func (s *Service) loginUser(rctx request.CTX, ss *model.SamlSettings, p *samlProfile, info *saml2.AssertionInfo, relayState map[string]string) (*model.User, *model.AppError) {
	cfg := s.b.Config()
	guestAccountsEnabled := model.SafeDereference(cfg.GuestAccountsSettings.Enable)

	if p.isGuest && !guestAccountsEnabled {
		return nil, model.NewAppError("DoLogin", "api.user.login.guest_accounts.disabled.error", nil, "the SAML guest attribute matches but guest accounts are disabled", http.StatusUnauthorized)
	}

	// Candidate AuthData values, most preferred first. A user found through a
	// fallback candidate is migrated to the primary value.
	candidates := []string{p.primaryAuthData()}
	if p.idValue != "" {
		candidates = append(candidates, p.email)
	}

	ldapConfigured := model.SafeDereference(cfg.LdapSettings.Enable) || model.SafeDereference(cfg.LdapSettings.EnableSync)
	syncWithLdap := model.SafeDereference(ss.EnableSyncWithLdap) && ldapConfigured && s.b.Ldap() != nil
	if syncWithLdap && !(p.isGuest && model.SafeDereference(ss.IgnoreGuestsLdapSync)) {
		probe := &model.User{
			Email:       p.email,
			Username:    p.username,
			AuthService: model.UserAuthServiceSaml,
			AuthData:    model.NewPointer(p.primaryAuthData()),
		}
		ldapUser, _, ldapErr := s.b.Ldap().GetLDAPUserForMMUser(rctx, probe)
		if ldapErr != nil || ldapUser == nil {
			appErr := model.NewAppError("DoLogin", "ent.saml.login.ldap_user_missing", nil, "email="+p.email, http.StatusBadRequest)
			if ldapErr != nil {
				appErr = appErr.Wrap(ldapErr)
			}
			return nil, appErr
		}

		if model.SafeDereference(ss.EnableSyncWithLdapIncludeAuth) {
			override := ""
			if ldapUser.AuthData != nil && *ldapUser.AuthData != "" {
				override = *ldapUser.AuthData
			} else if ldapUser.Email != "" {
				override = model.NormalizeEmail(ldapUser.Email)
			}
			if override != "" && override != candidates[0] {
				candidates = append([]string{override}, candidates...)
			}
		}
	}
	primary := candidates[0]

	var user *model.User
	for i, candidate := range candidates {
		found, appErr := s.b.GetUserByAuth(candidate)
		if appErr != nil {
			if appErr.Id == app.MissingAuthAccountError {
				continue
			}
			return nil, appErr
		}
		user = found
		if i > 0 {
			rctx.Logger().Info("Migrating SAML user binding",
				mlog.String("user_id", user.Id), mlog.Int("from_candidate", i))
			if appErr := s.b.UpdateAuthData(user.Id, primary, "", false); appErr != nil {
				return nil, appErr
			}
			user.AuthData = model.NewPointer(primary)
		}
		break
	}

	if relayState[relayStateAction] == model.OAuthActionEmailToSSO {
		switched, appErr := s.switchEmailToSaml(rctx, user, primary, p, relayState[relayStateEmailToken])
		if appErr != nil {
			return nil, appErr
		}
		user = switched
	}

	created := false
	if user == nil {
		newUser, appErr := s.createUser(rctx, primary, p)
		if appErr != nil {
			return nil, appErr
		}
		user = newUser
		created = true
	} else if !syncWithLdap {
		updated, appErr := s.updateUserProfile(rctx, user, p)
		if appErr != nil {
			return nil, appErr
		}
		user = updated
	}

	if user.IsGuest() && !guestAccountsEnabled {
		return nil, model.NewAppError("DoLogin", "api.user.login.guest_accounts.disabled.error", nil, "", http.StatusUnauthorized)
	}

	user = s.applyRoles(rctx, user, p, created)

	if err := s.syncCustomProfileAttributes(rctx, user, info); err != nil {
		rctx.Logger().Warn("Failed to synchronize custom profile attributes from SAML",
			mlog.String("user_id", user.Id), mlog.Err(err))
	}

	// Return a fresh copy reflecting every change made above.
	if fresh, appErr := s.b.GetUserByAuth(primary); appErr == nil {
		user = fresh
	}
	return user, nil
}

const (
	relayStateAction     = "action"
	relayStateEmailToken = "email_token"
)

// switchEmailToSaml completes the email to SAML sign-in method switch started
// by App.SwitchEmailToOAuth, binding the SAML identity to the account whose
// email is stored in the SAML token.
func (s *Service) switchEmailToSaml(rctx request.CTX, existing *model.User, authData string, p *samlProfile, tokenValue string) (*model.User, *model.AppError) {
	if tokenValue == "" {
		return nil, model.NewAppError("DoLogin", "api.saml.invalid_email_token.app_error", nil, "missing email token", http.StatusBadRequest)
	}
	token, appErr := s.b.GetSamlEmailToken(tokenValue)
	if appErr != nil {
		return nil, appErr
	}
	if token.IsExpired() {
		if err := s.b.DeleteToken(token.Token); err != nil {
			rctx.Logger().Warn("Failed to delete expired SAML email token", mlog.Err(err))
		}
		return nil, model.NewAppError("DoLogin", "api.saml.invalid_email_token.app_error", nil, "expired email token", http.StatusBadRequest)
	}

	user, appErr := s.b.GetUserByEmail(token.Extra)
	if appErr != nil {
		return nil, appErr
	}

	if existing != nil {
		if existing.Id != user.Id {
			return nil, model.NewAppError("DoLogin", "api.user.create_oauth_user.already_attached.app_error",
				map[string]any{"Service": "SAML", "Auth": "SAML"}, "the SAML identity is already bound to another account", http.StatusBadRequest)
		}
	} else {
		if user.AuthService != "" && user.AuthService != model.UserAuthServiceEmail {
			return nil, model.NewAppError("DoLogin", "ent.saml.save_user.email_exists.saml_app_error", nil, "user_id="+user.Id, http.StatusBadRequest)
		}
		if appErr := s.b.UpdateAuthData(user.Id, authData, p.email, true); appErr != nil {
			return nil, appErr
		}
		switched, appErr := s.b.GetUserByAuth(authData)
		if appErr != nil {
			return nil, appErr
		}
		user = switched
	}

	if err := s.b.DeleteToken(token.Token); err != nil {
		rctx.Logger().Warn("Failed to delete used SAML email token", mlog.Err(err))
	}
	return user, nil
}

func (s *Service) createUser(rctx request.CTX, authData string, p *samlProfile) (*model.User, *model.AppError) {
	cfg := s.b.Config()
	if !model.SafeDereference(cfg.TeamSettings.EnableUserCreation) {
		return nil, model.NewAppError("DoLogin", "api.user.create_user.disabled.app_error", nil, "", http.StatusNotImplemented)
	}

	if existing, _ := s.b.GetUserByEmail(p.email); existing != nil {
		return nil, model.NewAppError("DoLogin", "ent.saml.save_user.email_exists.saml_app_error", nil,
			"user_id="+existing.Id+" auth_service="+existing.AuthService, http.StatusBadRequest)
	}
	if existing, _ := s.b.GetUserByUsername(p.username); existing != nil {
		return nil, model.NewAppError("DoLogin", "ent.saml.save_user.username_exists.saml_app_error", nil,
			"username="+p.username, http.StatusBadRequest)
	}

	user := &model.User{
		Email:         p.email,
		Username:      p.username,
		FirstName:     p.firstName,
		LastName:      p.lastName,
		Nickname:      p.nickname,
		Position:      p.position,
		Locale:        p.locale,
		AuthService:   model.UserAuthServiceSaml,
		AuthData:      model.NewPointer(authData),
		EmailVerified: true,
	}

	created, appErr := s.b.CreateUser(rctx, user, p.isGuest)
	if appErr != nil {
		return nil, appErr
	}

	rctx.Logger().Info("Created user from SAML login", mlog.String("user_id", created.Id), mlog.Bool("guest", p.isGuest))
	return created, nil
}

// updateUserProfile refreshes the profile fields managed by the IdP.
func (s *Service) updateUserProfile(rctx request.CTX, user *model.User, p *samlProfile) (*model.User, *model.AppError) {
	changed := false

	if p.email != user.Email {
		if other, _ := s.b.GetUserByEmail(p.email); other == nil || other.Id == user.Id {
			user.Email = p.email
			changed = true
		} else {
			rctx.Logger().Warn("Not updating SAML user email: address already used by another account",
				mlog.String("user_id", user.Id), mlog.String("other_user_id", other.Id))
		}
	}

	if p.username != user.Username {
		if other, _ := s.b.GetUserByUsername(p.username); other == nil || other.Id == user.Id {
			user.Username = p.username
			changed = true
		} else {
			rctx.Logger().Warn("Not updating SAML user username: already used by another account",
				mlog.String("user_id", user.Id), mlog.String("username", p.username))
		}
	}

	set := func(present bool, value string, field *string) {
		if present && *field != value {
			*field = value
			changed = true
		}
	}
	set(p.hasFirstName, p.firstName, &user.FirstName)
	set(p.hasLastName, p.lastName, &user.LastName)
	set(p.hasNickname, p.nickname, &user.Nickname)
	set(p.hasPosition, p.position, &user.Position)
	set(p.hasLocale, p.locale, &user.Locale)

	if !changed {
		return user, nil
	}
	return s.b.SaveUser(rctx, user)
}

// applyRoles applies the guest and admin attributes. Failures are logged and
// do not prevent the login, except that guests are never promoted.
func (s *Service) applyRoles(rctx request.CTX, user *model.User, p *samlProfile, created bool) *model.User {
	if p.isGuest && !user.IsGuest() {
		// Guest attribute matches an existing member: demote. A guest whose
		// attribute no longer matches keeps the guest role (manual promotion).
		if appErr := s.b.DemoteUserToGuest(rctx, user); appErr != nil {
			rctx.Logger().Warn("Failed to demote SAML user to guest", mlog.String("user_id", user.Id), mlog.Err(appErr))
		} else if fresh, appErr := s.b.GetUserByAuth(model.SafeDereference(user.AuthData)); appErr == nil {
			user = fresh
		}
	}

	if !p.adminManaged || user.IsGuest() {
		return user
	}

	roles := strings.Fields(user.Roles)
	hasAdmin := slices.Contains(roles, model.SystemAdminRoleId)
	switch {
	case p.isAdmin && !hasAdmin:
		roles = append(roles, model.SystemAdminRoleId)
		if !slices.Contains(roles, model.SystemUserRoleId) {
			roles = append([]string{model.SystemUserRoleId}, roles...)
		}
	case !p.isAdmin && hasAdmin:
		roles = slices.DeleteFunc(roles, func(r string) bool { return r == model.SystemAdminRoleId })
		if !slices.Contains(roles, model.SystemUserRoleId) {
			roles = append([]string{model.SystemUserRoleId}, roles...)
		}
	default:
		return user
	}

	updated, appErr := s.b.UpdateUserRoles(rctx, user, strings.Join(roles, " "))
	if appErr != nil {
		rctx.Logger().Warn("Failed to update SAML user roles from the admin attribute",
			mlog.String("user_id", user.Id), mlog.Bool("admin", p.isAdmin), mlog.Bool("new_user", created), mlog.Err(appErr))
		return user
	}
	return updated
}
