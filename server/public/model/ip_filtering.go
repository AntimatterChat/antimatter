package model

import (
	"fmt"
	"net/http"
	"net/netip"
	"strings"
	"unicode/utf8"
)

// IP filtering rule actions.
const (
	IPFilterActionAllow = "allow"
	IPFilterActionDeny  = "deny"
)

const IPFilterDescriptionMaxRunes = 255

// IPFilteringSettings holds the rules deciding which client addresses may reach the
// server. A client matching an enabled deny rule is refused. If any allow rule is
// enabled, the client must also match one of them. With no enabled rule, every client
// is let through.
type IPFilteringSettings struct {
	Rules AllowedIPRanges `access:"site_ip_filters"`
}

func (s *IPFilteringSettings) SetDefaults() {
	if s.Rules == nil {
		s.Rules = AllowedIPRanges{}
	}
}

func (s *IPFilteringSettings) IsValid() *AppError {
	return s.Rules.IsValid()
}

// AllowedIPRanges is the list of IP filtering rules. It keeps its historical name,
// from when only allow rules existed, since it is part of the REST API.
type AllowedIPRanges []AllowedIPRange

// AllowedIPRange is a single IP filtering rule.
type AllowedIPRange struct {
	// CIDRBlock is an IPv4 or IPv6 range in CIDR notation. A bare address stands
	// for that single host.
	CIDRBlock   string `json:"cidr_block"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
	OwnerID     string `json:"owner_id"`
	// Action is IPFilterActionAllow or IPFilterActionDeny. Empty means allow, as
	// rules saved before deny rules existed are all allow rules.
	Action string `json:"action,omitempty"`
}

// IsDeny reports whether the rule refuses the clients it matches.
func (r *AllowedIPRange) IsDeny() bool {
	return r.Action == IPFilterActionDeny
}

// Prefix parses CIDRBlock into a masked prefix. IPv4-mapped IPv6 ranges are turned
// into their IPv4 form so that they match IPv4 clients.
func (r *AllowedIPRange) Prefix() (netip.Prefix, error) {
	s := strings.TrimSpace(r.CIDRBlock)

	if !strings.Contains(s, "/") {
		addr, err := netip.ParseAddr(s)
		if err != nil {
			return netip.Prefix{}, err
		}
		if addr.Zone() != "" {
			return netip.Prefix{}, fmt.Errorf("address %q has a zone", s)
		}
		addr = addr.Unmap()
		return netip.PrefixFrom(addr, addr.BitLen()), nil
	}

	prefix, err := netip.ParsePrefix(s)
	if err != nil {
		return netip.Prefix{}, err
	}
	if prefix.Addr().Is4In6() {
		bits := prefix.Bits() - 96
		if bits < 0 {
			return netip.Prefix{}, fmt.Errorf("range %q covers more than the IPv4-mapped space", s)
		}
		prefix = netip.PrefixFrom(prefix.Addr().Unmap(), bits)
	}
	return prefix.Masked(), nil
}

func (r *AllowedIPRange) IsValid() *AppError {
	if _, err := r.Prefix(); err != nil {
		return NewAppError("AllowedIPRange.IsValid", "model.ip_filtering.is_valid.cidr_block.app_error", map[string]any{"CIDRBlock": r.CIDRBlock}, "", http.StatusBadRequest).Wrap(err)
	}

	if r.Action != "" && r.Action != IPFilterActionAllow && r.Action != IPFilterActionDeny {
		return NewAppError("AllowedIPRange.IsValid", "model.ip_filtering.is_valid.action.app_error", map[string]any{"Action": r.Action}, "", http.StatusBadRequest)
	}

	if utf8.RuneCountInString(r.Description) > IPFilterDescriptionMaxRunes {
		return NewAppError("AllowedIPRange.IsValid", "model.ip_filtering.is_valid.description.app_error", map[string]any{"Max": IPFilterDescriptionMaxRunes}, "", http.StatusBadRequest)
	}

	return nil
}

func (air AllowedIPRanges) IsValid() *AppError {
	for i := range air {
		if appErr := air[i].IsValid(); appErr != nil {
			return appErr
		}
	}
	return nil
}

func (air *AllowedIPRanges) Auditable() map[string]any {
	return map[string]any{
		"AllowedIPRanges": air,
	}
}

type GetIPAddressResponse struct {
	IP string `json:"ip"`
}
