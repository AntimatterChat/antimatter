// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package ldap

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

func ldapAccount(env *testEnv, uid, email string) *model.User {
	authData := uid
	return env.b.addUser(&model.User{
		Username: uid, Email: email, AuthService: model.UserAuthServiceLdap, AuthData: &authData,
	})
}

func TestSynchronizeUsers(t *testing.T) {
	env := setupEnv(t)
	alice := ldapAccount(env, "alice", "old@example.com")
	bob := ldapAccount(env, "bob", "bob@example.com")
	gone := ldapAccount(env, "gone", "gone@example.com")
	carol := ldapAccount(env, "carol", "carol@example.com") // filtered out
	back := ldapAccount(env, "back", "back@example.com")
	env.b.SetUserActive(env.b.rctx(), back, false)
	addPerson(env.dir, "back", "back@example.com", "Back", "Again")

	stats, appErr := env.l.Synchronize(env.b.rctx(), syncOptions{})
	require.Nil(t, appErr)

	assert.Equal(t, 3, stats.LdapUsers)
	assert.Equal(t, "alice@example.com", env.b.user(alice.Id).Email)
	assert.Equal(t, "Liddell", env.b.user(alice.Id).LastName)
	assert.Equal(t, "Builder", env.b.user(bob.Id).LastName)
	assert.NotZero(t, env.b.user(gone.Id).DeleteAt)
	assert.NotZero(t, env.b.user(carol.Id).DeleteAt)
	assert.Zero(t, env.b.user(back.Id).DeleteAt, "user back in the directory is reactivated")
	assert.Equal(t, 2, stats.Deactivated)
	assert.Equal(t, 3, stats.Updated)

	// a second run changes nothing
	stats, appErr = env.l.Synchronize(env.b.rctx(), syncOptions{})
	require.Nil(t, appErr)
	assert.Zero(t, stats.Updated)
	assert.Zero(t, stats.Deactivated)

	data := stats.toJobData(nil)
	assert.Equal(t, "3", data[jobDataUsersCount])
	assert.Equal(t, "0", data[jobDataDeleteCount])
}

func TestSynchronizeRefusesEmptyDirectory(t *testing.T) {
	env := setupEnv(t)
	alice := ldapAccount(env, "alice", "alice@example.com")
	*env.b.cfg.LdapSettings.BaseDN = "ou=missing,dc=example,dc=com"

	_, appErr := env.l.Synchronize(env.b.rctx(), syncOptions{})
	require.NotNil(t, appErr)
	assert.Equal(t, "ent.ldap.no.users.checkcertificate", appErr.Id)
	assert.Zero(t, env.b.user(alice.Id).DeleteAt)
}

func TestSynchronizeGuests(t *testing.T) {
	env := setupEnv(t)
	*env.b.cfg.GuestAccountsSettings.Enable = true
	*env.b.cfg.LdapSettings.GuestFilter = "(uid=bob)"
	bob := ldapAccount(env, "bob", "bob@example.com")
	alice := ldapAccount(env, "alice", "alice@example.com")

	_, appErr := env.l.Synchronize(env.b.rctx(), syncOptions{})
	require.Nil(t, appErr)
	assert.True(t, env.b.user(bob.Id).IsGuest())
	assert.False(t, env.b.user(alice.Id).IsGuest())
}

func TestSynchronizeSamlUsers(t *testing.T) {
	env := setupEnv(t)
	*env.b.cfg.SamlSettings.EnableSyncWithLdap = true
	email := "bob@example.com"
	samlBob := env.b.addUser(&model.User{Username: "bobby", Email: email, AuthService: model.UserAuthServiceSaml, AuthData: &email})
	unknown := "ghost@example.com"
	samlGhost := env.b.addUser(&model.User{Username: "ghost", Email: unknown, AuthService: model.UserAuthServiceSaml, AuthData: &unknown})
	ldapAccount(env, "alice", "alice@example.com")

	_, appErr := env.l.Synchronize(env.b.rctx(), syncOptions{})
	require.Nil(t, appErr)

	u := env.b.user(samlBob.Id)
	assert.Equal(t, "Builder", u.LastName)
	assert.Equal(t, "bobby", u.Username, "the username of SAML users is not managed by AD/LDAP")
	assert.NotZero(t, env.b.user(samlGhost.Id).DeleteAt)

	// override the SAML bind data with the AD/LDAP ID
	*env.b.cfg.SamlSettings.EnableSyncWithLdapIncludeAuth = true
	*env.b.cfg.SamlSettings.IdAttribute = "employeeId"
	_, appErr = env.l.Synchronize(env.b.rctx(), syncOptions{})
	require.Nil(t, appErr)
	assert.Equal(t, "bob", *env.b.user(samlBob.Id).AuthData)
}

func TestSynchronizeGroups(t *testing.T) {
	env := setupEnv(t)
	alice := ldapAccount(env, "alice", "alice@example.com")
	bob := ldapAccount(env, "bob", "bob@example.com")
	dave := ldapAccount(env, "dave", "dave@example.com")
	addPerson(env.dir, "dave", "dave@example.com", "Dave", "Grohl")

	env.dir.add("cn=devs,ou=groups,dc=example,dc=com", map[string][]string{
		"objectClass": {"groupOfNames"}, "cn": {"Developers"}, "entryUUID": {"g-devs"},
		"member": {"uid=alice,ou=people,dc=example,dc=com", "uid=carol,ou=people,dc=example,dc=com"},
	})
	// nested group, not selected by the group filter
	env.dir.add("cn=contractors,ou=groups,dc=example,dc=com", map[string][]string{
		"objectClass": {"groupOfUniqueNames"}, "cn": {"contractors"}, "entryUUID": {"g-contractors"}, "businessCategory": {"hidden"},
		"uniqueMember": {"UID=Dave, OU=People, DC=Example, DC=Com"},
	})
	env.dir.add("cn=all,ou=groups,dc=example,dc=com", map[string][]string{
		"objectClass": {"groupOfNames"}, "cn": {"Everyone"}, "entryUUID": {"g-all"},
		"member": {"cn=devs,ou=groups,dc=example,dc=com", "uid=bob,ou=people,dc=example,dc=com", "cn=contractors,ou=groups,dc=example,dc=com", "cn=all,ou=groups,dc=example,dc=com"},
	})
	*env.b.cfg.LdapSettings.GroupFilter = "(&(objectClass=groupOfNames)(!(businessCategory=hidden)))"

	devs := env.b.linkGroup("g-devs", "old name")
	all := env.b.linkGroup("g-all", "Everyone")
	removed := env.b.linkGroup("g-removed", "Removed")
	env.b.AddGroupMembers(devs.Id, []string{bob.Id})

	env.b.lastSyncOK = &model.Job{StartAt: 1234}
	stats, appErr := env.l.Synchronize(env.b.rctx(), syncOptions{ReAddRemovedMembers: true})
	require.Nil(t, appErr)

	assert.Equal(t, 2, stats.TotalGroups)
	assert.Equal(t, []string{alice.Id}, env.b.memberIDs(devs.Id))
	assert.ElementsMatch(t, []string{alice.Id, bob.Id, dave.Id}, env.b.memberIDs(all.Id))
	assert.Equal(t, "Developers", env.b.groups[devs.Id].DisplayName)
	assert.NotZero(t, env.b.groups[removed.Id].DeleteAt)
	assert.Equal(t, 1, stats.GroupsDeleted)
	assert.Equal(t, 4, stats.GroupMembersAdded)
	assert.Equal(t, 1, stats.GroupMembersRemove)

	require.Len(t, env.b.defaultMemberships, 1)
	assert.Equal(t, int64(1234), env.b.defaultMemberships[0].Since)
	assert.True(t, env.b.defaultMemberships[0].ReAddRemovedMembers)
	assert.Nil(t, env.b.defaultMemberships[0].ScopedUserID)
	assert.Equal(t, 1, env.b.deleteConstrained)
	require.Len(t, env.b.syncedRoles, 1)
	assert.ElementsMatch(t, []string{devs.Id, all.Id}, env.b.syncedRoles[0])

	// removing a user from the directory removes them from the groups
	env.dir.remove("uid=dave,ou=people,dc=example,dc=com")
	_, appErr = env.l.Synchronize(env.b.rctx(), syncOptions{})
	require.Nil(t, appErr)
	assert.ElementsMatch(t, []string{alice.Id, bob.Id}, env.b.memberIDs(all.Id))
}

func TestRangedMemberRetrieval(t *testing.T) {
	env := setupEnv(t)
	env.dir.rangeSize = 3
	var members []string
	var users []*model.User
	for i := range 10 {
		uid := fmt.Sprintf("user%02d", i)
		addPerson(env.dir, uid, uid+"@example.com", "User", uid)
		members = append(members, fmt.Sprintf("uid=%s,ou=people,dc=example,dc=com", uid))
		users = append(users, ldapAccount(env, uid, uid+"@example.com"))
	}
	env.dir.add("cn=big,ou=groups,dc=example,dc=com", map[string][]string{
		"objectClass": {"group"}, "cn": {"big"}, "entryUUID": {"g-big"}, "member": members,
	})
	big := env.b.linkGroup("g-big", "big")

	_, appErr := env.l.Synchronize(env.b.rctx(), syncOptions{})
	require.Nil(t, appErr)
	var ids []string
	for _, u := range users {
		ids = append(ids, u.Id)
	}
	assert.ElementsMatch(t, ids, env.b.memberIDs(big.Id))
}

func TestGroupsAPI(t *testing.T) {
	env := setupEnv(t)
	for i, name := range []string{"Zeta", "alpha", "Beta", "gamma"} {
		env.dir.add(fmt.Sprintf("cn=%s,ou=groups,dc=example,dc=com", name), map[string][]string{
			"objectClass": {"groupOfNames"}, "cn": {name}, "entryUUID": {fmt.Sprintf("uuid-%d", i)},
		})
	}
	beta := env.b.linkGroup("uuid-2", "Beta")
	env.b.hasSyncables[beta.Id] = true

	groups, total, appErr := env.l.GetAllGroupsPage(env.b.rctx(), 0, 3, model.LdapGroupSearchOpts{})
	require.Nil(t, appErr)
	assert.Equal(t, 4, total)
	require.Len(t, groups, 3)
	assert.Equal(t, []string{"alpha", "Beta", "gamma"}, []string{groups[0].DisplayName, groups[1].DisplayName, groups[2].DisplayName})
	assert.Equal(t, beta.Id, groups[1].Id)
	assert.True(t, groups[1].HasSyncables)
	assert.Empty(t, groups[0].Id)

	groups, total, appErr = env.l.GetAllGroupsPage(env.b.rctx(), 1, 3, model.LdapGroupSearchOpts{})
	require.Nil(t, appErr)
	assert.Equal(t, 4, total)
	require.Len(t, groups, 1)
	assert.Equal(t, "Zeta", groups[0].DisplayName)

	groups, total, appErr = env.l.GetAllGroupsPage(env.b.rctx(), 0, 10, model.LdapGroupSearchOpts{Q: "ETA"})
	require.Nil(t, appErr)
	assert.Equal(t, 2, total)
	assert.Len(t, groups, 2)

	groups, _, appErr = env.l.GetAllGroupsPage(env.b.rctx(), 0, 10, model.LdapGroupSearchOpts{IsLinked: new(false)})
	require.Nil(t, appErr)
	assert.Len(t, groups, 3)

	groups, _, appErr = env.l.GetAllGroupsPage(env.b.rctx(), 0, 10, model.LdapGroupSearchOpts{IsConfigured: new(true)})
	require.Nil(t, appErr)
	require.Len(t, groups, 1)
	assert.Equal(t, "Beta", groups[0].DisplayName)

	group, appErr := env.l.GetGroup(env.b.rctx(), "uuid-3")
	require.Nil(t, appErr)
	assert.Equal(t, "gamma", group.DisplayName)
	assert.Equal(t, "uuid-3", *group.RemoteId)
	assert.Equal(t, model.GroupSourceLdap, group.Source)

	_, appErr = env.l.GetGroup(env.b.rctx(), "nope")
	require.NotNil(t, appErr)
	assert.Equal(t, "ent.ldap_groups.no_rows", appErr.Id)
}
