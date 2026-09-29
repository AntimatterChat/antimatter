// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package saml

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"html"
	"math/big"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/beevik/etree"
	dsig "github.com/russellhaering/goxmldsig"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/app"
	"github.com/mattermost/mattermost/server/v8/einterfaces"
)

const (
	testSiteURL = "https://mm.example.com"
	testACSURL  = testSiteURL + "/login/sso/saml"
	testSPID    = "https://mm.example.com/sp"
	testIdPID   = "https://idp.example.com/issuer"
	testIdPURL  = "https://idp.example.com/sso"
)

type keyPair struct {
	key     *rsa.PrivateKey
	cert    *x509.Certificate
	certPEM []byte
	keyPEM  []byte
}

func newKeyPair(t *testing.T, cn string, notBefore, notAfter time.Time) *keyPair {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               pkix.Name{CommonName: cn},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	return &keyPair{
		key:     key,
		cert:    cert,
		certPEM: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		keyPEM:  pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}),
	}
}

func validKeyPair(t *testing.T, cn string) *keyPair {
	return newKeyPair(t, cn, time.Now().Add(-time.Hour), time.Now().Add(24*time.Hour))
}

// ---------------------------------------------------------------------------
// Fake backend

type fakeBackend struct {
	cfg     *model.Config
	siteURL string
	files   map[string][]byte
	ldap    einterfaces.LdapInterface

	users  map[string]*model.User
	tokens map[string]*model.Token

	group  *model.PropertyGroup
	fields []*model.PropertyField
	values map[string]*model.PropertyValue

	cpaCallerIDs []string
}

func newFakeBackend() *fakeBackend {
	cfg := &model.Config{}
	cfg.SetDefaults()
	ss := &cfg.SamlSettings
	*ss.Enable = true
	*ss.Verify = true
	*ss.Encrypt = false
	*ss.IdpURL = testIdPURL
	*ss.IdpDescriptorURL = testIdPID
	*ss.ServiceProviderIdentifier = testSPID
	*ss.AssertionConsumerServiceURL = testACSURL
	*ss.IdpCertificateFile = app.SamlIdpCertificateName
	*ss.EmailAttribute = "mail"
	*ss.UsernameAttribute = "uid"
	*ss.IdAttribute = "objectId"
	*ss.FirstNameAttribute = "givenName"
	*ss.LastNameAttribute = "sn"
	*ss.NicknameAttribute = "nick"
	*ss.PositionAttribute = "title"
	*ss.LocaleAttribute = "lang"
	*cfg.ServiceSettings.SiteURL = testSiteURL
	*cfg.GuestAccountsSettings.Enable = true

	return &fakeBackend{
		cfg:     cfg,
		siteURL: testSiteURL,
		files:   map[string][]byte{},
		users:   map[string]*model.User{},
		tokens:  map[string]*model.Token{},
		group:   &model.PropertyGroup{ID: model.NewId(), Name: model.AccessControlPropertyGroupName},
		values:  map[string]*model.PropertyValue{},
	}
}

func notFound(where string) *model.AppError {
	return model.NewAppError(where, app.MissingAccountError, nil, "", http.StatusNotFound)
}

func copyUser(u *model.User) *model.User {
	c := *u
	if u.AuthData != nil {
		c.AuthData = model.NewPointer(*u.AuthData)
	}
	return &c
}

func (f *fakeBackend) Config() *model.Config           { return f.cfg }
func (f *fakeBackend) GetSiteURL() string              { return f.siteURL }
func (f *fakeBackend) Ldap() einterfaces.LdapInterface { return f.ldap }
func (f *fakeBackend) GetConfigFile(name string) ([]byte, error) {
	data, ok := f.files[name]
	if !ok {
		return nil, fmt.Errorf("file %s not found", name)
	}
	return data, nil
}

func (f *fakeBackend) GetUserByAuth(authData string) (*model.User, *model.AppError) {
	for _, u := range f.users {
		if u.AuthService == model.UserAuthServiceSaml && u.AuthData != nil && *u.AuthData == authData {
			return copyUser(u), nil
		}
	}
	return nil, model.NewAppError("GetUserByAuth", app.MissingAuthAccountError, nil, "", http.StatusBadRequest)
}

func (f *fakeBackend) GetUserByEmail(email string) (*model.User, *model.AppError) {
	for _, u := range f.users {
		if u.Email == strings.ToLower(email) {
			return copyUser(u), nil
		}
	}
	return nil, notFound("GetUserByEmail")
}

func (f *fakeBackend) GetUserByUsername(username string) (*model.User, *model.AppError) {
	for _, u := range f.users {
		if u.Username == username {
			return copyUser(u), nil
		}
	}
	return nil, notFound("GetUserByUsername")
}

func (f *fakeBackend) addUser(u *model.User) *model.User {
	if u.Id == "" {
		u.Id = model.NewId()
	}
	if u.Roles == "" {
		u.Roles = model.SystemUserRoleId
	}
	u.CreateAt = model.GetMillis()
	u.UpdateAt = u.CreateAt
	f.users[u.Id] = copyUser(u)
	return u
}

func (f *fakeBackend) CreateUser(rctx request.CTX, user *model.User, guest bool) (*model.User, *model.AppError) {
	if _, err := f.GetUserByEmail(user.Email); err == nil {
		return nil, model.NewAppError("CreateUser", "app.user.save.email_exists.app_error", nil, "", http.StatusBadRequest)
	}
	u := copyUser(user)
	u.Id = model.NewId()
	u.CreateAt = model.GetMillis()
	u.UpdateAt = u.CreateAt
	if guest {
		u.Roles = model.SystemGuestRoleId
	} else {
		u.Roles = model.SystemUserRoleId
	}
	if u.Locale == "" {
		u.Locale = model.DefaultLocale
	}
	if appErr := u.IsValid(); appErr != nil {
		return nil, appErr
	}
	f.users[u.Id] = copyUser(u)
	return u, nil
}

func (f *fakeBackend) SaveUser(rctx request.CTX, user *model.User) (*model.User, *model.AppError) {
	old, ok := f.users[user.Id]
	if !ok {
		return nil, notFound("SaveUser")
	}
	u := copyUser(user)
	u.Roles = old.Roles
	u.AuthData = old.AuthData
	u.AuthService = old.AuthService
	if appErr := u.IsValid(); appErr != nil {
		return nil, appErr
	}
	f.users[u.Id] = copyUser(u)
	return u, nil
}

func (f *fakeBackend) UpdateUserRoles(rctx request.CTX, user *model.User, roles string) (*model.User, *model.AppError) {
	u, ok := f.users[user.Id]
	if !ok {
		return nil, notFound("UpdateUserRoles")
	}
	u.Roles = roles
	return copyUser(u), nil
}

func (f *fakeBackend) DemoteUserToGuest(rctx request.CTX, user *model.User) *model.AppError {
	u, ok := f.users[user.Id]
	if !ok {
		return notFound("DemoteUserToGuest")
	}
	u.Roles = model.SystemGuestRoleId
	return nil
}

func (f *fakeBackend) UpdateAuthData(userID, authData, email string, resetMfa bool) *model.AppError {
	u, ok := f.users[userID]
	if !ok {
		return notFound("UpdateAuthData")
	}
	u.AuthService = model.UserAuthServiceSaml
	u.AuthData = model.NewPointer(authData)
	if email != "" {
		u.Email = strings.ToLower(email)
	}
	if resetMfa {
		u.Password = ""
		u.MfaActive = false
	}
	return nil
}

func (f *fakeBackend) GetSamlEmailToken(token string) (*model.Token, *model.AppError) {
	t, ok := f.tokens[token]
	if !ok || t.Type != model.TokenTypeSaml {
		return nil, model.NewAppError("GetSamlEmailToken", "api.saml.invalid_email_token.app_error", nil, "", http.StatusBadRequest)
	}
	return t, nil
}

func (f *fakeBackend) DeleteToken(token string) error {
	delete(f.tokens, token)
	return nil
}

func (f *fakeBackend) recordCaller(rctx request.CTX) {
	id, _ := model.CallerIDFromContext(rctx.Context())
	f.cpaCallerIDs = append(f.cpaCallerIDs, id)
}

func (f *fakeBackend) GetPropertyGroup(rctx request.CTX, name string) (*model.PropertyGroup, *model.AppError) {
	if name != f.group.Name {
		return nil, model.NewAppError("GetPropertyGroup", "app.property_group.get.app_error", nil, "", http.StatusNotFound)
	}
	return f.group, nil
}

func (f *fakeBackend) SearchPropertyFields(rctx request.CTX, groupID string, opts model.PropertyFieldSearchOpts) ([]*model.PropertyField, *model.AppError) {
	f.recordCaller(rctx)
	return f.fields, nil
}

func (f *fakeBackend) SearchPropertyValues(rctx request.CTX, groupID string, opts model.PropertyValueSearchOpts) ([]*model.PropertyValue, *model.AppError) {
	var out []*model.PropertyValue
	for _, v := range f.values {
		if slices.Contains(opts.TargetIDs, v.TargetID) {
			c := *v
			out = append(out, &c)
		}
	}
	return out, nil
}

func (f *fakeBackend) UpsertPropertyValues(rctx request.CTX, values []*model.PropertyValue, objectType, targetID string) ([]*model.PropertyValue, *model.AppError) {
	f.recordCaller(rctx)
	for _, v := range values {
		var existing *model.PropertyValue
		for _, e := range f.values {
			if e.FieldID == v.FieldID && e.TargetID == v.TargetID {
				existing = e
			}
		}
		if existing != nil {
			existing.Value = v.Value
			continue
		}
		c := *v
		c.ID = model.NewId()
		f.values[c.ID] = &c
	}
	return values, nil
}

func (f *fakeBackend) DeletePropertyValue(rctx request.CTX, groupID, valueID string) *model.AppError {
	f.recordCaller(rctx)
	delete(f.values, valueID)
	return nil
}

func (f *fakeBackend) valueFor(userID, fieldID string) (string, bool) {
	for _, v := range f.values {
		if v.TargetID == userID && v.FieldID == fieldID {
			var s string
			_ = json.Unmarshal(v.Value, &s)
			return s, true
		}
	}
	return "", false
}

// ---------------------------------------------------------------------------
// Fake Identity Provider producing SAML responses

type responseOptions struct {
	attributes     map[string][]string
	nameID         string
	issuer         string
	audience       string
	recipient      string
	destination    string
	notBefore      time.Time
	notOnOrAfter   time.Time
	signAssertion  bool
	signResponse   bool
	signer         *keyPair
	encryptFor     *keyPair
	assertionID    string
	omitAttributes bool
}

func defaultResponseOptions(idp *keyPair, attrs map[string][]string) *responseOptions {
	now := time.Now().UTC()
	return &responseOptions{
		attributes:    attrs,
		nameID:        "nameid-123",
		issuer:        testIdPID,
		audience:      testSPID,
		recipient:     testACSURL,
		destination:   testACSURL,
		notBefore:     now.Add(-time.Minute),
		notOnOrAfter:  now.Add(5 * time.Minute),
		signAssertion: true,
		signer:        idp,
		assertionID:   "_" + model.NewId(),
	}
}

func samlTime(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05Z")
}

func sign(t *testing.T, kp *keyPair, el *etree.Element) *etree.Element {
	t.Helper()
	ctx := dsig.NewDefaultSigningContext(dsig.TLSCertKeyStore(tls.Certificate{
		Certificate: [][]byte{kp.cert.Raw},
		PrivateKey:  kp.key,
	}))
	ctx.Canonicalizer = dsig.MakeC14N10ExclusiveCanonicalizerWithPrefixList("")
	signed, err := ctx.SignEnveloped(el)
	require.NoError(t, err)
	return signed
}

func elementString(t *testing.T, el *etree.Element) string {
	t.Helper()
	doc := etree.NewDocument()
	doc.SetRoot(el)
	s, err := doc.WriteToString()
	require.NoError(t, err)
	return s
}

func parseElement(t *testing.T, s string) *etree.Element {
	t.Helper()
	doc := etree.NewDocument()
	require.NoError(t, doc.ReadFromString(s))
	return doc.Root()
}

func encryptAssertion(t *testing.T, sp *keyPair, plaintext []byte) string {
	t.Helper()
	symKey := make([]byte, 16)
	_, err := rand.Read(symKey)
	require.NoError(t, err)
	block, err := aes.NewCipher(symKey)
	require.NoError(t, err)
	gcm, err := cipher.NewGCM(block)
	require.NoError(t, err)
	nonce := make([]byte, gcm.NonceSize())
	_, err = rand.Read(nonce)
	require.NoError(t, err)
	data := gcm.Seal(nonce, nonce, plaintext, nil)

	encKey, err := rsa.EncryptOAEP(sha1.New(), rand.Reader, &sp.key.PublicKey, symKey, nil)
	require.NoError(t, err)

	return `<saml:EncryptedAssertion xmlns:saml="urn:oasis:names:tc:SAML:2.0:assertion">` +
		`<xenc:EncryptedData xmlns:xenc="http://www.w3.org/2001/04/xmlenc#" Type="http://www.w3.org/2001/04/xmlenc#Element">` +
		`<xenc:EncryptionMethod Algorithm="http://www.w3.org/2009/xmlenc11#aes128-gcm"/>` +
		`<ds:KeyInfo xmlns:ds="http://www.w3.org/2000/09/xmldsig#"><xenc:EncryptedKey>` +
		`<xenc:EncryptionMethod Algorithm="http://www.w3.org/2001/04/xmlenc#rsa-oaep-mgf1p"><ds:DigestMethod Algorithm="http://www.w3.org/2000/09/xmldsig#sha1"/></xenc:EncryptionMethod>` +
		`<xenc:CipherData><xenc:CipherValue>` + base64.StdEncoding.EncodeToString(encKey) + `</xenc:CipherValue></xenc:CipherData>` +
		`</xenc:EncryptedKey></ds:KeyInfo>` +
		`<xenc:CipherData><xenc:CipherValue>` + base64.StdEncoding.EncodeToString(data) + `</xenc:CipherValue></xenc:CipherData>` +
		`</xenc:EncryptedData></saml:EncryptedAssertion>`
}

func buildResponse(t *testing.T, o *responseOptions) string {
	t.Helper()
	now := time.Now().UTC()

	var attrs strings.Builder
	if !o.omitAttributes {
		attrs.WriteString(`<saml:AttributeStatement>`)
		for name, values := range o.attributes {
			fmt.Fprintf(&attrs, `<saml:Attribute Name="%s">`, html.EscapeString(name))
			for _, v := range values {
				fmt.Fprintf(&attrs, `<saml:AttributeValue>%s</saml:AttributeValue>`, html.EscapeString(v))
			}
			attrs.WriteString(`</saml:Attribute>`)
		}
		attrs.WriteString(`</saml:AttributeStatement>`)
	}

	assertion := fmt.Sprintf(`<saml:Assertion xmlns:saml="urn:oasis:names:tc:SAML:2.0:assertion" ID="%s" Version="2.0" IssueInstant="%s">`+
		`<saml:Issuer>%s</saml:Issuer>`+
		`<saml:Subject><saml:NameID>%s</saml:NameID>`+
		`<saml:SubjectConfirmation Method="urn:oasis:names:tc:SAML:2.0:cm:bearer">`+
		`<saml:SubjectConfirmationData NotOnOrAfter="%s" Recipient="%s"/></saml:SubjectConfirmation></saml:Subject>`+
		`<saml:Conditions NotBefore="%s" NotOnOrAfter="%s"><saml:AudienceRestriction><saml:Audience>%s</saml:Audience></saml:AudienceRestriction></saml:Conditions>`+
		`<saml:AuthnStatement AuthnInstant="%s" SessionIndex="_session"><saml:AuthnContext><saml:AuthnContextClassRef>urn:oasis:names:tc:SAML:2.0:ac:classes:Password</saml:AuthnContextClassRef></saml:AuthnContext></saml:AuthnStatement>`+
		`%s</saml:Assertion>`,
		o.assertionID, samlTime(now), o.issuer, o.nameID,
		samlTime(o.notOnOrAfter), o.recipient,
		samlTime(o.notBefore), samlTime(o.notOnOrAfter), o.audience,
		samlTime(now), attrs.String())

	if o.signAssertion {
		assertion = elementString(t, sign(t, o.signer, parseElement(t, assertion)))
		assertion = strings.TrimPrefix(assertion, `<?xml version="1.0" encoding="UTF-8"?>`)
	}
	if o.encryptFor != nil {
		assertion = encryptAssertion(t, o.encryptFor, []byte(assertion))
	}

	response := fmt.Sprintf(`<samlp:Response xmlns:samlp="urn:oasis:names:tc:SAML:2.0:protocol" xmlns:saml="urn:oasis:names:tc:SAML:2.0:assertion" ID="_%s" Version="2.0" IssueInstant="%s" Destination="%s">`+
		`<saml:Issuer>%s</saml:Issuer>`+
		`<samlp:Status><samlp:StatusCode Value="urn:oasis:names:tc:SAML:2.0:status:Success"/></samlp:Status>`+
		`%s</samlp:Response>`,
		model.NewId(), samlTime(now), o.destination, o.issuer, assertion)

	if o.signResponse {
		response = elementString(t, sign(t, o.signer, parseElement(t, response)))
	}

	return base64.StdEncoding.EncodeToString([]byte(response))
}

func cloneSamlSettings(t *testing.T, s model.SamlSettings) *model.SamlSettings {
	t.Helper()
	data, err := json.Marshal(s)
	require.NoError(t, err)
	var out model.SamlSettings
	require.NoError(t, json.Unmarshal(data, &out))
	return &out
}

func x509MarshalPKCS8(kp *keyPair) ([]byte, error) {
	der, err := x509.MarshalPKCS8PrivateKey(kp.key)
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), nil
}
