// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package ldap

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"

	ber "github.com/go-asn1-ber/asn1-ber"
	"github.com/mattermost/ldap"

	"github.com/mattermost/mattermost/server/public/model"
)

// fakeDir is an in-memory directory server used by the tests. It evaluates
// the filters compiled by the ldap library, so that the real filters built by
// the implementation are exercised.
type fakeDir struct {
	mu sync.Mutex

	entries   []*fakeEntry
	passwords map[string]string // normalized DN -> password

	adminDN       string
	adminPassword string

	// rangeSize, when positive, emulates Active Directory ranged retrieval
	// of member attributes.
	rangeSize int

	dialErr    *model.AppError
	pageSizes  []uint32
	searches   []string
	openConns  int
	totalDials int
}

type fakeEntry struct {
	dn    string
	attrs map[string][][]byte // lower-cased attribute name -> values
	names map[string]string   // lower-cased attribute name -> original name
}

func newFakeDir() *fakeDir {
	return &fakeDir{
		passwords:     map[string]string{},
		adminDN:       "cn=admin,dc=example,dc=com",
		adminPassword: "adminpwd",
	}
}

func (d *fakeDir) add(dn string, attrs map[string][]string) *fakeEntry {
	e := &fakeEntry{dn: dn, attrs: map[string][][]byte{}, names: map[string]string{}}
	for name, values := range attrs {
		for _, v := range values {
			e.set(name, []byte(v))
		}
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.entries = append(d.entries, e)
	return e
}

func (d *fakeDir) remove(dn string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	norm := normalizeDN(dn)
	for i, e := range d.entries {
		if normalizeDN(e.dn) == norm {
			d.entries = append(d.entries[:i], d.entries[i+1:]...)
			return
		}
	}
}

func (d *fakeDir) get(dn string) *fakeEntry {
	d.mu.Lock()
	defer d.mu.Unlock()
	norm := normalizeDN(dn)
	for _, e := range d.entries {
		if normalizeDN(e.dn) == norm {
			return e
		}
	}
	return nil
}

func (e *fakeEntry) set(name string, value []byte) {
	key := strings.ToLower(name)
	e.names[key] = name
	e.attrs[key] = append(e.attrs[key], value)
}

func (e *fakeEntry) replace(name string, values ...string) {
	key := strings.ToLower(name)
	delete(e.attrs, key)
	for _, v := range values {
		e.set(name, []byte(v))
	}
}

func (d *fakeDir) setPassword(dn, password string) {
	d.passwords[normalizeDN(dn)] = password
}

func (d *fakeDir) dialer() Dialer {
	return func(_ *settings, _ func(string) ([]byte, error)) (Conn, *model.AppError) {
		d.mu.Lock()
		defer d.mu.Unlock()
		if d.dialErr != nil {
			return nil, d.dialErr
		}
		d.openConns++
		d.totalDials++
		return &fakeConn{d: d}, nil
	}
}

type fakeConn struct {
	d      *fakeDir
	closed bool
}

func (c *fakeConn) Bind(username, password string) error {
	c.d.mu.Lock()
	defer c.d.mu.Unlock()
	if normalizeDN(username) == normalizeDN(c.d.adminDN) && password == c.d.adminPassword {
		return nil
	}
	if pwd, ok := c.d.passwords[normalizeDN(username)]; ok && pwd == password && password != "" {
		return nil
	}
	return ldap.NewError(ldap.LDAPResultInvalidCredentials, errors.New("invalid credentials"))
}

func (c *fakeConn) ExternalBind() error { return nil }

func (c *fakeConn) Close() {
	if !c.closed {
		c.closed = true
		c.d.mu.Lock()
		c.d.openConns--
		c.d.mu.Unlock()
	}
}

func (c *fakeConn) SearchWithPaging(req *ldap.SearchRequest, pagingSize uint32) (*ldap.SearchResult, error) {
	c.d.mu.Lock()
	c.d.pageSizes = append(c.d.pageSizes, pagingSize)
	c.d.mu.Unlock()
	return c.Search(req)
}

func (c *fakeConn) Search(req *ldap.SearchRequest) (*ldap.SearchResult, error) {
	d := c.d
	d.mu.Lock()
	defer d.mu.Unlock()
	d.searches = append(d.searches, req.Filter)

	filter, err := ldap.CompileFilter(req.Filter)
	if err != nil {
		return nil, err
	}

	base := normalizeDN(req.BaseDN)
	baseFound := base == ""
	res := &ldap.SearchResult{}
	for _, e := range d.entries {
		dn := normalizeDN(e.dn)
		if dn == base {
			baseFound = true
		}
		switch req.Scope {
		case ldap.ScopeBaseObject:
			if dn != base {
				continue
			}
		default:
			if base != "" && dn != base && !strings.HasSuffix(dn, ","+base) {
				continue
			}
		}
		if !matchPacket(filter, e) {
			continue
		}
		if req.SizeLimit > 0 && len(res.Entries) >= req.SizeLimit {
			return nil, ldap.NewError(ldap.LDAPResultSizeLimitExceeded, errors.New("size limit exceeded"))
		}
		res.Entries = append(res.Entries, d.project(e, req.Attributes))
	}
	if !baseFound {
		return nil, ldap.NewError(ldap.LDAPResultNoSuchObject, errors.New("no such object"))
	}
	return res, nil
}

// project returns the requested attributes of the entry.
func (d *fakeDir) project(e *fakeEntry, requested []string) *ldap.Entry {
	out := &ldap.Entry{DN: e.dn}
	add := func(name string, values [][]byte) {
		a := &ldap.EntryAttribute{Name: name}
		for _, v := range values {
			a.Values = append(a.Values, string(v))
			a.ByteValues = append(a.ByteValues, v)
		}
		out.Attributes = append(out.Attributes, a)
	}

	if len(requested) == 0 {
		for key, values := range e.attrs {
			add(e.names[key], values)
		}
		return out
	}

	for _, r := range requested {
		if r == "1.1" {
			continue
		}
		name, opts, _ := strings.Cut(r, ";")
		key := strings.ToLower(name)
		values, ok := e.attrs[key]
		if !ok {
			continue
		}
		isMember := key == "member" || key == "uniquemember"
		if isMember && d.rangeSize > 0 {
			start := 0
			if strings.HasPrefix(strings.ToLower(opts), "range=") {
				start, _ = strconv.Atoi(strings.SplitN(opts[len("range="):], "-", 2)[0])
			}
			if start > len(values) {
				start = len(values)
			}
			if len(values)-start > d.rangeSize || start > 0 {
				end := min(start+d.rangeSize, len(values))
				suffix := strconv.Itoa(end - 1)
				if end == len(values) {
					suffix = "*"
				}
				add(fmt.Sprintf("%s;range=%d-%s", e.names[key], start, suffix), values[start:end])
				continue
			}
		}
		add(e.names[key], values)
	}
	return out
}

func packetString(p *ber.Packet) string {
	switch v := p.Value.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	}
	if p.Data != nil {
		return p.Data.String()
	}
	return ""
}

func matchPacket(p *ber.Packet, e *fakeEntry) bool {
	switch p.Tag {
	case ldap.FilterAnd:
		for _, c := range p.Children {
			if !matchPacket(c, e) {
				return false
			}
		}
		return true
	case ldap.FilterOr:
		for _, c := range p.Children {
			if matchPacket(c, e) {
				return true
			}
		}
		return false
	case ldap.FilterNot:
		return !matchPacket(p.Children[0], e)
	case ldap.FilterPresent:
		key := strings.ToLower(packetString(p))
		if key == "objectclass" {
			return true
		}
		return len(e.attrs[key]) > 0
	case ldap.FilterEqualityMatch, ldap.FilterApproxMatch:
		key := strings.ToLower(packetString(p.Children[0]))
		want := packetString(p.Children[1])
		for _, v := range e.attrs[key] {
			if key == "member" || key == "uniquemember" {
				if normalizeDN(string(v)) == normalizeDN(want) {
					return true
				}
				continue
			}
			if isGUIDAttribute(key) || isSIDAttribute(key) {
				if string(v) == want {
					return true
				}
				continue
			}
			if strings.EqualFold(string(v), want) {
				return true
			}
		}
		return false
	case ldap.FilterSubstrings:
		key := strings.ToLower(packetString(p.Children[0]))
		for _, v := range e.attrs[key] {
			if matchSubstrings(strings.ToLower(string(v)), p.Children[1].Children) {
				return true
			}
		}
		return false
	case ldap.FilterGreaterOrEqual, ldap.FilterLessOrEqual:
		key := strings.ToLower(packetString(p.Children[0]))
		want := packetString(p.Children[1])
		for _, v := range e.attrs[key] {
			cmp := strings.Compare(strings.ToLower(string(v)), strings.ToLower(want))
			if (p.Tag == ldap.FilterGreaterOrEqual && cmp >= 0) || (p.Tag == ldap.FilterLessOrEqual && cmp <= 0) {
				return true
			}
		}
		return false
	}
	return false
}

func matchSubstrings(value string, parts []*ber.Packet) bool {
	pos := 0
	for _, part := range parts {
		s := strings.ToLower(packetString(part))
		switch part.Tag {
		case ldap.FilterSubstringsInitial:
			if !strings.HasPrefix(value, s) {
				return false
			}
			pos = len(s)
		case ldap.FilterSubstringsAny:
			i := strings.Index(value[pos:], s)
			if i < 0 {
				return false
			}
			pos += i + len(s)
		case ldap.FilterSubstringsFinal:
			if !strings.HasSuffix(value[pos:], s) {
				return false
			}
		}
	}
	return true
}

// unableToConnect is a dial error used by the tests.
func unableToConnect() *model.AppError {
	return model.NewAppError("dial", "ent.ldap.do_login.unable_to_connect.app_error", nil, "", http.StatusInternalServerError)
}
