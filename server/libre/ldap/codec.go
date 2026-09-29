// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package ldap

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/mattermost/ldap"
)

// Some directory attributes (most notably Active Directory's objectGUID and
// objectSid, which are common choices for the ID attribute) hold binary
// values. Mattermost stores identifiers as text, so these values are converted
// to their canonical textual representation when read, and converted back to
// their binary form when used in a search filter.

var guidAttributes = map[string]bool{
	"objectguid":            true,
	"msexchmailboxguid":     true,
	"msexcharchiveguid":     true,
	"ms-ds-consistencyguid": true,
	"msds-consistencyguid":  true,
}

var sidAttributes = map[string]bool{
	"objectsid": true,
}

func baseAttributeName(attr string) string {
	// strip attribute options such as ";binary" or ";range=0-1499"
	if i := strings.IndexByte(attr, ';'); i >= 0 {
		attr = attr[:i]
	}
	return strings.ToLower(strings.TrimSpace(attr))
}

func isGUIDAttribute(attr string) bool { return guidAttributes[baseAttributeName(attr)] }
func isSIDAttribute(attr string) bool  { return sidAttributes[baseAttributeName(attr)] }

// findAttribute returns the attribute of the entry matching the given name,
// ignoring case and attribute options.
func findAttribute(e *ldap.Entry, name string) *ldap.EntryAttribute {
	if e == nil || name == "" {
		return nil
	}
	want := baseAttributeName(name)
	for _, a := range e.Attributes {
		if baseAttributeName(a.Name) == want {
			return a
		}
	}
	return nil
}

// attributeValues returns all the textual values of the given attribute.
func attributeValues(e *ldap.Entry, name string) []string {
	a := findAttribute(e, name)
	if a == nil {
		return nil
	}
	res := make([]string, 0, len(a.Values))
	for i, v := range a.Values {
		var raw []byte
		if i < len(a.ByteValues) {
			raw = a.ByteValues[i]
		} else {
			raw = []byte(v)
		}
		res = append(res, decodeValue(name, raw))
	}
	return res
}

// attributeValue returns the first textual value of the given attribute.
func attributeValue(e *ldap.Entry, name string) string {
	values := attributeValues(e, name)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

// rawAttributeValue returns the first raw value of the given attribute.
func rawAttributeValue(e *ldap.Entry, name string) []byte {
	a := findAttribute(e, name)
	if a == nil {
		return nil
	}
	if len(a.ByteValues) > 0 {
		return a.ByteValues[0]
	}
	if len(a.Values) > 0 {
		return []byte(a.Values[0])
	}
	return nil
}

// decodeValue converts a raw attribute value into the text representation
// used by Mattermost.
func decodeValue(attr string, raw []byte) string {
	switch {
	case isGUIDAttribute(attr) && len(raw) == 16:
		return formatGUID(raw)
	case isSIDAttribute(attr) && len(raw) >= 8:
		if sid, err := formatSID(raw); err == nil {
			return sid
		}
	}
	if !utf8.Valid(raw) {
		return hex.EncodeToString(raw)
	}
	return string(raw)
}

// filterValue converts a textual attribute value into an escaped value usable
// in a search filter, handling binary attributes.
func filterValue(attr string, value string) string {
	switch {
	case isGUIDAttribute(attr):
		if raw, err := parseGUID(value); err == nil {
			return escapeBytes(raw)
		}
	case isSIDAttribute(attr):
		if raw, err := parseSID(value); err == nil {
			return escapeBytes(raw)
		}
	}
	return ldap.EscapeFilter(value)
}

// equalityFilter builds an (attr=value) filter with proper escaping.
func equalityFilter(attr, value string) string {
	return "(" + attr + "=" + filterValue(attr, value) + ")"
}

func escapeBytes(raw []byte) string {
	var sb strings.Builder
	for _, b := range raw {
		fmt.Fprintf(&sb, "\\%02x", b)
	}
	return sb.String()
}

// formatGUID formats a 16 bytes Microsoft GUID (mixed endian) as
// xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx.
func formatGUID(b []byte) string {
	return fmt.Sprintf("%08x-%04x-%04x-%x-%x",
		binary.LittleEndian.Uint32(b[0:4]),
		binary.LittleEndian.Uint16(b[4:6]),
		binary.LittleEndian.Uint16(b[6:8]),
		b[8:10],
		b[10:16])
}

func parseGUID(s string) ([]byte, error) {
	s = strings.Trim(strings.TrimSpace(s), "{}")
	parts := strings.Split(s, "-")
	if len(parts) != 5 || len(parts[0]) != 8 || len(parts[1]) != 4 || len(parts[2]) != 4 || len(parts[3]) != 4 || len(parts[4]) != 12 {
		return nil, fmt.Errorf("invalid GUID %q", s)
	}
	d1, err := strconv.ParseUint(parts[0], 16, 32)
	if err != nil {
		return nil, err
	}
	d2, err := strconv.ParseUint(parts[1], 16, 16)
	if err != nil {
		return nil, err
	}
	d3, err := strconv.ParseUint(parts[2], 16, 16)
	if err != nil {
		return nil, err
	}
	tail, err := hex.DecodeString(parts[3] + parts[4])
	if err != nil {
		return nil, err
	}
	b := make([]byte, 16)
	binary.LittleEndian.PutUint32(b[0:4], uint32(d1))
	binary.LittleEndian.PutUint16(b[4:6], uint16(d2))
	binary.LittleEndian.PutUint16(b[6:8], uint16(d3))
	copy(b[8:], tail)
	return b, nil
}

// formatSID formats a binary security identifier as S-R-I-S-S...
func formatSID(b []byte) (string, error) {
	if len(b) < 8 {
		return "", fmt.Errorf("SID too short")
	}
	revision := b[0]
	count := int(b[1])
	if len(b) != 8+4*count {
		return "", fmt.Errorf("invalid SID length")
	}
	var authority uint64
	for i := 2; i < 8; i++ {
		authority = authority<<8 | uint64(b[i])
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "S-%d-%d", revision, authority)
	for i := range count {
		fmt.Fprintf(&sb, "-%d", binary.LittleEndian.Uint32(b[8+4*i:]))
	}
	return sb.String(), nil
}

func parseSID(s string) ([]byte, error) {
	parts := strings.Split(strings.TrimSpace(s), "-")
	if len(parts) < 3 || !strings.EqualFold(parts[0], "S") {
		return nil, fmt.Errorf("invalid SID %q", s)
	}
	revision, err := strconv.ParseUint(parts[1], 10, 8)
	if err != nil {
		return nil, err
	}
	authority, err := strconv.ParseUint(parts[2], 10, 48)
	if err != nil {
		return nil, err
	}
	subs := parts[3:]
	b := make([]byte, 8+4*len(subs))
	b[0] = byte(revision)
	b[1] = byte(len(subs))
	for i := 7; i >= 2; i-- {
		b[i] = byte(authority)
		authority >>= 8
	}
	for i, sub := range subs {
		v, err := strconv.ParseUint(sub, 10, 32)
		if err != nil {
			return nil, err
		}
		binary.LittleEndian.PutUint32(b[8+4*i:], uint32(v))
	}
	return b, nil
}

// normalizeDN returns a canonical form of a distinguished name suitable for
// comparisons: attribute types and values are lower-cased and the spacing
// around separators is removed.
func normalizeDN(dn string) string {
	parsed, err := ldap.ParseDN(dn)
	if err != nil || parsed == nil {
		return strings.ToLower(strings.TrimSpace(dn))
	}
	rdns := make([]string, 0, len(parsed.RDNs))
	for _, rdn := range parsed.RDNs {
		avas := make([]string, 0, len(rdn.Attributes))
		for _, ava := range rdn.Attributes {
			avas = append(avas, strings.ToLower(strings.TrimSpace(ava.Type))+"="+strings.ToLower(strings.TrimSpace(ava.Value)))
		}
		rdns = append(rdns, strings.Join(avas, "+"))
	}
	return strings.Join(rdns, ",")
}
