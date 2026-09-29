// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package saml

import (
	"bytes"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	saml2 "github.com/mattermost/gosaml2"
	dsig "github.com/russellhaering/goxmldsig"

	"github.com/mattermost/mattermost/server/public/model"
)

// samlLoginPath is the path of the Assertion Consumer Service exposed by the
// web layer (server/channels/web/saml.go).
const samlLoginPath = "/login/sso/saml"

// configFileSource gives access to the files stored alongside the
// configuration (uploaded SAML certificates and keys).
type configFileSource interface {
	GetConfigFile(name string) ([]byte, error)
}

// spInputs gathers everything that the Service Provider configuration depends
// on: the SAML settings, the site URL and the content of the certificate files.
type spInputs struct {
	settings model.SamlSettings
	siteURL  string

	idpCert  []byte
	spCert   []byte
	spKey    []byte
	fileErrs map[string]error
}

// fingerprint returns a digest of the inputs, used to detect whether a cached
// Service Provider is still up to date (e.g. after a certificate file has been
// replaced without the configuration changing).
func (in *spInputs) fingerprint() string {
	h := sha256.New()
	settingsJSON, _ := json.Marshal(in.settings)
	for _, part := range [][]byte{settingsJSON, []byte(in.siteURL), in.idpCert, in.spCert, in.spKey} {
		fmt.Fprintf(h, "%d:", len(part))
		h.Write(part)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// needsSPKeyPair reports whether the SP private key and public certificate are
// required by the settings.
func needsSPKeyPair(s *model.SamlSettings) bool {
	return model.SafeDereference(s.Encrypt) || model.SafeDereference(s.SignRequest)
}

// loadInputs reads the certificate files referenced by the settings through the
// configuration store (see app.writeSamlFile).
func loadInputs(files configFileSource, settings *model.SamlSettings, siteURL string) *spInputs {
	in := &spInputs{
		settings: *settings,
		siteURL:  siteURL,
		fileErrs: map[string]error{},
	}

	read := func(name string) []byte {
		if name == "" {
			return nil
		}
		data, err := files.GetConfigFile(name)
		if err != nil {
			in.fileErrs[name] = err
			return nil
		}
		return data
	}

	in.idpCert = read(model.SafeDereference(settings.IdpCertificateFile))
	if needsSPKeyPair(settings) {
		in.spCert = read(model.SafeDereference(settings.PublicCertificateFile))
		in.spKey = read(model.SafeDereference(settings.PrivateKeyFile))
	}
	return in
}

// assertionConsumerServiceURL returns the configured ACS URL, falling back to
// the canonical login endpoint below the site URL.
func assertionConsumerServiceURL(s *model.SamlSettings, siteURL string) string {
	if acs := strings.TrimSpace(model.SafeDereference(s.AssertionConsumerServiceURL)); acs != "" {
		return acs
	}
	if siteURL == "" {
		return ""
	}
	return strings.TrimRight(siteURL, "/") + samlLoginPath
}

func signatureMethod(alg string) (string, error) {
	switch alg {
	case model.SamlSettingsSignatureAlgorithmSha1:
		return dsig.RSASHA1SignatureMethod, nil
	case model.SamlSettingsSignatureAlgorithmSha256, "":
		return dsig.RSASHA256SignatureMethod, nil
	case model.SamlSettingsSignatureAlgorithmSha512:
		return dsig.RSASHA512SignatureMethod, nil
	}
	return "", fmt.Errorf("unsupported signature algorithm %q", alg)
}

func canonicalizer(alg string) (dsig.Canonicalizer, error) {
	switch alg {
	case model.SamlSettingsCanonicalAlgorithmC14n, "":
		return dsig.MakeC14N10ExclusiveCanonicalizerWithPrefixList(""), nil
	case model.SamlSettingsCanonicalAlgorithmC14n11:
		return dsig.MakeC14N11Canonicalizer(), nil
	}
	return nil, fmt.Errorf("unsupported canonicalization algorithm %q", alg)
}

// parseCertificates parses one or more certificates, either PEM encoded
// (possibly several blocks, to support IdP certificate rollover) or as a single
// DER blob. A bare base64 body without PEM armour, as found in IdP metadata,
// is accepted too.
func parseCertificates(data []byte) ([]*x509.Certificate, error) {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return nil, errors.New("empty certificate")
	}

	var certs []*x509.Certificate
	rest := data
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse certificate: %w", err)
		}
		certs = append(certs, cert)
	}
	if len(certs) > 0 {
		return certs, nil
	}

	if cert, err := x509.ParseCertificate(data); err == nil {
		return []*x509.Certificate{cert}, nil
	}

	wrapped := "-----BEGIN CERTIFICATE-----\n" + string(data) + "\n-----END CERTIFICATE-----\n"
	if block, _ := pem.Decode([]byte(wrapped)); block != nil {
		if cert, err := x509.ParseCertificate(block.Bytes); err == nil {
			return []*x509.Certificate{cert}, nil
		}
	}

	return nil, errors.New("no certificate found")
}

// parsePrivateKey parses an RSA private key in PKCS#1 or PKCS#8 form, PEM or DER
// encoded. XML signature and encryption support in goxmldsig is RSA only.
func parsePrivateKey(data []byte) (*rsa.PrivateKey, error) {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return nil, errors.New("empty private key")
	}

	var ders [][]byte
	rest := data
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if strings.Contains(block.Type, "PRIVATE KEY") {
			ders = append(ders, block.Bytes)
		}
	}
	if len(ders) == 0 {
		ders = append(ders, data)
	}

	for _, der := range ders {
		if key, err := x509.ParsePKCS1PrivateKey(der); err == nil {
			return key, nil
		}
		if key, err := x509.ParsePKCS8PrivateKey(der); err == nil {
			rsaKey, ok := key.(*rsa.PrivateKey)
			if !ok {
				return nil, fmt.Errorf("unsupported private key type %T, only RSA keys are supported", key)
			}
			return rsaKey, nil
		}
	}
	return nil, errors.New("no RSA private key found")
}

// loadSPKeyPair builds the key store used to decrypt assertions and sign
// requests from the SP private key and public certificate.
func loadSPKeyPair(in *spInputs) (dsig.X509KeyStore, *x509.Certificate, *model.AppError) {
	s := &in.settings
	keyName := model.SafeDereference(s.PrivateKeyFile)
	certName := model.SafeDereference(s.PublicCertificateFile)

	if keyName == "" || len(in.spKey) == 0 {
		return nil, nil, model.NewAppError("ConfigureSP", "ent.saml.configure.load_private_key.app_error", nil,
			fmt.Sprintf("private key file %q is not available: %v", keyName, in.fileErrs[keyName]), http.StatusInternalServerError)
	}
	key, err := parsePrivateKey(in.spKey)
	if err != nil {
		return nil, nil, model.NewAppError("ConfigureSP", "ent.saml.configure.load_private_key.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	if certName == "" || len(in.spCert) == 0 {
		return nil, nil, model.NewAppError("ConfigureSP", "ent.saml.configure.certificate_parse_error.app_error", nil,
			fmt.Sprintf("service provider public certificate file %q is not available: %v", certName, in.fileErrs[certName]), http.StatusInternalServerError)
	}
	certs, err := parseCertificates(in.spCert)
	if err != nil {
		return nil, nil, model.NewAppError("ConfigureSP", "ent.saml.configure.certificate_parse_error.app_error", nil, "service provider public certificate", http.StatusInternalServerError).Wrap(err)
	}
	cert := certs[0]

	pub, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok || !pub.Equal(&key.PublicKey) {
		return nil, nil, model.NewAppError("ConfigureSP", "ent.saml.configure.load_private_key.app_error", nil,
			"the service provider private key does not match the service provider public certificate", http.StatusInternalServerError)
	}

	return dsig.TLSCertKeyStore(tls.Certificate{
		Certificate: [][]byte{cert.Raw},
		PrivateKey:  key,
		Leaf:        cert,
	}), cert, nil
}

// buildServiceProvider turns the SAML settings into a configured gosaml2
// Service Provider. When forMetadata is true, the Identity Provider parameters
// are not required, since the SP metadata does not depend on them.
func buildServiceProvider(in *spInputs, forMetadata bool) (*saml2.SAMLServiceProvider, *model.AppError) {
	s := &in.settings

	spEntityID := strings.TrimSpace(model.SafeDereference(s.ServiceProviderIdentifier))
	acsURL := assertionConsumerServiceURL(s, in.siteURL)

	sp := &saml2.SAMLServiceProvider{
		IdentityProviderSSOURL:      strings.TrimSpace(model.SafeDereference(s.IdpURL)),
		IdentityProviderSSOBinding:  saml2.BindingHttpRedirect,
		IdentityProviderIssuer:      strings.TrimSpace(model.SafeDereference(s.IdpDescriptorURL)),
		ServiceProviderIssuer:       spEntityID,
		AssertionConsumerServiceURL: acsURL,
		AudienceURI:                 spEntityID,
		SkipSignatureValidation:     !model.SafeDereference(s.Verify),
		SignAuthnRequests:           model.SafeDereference(s.SignRequest),
		ScopingIDPProviderId:        strings.TrimSpace(model.SafeDereference(s.ScopingIDPProviderId)),
		ScopingIDPProviderName:      strings.TrimSpace(model.SafeDereference(s.ScopingIDPName)),
		Clock:                       dsig.NewRealClock(),
	}

	if spEntityID == "" {
		return nil, model.NewAppError("ConfigureSP", "model.config.is_valid.saml_spidentifier_attribute.app_error", nil, "", http.StatusBadRequest)
	}
	if acsURL == "" {
		return nil, model.NewAppError("ConfigureSP", "model.config.is_valid.saml_assertion_consumer_service_url.app_error", nil, "", http.StatusBadRequest)
	}

	if !forMetadata {
		if sp.IdentityProviderSSOURL == "" {
			return nil, model.NewAppError("ConfigureSP", "model.config.is_valid.saml_idp_url.app_error", nil, "", http.StatusBadRequest)
		}
		if _, err := url.Parse(sp.IdentityProviderSSOURL); err != nil {
			return nil, model.NewAppError("ConfigureSP", "model.config.is_valid.saml_idp_url.app_error", nil, "", http.StatusBadRequest).Wrap(err)
		}

		idpCertName := model.SafeDereference(s.IdpCertificateFile)
		if (idpCertName == "" || len(in.idpCert) == 0) && sp.SkipSignatureValidation {
			// Without signature verification the IdP certificate is not needed.
			return finishServiceProvider(in, sp)
		}
		if idpCertName == "" || len(in.idpCert) == 0 {
			return nil, model.NewAppError("ConfigureSP", "ent.saml.configure.certificate_parse_error.app_error", nil,
				fmt.Sprintf("identity provider certificate file %q is not available: %v", idpCertName, in.fileErrs[idpCertName]), http.StatusInternalServerError)
		}
		idpCerts, err := parseCertificates(in.idpCert)
		if err != nil {
			return nil, model.NewAppError("ConfigureSP", "ent.saml.configure.certificate_parse_error.app_error", nil, "identity provider certificate", http.StatusInternalServerError).Wrap(err)
		}
		sp.IDPCertificateStore = &dsig.MemoryX509CertificateStore{Roots: idpCerts}
	}

	return finishServiceProvider(in, sp)
}

// finishServiceProvider configures the SP key pair and request signing.
func finishServiceProvider(in *spInputs, sp *saml2.SAMLServiceProvider) (*saml2.SAMLServiceProvider, *model.AppError) {
	s := &in.settings
	if needsSPKeyPair(s) {
		keyStore, _, appErr := loadSPKeyPair(in)
		if appErr != nil {
			if !model.SafeDereference(s.Encrypt) {
				// Request signing needs the key pair that is configured with encryption.
				appErr = model.NewAppError("ConfigureSP", "ent.saml.configure.encryption_not_enabled.app_error", nil, "", http.StatusInternalServerError).Wrap(appErr)
			}
			return nil, appErr
		}
		if model.SafeDereference(s.Encrypt) {
			// Setting SPKeyStore makes gosaml2 require every assertion to be encrypted.
			sp.SPKeyStore = keyStore
		}
		sp.SPSigningKeyStore = keyStore
	}

	if sp.SignAuthnRequests {
		method, err := signatureMethod(model.SafeDereference(s.SignatureAlgorithm))
		if err != nil {
			return nil, model.NewAppError("ConfigureSP", "model.config.is_valid.saml_signature_algorithm.app_error", nil, "", http.StatusBadRequest).Wrap(err)
		}
		canon, err := canonicalizer(model.SafeDereference(s.CanonicalAlgorithm))
		if err != nil {
			return nil, model.NewAppError("ConfigureSP", "model.config.is_valid.saml_canonical_algorithm.app_error", nil, "", http.StatusBadRequest).Wrap(err)
		}
		sp.SignAuthnRequestsAlgorithm = method
		sp.SignAuthnRequestsCanonicalizer = canon
	}

	return sp, nil
}

// certificateExpiryProblems returns human readable problems about expired or
// not yet valid certificates; used by the diagnostics.
func certificateExpiryProblems(label string, certs []*x509.Certificate, now time.Time) []string {
	var problems []string
	for _, c := range certs {
		if now.After(c.NotAfter) {
			problems = append(problems, fmt.Sprintf("%s (subject %q) expired on %s", label, c.Subject.String(), c.NotAfter.UTC().Format(time.RFC3339)))
		} else if now.Before(c.NotBefore) {
			problems = append(problems, fmt.Sprintf("%s (subject %q) is not valid before %s", label, c.Subject.String(), c.NotBefore.UTC().Format(time.RFC3339)))
		}
	}
	return problems
}
