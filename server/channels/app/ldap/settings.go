// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package ldap

import (
	"strings"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
)

// defaultGroupFilter is the filter used to find group objects when
// LdapSettings.GroupFilter is left blank.
const defaultGroupFilter = "(|(objectClass=group)(objectClass=groupOfNames)(objectClass=groupOfUniqueNames))"

// groupMemberAttributes are the attributes holding the DNs of the members of
// a group object.
var groupMemberAttributes = []string{"member", "uniqueMember"}

// settings is a fully dereferenced copy of model.LdapSettings, so that the
// rest of the package never has to deal with nil pointers (settings submitted
// through the "test connection" endpoints may be partially filled).
type settings struct {
	Enable     bool
	EnableSync bool

	Server             string
	Port               int
	ConnectionSecurity string
	BaseDN             string
	BindUsername       string
	BindPassword       string

	UserFilter        string
	GroupFilter       string
	GuestFilter       string
	EnableAdminFilter bool
	AdminFilter       string

	GroupDisplayNameAttribute string
	GroupIdAttribute          string

	FirstNameAttribute string
	LastNameAttribute  string
	EmailAttribute     string
	UsernameAttribute  string
	NicknameAttribute  string
	IdAttribute        string
	PositionAttribute  string
	LoginIdAttribute   string
	PictureAttribute   string

	SyncIntervalMinutes int
	ReAddRemovedMembers bool

	SkipCertificateVerification bool
	PublicCertificateFile       string
	PrivateKeyFile              string
	QueryTimeout                time.Duration
	MaxPageSize                 int
}

func newSettings(s *model.LdapSettings) *settings {
	c := *s // shallow copy, SetDefaults only replaces nil pointers
	c.SetDefaults()

	res := &settings{
		Enable:                      *c.Enable,
		EnableSync:                  *c.EnableSync,
		Server:                      strings.TrimSpace(*c.LdapServer),
		Port:                        *c.LdapPort,
		ConnectionSecurity:          *c.ConnectionSecurity,
		BaseDN:                      *c.BaseDN,
		BindUsername:                *c.BindUsername,
		BindPassword:                *c.BindPassword,
		UserFilter:                  strings.TrimSpace(*c.UserFilter),
		GroupFilter:                 strings.TrimSpace(*c.GroupFilter),
		GuestFilter:                 strings.TrimSpace(*c.GuestFilter),
		EnableAdminFilter:           *c.EnableAdminFilter,
		AdminFilter:                 strings.TrimSpace(*c.AdminFilter),
		GroupDisplayNameAttribute:   strings.TrimSpace(*c.GroupDisplayNameAttribute),
		GroupIdAttribute:            strings.TrimSpace(*c.GroupIdAttribute),
		FirstNameAttribute:          strings.TrimSpace(*c.FirstNameAttribute),
		LastNameAttribute:           strings.TrimSpace(*c.LastNameAttribute),
		EmailAttribute:              strings.TrimSpace(*c.EmailAttribute),
		UsernameAttribute:           strings.TrimSpace(*c.UsernameAttribute),
		NicknameAttribute:           strings.TrimSpace(*c.NicknameAttribute),
		IdAttribute:                 strings.TrimSpace(*c.IdAttribute),
		PositionAttribute:           strings.TrimSpace(*c.PositionAttribute),
		LoginIdAttribute:            strings.TrimSpace(*c.LoginIdAttribute),
		PictureAttribute:            strings.TrimSpace(*c.PictureAttribute),
		SyncIntervalMinutes:         *c.SyncIntervalMinutes,
		ReAddRemovedMembers:         *c.ReAddRemovedMembers,
		SkipCertificateVerification: *c.SkipCertificateVerification,
		PublicCertificateFile:       *c.PublicCertificateFile,
		PrivateKeyFile:              *c.PrivateKeyFile,
		QueryTimeout:                time.Duration(*c.QueryTimeout) * time.Second,
		MaxPageSize:                 *c.MaxPageSize,
	}
	if res.LoginIdAttribute == "" {
		res.LoginIdAttribute = res.IdAttribute
	}
	if res.Port <= 0 {
		if res.ConnectionSecurity == model.ConnSecurityTLS {
			res.Port = 636
		} else {
			res.Port = 389
		}
	}
	if res.QueryTimeout <= 0 {
		res.QueryTimeout = 60 * time.Second
	}
	if res.MaxPageSize < 0 {
		res.MaxPageSize = 0
	}
	if res.SyncIntervalMinutes < 1 {
		res.SyncIntervalMinutes = 60
	}
	return res
}

// userFilter returns the filter used to select Mattermost users. When no
// user filter is configured, every object carrying the ID attribute is a
// candidate.
func (s *settings) userFilter() string {
	if s.UserFilter != "" {
		return ensureParens(s.UserFilter)
	}
	if s.IdAttribute != "" {
		return "(" + s.IdAttribute + "=*)"
	}
	return "(objectClass=*)"
}

// groupFilter returns the filter used to select groups available for linking.
func (s *settings) groupFilter() string {
	if s.GroupFilter != "" {
		return ensureParens(s.GroupFilter)
	}
	return defaultGroupFilter
}

// anyGroupFilter matches every object that can act as a group, either
// because it is selected by the configured group filter or because it has one
// of the usual group object classes. It is used to resolve nested groups.
func (s *settings) anyGroupFilter() string {
	if s.GroupFilter == "" {
		return defaultGroupFilter
	}
	return "(|" + ensureParens(s.GroupFilter) + defaultGroupFilter + ")"
}

// userAttributes returns the list of attributes to request for user entries.
func (s *settings) userAttributes(extra ...string) []string {
	attrs := []string{}
	seen := map[string]bool{}
	add := func(a string) {
		if a == "" {
			return
		}
		key := strings.ToLower(a)
		if seen[key] {
			return
		}
		seen[key] = true
		attrs = append(attrs, a)
	}
	for _, a := range []string{
		s.IdAttribute, s.LoginIdAttribute, s.UsernameAttribute, s.EmailAttribute,
		s.FirstNameAttribute, s.LastNameAttribute, s.NicknameAttribute, s.PositionAttribute,
	} {
		add(a)
	}
	for _, a := range extra {
		add(a)
	}
	return attrs
}

// groupAttributes returns the list of attributes to request for group entries.
func (s *settings) groupAttributes(withMembers bool) []string {
	attrs := []string{"objectClass"}
	if s.GroupIdAttribute != "" {
		attrs = append(attrs, s.GroupIdAttribute)
	}
	if s.GroupDisplayNameAttribute != "" && !strings.EqualFold(s.GroupDisplayNameAttribute, s.GroupIdAttribute) {
		attrs = append(attrs, s.GroupDisplayNameAttribute)
	}
	if withMembers {
		attrs = append(attrs, groupMemberAttributes...)
	}
	return attrs
}

// guestFilterEnabled reports whether the guest filter must be applied.
func guestFilterEnabled(cfg *model.Config, s *settings) bool {
	return s.GuestFilter != "" && cfg.GuestAccountsSettings.Enable != nil && *cfg.GuestAccountsSettings.Enable
}

// adminFilterEnabled reports whether the admin filter must be applied.
func adminFilterEnabled(s *settings) bool {
	return s.EnableAdminFilter && s.AdminFilter != ""
}

func ensureParens(filter string) string {
	filter = strings.TrimSpace(filter)
	if filter == "" {
		return filter
	}
	if !strings.HasPrefix(filter, "(") {
		return "(" + filter + ")"
	}
	return filter
}

// andFilters combines several filters with a logical AND.
func andFilters(filters ...string) string {
	parts := make([]string, 0, len(filters))
	for _, f := range filters {
		if f = ensureParens(f); f != "" {
			parts = append(parts, f)
		}
	}
	switch len(parts) {
	case 0:
		return "(objectClass=*)"
	case 1:
		return parts[0]
	default:
		return "(&" + strings.Join(parts, "") + ")"
	}
}
