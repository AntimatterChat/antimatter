// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package ldap

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/mattermost/ldap"

	"github.com/mattermost/mattermost/server/public/model"
)

// ldapUser is a user entry read from the directory.
type ldapUser struct {
	DN    string
	ID    string // value of the ID attribute, stored as AuthData
	Entry *ldap.Entry
}

func newLdapUser(s *settings, e *ldap.Entry) *ldapUser {
	return &ldapUser{
		DN:    e.DN,
		ID:    attributeValue(e, s.IdAttribute),
		Entry: e,
	}
}

var invalidUsernameChars = regexp.MustCompile(`[^a-z0-9.\-_]`)

// usernameFromDirectory converts a directory value into a valid Mattermost
// username. It returns "" if the value cannot be converted.
func usernameFromDirectory(value string) string {
	s := strings.ToLower(strings.TrimSpace(value))
	s = strings.ReplaceAll(s, " ", "-")
	s = invalidUsernameChars.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if !model.IsValidUsername(s) {
		return ""
	}
	return s
}

// truncateRunes truncates a string to at most n runes.
func truncateRunes(s string, n int) string {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}

// profile holds the profile fields provided by the directory. A nil field
// means that the directory does not manage it.
type profile struct {
	Username  *string
	Email     *string
	FirstName *string
	LastName  *string
	Nickname  *string
	Position  *string
}

// profileFromEntry extracts the managed profile fields of an entry.
func profileFromEntry(s *settings, e *ldap.Entry) profile {
	var p profile
	if s.UsernameAttribute != "" {
		if u := usernameFromDirectory(attributeValue(e, s.UsernameAttribute)); u != "" {
			p.Username = &u
		}
	}
	if s.EmailAttribute != "" {
		if email := strings.ToLower(strings.TrimSpace(attributeValue(e, s.EmailAttribute))); email != "" && len(email) <= model.UserEmailMaxLength {
			p.Email = &email
		}
	}
	str := func(attr string, maxRunes int) *string {
		if attr == "" {
			return nil
		}
		v := truncateRunes(attributeValue(e, attr), maxRunes)
		return &v
	}
	p.FirstName = str(s.FirstNameAttribute, model.UserFirstNameMaxRunes)
	p.LastName = str(s.LastNameAttribute, model.UserLastNameMaxRunes)
	p.Nickname = str(s.NicknameAttribute, model.UserNicknameMaxRunes)
	p.Position = str(s.PositionAttribute, model.UserPositionMaxRunes)
	return p
}

// applyProfile copies the managed fields into the user. It returns true if
// something changed. When includeIdentity is false, the username and email
// are left untouched.
func applyProfile(u *model.User, p profile, includeIdentity bool) bool {
	changed := false
	set := func(dst *string, v *string) {
		if v != nil && *dst != *v {
			*dst = *v
			changed = true
		}
	}
	if includeIdentity {
		set(&u.Username, p.Username)
		set(&u.Email, p.Email)
	}
	set(&u.FirstName, p.FirstName)
	set(&u.LastName, p.LastName)
	set(&u.Nickname, p.Nickname)
	set(&u.Position, p.Position)
	return changed
}

// userFromEntry builds a (not persisted) Mattermost user from an entry.
func userFromEntry(s *settings, lu *ldapUser) *model.User {
	id := lu.ID
	u := &model.User{
		AuthService:   model.UserAuthServiceLdap,
		AuthData:      &id,
		EmailVerified: true,
	}
	applyProfile(u, profileFromEntry(s, lu.Entry), true)
	return u
}
