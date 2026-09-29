// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package ldap

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/mattermost/ldap"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/einterfaces"
)

// maxDiagnosticSamples is the number of sample entries returned by the
// diagnostics.
const maxDiagnosticSamples = 5

// diagnosticPlatform is the subset of the platform service used by the
// diagnostics.
type diagnosticPlatform interface {
	Config() *model.Config
	GetConfigFile(name string) ([]byte, error)
}

// Diagnostic implements einterfaces.LdapDiagnosticInterface.
type Diagnostic struct {
	p    diagnosticPlatform
	dial Dialer
}

var _ einterfaces.LdapDiagnosticInterface = (*Diagnostic)(nil)

func newDiagnostic(p diagnosticPlatform, dial Dialer) *Diagnostic {
	if dial == nil {
		dial = dialLDAP
	}
	return &Diagnostic{p: p, dial: dial}
}

// submittedSettings merges settings submitted by the System Console with the
// saved configuration: the bind password is usually not sent back.
func (d *Diagnostic) submittedSettings(submitted model.LdapSettings) *settings {
	saved := d.p.Config().LdapSettings
	if pwd := model.SafeDereference(submitted.BindPassword); pwd == "" || pwd == model.FakeSetting {
		submitted.BindPassword = saved.BindPassword
	}
	return newSettings(&submitted)
}

// RunTest checks that the saved configuration allows to connect, bind and
// find users.
func (d *Diagnostic) RunTest(rctx request.CTX) *model.AppError {
	s := newSettings(&d.p.Config().LdapSettings)
	ss, appErr := openSession(d.dial, s, d.p.GetConfigFile)
	if appErr != nil {
		return appErr
	}
	defer ss.Close()

	entries, err := ss.searchLimit(s.userFilter(), []string{"1.1"}, 1)
	if errors.Is(err, errSizeLimitExceeded) {
		return nil
	}
	if err != nil {
		return searchError("LdapDiagnostic.RunTest", err)
	}
	if len(entries) == 0 {
		return model.NewAppError("LdapDiagnostic.RunTest", "ent.ldap.no.users.checkcertificate", nil, "", http.StatusInternalServerError)
	}
	return nil
}

// RunTestConnection checks that the given settings allow to connect and bind
// to the server.
func (d *Diagnostic) RunTestConnection(rctx request.CTX, submitted model.LdapSettings) *model.AppError {
	s := d.submittedSettings(submitted)
	ss, appErr := openSession(d.dial, s, d.p.GetConfigFile)
	if appErr != nil {
		connType := s.ConnectionSecurity
		if connType == "" {
			connType = "None"
		}
		return model.NewAppError("LdapDiagnostic.RunTestConnection", "ent.ldap.connection.test_failed", map[string]any{
			"Server":             s.Server,
			"Port":               s.Port,
			"ConnectionType":     connType,
			"PrivateKeyFilename": s.PrivateKeyFile,
			"PublicCertFilename": s.PublicCertificateFile,
			"BindUsername":       s.BindUsername,
			"Error":              appErr.Error(),
		}, "", http.StatusBadRequest).Wrap(appErr)
	}
	ss.Close()
	return nil
}

// RunTestDiagnostics runs the filter, attribute or group attribute tests
// displayed by the AD/LDAP wizard of the System Console.
func (d *Diagnostic) RunTestDiagnostics(rctx request.CTX, testType model.LdapDiagnosticTestType, submitted model.LdapSettings) ([]model.LdapDiagnosticResult, *model.AppError) {
	if !testType.IsValid() {
		return nil, model.NewAppError("LdapDiagnostic.RunTestDiagnostics", "api.ldap.invalid_test_type.app_error", map[string]any{"TestType": testType}, "", http.StatusBadRequest)
	}

	s := d.submittedSettings(submitted)
	ss, appErr := openSession(d.dial, s, d.p.GetConfigFile)
	if appErr != nil {
		return nil, appErr
	}
	defer ss.Close()

	switch testType {
	case model.LdapDiagnosticTestTypeFilters:
		return runFilterTests(ss), nil
	case model.LdapDiagnosticTestTypeAttributes:
		return runAttributeTests(ss), nil
	default:
		return runGroupAttributeTests(ss), nil
	}
}

func userSample(s *settings, e *ldap.Entry) model.LdapSampleEntry {
	return model.LdapSampleEntry{
		DN:        e.DN,
		Username:  attributeValue(e, s.UsernameAttribute),
		Email:     attributeValue(e, s.EmailAttribute),
		FirstName: attributeValue(e, s.FirstNameAttribute),
		LastName:  attributeValue(e, s.LastNameAttribute),
		ID:        attributeValue(e, s.IdAttribute),
	}
}

func groupSample(s *settings, e *ldap.Entry) model.LdapSampleEntry {
	return model.LdapSampleEntry{
		DN:          e.DN,
		ID:          attributeValue(e, s.GroupIdAttribute),
		DisplayName: attributeValue(e, s.GroupDisplayNameAttribute),
	}
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func runFilterTests(ss *session) []model.LdapDiagnosticResult {
	s := ss.s
	results := []model.LdapDiagnosticResult{}

	// Base DN: the entry must exist.
	baseResult := model.LdapDiagnosticResult{TestName: "BaseDN", TestValue: s.BaseDN, SampleResults: []model.LdapSampleEntry{}}
	if entry, err := ss.readEntry(s.BaseDN, "", []string{"1.1"}); err != nil {
		baseResult.Error = err.Error()
	} else if entry == nil {
		baseResult.Error = "the Base DN was not found"
	} else {
		baseResult.TotalCount = 1
		baseResult.SampleResults = append(baseResult.SampleResults, model.LdapSampleEntry{DN: entry.DN})
	}
	results = append(results, baseResult)

	type filterTest struct {
		name     string
		filter   string
		search   string
		isGroups bool
	}
	tests := []filterTest{
		{name: "UserFilter", filter: s.userFilter(), search: s.userFilter()},
		{name: "GroupFilter", filter: s.groupFilter(), search: s.groupFilter(), isGroups: true},
	}
	if s.GuestFilter != "" {
		tests = append(tests, filterTest{name: "GuestFilter", filter: s.GuestFilter, search: andFilters(s.userFilter(), s.GuestFilter)})
	}
	if s.AdminFilter != "" {
		tests = append(tests, filterTest{name: "AdminFilter", filter: s.AdminFilter, search: andFilters(s.userFilter(), s.AdminFilter)})
	}

	for _, t := range tests {
		res := model.LdapDiagnosticResult{TestName: t.name, TestValue: t.filter, SampleResults: []model.LdapSampleEntry{}}
		if _, err := ldap.CompileFilter(ensureParens(t.filter)); err != nil {
			res.Error = err.Error()
			results = append(results, res)
			continue
		}
		var attrs []string
		if t.isGroups {
			attrs = s.groupAttributes(false)
		} else {
			attrs = s.userAttributes()
		}
		entries, err := ss.search(t.search, attrs)
		if err != nil {
			res.Error = errorString(err)
			results = append(results, res)
			continue
		}
		res.TotalCount = len(entries)
		for i, e := range entries {
			if i >= maxDiagnosticSamples {
				break
			}
			if t.isGroups {
				res.SampleResults = append(res.SampleResults, groupSample(s, e))
			} else {
				res.SampleResults = append(res.SampleResults, userSample(s, e))
			}
		}
		results = append(results, res)
	}
	return results
}

func runAttributeTests(ss *session) []model.LdapDiagnosticResult {
	s := ss.s
	type attrTest struct {
		name string
		attr string
	}
	tests := []attrTest{
		{"IdAttribute", s.IdAttribute},
		{"LoginIdAttribute", s.LoginIdAttribute},
		{"UsernameAttribute", s.UsernameAttribute},
		{"EmailAttribute", s.EmailAttribute},
		{"FirstNameAttribute", s.FirstNameAttribute},
		{"LastNameAttribute", s.LastNameAttribute},
		{"NicknameAttribute", s.NicknameAttribute},
		{"PositionAttribute", s.PositionAttribute},
		{"PictureAttribute", s.PictureAttribute},
	}

	results := []model.LdapDiagnosticResult{}
	entries, searchErr := ss.search(s.userFilter(), s.userAttributes())

	for _, t := range tests {
		if t.attr == "" {
			continue
		}
		res := model.LdapDiagnosticResult{TestName: t.name, TestValue: t.attr, SampleResults: []model.LdapSampleEntry{}}
		if searchErr != nil {
			res.Error = searchErr.Error()
			results = append(results, res)
			continue
		}
		res.TotalCount = len(entries)

		if t.name == "PictureAttribute" {
			// Avoid downloading every picture: count the entries having one.
			withPicture, err := ss.search(andFilters(s.userFilter(), "("+t.attr+"=*)"), []string{"1.1"})
			if err != nil {
				res.Error = err.Error()
			} else {
				res.EntriesWithValue = len(withPicture)
				for i, e := range withPicture {
					if i >= maxDiagnosticSamples {
						break
					}
					res.SampleResults = append(res.SampleResults, model.LdapSampleEntry{DN: e.DN})
				}
			}
			results = append(results, res)
			continue
		}

		for _, e := range entries {
			v := attributeValue(e, t.attr)
			if strings.TrimSpace(v) == "" {
				continue
			}
			res.EntriesWithValue++
			if len(res.SampleResults) < maxDiagnosticSamples {
				sample := userSample(s, e)
				sample.AvailableAttributes = map[string]string{t.attr: v}
				res.SampleResults = append(res.SampleResults, sample)
			}
		}
		results = append(results, res)
	}
	return results
}

func runGroupAttributeTests(ss *session) []model.LdapDiagnosticResult {
	s := ss.s
	tests := []struct {
		name string
		attr string
	}{
		{"GroupDisplayNameAttribute", s.GroupDisplayNameAttribute},
		{"GroupIdAttribute", s.GroupIdAttribute},
	}

	results := []model.LdapDiagnosticResult{}
	entries, searchErr := ss.search(s.groupFilter(), s.groupAttributes(false))
	for _, t := range tests {
		if t.attr == "" {
			continue
		}
		res := model.LdapDiagnosticResult{TestName: t.name, TestValue: t.attr, SampleResults: []model.LdapSampleEntry{}}
		if searchErr != nil {
			res.Error = searchErr.Error()
			results = append(results, res)
			continue
		}
		res.TotalCount = len(entries)
		for _, e := range entries {
			v := attributeValue(e, t.attr)
			if strings.TrimSpace(v) == "" {
				continue
			}
			res.EntriesWithValue++
			if len(res.SampleResults) < maxDiagnosticSamples {
				sample := groupSample(s, e)
				sample.AvailableAttributes = map[string]string{t.attr: v}
				res.SampleResults = append(res.SampleResults, sample)
			}
		}
		results = append(results, res)
	}
	return results
}

// GetVendorNameAndVendorVersion reads the vendor information published in the
// root DSE of the server.
func (d *Diagnostic) GetVendorNameAndVendorVersion(rctx request.CTX) (string, string, error) {
	s := newSettings(&d.p.Config().LdapSettings)
	ss, appErr := openSession(d.dial, s, d.p.GetConfigFile)
	if appErr != nil {
		return "", "", appErr
	}
	defer ss.Close()

	entry, err := ss.readEntry("", "(objectClass=*)", []string{
		"vendorName", "vendorVersion", "domainControllerFunctionality", "rootDomainNamingContext",
	})
	if err != nil {
		return "", "", err
	}
	if entry == nil {
		return "", "", nil
	}

	name := attributeValue(entry, "vendorName")
	version := attributeValue(entry, "vendorVersion")
	if name == "" && (attributeValue(entry, "rootDomainNamingContext") != "" || attributeValue(entry, "domainControllerFunctionality") != "") {
		name = "Microsoft Active Directory"
		if level := attributeValue(entry, "domainControllerFunctionality"); level != "" && version == "" {
			if _, err := strconv.Atoi(level); err == nil {
				version = "domain controller functionality level " + level
			}
		}
	}
	return name, version, nil
}
