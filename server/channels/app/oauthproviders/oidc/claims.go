// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package oidc

import (
	"encoding/json"
	"strconv"
	"strings"
)

// ClaimValues returns the values of the claim at path, as strings, in claim
// order. A path names a top-level claim, or uses dots for nested claims
// ("address.country"). A top-level claim whose own name contains dots, such
// as a namespaced claim ("https://example.com/roles"), is matched before the
// path is split. A string, number or boolean claim gives one value, an array
// gives one per scalar element; objects, nulls and empty strings give none.
func ClaimValues(claims map[string]any, path string) []string {
	path = strings.TrimSpace(path)
	if path == "" || claims == nil {
		return nil
	}
	value, ok := lookupClaim(claims, path)
	if !ok {
		return nil
	}
	if list, isList := value.([]any); isList {
		out := make([]string, 0, len(list))
		for _, item := range list {
			if s, ok := scalarClaim(item); ok {
				out = append(out, s)
			}
		}
		return out
	}
	if s, ok := scalarClaim(value); ok {
		return []string{s}
	}
	return nil
}

// lookupClaim resolves path in claims: the whole path as a claim name first,
// then its longest dotted prefix naming an object, recursively.
func lookupClaim(claims map[string]any, path string) (any, bool) {
	if value, ok := claims[path]; ok {
		return value, true
	}
	for i := strings.LastIndex(path, "."); i > 0; i = strings.LastIndex(path[:i], ".") {
		nested, ok := claims[path[:i]].(map[string]any)
		if !ok {
			continue
		}
		if value, ok := lookupClaim(nested, path[i+1:]); ok {
			return value, true
		}
	}
	return nil, false
}

func scalarClaim(v any) (string, bool) {
	switch t := v.(type) {
	case string:
		return t, t != ""
	case json.Number:
		return t.String(), true
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64), true
	case int64:
		return strconv.FormatInt(t, 10), true
	case int:
		return strconv.Itoa(t), true
	case bool:
		return strconv.FormatBool(t), true
	}
	return "", false
}
