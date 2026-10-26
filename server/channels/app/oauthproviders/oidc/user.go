// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package oidc

import (
	"strconv"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
)

const (
	// MicrosoftLoginURL is the Microsoft identity platform authority.
	MicrosoftLoginURL = "https://login.microsoftonline.com"
	// GoogleIssuer is the issuer of Google ID tokens.
	GoogleIssuer = "https://accounts.google.com"
	// GoogleJWKSURL is where Google publishes its signing keys.
	GoogleJWKSURL = "https://www.googleapis.com/oauth2/v3/certs"
	// GoogleDiscoveryURL is Google's OpenID Connect discovery document.
	GoogleDiscoveryURL = "https://accounts.google.com/.well-known/openid-configuration"
)

// MicrosoftTenantOrCommon returns the tenant segment to use in Microsoft
// identity platform URLs.
func MicrosoftTenantOrCommon(tenant string) string {
	tenant = strings.TrimSpace(tenant)
	if tenant == "" {
		return "common"
	}
	return tenant
}

// IsMicrosoftMultiTenant reports whether the tenant segment is one of the
// Microsoft multi-tenant aliases, which accept users from any tenant.
func IsMicrosoftMultiTenant(tenant string) bool {
	switch strings.ToLower(strings.TrimSpace(tenant)) {
	case "", "common", "organizations", "consumers":
		return true
	}
	return false
}

// MicrosoftDiscoveryURL returns the v2.0 discovery document URL of a tenant.
func MicrosoftDiscoveryURL(tenant string) string {
	return MicrosoftLoginURL + "/" + MicrosoftTenantOrCommon(tenant) + "/v2.0/.well-known/openid-configuration"
}

// MicrosoftJWKSURL returns the v2.0 signing keys URL of a tenant.
func MicrosoftJWKSURL(tenant string) string {
	return MicrosoftLoginURL + "/" + MicrosoftTenantOrCommon(tenant) + "/discovery/v2.0/keys"
}

// MicrosoftIssuer returns the v2.0 issuer for a tenant id.
func MicrosoftIssuer(tid string) string {
	return MicrosoftLoginURL + "/" + tid + "/v2.0"
}

// EmailLocalPart returns the part of an email address before the "@".
func EmailLocalPart(email string) string {
	local, _, _ := strings.Cut(email, "@")
	return local
}

// LooksLikeEmail is a cheap check used to decide whether an identifier such as
// a UPN can stand in for a missing email claim.
func LooksLikeEmail(s string) bool {
	local, domain, ok := strings.Cut(s, "@")
	return ok && local != "" && strings.Contains(domain, ".") && model.IsValidEmail(strings.ToLower(s))
}

// SplitName splits a display name into first and last name.
func SplitName(name string) (first, last string) {
	parts := strings.Fields(name)
	switch len(parts) {
	case 0:
		return "", ""
	case 1:
		return parts[0], ""
	default:
		return parts[0], strings.Join(parts[1:], " ")
	}
}

// Username chooses the Mattermost username: the preferred username (up to the
// "@") when requested and available, the local part of the email otherwise.
func Username(logger mlog.LoggerIFace, preferredUsername, email string, usePreferred bool) string {
	candidate := ""
	if usePreferred && preferredUsername != "" {
		candidate = EmailLocalPart(preferredUsername)
	}
	if candidate == "" {
		candidate = EmailLocalPart(email)
	}
	if candidate == "" {
		candidate = preferredUsername
	}
	return model.CleanUsername(logger, candidate)
}

// Identity is the normalized set of identity claims Mattermost cares about,
// extracted from an ID token, an access token or a userinfo response.
type Identity struct {
	Subject           string
	ObjectID          string // Microsoft "oid"
	TenantID          string // Microsoft "tid"
	Email             string
	GivenName         string
	FamilyName        string
	Name              string
	PreferredUsername string
	UPN               string
	// EmailVerified is the email_verified claim: "true", "false", or "" when
	// the provider didn't send it.
	EmailVerified string
	// EmailDomainVerified is Microsoft's xms_edov claim (the owner of the
	// email's domain is verified): "true", "false" or "".
	EmailDomainVerified string
	// TenantUnrestricted is set by the provider, not read from a claim: the
	// sign-in accepts users of any Microsoft tenant, whose admins choose
	// their users' email addresses.
	TenantUnrestricted bool
}

// boolClaim gives a boolean claim as "true", "false", or "" when absent.
func boolClaim(c Claims, name string) string {
	value, present := Bool(c, name)
	if !present {
		return ""
	}
	return strconv.FormatBool(value)
}

// IdentityFromClaims extracts the identity claims. It understands both the
// standard OpenID Connect claim names and the Microsoft Graph /me names.
func IdentityFromClaims(c Claims) Identity {
	first := func(names ...string) string {
		for _, n := range names {
			if v := String(c, n); v != "" {
				return v
			}
		}
		return ""
	}
	return Identity{
		Subject:             first("sub"),
		ObjectID:            first("oid"),
		TenantID:            first("tid"),
		Email:               strings.ToLower(first("email", "mail")),
		GivenName:           first("given_name", "givenName"),
		FamilyName:          first("family_name", "surname"),
		Name:                first("name", "displayName"),
		PreferredUsername:   first("preferred_username"),
		UPN:                 first("upn", "userPrincipalName", "unique_name"),
		EmailVerified:       boolClaim(c, "email_verified"),
		EmailDomainVerified: boolClaim(c, "xms_edov"),
	}
}

// Merge fills the empty fields of i with the values of other.
func (i Identity) Merge(other Identity) Identity {
	fill := func(dst *string, src string) {
		if *dst == "" {
			*dst = src
		}
	}
	fill(&i.Subject, other.Subject)
	fill(&i.ObjectID, other.ObjectID)
	fill(&i.TenantID, other.TenantID)
	fill(&i.Email, other.Email)
	fill(&i.GivenName, other.GivenName)
	fill(&i.FamilyName, other.FamilyName)
	fill(&i.Name, other.Name)
	fill(&i.PreferredUsername, other.PreferredUsername)
	fill(&i.UPN, other.UPN)
	fill(&i.EmailVerified, other.EmailVerified)
	fill(&i.EmailDomainVerified, other.EmailDomainVerified)
	i.TenantUnrestricted = i.TenantUnrestricted || other.TenantUnrestricted
	return i
}

// EmailUnverified tells whether the email claim (Email, not a UPN standing in
// for it) can't be trusted as the user's own address: the provider says it
// isn't verified, or a Microsoft sign-in open to any tenant gives an address
// in a domain the user's tenant isn't known to own. Such a tenant's admins
// can give their users any address (the "nOAuth" attack), so the address is
// trusted only with xms_edov=true or in the UPN's domain, which the tenant
// has verified. A provider that doesn't send email_verified is trusted.
func (i Identity) EmailUnverified() bool {
	if i.Email == "" {
		return false
	}
	if i.EmailVerified == "false" {
		return true
	}
	if !i.TenantUnrestricted || i.EmailDomainVerified == "true" {
		return false
	}
	_, emailDomain, _ := strings.Cut(i.Email, "@")
	for _, candidate := range []string{i.UPN, i.PreferredUsername} {
		if LooksLikeEmail(candidate) {
			_, domain, _ := strings.Cut(strings.ToLower(candidate), "@")
			return domain != emailDomain
		}
	}
	return true
}

// BestEmail returns the email claim, falling back to the UPN or the preferred
// username when they are email addresses (Microsoft Entra ID often omits the
// email claim).
func (i Identity) BestEmail() string {
	if i.Email != "" {
		return i.Email
	}
	for _, candidate := range []string{i.UPN, i.PreferredUsername} {
		if LooksLikeEmail(candidate) {
			return strings.ToLower(candidate)
		}
	}
	return ""
}

// Names returns the first and last name.
func (i Identity) Names() (string, string) {
	if i.GivenName != "" || i.FamilyName != "" {
		return i.GivenName, i.FamilyName
	}
	return SplitName(i.Name)
}

const propPrefix = "antimatter_oidc_"

// ToUser converts the identity to a user whose Props carry the raw identity
// so that it can be recovered with IdentityFromUser. Such users are only
// used as the transient "token user" handed back to GetUserFromJSON and are
// never persisted.
func (i Identity) ToUser(logger mlog.LoggerIFace, authData string, usePreferred bool) *model.User {
	first, last := i.Names()
	email := i.BestEmail()
	user := &model.User{
		Email:     email,
		FirstName: first,
		LastName:  last,
		Username:  Username(logger, i.PreferredUsername, email, usePreferred),
		AuthData:  model.NewPointer(authData),
		Props: model.StringMap{
			propPrefix + "sub":                 i.Subject,
			propPrefix + "oid":                 i.ObjectID,
			propPrefix + "tid":                 i.TenantID,
			propPrefix + "email":               i.Email,
			propPrefix + "given_name":          i.GivenName,
			propPrefix + "family_name":         i.FamilyName,
			propPrefix + "name":                i.Name,
			propPrefix + "preferred_username":  i.PreferredUsername,
			propPrefix + "upn":                 i.UPN,
			propPrefix + "email_verified":      i.EmailVerified,
			propPrefix + "xms_edov":            i.EmailDomainVerified,
			propPrefix + "tenant_unrestricted": strconv.FormatBool(i.TenantUnrestricted),
		},
	}
	return user
}

// IdentityFromUser recovers the identity stored by ToUser. It returns false
// when the user does not carry one.
func IdentityFromUser(u *model.User) (Identity, bool) {
	if u == nil || u.Props == nil {
		return Identity{}, false
	}
	if _, ok := u.Props[propPrefix+"sub"]; !ok {
		return Identity{}, false
	}
	p := u.Props
	return Identity{
		Subject:             p[propPrefix+"sub"],
		ObjectID:            p[propPrefix+"oid"],
		TenantID:            p[propPrefix+"tid"],
		Email:               p[propPrefix+"email"],
		GivenName:           p[propPrefix+"given_name"],
		FamilyName:          p[propPrefix+"family_name"],
		Name:                p[propPrefix+"name"],
		PreferredUsername:   p[propPrefix+"preferred_username"],
		UPN:                 p[propPrefix+"upn"],
		EmailVerified:       p[propPrefix+"email_verified"],
		EmailDomainVerified: p[propPrefix+"xms_edov"],
		TenantUnrestricted:  p[propPrefix+"tenant_unrestricted"] == "true",
	}, true
}
