// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package ldap

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

type testEnv struct {
	dir *fakeDir
	b   *fakeBackend
	l   *Ldap
}

func addPerson(d *fakeDir, uid, mail, given, sn string) string {
	dn := fmt.Sprintf("uid=%s,ou=people,dc=example,dc=com", uid)
	d.add(dn, map[string][]string{
		"objectClass": {"inetOrgPerson"},
		"uid":         {uid},
		"mail":        {mail},
		"givenName":   {given},
		"sn":          {sn},
		"title":       {"Engineer"},
	})
	d.setPassword(dn, uid+"-pwd")
	return dn
}

func setupEnv(t *testing.T) *testEnv {
	t.Helper()
	d := newFakeDir()
	d.add("dc=example,dc=com", map[string][]string{"objectClass": {"domain"}, "dc": {"example"}})
	d.add("ou=people,dc=example,dc=com", map[string][]string{"objectClass": {"organizationalUnit"}})
	d.add("ou=groups,dc=example,dc=com", map[string][]string{"objectClass": {"organizationalUnit"}})
	addPerson(d, "alice", "Alice@Example.com", "Alice", "Liddell")
	addPerson(d, "bob", "bob@example.com", "Bob", "Builder")
	// carol does not match the user filter
	d.add("uid=carol,ou=people,dc=example,dc=com", map[string][]string{
		"objectClass": {"person"}, "uid": {"carol"}, "mail": {"carol@example.com"},
	})
	d.setPassword("uid=carol,ou=people,dc=example,dc=com", "carol-pwd")

	b := newFakeBackend()
	return &testEnv{dir: d, b: b, l: newLdap(b, d.dialer())}
}

func TestDoLogin(t *testing.T) {
	t.Run("creates the account on first login", func(t *testing.T) {
		env := setupEnv(t)
		user, appErr := env.l.DoLogin(env.b.rctx(), "alice", "alice-pwd")
		require.Nil(t, appErr)
		require.NotEmpty(t, user.Id)
		assert.Equal(t, "alice", user.Username)
		assert.Equal(t, "alice@example.com", user.Email)
		assert.Equal(t, "Alice", user.FirstName)
		assert.Equal(t, "Liddell", user.LastName)
		assert.Equal(t, "Engineer", user.Position)
		assert.Equal(t, model.UserAuthServiceLdap, user.AuthService)
		assert.Equal(t, "alice", *user.AuthData)
		assert.True(t, user.EmailVerified)
		assert.Zero(t, env.dir.openConns, "all connections must be closed")
	})

	t.Run("wrong password creates the account but fails", func(t *testing.T) {
		env := setupEnv(t)
		_, appErr := env.l.DoLogin(env.b.rctx(), "alice", "wrong")
		require.NotNil(t, appErr)
		assert.Equal(t, "ent.ldap.do_login.invalid_password.app_error", appErr.Id)
		assert.NotNil(t, env.b.userByAuth("alice"), "the application expects the account to exist")

		_, appErr = env.l.DoLogin(env.b.rctx(), "alice", "")
		require.NotNil(t, appErr)
		assert.Equal(t, "ent.ldap.do_login.invalid_password.app_error", appErr.Id)
	})

	t.Run("existing account is updated", func(t *testing.T) {
		env := setupEnv(t)
		authData := "bob"
		existing := env.b.addUser(&model.User{
			Username: "bob", Email: "bob@example.com", FirstName: "Robert", AuthService: model.UserAuthServiceLdap, AuthData: &authData, LastLogin: 1,
		})
		user, appErr := env.l.DoLogin(env.b.rctx(), "bob", "bob-pwd")
		require.Nil(t, appErr)
		assert.Equal(t, existing.Id, user.Id)
		assert.Equal(t, "Bob", user.FirstName)
		assert.Equal(t, "Bob", env.b.user(existing.Id).FirstName)
		assert.Empty(t, env.b.defaultMemberships, "no first login sync for returning users")
	})

	t.Run("user rejected by the user filter", func(t *testing.T) {
		env := setupEnv(t)
		_, appErr := env.l.DoLogin(env.b.rctx(), "carol", "carol-pwd")
		require.NotNil(t, appErr)
		assert.Equal(t, "ent.ldap.do_login.user_filtered.app_error", appErr.Id)
	})

	t.Run("unknown user", func(t *testing.T) {
		env := setupEnv(t)
		_, appErr := env.l.DoLogin(env.b.rctx(), "nobody", "pwd")
		require.NotNil(t, appErr)
		assert.Equal(t, "ent.ldap.do_login.user_not_registered.app_error", appErr.Id)
	})

	t.Run("unreachable server", func(t *testing.T) {
		env := setupEnv(t)
		env.dir.dialErr = unableToConnect()
		_, appErr := env.l.DoLogin(env.b.rctx(), "alice", "alice-pwd")
		require.NotNil(t, appErr)
		assert.Equal(t, "ent.ldap.do_login.unable_to_connect.app_error", appErr.Id)
	})

	t.Run("wrong service account password", func(t *testing.T) {
		env := setupEnv(t)
		*env.b.cfg.LdapSettings.BindPassword = "nope"
		_, appErr := env.l.DoLogin(env.b.rctx(), "alice", "alice-pwd")
		require.NotNil(t, appErr)
		assert.Equal(t, "ent.ldap.do_login.bind_admin_user.app_error", appErr.Id)
		assert.Zero(t, env.dir.openConns)
	})

	t.Run("admin filter promotes and demotes", func(t *testing.T) {
		env := setupEnv(t)
		*env.b.cfg.LdapSettings.EnableAdminFilter = true
		*env.b.cfg.LdapSettings.AdminFilter = "(title=Engineer)"

		user, appErr := env.l.DoLogin(env.b.rctx(), "alice", "alice-pwd")
		require.Nil(t, appErr)
		assert.True(t, user.IsSystemAdmin())
		assert.Equal(t, "true", user.Props[userPropAdminFilter])

		env.dir.get("uid=alice,ou=people,dc=example,dc=com").replace("title", "Manager")
		user, appErr = env.l.DoLogin(env.b.rctx(), "alice", "alice-pwd")
		require.Nil(t, appErr)
		assert.False(t, user.IsSystemAdmin())
		assert.Empty(t, user.Props[userPropAdminFilter])
	})

	t.Run("admin filter does not demote manually promoted admins", func(t *testing.T) {
		env := setupEnv(t)
		*env.b.cfg.LdapSettings.EnableAdminFilter = true
		*env.b.cfg.LdapSettings.AdminFilter = "(title=Boss)"
		authData := "bob"
		env.b.addUser(&model.User{
			Username: "bob", Email: "bob@example.com", Roles: "system_user system_admin", AuthService: model.UserAuthServiceLdap, AuthData: &authData, LastLogin: 1,
		})
		user, appErr := env.l.DoLogin(env.b.rctx(), "bob", "bob-pwd")
		require.Nil(t, appErr)
		assert.True(t, user.IsSystemAdmin())
	})

	t.Run("guest filter creates guests", func(t *testing.T) {
		env := setupEnv(t)
		*env.b.cfg.GuestAccountsSettings.Enable = true
		*env.b.cfg.LdapSettings.GuestFilter = "(uid=bob)"

		user, appErr := env.l.DoLogin(env.b.rctx(), "bob", "bob-pwd")
		require.Nil(t, appErr)
		assert.True(t, user.IsGuest())

		user, appErr = env.l.DoLogin(env.b.rctx(), "alice", "alice-pwd")
		require.Nil(t, appErr)
		assert.False(t, user.IsGuest())
	})

	t.Run("guest filter ignored when guest accounts are disabled", func(t *testing.T) {
		env := setupEnv(t)
		*env.b.cfg.GuestAccountsSettings.Enable = false
		*env.b.cfg.LdapSettings.GuestFilter = "(uid=bob)"
		user, appErr := env.l.DoLogin(env.b.rctx(), "bob", "bob-pwd")
		require.Nil(t, appErr)
		assert.False(t, user.IsGuest())
	})

	t.Run("first login synchronizes group memberships", func(t *testing.T) {
		env := setupEnv(t)
		env.dir.add("cn=devs,ou=groups,dc=example,dc=com", map[string][]string{
			"objectClass": {"groupOfNames"}, "cn": {"devs"}, "entryUUID": {"g-devs"},
			"member": {"uid=alice,ou=people,dc=example,dc=com"},
		})
		env.dir.add("cn=all,ou=groups,dc=example,dc=com", map[string][]string{
			"objectClass": {"groupOfNames"}, "cn": {"all"}, "entryUUID": {"g-all"},
			"member": {"cn=devs,ou=groups,dc=example,dc=com"},
		})
		env.dir.add("cn=ops,ou=groups,dc=example,dc=com", map[string][]string{
			"objectClass": {"groupOfNames"}, "cn": {"ops"}, "entryUUID": {"g-ops"},
			"member": {"uid=bob,ou=people,dc=example,dc=com"},
		})
		devs := env.b.linkGroup("g-devs", "devs")
		all := env.b.linkGroup("g-all", "all")
		ops := env.b.linkGroup("g-ops", "ops")

		user, appErr := env.l.DoLogin(env.b.rctx(), "alice", "alice-pwd")
		require.Nil(t, appErr)
		assert.Equal(t, []string{user.Id}, env.b.memberIDs(devs.Id))
		assert.Equal(t, []string{user.Id}, env.b.memberIDs(all.Id), "nested group membership")
		assert.Empty(t, env.b.memberIDs(ops.Id))
		require.Len(t, env.b.defaultMemberships, 1)
		assert.Equal(t, user.Id, *env.b.defaultMemberships[0].ScopedUserID)
	})
}

func TestGetUser(t *testing.T) {
	env := setupEnv(t)
	*env.b.cfg.LdapSettings.LoginIdAttribute = "mail"

	user, appErr := env.l.GetUser(env.b.rctx(), "bob@example.com")
	require.Nil(t, appErr)
	assert.Equal(t, "bob", *user.AuthData)
	assert.Empty(t, user.Id)

	env.dir.add("uid=bob2,ou=people,dc=example,dc=com", map[string][]string{
		"objectClass": {"inetOrgPerson"}, "uid": {"bob2"}, "mail": {"bob@example.com"},
	})
	_, appErr = env.l.GetUser(env.b.rctx(), "bob@example.com")
	require.NotNil(t, appErr)
	assert.Equal(t, "ent.ldap.do_login.matched_to_many_users.app_error", appErr.Id)

	// filter injection attempts are escaped
	_, appErr = env.l.GetUser(env.b.rctx(), "*")
	require.NotNil(t, appErr)
	assert.Equal(t, "ent.ldap.do_login.user_not_registered.app_error", appErr.Id)
}

func TestGetUserAttributesAndMMUser(t *testing.T) {
	env := setupEnv(t)
	attrs, appErr := env.l.GetUserAttributes(env.b.rctx(), "alice", []string{"title", "sn", "missing"})
	require.Nil(t, appErr)
	assert.Equal(t, map[string]string{"title": "Engineer", "sn": "Liddell", "missing": ""}, attrs)

	*env.b.cfg.SamlSettings.EnableSyncWithLdap = true
	samlUser := &model.User{Email: "bob@example.com", AuthService: model.UserAuthServiceSaml}
	ldapUser, dn, appErr := env.l.GetLDAPUserForMMUser(env.b.rctx(), samlUser)
	require.Nil(t, appErr)
	assert.Equal(t, "bob", *ldapUser.AuthData)
	assert.Equal(t, "uid=bob,ou=people,dc=example,dc=com", dn)
}

func TestCheckProviderAttributes(t *testing.T) {
	env := setupEnv(t)
	ls := &env.b.cfg.LdapSettings
	user := &model.User{Username: "alice", Email: "alice@example.com", FirstName: "Alice", Nickname: "al", Position: "Engineer"}

	assert.Equal(t, "", env.l.CheckProviderAttributes(env.b.rctx(), ls, user, &model.UserPatch{FirstName: new("Alice")}))
	assert.Equal(t, "first name", env.l.CheckProviderAttributes(env.b.rctx(), ls, user, &model.UserPatch{FirstName: new("Al")}))
	assert.Equal(t, "email", env.l.CheckProviderAttributes(env.b.rctx(), ls, user, &model.UserPatch{Email: new("x@example.com")}))
	assert.Equal(t, "position", env.l.CheckProviderAttributes(env.b.rctx(), ls, user, &model.UserPatch{Position: new("CEO")}))

	*ls.NicknameAttribute = ""
	assert.Equal(t, "", env.l.CheckProviderAttributes(env.b.rctx(), ls, user, &model.UserPatch{Nickname: new("ally")}))
}

func TestSwitchToLdap(t *testing.T) {
	env := setupEnv(t)
	emailUser := env.b.addUser(&model.User{Username: "alice-email", Email: "alice@example.com", Password: "hash"})

	appErr := env.l.SwitchToLdap(env.b.rctx(), emailUser.Id, "alice", "wrong")
	require.NotNil(t, appErr)
	assert.Equal(t, "ent.ldap.do_login.invalid_password.app_error", appErr.Id)

	appErr = env.l.SwitchToLdap(env.b.rctx(), emailUser.Id, "alice", "alice-pwd")
	require.Nil(t, appErr)
	u := env.b.user(emailUser.Id)
	assert.Equal(t, model.UserAuthServiceLdap, u.AuthService)
	assert.Equal(t, "alice", *u.AuthData)

	other := env.b.addUser(&model.User{Username: "other", Email: "other@example.com"})
	appErr = env.l.SwitchToLdap(env.b.rctx(), other.Id, "alice", "alice-pwd")
	require.NotNil(t, appErr)
	assert.Equal(t, "ent.ldap.switch_to_ldap.already_attached.app_error", appErr.Id)
}

func TestGetAllLdapUsers(t *testing.T) {
	env := setupEnv(t)
	users, appErr := env.l.GetAllLdapUsers(env.b.rctx())
	require.Nil(t, appErr)
	require.Len(t, users, 2)
	names := []string{users[0].Username, users[1].Username}
	assert.ElementsMatch(t, []string{"alice", "bob"}, names)

	*env.b.cfg.LdapSettings.MaxPageSize = 500
	_, appErr = env.l.GetAllLdapUsers(env.b.rctx())
	require.Nil(t, appErr)
	assert.Equal(t, []uint32{500}, env.dir.pageSizes)
}

func TestMigrateIDAttribute(t *testing.T) {
	env := setupEnv(t)
	env.dir.get("uid=alice,ou=people,dc=example,dc=com").replace("employeeNumber", "1001")
	env.dir.get("uid=bob,ou=people,dc=example,dc=com").replace("employeeNumber", "1002")
	a, b := "alice", "bob"
	alice := env.b.addUser(&model.User{Username: "alice", Email: "alice@example.com", AuthService: model.UserAuthServiceLdap, AuthData: &a})
	bob := env.b.addUser(&model.User{Username: "bob", Email: "bob@example.com", AuthService: model.UserAuthServiceLdap, AuthData: &b})

	require.NoError(t, env.l.MigrateIDAttribute(env.b.rctx(), "employeeNumber"))
	assert.Equal(t, "1001", *env.b.user(alice.Id).AuthData)
	assert.Equal(t, "1002", *env.b.user(bob.Id).AuthData)

	// duplicates are rejected
	env.dir.get("uid=bob,ou=people,dc=example,dc=com").replace("cn", "same")
	env.dir.get("uid=alice,ou=people,dc=example,dc=com").replace("cn", "same")
	*env.b.cfg.LdapSettings.IdAttribute = "employeeNumber"
	require.Error(t, env.l.MigrateIDAttribute(env.b.rctx(), "cn"))
}

func TestUpdateProfilePicture(t *testing.T) {
	env := setupEnv(t)
	*env.b.cfg.LdapSettings.PictureAttribute = "jpegPhoto"
	env.dir.get("uid=alice,ou=people,dc=example,dc=com").set("jpegPhoto", []byte{0xff, 0xd8, 0x01})
	a := "alice"
	alice := env.b.addUser(&model.User{Username: "alice", Email: "alice@example.com", AuthService: model.UserAuthServiceLdap, AuthData: &a})

	env.l.UpdateProfilePictureIfNecessary(env.b.rctx(), *alice, model.Session{})
	assert.Equal(t, []byte{0xff, 0xd8, 0x01}, env.b.pictures[alice.Id])

	// removed from the directory: reset to the default picture
	env.dir.get("uid=alice,ou=people,dc=example,dc=com").replace("jpegPhoto")
	env.l.UpdateProfilePictureIfNecessary(env.b.rctx(), *env.b.user(alice.Id), model.Session{})
	assert.Equal(t, 1, env.b.defaultPicture[alice.Id])
	assert.Nil(t, env.b.pictures[alice.Id])
}

func TestObjectGUIDAsID(t *testing.T) {
	env := setupEnv(t)
	guid := []byte{0x78, 0x56, 0x34, 0x12, 0x34, 0x12, 0x78, 0x56, 0x12, 0x34, 0x56, 0x78, 0x9a, 0xbc, 0xde, 0xf0}
	env.dir.get("uid=alice,ou=people,dc=example,dc=com").set("objectGUID", guid)
	*env.b.cfg.LdapSettings.IdAttribute = "objectGUID"

	user, appErr := env.l.GetUser(env.b.rctx(), "alice")
	require.Nil(t, appErr)
	assert.Equal(t, "12345678-1234-5678-1234-56789abcdef0", *user.AuthData)

	logged, appErr := env.l.DoLogin(env.b.rctx(), *user.AuthData, "alice-pwd")
	require.Nil(t, appErr)
	assert.Equal(t, "12345678-1234-5678-1234-56789abcdef0", *logged.AuthData)
}

func TestCustomProfileAttributes(t *testing.T) {
	t.Run("sign-in hands every directory value of the synced attributes to the syncer", func(t *testing.T) {
		env := setupEnv(t)
		syncer := newFakeAttributeSyncer("departmentNumber", "l", "businessCategory")
		env.b.attrSyncer = syncer

		entry := env.dir.get("uid=alice,ou=people,dc=example,dc=com")
		entry.replace("departmentNumber", "R&D")
		entry.replace("l", "Paris")
		entry.replace("businessCategory", "Sales", "Support")

		user, appErr := env.l.DoLogin(env.b.rctx(), "alice", "alice-pwd")
		require.Nil(t, appErr)

		// The directory only returns the attributes the search asks for, so
		// getting the values shows the synced attributes were requested.
		assert.Equal(t, map[string][]string{
			"departmentNumber": {"R&D"},
			"l":                {"Paris"},
			"businessCategory": {"Sales", "Support"},
		}, syncer.synced[user.Id])
		assert.True(t, syncer.syncOpts[user.Id].PruneOrphanedOptions, "a sign-in prunes the options the user was the last to hold")
	})

	t.Run("an attribute removed from the directory is left out, which clears the value", func(t *testing.T) {
		env := setupEnv(t)
		syncer := newFakeAttributeSyncer("departmentNumber", "l")
		env.b.attrSyncer = syncer
		entry := env.dir.get("uid=alice,ou=people,dc=example,dc=com")
		entry.replace("departmentNumber", "R&D")
		entry.replace("l", "Paris")

		user, appErr := env.l.DoLogin(env.b.rctx(), "alice", "alice-pwd")
		require.Nil(t, appErr)
		require.Contains(t, syncer.synced[user.Id], "departmentNumber")

		entry.replace("departmentNumber")
		_, appErr = env.l.DoLogin(env.b.rctx(), "alice", "alice-pwd")
		require.Nil(t, appErr)
		assert.NotContains(t, syncer.synced[user.Id], "departmentNumber")
		assert.Equal(t, []string{"Paris"}, syncer.synced[user.Id]["l"])
	})

	t.Run("nothing is synced when no attribute is synced from AD/LDAP", func(t *testing.T) {
		env := setupEnv(t)
		syncer := newFakeAttributeSyncer()
		env.b.attrSyncer = syncer

		_, appErr := env.l.DoLogin(env.b.rctx(), "alice", "alice-pwd")
		require.Nil(t, appErr)
		assert.Zero(t, syncer.syncCalls)
	})

	t.Run("sign-in still works when the synced attributes cannot be loaded", func(t *testing.T) {
		env := setupEnv(t)
		env.b.attrSyncerErr = model.NewAppError("test", "app.property_group.get.app_error", nil, "", 500)

		user, appErr := env.l.DoLogin(env.b.rctx(), "alice", "alice-pwd")
		require.Nil(t, appErr)
		assert.Equal(t, "alice", user.Username)
	})

	t.Run("a failed password check syncs nothing", func(t *testing.T) {
		env := setupEnv(t)
		syncer := newFakeAttributeSyncer("l")
		env.b.attrSyncer = syncer
		env.dir.get("uid=alice,ou=people,dc=example,dc=com").replace("l", "Paris")

		_, appErr := env.l.DoLogin(env.b.rctx(), "alice", "wrong")
		require.NotNil(t, appErr)
		assert.Zero(t, syncer.syncCalls)
	})
}

func TestStartSynchronizeJob(t *testing.T) {
	env := setupEnv(t)
	job, appErr := env.l.StartSynchronizeJob(env.b.rctx(), false)
	require.Nil(t, appErr)
	assert.Equal(t, model.JobTypeLdapSync, job.Type)

	start := time.Now()
	job, appErr = env.l.StartSynchronizeJob(env.b.rctx(), true)
	require.Nil(t, appErr)
	assert.Equal(t, model.JobStatusSuccess, job.Status)
	assert.Less(t, time.Since(start), 5*time.Second)
}

func TestNextSyncTime(t *testing.T) {
	cfg := &model.Config{}
	cfg.SetDefaults()
	*cfg.LdapSettings.SyncIntervalMinutes = 30
	now := time.Now()

	next := nextSyncTime(cfg, now, nil)
	assert.WithinDuration(t, now.Add(30*time.Minute), *next, time.Second)

	last := &model.Job{StartAt: now.Add(-10 * time.Minute).UnixMilli()}
	next = nextSyncTime(cfg, now, last)
	assert.WithinDuration(t, now.Add(20*time.Minute), *next, time.Second)

	last = &model.Job{StartAt: now.Add(-2 * time.Hour).UnixMilli()}
	next = nextSyncTime(cfg, now, last)
	assert.WithinDuration(t, now, *next, time.Second)
}
