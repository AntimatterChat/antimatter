// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package ldap

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

func testKeyPair(t *testing.T) ([]byte, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "mattermost"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
}

func TestBuildTLSConfig(t *testing.T) {
	certPEM, keyPEM := testKeyPair(t)
	files := map[string][]byte{model.LdapPublicCertificateName: certPEM, model.LdapPrivateKeyName: keyPEM, "bad": []byte("garbage")}
	get := func(name string) ([]byte, error) {
		if f, ok := files[name]; ok {
			return f, nil
		}
		return nil, errors.New("missing")
	}

	s := newSettings(&model.LdapSettings{LdapServer: new("ldap.example.com"), SkipCertificateVerification: new(true)})
	cfg, appErr := buildTLSConfig(s, get)
	require.Nil(t, appErr)
	assert.True(t, cfg.InsecureSkipVerify)
	assert.Equal(t, "ldap.example.com", cfg.ServerName)
	assert.Empty(t, cfg.Certificates)

	s.PublicCertificateFile = model.LdapPublicCertificateName
	s.PrivateKeyFile = model.LdapPrivateKeyName
	cfg, appErr = buildTLSConfig(s, get)
	require.Nil(t, appErr)
	assert.Len(t, cfg.Certificates, 1)

	s.PrivateKeyFile = "missing"
	_, appErr = buildTLSConfig(s, get)
	require.NotNil(t, appErr)
	assert.Equal(t, "ent.ldap.do_login.key.app_error", appErr.Id)

	s.PrivateKeyFile = "bad"
	_, appErr = buildTLSConfig(s, get)
	require.NotNil(t, appErr)
	assert.Equal(t, "ent.ldap.do_login.x509.app_error", appErr.Id)
}

func TestDialLDAPUnreachable(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := l.Addr().(*net.TCPAddr).Port
	require.NoError(t, l.Close())

	s := newSettings(&model.LdapSettings{LdapServer: new("127.0.0.1"), LdapPort: new(port), QueryTimeout: new(2)})
	_, appErr := dialLDAP(s, nil)
	require.NotNil(t, appErr)
	assert.Equal(t, "ent.ldap.do_login.unable_to_connect.app_error", appErr.Id)
}
