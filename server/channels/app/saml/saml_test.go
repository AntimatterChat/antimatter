// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package saml

import (
	"bytes"
	"compress/flate"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/xml"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	saml2 "github.com/mattermost/gosaml2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/einterfaces/mocks"
)

type testEnv struct {
	t   *testing.T
	b   *fakeBackend
	s   *Service
	idp *keyPair
	sp  *keyPair
}

func newTestEnv(t *testing.T) *testEnv {
	b := newFakeBackend()
	idp := validKeyPair(t, "idp")
	sp := validKeyPair(t, "sp")
	b.files[testIdpCertificateName] = idp.certPEM
	b.files[testPublicCertificateName] = sp.certPEM
	b.files[testPrivateKeyName] = sp.keyPEM
	*b.cfg.SamlSettings.PublicCertificateFile = testPublicCertificateName
	*b.cfg.SamlSettings.PrivateKeyFile = testPrivateKeyName
	return &testEnv{t: t, b: b, s: newService(b), idp: idp, sp: sp}
}

func (e *testEnv) rctx() request.CTX { return request.TestContext(e.t) }

func defaultAttrs() map[string][]string {
	return map[string][]string{
		"mail":      {"Alice@Example.com"},
		"uid":       {"alice"},
		"objectId":  {"obj-alice"},
		"givenName": {"Alice"},
		"sn":        {"Liddell"},
		"nick":      {"ali"},
		"title":     {"Engineer"},
		"lang":      {"fr"},
	}
}

func (e *testEnv) login(o *responseOptions, relay map[string]string) (*model.User, *model.AppError) {
	if relay == nil {
		relay = map[string]string{}
	}
	u, _, _, appErr := e.s.DoLogin(e.rctx(), buildResponse(e.t, o), relay)
	return u, appErr
}

func TestConfigureSP(t *testing.T) {
	t.Run("disabled", func(t *testing.T) {
		e := newTestEnv(t)
		*e.b.cfg.SamlSettings.Enable = false
		delete(e.b.files, testIdpCertificateName)
		require.NoError(t, e.s.ConfigureSP(e.rctx()))

		_, appErr := e.s.BuildRequest(e.rctx(), "")
		require.NotNil(t, appErr)
		assert.Equal(t, "ent.saml.service_disable.app_error", appErr.Id)
	})

	t.Run("valid", func(t *testing.T) {
		e := newTestEnv(t)
		require.NoError(t, e.s.ConfigureSP(e.rctx()))
	})

	t.Run("missing idp certificate", func(t *testing.T) {
		e := newTestEnv(t)
		delete(e.b.files, testIdpCertificateName)
		err := e.s.ConfigureSP(e.rctx())
		require.Error(t, err)
		assert.Equal(t, "ent.saml.configure.certificate_parse_error.app_error", err.(*model.AppError).Id)
	})

	t.Run("invalid idp certificate", func(t *testing.T) {
		e := newTestEnv(t)
		e.b.files[testIdpCertificateName] = []byte("garbage")
		require.Error(t, e.s.ConfigureSP(e.rctx()))
	})

	t.Run("encryption with mismatched key", func(t *testing.T) {
		e := newTestEnv(t)
		*e.b.cfg.SamlSettings.Encrypt = true
		e.b.files[testPrivateKeyName] = validKeyPair(t, "other").keyPEM
		err := e.s.ConfigureSP(e.rctx())
		require.Error(t, err)
		assert.Equal(t, "ent.saml.configure.load_private_key.app_error", err.(*model.AppError).Id)
	})

	t.Run("encryption with missing key", func(t *testing.T) {
		e := newTestEnv(t)
		*e.b.cfg.SamlSettings.Encrypt = true
		delete(e.b.files, testPrivateKeyName)
		err := e.s.ConfigureSP(e.rctx())
		require.Error(t, err)
		assert.Equal(t, "ent.saml.configure.load_private_key.app_error", err.(*model.AppError).Id)
	})

	t.Run("idp certificate replaced under the same name is picked up", func(t *testing.T) {
		e := newTestEnv(t)
		require.NoError(t, e.s.ConfigureSP(e.rctx()))

		newIdP := validKeyPair(t, "idp2")
		e.b.files[testIdpCertificateName] = newIdP.certPEM

		_, appErr := e.login(defaultResponseOptions(newIdP, defaultAttrs()), nil)
		require.Nil(t, appErr)
	})
}

func decodeRedirectRequest(t *testing.T, redirect string) (*url.URL, string) {
	t.Helper()
	u, err := url.Parse(redirect)
	require.NoError(t, err)
	raw, err := base64.StdEncoding.DecodeString(u.Query().Get("SAMLRequest"))
	require.NoError(t, err)
	xmlBytes, err := io.ReadAll(flate.NewReader(bytes.NewReader(raw)))
	require.NoError(t, err)
	return u, string(xmlBytes)
}

func TestBuildRequest(t *testing.T) {
	t.Run("unsigned", func(t *testing.T) {
		e := newTestEnv(t)
		*e.b.cfg.SamlSettings.ScopingIDPProviderId = "https://scoped.example.com"
		*e.b.cfg.SamlSettings.ScopingIDPName = "Scoped"

		req, appErr := e.s.BuildRequest(e.rctx(), "relay-123")
		require.Nil(t, appErr)
		assert.Equal(t, "relay-123", req.RelayState)
		require.NotEmpty(t, req.Base64AuthRequest)

		u, reqXML := decodeRedirectRequest(t, req.URL)
		assert.Equal(t, "idp.example.com", u.Host)
		assert.Equal(t, "/sso", u.Path)
		assert.Equal(t, "relay-123", u.Query().Get("RelayState"))
		assert.Empty(t, u.Query().Get("Signature"))
		assert.Contains(t, reqXML, testSPID)
		assert.Contains(t, reqXML, `AssertionConsumerServiceURL="`+testACSURL+`"`)
		assert.Contains(t, reqXML, `Destination="`+testIdPURL+`"`)
		assert.Contains(t, reqXML, `ProviderID="https://scoped.example.com"`)
		assert.Contains(t, reqXML, `Name="Scoped"`)

		decoded, err := base64.StdEncoding.DecodeString(req.Base64AuthRequest)
		require.NoError(t, err)
		assert.Contains(t, string(decoded), "AuthnRequest")
	})

	t.Run("acs url defaults to site url", func(t *testing.T) {
		e := newTestEnv(t)
		*e.b.cfg.SamlSettings.AssertionConsumerServiceURL = ""
		e.b.siteURL = "https://chat.example.org/"
		req, appErr := e.s.BuildRequest(e.rctx(), "")
		require.Nil(t, appErr)
		_, reqXML := decodeRedirectRequest(t, req.URL)
		assert.Contains(t, reqXML, `AssertionConsumerServiceURL="https://chat.example.org/login/sso/saml"`)
	})

	t.Run("signed", func(t *testing.T) {
		e := newTestEnv(t)
		*e.b.cfg.SamlSettings.Encrypt = true
		*e.b.cfg.SamlSettings.SignRequest = true
		*e.b.cfg.SamlSettings.SignatureAlgorithm = model.SamlSettingsSignatureAlgorithmSha256

		req, appErr := e.s.BuildRequest(e.rctx(), "relay")
		require.Nil(t, appErr)
		u, _ := decodeRedirectRequest(t, req.URL)
		q := u.Query()
		assert.Equal(t, "http://www.w3.org/2001/04/xmldsig-more#rsa-sha256", q.Get("SigAlg"))
		sig, err := base64.StdEncoding.DecodeString(q.Get("Signature"))
		require.NoError(t, err)

		signed := "SAMLRequest=" + url.QueryEscape(q.Get("SAMLRequest")) +
			"&RelayState=" + url.QueryEscape(q.Get("RelayState")) +
			"&SigAlg=" + url.QueryEscape(q.Get("SigAlg"))
		digest := sha256.Sum256([]byte(signed))
		require.NoError(t, rsa.VerifyPKCS1v15(&e.sp.key.PublicKey, crypto.SHA256, digest[:], sig))
	})

	t.Run("signing without key", func(t *testing.T) {
		e := newTestEnv(t)
		*e.b.cfg.SamlSettings.SignRequest = true
		delete(e.b.files, testPrivateKeyName)
		_, appErr := e.s.BuildRequest(e.rctx(), "")
		require.NotNil(t, appErr)
		assert.Equal(t, "ent.saml.configure.encryption_not_enabled.app_error", appErr.Id)
	})
}

func TestGetMetadata(t *testing.T) {
	e := newTestEnv(t)
	*e.b.cfg.SamlSettings.Encrypt = true
	*e.b.cfg.SamlSettings.SignRequest = true

	md, appErr := e.s.GetMetadata(e.rctx())
	require.Nil(t, appErr)
	assert.True(t, strings.HasPrefix(md, "<?xml"))
	assert.Contains(t, md, `entityID="`+testSPID+`"`)
	assert.Contains(t, md, `Location="`+testACSURL+`"`)
	assert.Contains(t, md, `AuthnRequestsSigned="true"`)
	assert.Contains(t, md, `WantAssertionsSigned="true"`)
	assert.Contains(t, md, `use="encryption"`)
	assert.Contains(t, md, `use="signing"`)
	assert.Contains(t, md, base64.StdEncoding.EncodeToString(e.sp.cert.Raw))

	var parsed struct {
		XMLName  xml.Name
		EntityID string `xml:"entityID,attr"`
	}
	require.NoError(t, xml.Unmarshal([]byte(md), &parsed))
	assert.Equal(t, testSPID, parsed.EntityID)

	t.Run("works without idp configuration and without keys", func(t *testing.T) {
		e := newTestEnv(t)
		*e.b.cfg.SamlSettings.Enable = false
		*e.b.cfg.SamlSettings.IdpURL = ""
		delete(e.b.files, testIdpCertificateName)
		md, appErr := e.s.GetMetadata(e.rctx())
		require.Nil(t, appErr)
		assert.NotContains(t, md, "KeyDescriptor")
	})

	t.Run("missing sp identifier", func(t *testing.T) {
		e := newTestEnv(t)
		*e.b.cfg.SamlSettings.ServiceProviderIdentifier = ""
		_, appErr := e.s.GetMetadata(e.rctx())
		require.NotNil(t, appErr)
		assert.Equal(t, "ent.saml.metadata.app_error", appErr.Id)
	})
}

func TestDoLoginCreatesUser(t *testing.T) {
	e := newTestEnv(t)
	user, info, created, appErr := e.s.DoLogin(e.rctx(), buildResponse(t, defaultResponseOptions(e.idp, defaultAttrs())), map[string]string{})
	require.Nil(t, appErr)
	require.NotNil(t, info)
	assert.Equal(t, "nameid-123", info.NameID)

	assert.Equal(t, "alice@example.com", user.Email)
	assert.Equal(t, "alice", user.Username)
	assert.Equal(t, "Alice", user.FirstName)
	assert.Equal(t, "Liddell", user.LastName)
	assert.Equal(t, "ali", user.Nickname)
	assert.Equal(t, "Engineer", user.Position)
	assert.Equal(t, "fr", user.Locale)
	assert.Equal(t, model.UserAuthServiceSaml, user.AuthService)
	assert.Equal(t, "obj-alice", *user.AuthData)
	assert.True(t, user.EmailVerified)
	assert.False(t, user.IsGuest())
	assert.False(t, user.IsSystemAdmin())
	assert.Len(t, e.b.users, 1)
	assert.True(t, created, "the first sign-in creates the account")

	// The next sign-in finds the account.
	again, _, created, appErr := e.s.DoLogin(e.rctx(), buildResponse(t, defaultResponseOptions(e.idp, defaultAttrs())), map[string]string{})
	require.Nil(t, appErr)
	assert.Equal(t, user.Id, again.Id)
	assert.False(t, created)
}

func TestDoLoginReportsNoUserCreatedOnFailure(t *testing.T) {
	e := newTestEnv(t)
	*e.b.cfg.GuestAccountsSettings.Enable = false
	*e.b.cfg.SamlSettings.GuestAttribute = "role=guest"
	attrs := defaultAttrs()
	attrs["role"] = []string{"guest"}

	user, _, created, appErr := e.s.DoLogin(e.rctx(), buildResponse(t, defaultResponseOptions(e.idp, attrs)), map[string]string{})
	require.NotNil(t, appErr)
	assert.Nil(t, user)
	assert.False(t, created)
	assert.Empty(t, e.b.users, "a refused guest gets no account")
}

func TestDoLoginUpdatesUser(t *testing.T) {
	e := newTestEnv(t)
	existing := e.b.addUser(&model.User{
		Email: "alice@example.com", Username: "alice", FirstName: "Old", LastName: "Name",
		Position: "Intern", AuthService: model.UserAuthServiceSaml, AuthData: model.NewPointer("obj-alice"),
	})

	attrs := defaultAttrs()
	attrs["mail"] = []string{"alice.new@example.com"}
	attrs["uid"] = []string{"alice.new"}
	delete(attrs, "nick")

	user, appErr := e.login(defaultResponseOptions(e.idp, attrs), nil)
	require.Nil(t, appErr)
	assert.Equal(t, existing.Id, user.Id)
	assert.Equal(t, "alice.new@example.com", user.Email)
	assert.Equal(t, "alice.new", user.Username)
	assert.Equal(t, "Alice", user.FirstName)
	assert.Equal(t, "Engineer", user.Position)
	assert.Equal(t, "", user.Nickname, "absent attributes leave the field unchanged")
	assert.Len(t, e.b.users, 1)

	t.Run("does not steal the username of another user", func(t *testing.T) {
		e.b.addUser(&model.User{Email: "bob@example.com", Username: "bob"})
		attrs["uid"] = []string{"bob"}
		user, appErr := e.login(defaultResponseOptions(e.idp, attrs), nil)
		require.Nil(t, appErr)
		assert.Equal(t, "alice.new", user.Username)
	})
}

func TestDoLoginEmailBindingMigration(t *testing.T) {
	e := newTestEnv(t)
	existing := e.b.addUser(&model.User{
		Email: "alice@example.com", Username: "alice",
		AuthService: model.UserAuthServiceSaml, AuthData: model.NewPointer("alice@example.com"),
	})

	user, appErr := e.login(defaultResponseOptions(e.idp, defaultAttrs()), nil)
	require.Nil(t, appErr)
	assert.Equal(t, existing.Id, user.Id)
	assert.Equal(t, "obj-alice", *user.AuthData)

	t.Run("email binding when no id attribute", func(t *testing.T) {
		e := newTestEnv(t)
		*e.b.cfg.SamlSettings.IdAttribute = ""
		user, appErr := e.login(defaultResponseOptions(e.idp, defaultAttrs()), nil)
		require.Nil(t, appErr)
		assert.Equal(t, "alice@example.com", *user.AuthData)
	})
}

func TestDoLoginValidation(t *testing.T) {
	cases := []struct {
		name   string
		modify func(e *testEnv, o *responseOptions)
		errID  string
	}{
		{"empty response", nil, "ent.saml.do_login.empty_response.app_error"},
		{"unsigned", func(e *testEnv, o *responseOptions) { o.signAssertion = false }, "ent.saml.do_login.invalid_signature.app_error"},
		{"signed by another key", func(e *testEnv, o *responseOptions) { o.signer = validKeyPair(e.t, "evil") }, "ent.saml.do_login.invalid_signature.app_error"},
		{"expired", func(e *testEnv, o *responseOptions) {
			o.notBefore = time.Now().Add(-time.Hour)
			o.notOnOrAfter = time.Now().Add(-30 * time.Minute)
		}, "ent.saml.do_login.invalid_time.app_error"},
		{"not yet valid", func(e *testEnv, o *responseOptions) {
			o.notBefore = time.Now().Add(30 * time.Minute)
		}, "ent.saml.do_login.invalid_time.app_error"},
		{"wrong audience", func(e *testEnv, o *responseOptions) { o.audience = "https://other.example.com" }, "ent.saml.do_login.parse.app_error"},
		{"wrong issuer", func(e *testEnv, o *responseOptions) { o.issuer = "https://evil.example.com" }, "ent.saml.do_login.parse.app_error"},
		{"wrong recipient", func(e *testEnv, o *responseOptions) { o.recipient = "https://evil.example.com/acs" }, "ent.saml.do_login.parse.app_error"},
		{"missing email", func(e *testEnv, o *responseOptions) { delete(o.attributes, "mail") }, "ent.saml.attribute.app_error"},
		{"invalid email", func(e *testEnv, o *responseOptions) { o.attributes["mail"] = []string{"not-an-email"} }, "ent.saml.attribute.app_error"},
		{"missing username", func(e *testEnv, o *responseOptions) { delete(o.attributes, "uid") }, "ent.saml.attribute.app_error"},
		{"missing id", func(e *testEnv, o *responseOptions) { delete(o.attributes, "objectId") }, "ent.saml.attribute.app_error"},
		{"no attribute statement", func(e *testEnv, o *responseOptions) { o.omitAttributes = true }, "ent.saml.do_login.parse.app_error"},
		{"encryption required", func(e *testEnv, o *responseOptions) {
			*e.b.cfg.SamlSettings.Encrypt = true
		}, "ent.saml.configure.not_encrypted_response.app_error"},
		{"email of a non saml user", func(e *testEnv, o *responseOptions) {
			e.b.addUser(&model.User{Email: "alice@example.com", Username: "someone"})
		}, "ent.saml.save_user.email_exists.saml_app_error"},
		{"username taken", func(e *testEnv, o *responseOptions) {
			e.b.addUser(&model.User{Email: "other@example.com", Username: "alice"})
		}, "ent.saml.save_user.username_exists.saml_app_error"},
		{"user creation disabled", func(e *testEnv, o *responseOptions) {
			*e.b.cfg.TeamSettings.EnableUserCreation = false
		}, "api.user.create_user.disabled.app_error"},
		{"guest while guest accounts disabled", func(e *testEnv, o *responseOptions) {
			*e.b.cfg.SamlSettings.GuestAttribute = "role=guest"
			*e.b.cfg.GuestAccountsSettings.Enable = false
			o.attributes["role"] = []string{"guest"}
		}, "api.user.login.guest_accounts.disabled.error"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newTestEnv(t)
			o := defaultResponseOptions(e.idp, defaultAttrs())
			if tc.modify == nil {
				_, _, _, appErr := e.s.DoLogin(e.rctx(), "  ", map[string]string{})
				require.NotNil(t, appErr)
				assert.Equal(t, tc.errID, appErr.Id)
				return
			}
			tc.modify(e, o)
			_, appErr := e.login(o, nil)
			require.NotNil(t, appErr)
			assert.Equal(t, tc.errID, appErr.Id, appErr.Error())
		})
	}
}

func TestDoLoginSignatureVariants(t *testing.T) {
	t.Run("signed response, unsigned assertion", func(t *testing.T) {
		e := newTestEnv(t)
		o := defaultResponseOptions(e.idp, defaultAttrs())
		o.signAssertion = false
		o.signResponse = true
		_, appErr := e.login(o, nil)
		require.Nil(t, appErr)
	})

	t.Run("verification disabled accepts unsigned", func(t *testing.T) {
		e := newTestEnv(t)
		*e.b.cfg.SamlSettings.Verify = false
		o := defaultResponseOptions(e.idp, defaultAttrs())
		o.signAssertion = false
		_, appErr := e.login(o, nil)
		require.Nil(t, appErr)
	})

	t.Run("multiple idp certificates (rollover)", func(t *testing.T) {
		e := newTestEnv(t)
		next := validKeyPair(t, "idp-next")
		e.b.files[testIdpCertificateName] = append(append([]byte{}, e.idp.certPEM...), next.certPEM...)
		_, appErr := e.login(defaultResponseOptions(next, defaultAttrs()), nil)
		require.Nil(t, appErr)
	})
}

func TestDoLoginEncrypted(t *testing.T) {
	e := newTestEnv(t)
	*e.b.cfg.SamlSettings.Encrypt = true
	o := defaultResponseOptions(e.idp, defaultAttrs())
	o.encryptFor = e.sp
	user, appErr := e.login(o, nil)
	require.Nil(t, appErr)
	assert.Equal(t, "alice@example.com", user.Email)

	t.Run("encrypted for another key", func(t *testing.T) {
		e := newTestEnv(t)
		*e.b.cfg.SamlSettings.Encrypt = true
		o := defaultResponseOptions(e.idp, defaultAttrs())
		o.encryptFor = validKeyPair(t, "other-sp")
		_, appErr := e.login(o, nil)
		require.NotNil(t, appErr)
	})
}

func TestDoLoginReplay(t *testing.T) {
	e := newTestEnv(t)
	encoded := buildResponse(t, defaultResponseOptions(e.idp, defaultAttrs()))
	_, _, _, appErr := e.s.DoLogin(e.rctx(), encoded, map[string]string{})
	require.Nil(t, appErr)
	_, _, _, appErr = e.s.DoLogin(e.rctx(), encoded, map[string]string{})
	require.NotNil(t, appErr)
	assert.Equal(t, "ent.saml.do_login.parse.app_error", appErr.Id)
}

func TestDoLoginRoles(t *testing.T) {
	t.Run("guest attribute creates a guest", func(t *testing.T) {
		e := newTestEnv(t)
		*e.b.cfg.SamlSettings.GuestAttribute = "role=guest"
		attrs := defaultAttrs()
		attrs["role"] = []string{"staff", "guest"}
		user, appErr := e.login(defaultResponseOptions(e.idp, attrs), nil)
		require.Nil(t, appErr)
		assert.True(t, user.IsGuest())
	})

	t.Run("guest attribute demotes an existing member", func(t *testing.T) {
		e := newTestEnv(t)
		*e.b.cfg.SamlSettings.GuestAttribute = "role=guest"
		e.b.addUser(&model.User{Email: "alice@example.com", Username: "alice", AuthService: model.UserAuthServiceSaml, AuthData: model.NewPointer("obj-alice")})
		attrs := defaultAttrs()
		attrs["role"] = []string{"guest"}
		user, appErr := e.login(defaultResponseOptions(e.idp, attrs), nil)
		require.Nil(t, appErr)
		assert.True(t, user.IsGuest())
	})

	t.Run("guests are not promoted automatically", func(t *testing.T) {
		e := newTestEnv(t)
		*e.b.cfg.SamlSettings.GuestAttribute = "role=guest"
		e.b.addUser(&model.User{Email: "alice@example.com", Username: "alice", Roles: model.SystemGuestRoleId, AuthService: model.UserAuthServiceSaml, AuthData: model.NewPointer("obj-alice")})
		user, appErr := e.login(defaultResponseOptions(e.idp, defaultAttrs()), nil)
		require.Nil(t, appErr)
		assert.True(t, user.IsGuest())
	})

	t.Run("admin attribute promotes and demotes", func(t *testing.T) {
		e := newTestEnv(t)
		*e.b.cfg.SamlSettings.EnableAdminAttribute = true
		*e.b.cfg.SamlSettings.AdminAttribute = "group=admins"
		attrs := defaultAttrs()
		attrs["group"] = []string{"admins"}
		user, appErr := e.login(defaultResponseOptions(e.idp, attrs), nil)
		require.Nil(t, appErr)
		assert.True(t, user.IsSystemAdmin())

		attrs["group"] = []string{"users"}
		user, appErr = e.login(defaultResponseOptions(e.idp, attrs), nil)
		require.Nil(t, appErr)
		assert.False(t, user.IsSystemAdmin())
		assert.Equal(t, model.SystemUserRoleId, user.Roles)
	})

	t.Run("admin attribute ignored when disabled", func(t *testing.T) {
		e := newTestEnv(t)
		*e.b.cfg.SamlSettings.EnableAdminAttribute = false
		*e.b.cfg.SamlSettings.AdminAttribute = "group=admins"
		e.b.addUser(&model.User{Email: "alice@example.com", Username: "alice", Roles: "system_user system_admin", AuthService: model.UserAuthServiceSaml, AuthData: model.NewPointer("obj-alice")})
		user, appErr := e.login(defaultResponseOptions(e.idp, defaultAttrs()), nil)
		require.Nil(t, appErr)
		assert.True(t, user.IsSystemAdmin())
	})
}

func TestDoLoginEmailToSaml(t *testing.T) {
	e := newTestEnv(t)
	existing := e.b.addUser(&model.User{Email: "alice@example.com", Username: "alice", Password: "hash"})
	token := model.NewToken(model.TokenTypeSaml, "alice@example.com")
	e.b.tokens[token.Token] = token

	relay := map[string]string{"action": model.OAuthActionEmailToSSO, "email_token": token.Token}
	user, appErr := e.login(defaultResponseOptions(e.idp, defaultAttrs()), relay)
	require.Nil(t, appErr)
	assert.Equal(t, existing.Id, user.Id)
	assert.Equal(t, model.UserAuthServiceSaml, user.AuthService)
	assert.Equal(t, "obj-alice", *user.AuthData)
	assert.Empty(t, user.Password)
	assert.Empty(t, e.b.tokens, "token must be consumed")

	t.Run("invalid token", func(t *testing.T) {
		e := newTestEnv(t)
		e.b.addUser(&model.User{Email: "alice@example.com", Username: "alice"})
		relay := map[string]string{"action": model.OAuthActionEmailToSSO, "email_token": "bogus"}
		_, appErr := e.login(defaultResponseOptions(e.idp, defaultAttrs()), relay)
		require.NotNil(t, appErr)
		assert.Equal(t, "api.saml.invalid_email_token.app_error", appErr.Id)
	})

	t.Run("expired token", func(t *testing.T) {
		e := newTestEnv(t)
		e.b.addUser(&model.User{Email: "alice@example.com", Username: "alice"})
		token := model.NewToken(model.TokenTypeSaml, "alice@example.com")
		token.CreateAt = model.GetMillis() - model.MaxTokenExipryTime - 1000
		e.b.tokens[token.Token] = token
		relay := map[string]string{"action": model.OAuthActionEmailToSSO, "email_token": token.Token}
		_, appErr := e.login(defaultResponseOptions(e.idp, defaultAttrs()), relay)
		require.NotNil(t, appErr)
		assert.Equal(t, "api.saml.invalid_email_token.app_error", appErr.Id)
	})
}

func TestDoLoginSyncWithLdap(t *testing.T) {
	t.Run("user missing from ldap", func(t *testing.T) {
		e := newTestEnv(t)
		*e.b.cfg.SamlSettings.EnableSyncWithLdap = true
		*e.b.cfg.LdapSettings.EnableSync = true
		ldap := &mocks.LdapInterface{}
		ldap.On("GetLDAPUserForMMUser", mock.Anything, mock.Anything).
			Return(nil, "", model.NewAppError("x", "ent.ldap.do_login.user_not_registered.app_error", nil, "", 404))
		e.b.ldap = ldap
		_, appErr := e.login(defaultResponseOptions(e.idp, defaultAttrs()), nil)
		require.NotNil(t, appErr)
		assert.Equal(t, "ent.saml.login.ldap_user_missing", appErr.Id)
	})

	t.Run("include auth overrides the binding and skips attribute updates", func(t *testing.T) {
		e := newTestEnv(t)
		*e.b.cfg.SamlSettings.EnableSyncWithLdap = true
		*e.b.cfg.LdapSettings.EnableSync = true
		*e.b.cfg.SamlSettings.EnableSyncWithLdapIncludeAuth = true
		ldap := &mocks.LdapInterface{}
		ldap.On("GetLDAPUserForMMUser", mock.Anything, mock.MatchedBy(func(u *model.User) bool {
			return u.Email == "alice@example.com"
		})).Return(&model.User{Email: "alice@example.com", AuthData: model.NewPointer("ldap-alice")}, "cn=alice", nil)
		e.b.ldap = ldap

		existing := e.b.addUser(&model.User{Email: "alice@example.com", Username: "alice", FirstName: "FromLdap",
			AuthService: model.UserAuthServiceSaml, AuthData: model.NewPointer("obj-alice")})

		user, appErr := e.login(defaultResponseOptions(e.idp, defaultAttrs()), nil)
		require.Nil(t, appErr)
		assert.Equal(t, existing.Id, user.Id)
		assert.Equal(t, "ldap-alice", *user.AuthData)
		assert.Equal(t, "FromLdap", user.FirstName)
	})

	t.Run("guests ignored for ldap sync", func(t *testing.T) {
		e := newTestEnv(t)
		*e.b.cfg.SamlSettings.EnableSyncWithLdap = true
		*e.b.cfg.LdapSettings.EnableSync = true
		*e.b.cfg.SamlSettings.IgnoreGuestsLdapSync = true
		*e.b.cfg.SamlSettings.GuestAttribute = "role=guest"
		e.b.ldap = &mocks.LdapInterface{} // any call would panic
		attrs := defaultAttrs()
		attrs["role"] = []string{"guest"}
		user, appErr := e.login(defaultResponseOptions(e.idp, attrs), nil)
		require.Nil(t, appErr)
		assert.True(t, user.IsGuest())
	})
}

func TestDoLoginCustomProfileAttributes(t *testing.T) {
	t.Run("hands every value of the synced attributes to the syncer and prunes", func(t *testing.T) {
		e := newTestEnv(t)
		syncer := newFakeAttributeSyncer("department", "office", "groups")
		e.b.attrSyncer = syncer

		attrs := defaultAttrs()
		attrs["department"] = []string{"R&D"}
		attrs["office"] = []string{"Paris"}
		attrs["groups"] = []string{"Sales", "Support"}
		attrs["unmapped"] = []string{"ignored"}
		user, appErr := e.login(defaultResponseOptions(e.idp, attrs), nil)
		require.Nil(t, appErr)

		assert.Equal(t, map[string][]string{
			"department": {"R&D"},
			"office":     {"Paris"},
			"groups":     {"Sales", "Support"},
		}, syncer.synced[user.Id], "select and multiselect attributes get every value, not only the first")
		assert.True(t, syncer.syncOpts[user.Id].PruneOrphanedOptions)

		// Office removed from the assertion, department changed: the missing
		// attribute is left out, which clears the user's value.
		attrs["department"] = []string{"Sales"}
		delete(attrs, "office")
		user, appErr = e.login(defaultResponseOptions(e.idp, attrs), nil)
		require.Nil(t, appErr)
		assert.Equal(t, map[string][]string{
			"department": {"Sales"},
			"groups":     {"Sales", "Support"},
		}, syncer.synced[user.Id])
	})

	t.Run("nothing is synced when no attribute is synced from SAML", func(t *testing.T) {
		e := newTestEnv(t)
		syncer := newFakeAttributeSyncer()
		e.b.attrSyncer = syncer
		_, appErr := e.login(defaultResponseOptions(e.idp, defaultAttrs()), nil)
		require.Nil(t, appErr)
		assert.Zero(t, syncer.syncCalls)
	})

	t.Run("a sync failure does not fail the sign-in", func(t *testing.T) {
		e := newTestEnv(t)
		syncer := newFakeAttributeSyncer("department")
		syncer.syncErr = model.NewAppError("test", "app.property_value.search.app_error", nil, "", http.StatusInternalServerError)
		e.b.attrSyncer = syncer
		user, appErr := e.login(defaultResponseOptions(e.idp, defaultAttrs()), nil)
		require.Nil(t, appErr)
		assert.Equal(t, "alice", user.Username)
		assert.Equal(t, 1, syncer.syncCalls)

		e.b.attrSyncerErr = model.NewAppError("test", "app.property_group.get.app_error", nil, "", http.StatusInternalServerError)
		_, appErr = e.login(defaultResponseOptions(e.idp, defaultAttrs()), nil)
		require.Nil(t, appErr)
	})

	t.Run("an assertion without attribute statement leaves the values alone", func(t *testing.T) {
		e := newTestEnv(t)
		syncer := newFakeAttributeSyncer("department")
		e.b.attrSyncer = syncer
		appErr := e.s.syncCustomProfileAttributes(e.rctx(), &model.User{Id: model.NewId()}, &saml2.AssertionInfo{})
		require.NotNil(t, appErr)
		assert.Equal(t, "ent.saml.update_cpa.empty_attribute_statement", appErr.Id)
		assert.Zero(t, syncer.syncCalls, "no attribute statement must not clear every synced value")
	})
}

func TestCheckProviderAttributes(t *testing.T) {
	e := newTestEnv(t)
	ss := &e.b.cfg.SamlSettings
	*ss.NicknameAttribute = ""
	user := &model.User{Username: "alice", Email: "alice@example.com", FirstName: "Alice", Nickname: "ali", Locale: "en"}

	assert.Equal(t, "", e.s.CheckProviderAttributes(e.rctx(), ss, user, &model.UserPatch{}))
	assert.Equal(t, "", e.s.CheckProviderAttributes(e.rctx(), ss, user, &model.UserPatch{FirstName: model.NewPointer("Alice")}))
	assert.Equal(t, "first name", e.s.CheckProviderAttributes(e.rctx(), ss, user, &model.UserPatch{FirstName: model.NewPointer("Bob")}))
	assert.Equal(t, "email", e.s.CheckProviderAttributes(e.rctx(), ss, user, &model.UserPatch{Email: model.NewPointer("b@example.com")}))
	assert.Equal(t, "username", e.s.CheckProviderAttributes(e.rctx(), ss, user, &model.UserPatch{Username: model.NewPointer("bob")}))
	assert.Equal(t, "", e.s.CheckProviderAttributes(e.rctx(), ss, user, &model.UserPatch{Nickname: model.NewPointer("x")}))
	assert.Equal(t, "locale", e.s.CheckProviderAttributes(e.rctx(), ss, user, &model.UserPatch{Locale: model.NewPointer("fr")}))
}

func TestDiagnostic(t *testing.T) {
	e := newTestEnv(t)
	d := NewDiagnostic(e.b, func() string { return testSiteURL })

	t.Run("ok", func(t *testing.T) {
		require.NoError(t, d.RunSupportPacketTest(e.rctx(), e.b.cfg.SamlSettings))
	})

	t.Run("signed and encrypted ok", func(t *testing.T) {
		ss := cloneSamlSettings(t, e.b.cfg.SamlSettings)
		*ss.Encrypt = true
		*ss.SignRequest = true
		require.NoError(t, d.RunSupportPacketTest(e.rctx(), *ss))
	})

	t.Run("missing certificate", func(t *testing.T) {
		ss := cloneSamlSettings(t, e.b.cfg.SamlSettings)
		*ss.IdpCertificateFile = "missing.crt"
		err := d.RunSupportPacketTest(e.rctx(), *ss)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "missing.crt")
	})

	t.Run("expired idp certificate", func(t *testing.T) {
		expired := newKeyPair(t, "old-idp", time.Now().Add(-48*time.Hour), time.Now().Add(-24*time.Hour))
		e.b.files["expired.crt"] = expired.certPEM
		ss := cloneSamlSettings(t, e.b.cfg.SamlSettings)
		*ss.IdpCertificateFile = "expired.crt"
		err := d.RunSupportPacketTest(e.rctx(), *ss)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expired")
	})
}

func TestParseHelpers(t *testing.T) {
	kp := validKeyPair(t, "x")

	certs, err := parseCertificates(kp.cert.Raw)
	require.NoError(t, err)
	require.Len(t, certs, 1)

	body := base64.StdEncoding.EncodeToString(kp.cert.Raw)
	certs, err = parseCertificates([]byte(body))
	require.NoError(t, err)
	require.Len(t, certs, 1)

	pkcs8, err := x509MarshalPKCS8(kp)
	require.NoError(t, err)
	key, err := parsePrivateKey(pkcs8)
	require.NoError(t, err)
	assert.True(t, key.Equal(kp.key))

	_, err = parsePrivateKey([]byte("nope"))
	require.Error(t, err)
}
