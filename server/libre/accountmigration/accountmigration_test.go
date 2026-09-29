// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package accountmigration

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

var errNotFound = errors.New("not found")

type fakeBackend struct {
	users     map[string]*model.User
	ldapUsers []*model.User
}

func newFakeBackend() *fakeBackend {
	return &fakeBackend{users: map[string]*model.User{}}
}

func (b *fakeBackend) add(u *model.User) *model.User {
	u.Id = model.NewId()
	b.users[u.Id] = u
	return u
}

func (b *fakeBackend) addLdap(uid, username, email string) {
	id := uid
	b.ldapUsers = append(b.ldapUsers, &model.User{Username: username, Email: email, AuthService: model.UserAuthServiceLdap, AuthData: &id})
}

func (b *fakeBackend) UsersByAuthService(service string) ([]*model.User, error) {
	var res []*model.User
	for _, u := range b.users {
		if u.AuthService == service {
			res = append(res, u.DeepCopy())
		}
	}
	return res, nil
}

func (b *fakeBackend) UserByAuth(authData string, service string) (*model.User, error) {
	for _, u := range b.users {
		if u.AuthService == service && u.AuthData != nil && *u.AuthData == authData {
			return u.DeepCopy(), nil
		}
	}
	return nil, errNotFound
}

func (b *fakeBackend) UserByUsername(username string) (*model.User, error) {
	for _, u := range b.users {
		if u.Username == username {
			return u.DeepCopy(), nil
		}
	}
	return nil, errNotFound
}

func (b *fakeBackend) LdapUsers(_ request.CTX) ([]*model.User, *model.AppError) {
	return b.ldapUsers, nil
}

func (b *fakeBackend) UpdateAuthData(userID string, service string, authData string) error {
	u := b.users[userID]
	u.AuthService = service
	u.AuthData = &authData
	u.Password = ""
	return nil
}

func (b *fakeBackend) UpdateUsername(_ request.CTX, userID string, username string) error {
	b.users[userID].Username = username
	return nil
}

func rctx() request.CTX {
	logger, _ := mlog.NewLogger()
	return request.EmptyContext(logger)
}

func TestMigrateToLdap(t *testing.T) {
	setup := func() (*fakeBackend, *AccountMigration, *model.User, *model.User, *model.User) {
		b := newFakeBackend()
		alice := b.add(&model.User{Username: "alice", Email: "alice@example.com", Password: "x"})
		bob := b.add(&model.User{Username: "bob", Email: "Bob@example.com", Password: "x"})
		stranger := b.add(&model.User{Username: "stranger", Email: "stranger@example.com", Password: "x"})
		b.add(&model.User{Username: "botty", Email: "botty@example.com", IsBot: true})
		b.addLdap("id-alice", "alice", "alice@example.com")
		b.addLdap("id-bob", "robert", "bob@example.com")
		return b, &AccountMigration{b: b}, alice, bob, stranger
	}

	t.Run("by email", func(t *testing.T) {
		b, m, alice, bob, stranger := setup()
		require.Nil(t, m.MigrateToLdap(rctx(), "", "email", false, false))
		assert.Equal(t, "id-alice", *b.users[alice.Id].AuthData)
		assert.Equal(t, model.UserAuthServiceLdap, b.users[bob.Id].AuthService)
		assert.Equal(t, "id-bob", *b.users[bob.Id].AuthData)
		assert.Equal(t, "", b.users[stranger.Id].AuthService)
	})

	t.Run("by username", func(t *testing.T) {
		b, m, alice, bob, _ := setup()
		require.Nil(t, m.MigrateToLdap(rctx(), "", "username", false, false))
		assert.Equal(t, "id-alice", *b.users[alice.Id].AuthData)
		assert.Equal(t, "", b.users[bob.Id].AuthService, "bob's AD/LDAP username differs")
	})

	t.Run("dry run", func(t *testing.T) {
		b, m, alice, _, _ := setup()
		require.Nil(t, m.MigrateToLdap(rctx(), "", "email", false, true))
		assert.Equal(t, "", b.users[alice.Id].AuthService)
	})

	t.Run("duplicates", func(t *testing.T) {
		b, m, alice, bob, _ := setup()
		b.addLdap("id-alice2", "alice2", "alice@example.com")
		appErr := m.MigrateToLdap(rctx(), "", "email", false, false)
		require.NotNil(t, appErr)
		assert.Equal(t, "ent.migration.migratetoldap.duplicate_field", appErr.Id)
		assert.Equal(t, "", b.users[alice.Id].AuthService)

		require.Nil(t, m.MigrateToLdap(rctx(), "", "email", true, false))
		assert.Equal(t, "", b.users[alice.Id].AuthService, "duplicates are skipped with force")
		assert.Equal(t, "id-bob", *b.users[bob.Id].AuthData)
	})

	t.Run("invalid match field", func(t *testing.T) {
		_, m, _, _, _ := setup()
		require.NotNil(t, m.MigrateToLdap(rctx(), "", "nickname", false, false))
	})

	t.Run("AD/LDAP account already bound", func(t *testing.T) {
		b, m, alice, _, _ := setup()
		id := "id-alice"
		b.add(&model.User{Username: "alice-ldap", Email: "other@example.com", AuthService: model.UserAuthServiceLdap, AuthData: &id})
		require.Nil(t, m.MigrateToLdap(rctx(), "", "email", false, false))
		assert.Equal(t, "", b.users[alice.Id].AuthService)
	})
}

func TestMigrateToSaml(t *testing.T) {
	setup := func() (*fakeBackend, *AccountMigration, *model.User, *model.User) {
		b := newFakeBackend()
		alice := b.add(&model.User{Username: "alice", Email: "alice@example.com"})
		bob := b.add(&model.User{Username: "bob", Email: "bob@example.com"})
		b.add(&model.User{Username: "carol", Email: "carol@example.com"})
		return b, &AccountMigration{b: b}, alice, bob
	}

	t.Run("auto", func(t *testing.T) {
		b, m, alice, bob := setup()
		require.Nil(t, m.MigrateToSaml(rctx(), "", nil, true, false))
		assert.Equal(t, model.UserAuthServiceSaml, b.users[alice.Id].AuthService)
		assert.Equal(t, "alice@example.com", *b.users[alice.Id].AuthData)
		assert.Equal(t, model.UserAuthServiceSaml, b.users[bob.Id].AuthService)
	})

	t.Run("with users file", func(t *testing.T) {
		b, m, alice, bob := setup()
		usersMap := map[string]string{
			"Alice@example.com": "alice.saml",
			"bob@example.com":   "carol", // username taken
		}
		require.Nil(t, m.MigrateToSaml(rctx(), "", usersMap, false, false))
		assert.Equal(t, model.UserAuthServiceSaml, b.users[alice.Id].AuthService)
		assert.Equal(t, "alice.saml", b.users[alice.Id].Username)
		assert.Equal(t, "", b.users[bob.Id].AuthService)
	})

	t.Run("email already bound", func(t *testing.T) {
		b, m, alice, _ := setup()
		email := "alice@example.com"
		b.add(&model.User{Username: "alice-saml", Email: "x@example.com", AuthService: model.UserAuthServiceSaml, AuthData: &email})
		require.Nil(t, m.MigrateToSaml(rctx(), "", nil, true, false))
		assert.Equal(t, "", b.users[alice.Id].AuthService)
	})

	t.Run("dry run", func(t *testing.T) {
		b, m, alice, _ := setup()
		require.Nil(t, m.MigrateToSaml(rctx(), "", nil, true, true))
		assert.Equal(t, "", b.users[alice.Id].AuthService)
	})
}
