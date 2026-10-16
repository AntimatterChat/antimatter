// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

// Package saml implements SAML 2.0 single sign-on (einterfaces.SamlInterface)
// and its support packet diagnostics (einterfaces.SamlDiagnosticInterface) on
// top of github.com/mattermost/gosaml2.
package saml

import (
	"encoding/base64"
	"encoding/xml"
	"net/http"
	"sync"

	saml2 "github.com/mattermost/gosaml2"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/app"
	"github.com/mattermost/mattermost/server/v8/channels/app/platform"
	"github.com/mattermost/mattermost/server/v8/einterfaces"
)

func init() {
	app.RegisterSamlInterface(func(a *app.App) einterfaces.SamlInterface {
		return newService(&appBackend{a: a})
	})
	platform.RegisterSamlDiagnosticInterface(func(ps *platform.PlatformService) einterfaces.SamlDiagnosticInterface {
		return &Diagnostic{files: ps, siteURL: func() string { return model.SafeDereference(ps.Config().ServiceSettings.SiteURL) }}
	})
}

// Service implements einterfaces.SamlInterface.
type Service struct {
	b backend

	mu            sync.Mutex
	sp            *saml2.SAMLServiceProvider
	spFingerprint string

	replay *replayCache
}

var _ einterfaces.SamlInterface = (*Service)(nil)

func newService(b backend) *Service {
	return &Service{
		b:      b,
		replay: newReplayCache(defaultReplayCacheSize),
	}
}

// ConfigureSP (re)builds the Service Provider from the current configuration.
// It is called at startup and whenever the configuration changes. When SAML
// is disabled the cached Service Provider is dropped and no error is returned.
func (s *Service) ConfigureSP(rctx request.CTX) error {
	cfg := s.b.Config()
	if !model.SafeDereference(cfg.SamlSettings.Enable) {
		s.mu.Lock()
		s.sp = nil
		s.spFingerprint = ""
		s.mu.Unlock()
		return nil
	}

	if _, appErr := s.serviceProvider(rctx, true); appErr != nil {
		return appErr
	}
	return nil
}

// serviceProvider returns an up to date Service Provider. The certificate files
// are re-read on every call so that a certificate replaced under the same file
// name (which does not trigger a configuration change) is picked up, on every
// node of a cluster.
func (s *Service) serviceProvider(rctx request.CTX, force bool) (*saml2.SAMLServiceProvider, *model.AppError) {
	cfg := s.b.Config()
	if !model.SafeDereference(cfg.SamlSettings.Enable) {
		return nil, model.NewAppError("SamlInterface", "ent.saml.service_disable.app_error", nil, "", http.StatusNotImplemented)
	}

	in := loadInputs(s.b, &cfg.SamlSettings, s.b.GetSiteURL())
	fp := in.fingerprint()

	s.mu.Lock()
	defer s.mu.Unlock()

	if !force && s.sp != nil && s.spFingerprint == fp {
		return s.sp, nil
	}

	sp, appErr := buildServiceProvider(in, false)
	if appErr != nil {
		s.sp = nil
		s.spFingerprint = ""
		return nil, appErr
	}

	s.sp = sp
	s.spFingerprint = fp
	rctx.Logger().Debug("SAML Service Provider configured",
		mlog.String("sp_entity_id", sp.ServiceProviderIssuer),
		mlog.String("acs_url", sp.AssertionConsumerServiceURL),
		mlog.String("idp_sso_url", sp.IdentityProviderSSOURL),
		mlog.Bool("verify", !sp.SkipSignatureValidation),
		mlog.Bool("encrypt", sp.SPKeyStore != nil),
		mlog.Bool("sign_request", sp.SignAuthnRequests),
	)
	return sp, nil
}

// BuildRequest builds the AuthnRequest for the HTTP-Redirect binding. The web
// layer redirects the browser to the returned URL. When request signing is
// enabled the query string is signed as required by the redirect binding
// (SAML Bindings 3.4.4.1).
func (s *Service) BuildRequest(rctx request.CTX, relayState string) (*model.SamlAuthRequest, *model.AppError) {
	sp, appErr := s.serviceProvider(rctx, false)
	if appErr != nil {
		return nil, appErr
	}

	doc, err := sp.BuildAuthRequestDocumentNoSig()
	if err != nil {
		return nil, model.NewAppError("BuildRequest", "ent.saml.build_request.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	raw, err := doc.WriteToBytes()
	if err != nil {
		return nil, model.NewAppError("BuildRequest", "ent.saml.build_request.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	redirectURL, err := sp.BuildAuthURLRedirect(relayState, doc)
	if err != nil {
		return nil, model.NewAppError("BuildRequest", "ent.saml.build_request.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	return &model.SamlAuthRequest{
		Base64AuthRequest: base64.StdEncoding.EncodeToString(raw),
		URL:               redirectURL,
		RelayState:        relayState,
	}, nil
}

// GetMetadata returns the Service Provider metadata XML document. It does not
// require the Identity Provider to be configured, so that administrators can
// hand the metadata to their IdP before completing the configuration.
func (s *Service) GetMetadata(rctx request.CTX) (string, *model.AppError) {
	cfg := s.b.Config()
	in := loadInputs(s.b, &cfg.SamlSettings, s.b.GetSiteURL())

	sp, appErr := buildServiceProvider(in, true)
	if appErr != nil {
		return "", model.NewAppError("GetMetadata", "ent.saml.metadata.app_error", nil, "", appErr.StatusCode).Wrap(appErr)
	}

	md, err := sp.Metadata()
	if err != nil {
		return "", model.NewAppError("GetMetadata", "ent.saml.metadata.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	if nameIDFormat := nameIDFormat(); nameIDFormat != "" && md.SPSSODescriptor != nil {
		md.SPSSODescriptor.NameIDFormats = []string{nameIDFormat}
	}

	out, err := xml.MarshalIndent(md, "", "  ")
	if err != nil {
		return "", model.NewAppError("GetMetadata", "ent.saml.metadata.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	return xml.Header + string(out), nil
}

// nameIDFormat is the NameID format advertised in the SP metadata. Users are
// identified through the configured attributes, so any format is acceptable.
func nameIDFormat() string {
	return "urn:oasis:names:tc:SAML:1.1:nameid-format:unspecified"
}

// CheckProviderAttributes returns the name of the first field of the patch
// that would override a value managed by the SAML Identity Provider, or the
// empty string if the patch can be applied.
func (s *Service) CheckProviderAttributes(rctx request.CTX, ss *model.SamlSettings, ouser *model.User, patch *model.UserPatch) string {
	if ss == nil || ouser == nil || patch == nil {
		return ""
	}

	changing := func(current string, next *string) bool {
		return next != nil && *next != current
	}
	configured := func(attr *string) bool {
		return model.SafeDereference(attr) != ""
	}

	switch {
	case configured(ss.UsernameAttribute) && changing(ouser.Username, patch.Username):
		return "username"
	case configured(ss.EmailAttribute) && changing(ouser.Email, patch.Email):
		return "email"
	case configured(ss.FirstNameAttribute) && changing(ouser.FirstName, patch.FirstName):
		return "first name"
	case configured(ss.LastNameAttribute) && changing(ouser.LastName, patch.LastName):
		return "last name"
	case configured(ss.NicknameAttribute) && changing(ouser.Nickname, patch.Nickname):
		return "nickname"
	case configured(ss.PositionAttribute) && changing(ouser.Position, patch.Position):
		return "position"
	case configured(ss.LocaleAttribute) && changing(ouser.Locale, patch.Locale):
		return "locale"
	}
	return ""
}
