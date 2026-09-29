// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

// Package ipfiltering decides which client addresses may reach the server, from the
// allow and deny rules in IPFilteringSettings.
//
// A client matching an enabled deny rule is always refused. If any allow rule is
// enabled, the client must also match one of them. With no enabled rule, every client
// is let through.
package ipfiltering

import (
	"fmt"
	"net/http"
	"net/netip"
	"strings"
	"sync/atomic"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
)

// Rules is a compiled, immutable set of IP filtering rules.
type Rules struct {
	allow []netip.Prefix
	deny  []netip.Prefix
}

// Compile turns the enabled rules into a Rules. Disabled rules are skipped.
func Compile(ranges model.AllowedIPRanges) (*Rules, error) {
	rules := &Rules{}
	for i := range ranges {
		r := &ranges[i]
		if !r.Enabled {
			continue
		}
		prefix, err := r.Prefix()
		if err != nil {
			return nil, fmt.Errorf("invalid IP filtering range %q: %w", r.CIDRBlock, err)
		}
		if r.IsDeny() {
			rules.deny = append(rules.deny, prefix)
		} else {
			rules.allow = append(rules.allow, prefix)
		}
	}
	return rules, nil
}

// Active reports whether any rule is enabled.
func (r *Rules) Active() bool {
	return r != nil && (len(r.allow) > 0 || len(r.deny) > 0)
}

// Allows reports whether a client with the given address may reach the server.
func (r *Rules) Allows(addr netip.Addr) bool {
	if !r.Active() {
		return true
	}
	if !addr.IsValid() {
		return false
	}
	addr = addr.Unmap().WithZone("")

	for _, p := range r.deny {
		if p.Contains(addr) {
			return false
		}
	}

	if len(r.allow) == 0 {
		return true
	}
	for _, p := range r.allow {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}

// AllowsString is Allows for an address in text form. An address that cannot be
// parsed is only let through when no rule is enabled.
func (r *Rules) AllowsString(ip string) bool {
	addr, err := netip.ParseAddr(strings.TrimSpace(ip))
	if err != nil {
		return !r.Active()
	}
	return r.Allows(addr)
}

// Filter holds the rules currently in force. It is safe for concurrent use; the rules
// are swapped atomically when the configuration changes.
type Filter struct {
	rules atomic.Pointer[Rules]
}

// Set replaces the rules in force.
func (f *Filter) Set(rules *Rules) {
	f.rules.Store(rules)
}

// Rules returns the rules in force, or nil if none were set.
func (f *Filter) Rules() *Rules {
	return f.rules.Load()
}

// Middleware refuses requests from clients the rules in force do not allow.
// clientIP extracts the client address from a request, honouring any trusted proxy
// headers.
func (f *Filter) Middleware(next http.Handler, clientIP func(*http.Request) string, logger mlog.LoggerIFace) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rules := f.rules.Load()
		if !rules.Active() {
			next.ServeHTTP(w, r)
			return
		}

		ip := clientIP(r)
		if !rules.AllowsString(ip) {
			logger.Debug("Refused request from a filtered IP address", mlog.String("ip", ip), mlog.String("path", r.URL.Path))
			http.Error(w, "Access from your network address is not allowed.", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}
