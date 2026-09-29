// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package ldap

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

func resultByName(results []model.LdapDiagnosticResult, name string) *model.LdapDiagnosticResult {
	for i := range results {
		if results[i].TestName == name {
			return &results[i]
		}
	}
	return nil
}

func TestDiagnostics(t *testing.T) {
	env := setupEnv(t)
	env.dir.add("cn=devs,ou=groups,dc=example,dc=com", map[string][]string{
		"objectClass": {"groupOfNames"}, "cn": {"devs"}, "entryUUID": {"g-devs"},
	})
	env.dir.add("cn=nameless,ou=groups,dc=example,dc=com", map[string][]string{
		"objectClass": {"groupOfNames"}, "entryUUID": {"g-nameless"},
	})
	d := newDiagnostic(env.b, env.dir.dialer())
	rctx := env.b.rctx()

	t.Run("RunTest", func(t *testing.T) {
		require.Nil(t, d.RunTest(rctx))

		*env.b.cfg.LdapSettings.UserFilter = "(objectClass=nothing)"
		defer func() { *env.b.cfg.LdapSettings.UserFilter = "(objectClass=inetOrgPerson)" }()
		appErr := d.RunTest(rctx)
		require.NotNil(t, appErr)
		assert.Equal(t, "ent.ldap.no.users.checkcertificate", appErr.Id)
	})

	t.Run("RunTestConnection reuses the saved password", func(t *testing.T) {
		submitted := env.b.cfg.LdapSettings
		submitted.BindPassword = new(model.FakeSetting)
		require.Nil(t, d.RunTestConnection(rctx, submitted))

		submitted.BindPassword = new("wrong")
		appErr := d.RunTestConnection(rctx, submitted)
		require.NotNil(t, appErr)
		assert.Equal(t, "ent.ldap.connection.test_failed", appErr.Id)
	})

	t.Run("RunTestConnection with partial settings", func(t *testing.T) {
		submitted := model.LdapSettings{
			LdapServer:   new("ldap.example.com"),
			BindUsername: new("cn=admin,dc=example,dc=com"),
		}
		require.Nil(t, d.RunTestConnection(rctx, submitted))
	})

	t.Run("filters", func(t *testing.T) {
		submitted := env.b.cfg.LdapSettings
		submitted.GuestFilter = new("(uid=bob)")
		submitted.AdminFilter = new("(uid=")
		results, appErr := d.RunTestDiagnostics(rctx, model.LdapDiagnosticTestTypeFilters, submitted)
		require.Nil(t, appErr)

		base := resultByName(results, "BaseDN")
		require.NotNil(t, base)
		assert.Equal(t, 1, base.TotalCount)
		assert.Empty(t, base.Error)

		users := resultByName(results, "UserFilter")
		require.NotNil(t, users)
		assert.Equal(t, 2, users.TotalCount)
		assert.Len(t, users.SampleResults, 2)

		groups := resultByName(results, "GroupFilter")
		require.NotNil(t, groups)
		assert.Equal(t, defaultGroupFilter, groups.TestValue)
		assert.Equal(t, 2, groups.TotalCount)

		guests := resultByName(results, "GuestFilter")
		require.NotNil(t, guests)
		assert.Equal(t, 1, guests.TotalCount)

		admins := resultByName(results, "AdminFilter")
		require.NotNil(t, admins)
		assert.NotEmpty(t, admins.Error)
	})

	t.Run("attributes", func(t *testing.T) {
		env.dir.get("uid=alice,ou=people,dc=example,dc=com").set("jpegPhoto", []byte{1, 2, 3})
		submitted := env.b.cfg.LdapSettings
		submitted.PictureAttribute = new("jpegPhoto")
		results, appErr := d.RunTestDiagnostics(rctx, model.LdapDiagnosticTestTypeAttributes, submitted)
		require.Nil(t, appErr)

		email := resultByName(results, "EmailAttribute")
		require.NotNil(t, email)
		assert.Equal(t, 2, email.TotalCount)
		assert.Equal(t, 2, email.EntriesWithValue)

		nick := resultByName(results, "NicknameAttribute")
		require.NotNil(t, nick)
		assert.Equal(t, 0, nick.EntriesWithValue)

		picture := resultByName(results, "PictureAttribute")
		require.NotNil(t, picture)
		assert.Equal(t, 1, picture.EntriesWithValue)
	})

	t.Run("group attributes", func(t *testing.T) {
		results, appErr := d.RunTestDiagnostics(rctx, model.LdapDiagnosticTestTypeGroupAttributes, env.b.cfg.LdapSettings)
		require.Nil(t, appErr)
		name := resultByName(results, "GroupDisplayNameAttribute")
		require.NotNil(t, name)
		assert.Equal(t, 2, name.TotalCount)
		assert.Equal(t, 1, name.EntriesWithValue)
		id := resultByName(results, "GroupIdAttribute")
		require.NotNil(t, id)
		assert.Equal(t, 2, id.EntriesWithValue)
	})

	t.Run("vendor", func(t *testing.T) {
		env.dir.add("", map[string][]string{"vendorName": {"Example Inc."}, "vendorVersion": {"1.2.3"}})
		name, version, err := d.GetVendorNameAndVendorVersion(rctx)
		require.NoError(t, err)
		assert.Equal(t, "Example Inc.", name)
		assert.Equal(t, "1.2.3", version)
	})
}

func TestCodec(t *testing.T) {
	guid := []byte{0x78, 0x56, 0x34, 0x12, 0x34, 0x12, 0x78, 0x56, 0x12, 0x34, 0x56, 0x78, 0x9a, 0xbc, 0xde, 0xf0}
	s := formatGUID(guid)
	assert.Equal(t, "12345678-1234-5678-1234-56789abcdef0", s)
	parsed, err := parseGUID("{" + s + "}")
	require.NoError(t, err)
	assert.Equal(t, guid, parsed)
	assert.Equal(t, `\78\56\34\12\34\12\78\56\12\34\56\78\9a\bc\de\f0`, filterValue("objectGUID", s))

	sid := []byte{1, 5, 0, 0, 0, 0, 0, 5, 21, 0, 0, 0, 1, 0, 0, 0, 2, 0, 0, 0, 3, 0, 0, 0, 0xe9, 0x03, 0, 0}
	str, err := formatSID(sid)
	require.NoError(t, err)
	assert.Equal(t, "S-1-5-21-1-2-3-1001", str)
	back, err := parseSID(str)
	require.NoError(t, err)
	assert.Equal(t, sid, back)

	assert.Equal(t, `a\2ab\28c\29`, filterValue("uid", "a*b(c)"))
	assert.Equal(t, "cn=john smith,ou=people,dc=example,dc=com", normalizeDN("CN=John Smith, OU=People,DC=Example , DC=com"))
	assert.Equal(t, "ff00", decodeValue("unknownBinary", []byte{0xff, 0x00}))
}

func TestSettings(t *testing.T) {
	s := newSettings(&model.LdapSettings{IdAttribute: new("uid")})
	assert.Equal(t, "(uid=*)", s.userFilter())
	assert.Equal(t, defaultGroupFilter, s.groupFilter())
	assert.Equal(t, "uid", s.LoginIdAttribute)
	assert.Equal(t, 389, s.Port)

	s = newSettings(&model.LdapSettings{UserFilter: new("objectClass=person"), GroupFilter: new("(cn=x)")})
	assert.Equal(t, "(objectClass=person)", s.userFilter())
	assert.Equal(t, "(|(cn=x)"+defaultGroupFilter+")", s.anyGroupFilter())
	assert.Equal(t, "(&(a=1)(b=2))", andFilters("(a=1)", "", "b=2"))

	assert.Equal(t, "john.smith", usernameFromDirectory("John.Smith"))
	assert.Equal(t, "john-smith", usernameFromDirectory("John Smith"))
	assert.Equal(t, "", usernameFromDirectory("all"))
	assert.Equal(t, "", usernameFromDirectory("---"))
}
