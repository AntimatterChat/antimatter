// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package ldap

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/mattermost/ldap"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

// cpaMapping describes the custom profile attributes (user attributes) whose
// values are synchronized from the directory.
type cpaMapping struct {
	groupID string
	fields  []*model.CPAField
}

// attributes returns the directory attributes used by the mapping.
func (m *cpaMapping) attributes() []string {
	if m == nil {
		return nil
	}
	attrs := make([]string, 0, len(m.fields))
	for _, f := range m.fields {
		attrs = append(attrs, f.Attrs.LDAP)
	}
	return attrs
}

// loadCPAMapping returns the custom profile attribute fields synchronized
// from the directory, or nil when there are none.
func (l *Ldap) loadCPAMapping(rctx request.CTX) (*cpaMapping, error) {
	groupID, err := l.b.GetCPAGroupID(rctx)
	if err != nil {
		return nil, err
	}
	fields, err := l.b.GetCPAFields(rctx, groupID)
	if err != nil {
		return nil, err
	}

	m := &cpaMapping{groupID: groupID}
	for _, pf := range fields {
		if pf.DeleteAt != 0 {
			continue
		}
		field, err := model.NewCPAFieldFromPropertyField(pf)
		if err != nil {
			rctx.Logger().Debug("Skipping invalid user attribute field", mlog.String("field_id", pf.ID), mlog.Err(err))
			continue
		}
		if strings.TrimSpace(field.Attrs.LDAP) == "" {
			continue
		}
		switch field.Type {
		case model.PropertyFieldTypeText, model.PropertyFieldTypeSelect, model.PropertyFieldTypeMultiselect:
			m.fields = append(m.fields, field)
		default:
			rctx.Logger().Debug("User attribute type not supported for AD/LDAP synchronization", mlog.String("field_id", field.ID), mlog.String("type", string(field.Type)))
		}
	}
	if len(m.fields) == 0 {
		return nil, nil
	}
	return m, nil
}

// cpaValue computes the JSON value of a field from the directory entry. It
// returns nil when the directory has no value for the field.
func cpaValue(field *model.CPAField, entry *ldap.Entry) json.RawMessage {
	values := attributeValues(entry, field.Attrs.LDAP)
	nonEmpty := make([]string, 0, len(values))
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			nonEmpty = append(nonEmpty, v)
		}
	}
	if len(nonEmpty) == 0 {
		return nil
	}

	optionID := func(name string) string {
		for _, o := range field.Attrs.Options {
			if o != nil && strings.EqualFold(o.Name, name) {
				return o.ID
			}
		}
		return ""
	}

	var v any
	switch field.Type {
	case model.PropertyFieldTypeText:
		text := nonEmpty[0]
		if len([]rune(text)) > model.CPAValueTypeTextMaxLength {
			text = string([]rune(text)[:model.CPAValueTypeTextMaxLength])
		}
		v = text
	case model.PropertyFieldTypeSelect:
		id := optionID(nonEmpty[0])
		if id == "" {
			return nil
		}
		v = id
	case model.PropertyFieldTypeMultiselect:
		ids := []string{}
		seen := map[string]bool{}
		for _, name := range nonEmpty {
			if id := optionID(name); id != "" && !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
		if len(ids) == 0 {
			return nil
		}
		v = ids
	default:
		return nil
	}

	raw, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return raw
}

func sameJSON(a, b json.RawMessage) bool {
	var ca, cb bytes.Buffer
	if json.Compact(&ca, a) != nil || json.Compact(&cb, b) != nil {
		return bytes.Equal(a, b)
	}
	return bytes.Equal(ca.Bytes(), cb.Bytes())
}

// syncCPAWithMapping writes the values of the synchronized fields for a user,
// only touching the values that changed.
func (l *Ldap) syncCPAWithMapping(rctx request.CTX, m *cpaMapping, userID string, entry *ldap.Entry) error {
	if m == nil || entry == nil {
		return nil
	}

	existing, err := l.b.GetCPAValues(rctx, m.groupID, userID)
	if err != nil {
		return err
	}
	byField := make(map[string]*model.PropertyValue, len(existing))
	for _, v := range existing {
		if v.DeleteAt == 0 {
			byField[v.FieldID] = v
		}
	}

	var upserts []*model.PropertyValue
	for _, field := range m.fields {
		value := cpaValue(field, entry)
		current := byField[field.ID]
		switch {
		case value == nil && current != nil:
			if err := l.b.DeleteCPAValue(rctx, m.groupID, current.ID); err != nil {
				rctx.Logger().Warn("Failed to delete user attribute removed from AD/LDAP", mlog.String("field_id", field.ID), mlog.Err(err))
			}
		case value == nil:
			// nothing to do
		case current != nil && sameJSON(current.Value, value):
			// unchanged
		default:
			upserts = append(upserts, &model.PropertyValue{
				TargetID:   userID,
				TargetType: model.PropertyFieldObjectTypeUser,
				GroupID:    m.groupID,
				FieldID:    field.ID,
				Value:      value,
			})
		}
	}

	if len(upserts) == 0 {
		return nil
	}
	return l.b.UpsertCPAValues(rctx, userID, upserts)
}
