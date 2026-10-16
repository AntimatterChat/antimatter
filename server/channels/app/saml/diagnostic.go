// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package saml

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/einterfaces"
)

// Diagnostic implements einterfaces.SamlDiagnosticInterface. It checks, without
// contacting the Identity Provider, that the SAML settings can be turned into a
// working Service Provider: certificates and keys load, match and are within
// their validity period, and an authentication request can be built (and signed
// when request signing is enabled).
type Diagnostic struct {
	files   configFileSource
	siteURL func() string
	now     func() time.Time
}

var _ einterfaces.SamlDiagnosticInterface = (*Diagnostic)(nil)

func (d *Diagnostic) RunSupportPacketTest(rctx request.CTX, settings model.SamlSettings) error {
	settings.SetDefaults()

	now := time.Now()
	if d.now != nil {
		now = d.now()
	}
	siteURL := ""
	if d.siteURL != nil {
		siteURL = d.siteURL()
	}

	in := loadInputs(d.files, &settings, siteURL)

	var problems []string
	for _, name := range slices.Sorted(maps.Keys(in.fileErrs)) {
		problems = append(problems, fmt.Sprintf("cannot read file %q: %v", name, in.fileErrs[name]))
	}

	sp, appErr := buildServiceProvider(in, false)
	if appErr != nil {
		problems = append(problems, describeAppError(appErr))
	} else {
		if doc, err := sp.BuildAuthRequestDocumentNoSig(); err != nil {
			problems = append(problems, fmt.Sprintf("cannot build an authentication request: %v", err))
		} else if _, err := sp.BuildAuthURLRedirect("diagnostic", doc); err != nil {
			problems = append(problems, fmt.Sprintf("cannot build the redirect URL for the authentication request: %v", err))
		}
	}

	if len(in.idpCert) > 0 {
		if certs, err := parseCertificates(in.idpCert); err == nil {
			problems = append(problems, certificateExpiryProblems("identity provider certificate", certs, now)...)
		}
	}
	if len(in.spCert) > 0 {
		if certs, err := parseCertificates(in.spCert); err == nil {
			problems = append(problems, certificateExpiryProblems("service provider certificate", certs, now)...)
		}
	}

	if model.SafeDereference(settings.EmailAttribute) == "" {
		problems = append(problems, "the email attribute is not configured")
	}
	if model.SafeDereference(settings.UsernameAttribute) == "" {
		problems = append(problems, "the username attribute is not configured")
	}

	if len(problems) == 0 {
		return nil
	}
	return errors.New(strings.Join(dedupe(problems), "; "))
}

func describeAppError(appErr *model.AppError) string {
	msg := appErr.Id
	if appErr.DetailedError != "" {
		msg += ": " + appErr.DetailedError
	}
	if wrapped := appErr.Unwrap(); wrapped != nil {
		msg += ": " + wrapped.Error()
	}
	return msg
}

func dedupe(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := in[:0]
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
