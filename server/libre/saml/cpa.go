// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package saml

import (
	"encoding/json"
	"net/http"
	"strings"

	saml2 "github.com/mattermost/gosaml2"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/app"
)

// syncCustomProfileAttributes copies the SAML attributes mapped on custom
// profile attribute fields (field attrs "saml": "<attribute name>") into the
// user's attribute values. Only text fields can be SAML synced; a field that
// is also LDAP synced is left to the LDAP synchronization. A mapped attribute
// absent from the assertion clears the value, so that revoked attributes do not
// linger (they may drive access control policies).
func (s *Service) syncCustomProfileAttributes(rctx request.CTX, user *model.User, info *saml2.AssertionInfo) *model.AppError {
	rctx = app.RequestContextWithCallerID(rctx, model.CallerIDSAMLSync)

	group, appErr := s.b.GetPropertyGroup(rctx, model.AccessControlPropertyGroupName)
	if appErr != nil {
		return model.NewAppError("syncCustomProfileAttributes", "ent.saml.cpa_field_mapping.list_error", nil, "", http.StatusInternalServerError).Wrap(appErr)
	}

	fields, appErr := s.b.SearchPropertyFields(rctx, group.ID, model.PropertyFieldSearchOpts{
		GroupID:    group.ID,
		ObjectType: model.PropertyFieldObjectTypeUser,
		PerPage:    model.AccessControlGroupFieldLimit + 5,
	})
	if appErr != nil {
		return model.NewAppError("syncCustomProfileAttributes", "ent.saml.cpa_field_mapping.list_error", nil, "", http.StatusInternalServerError).Wrap(appErr)
	}

	type mapping struct {
		field     *model.PropertyField
		attribute string
	}
	var mappings []mapping
	for _, f := range fields {
		if f == nil || f.DeleteAt != 0 || f.Type != model.PropertyFieldTypeText {
			continue
		}
		if model.GetPropertyFieldSyncSource(f) != model.PropertyFieldAttrSAML {
			continue
		}
		attr, _ := f.Attrs[model.PropertyFieldAttrSAML].(string)
		if attr = strings.TrimSpace(attr); attr != "" {
			mappings = append(mappings, mapping{field: f, attribute: attr})
		}
	}
	if len(mappings) == 0 {
		return nil
	}

	if !hasAttributeStatement(info) {
		return model.NewAppError("syncCustomProfileAttributes", "ent.saml.update_cpa.empty_attribute_statement", nil, "", http.StatusBadRequest)
	}

	existing, appErr := s.b.SearchPropertyValues(rctx, group.ID, model.PropertyValueSearchOpts{
		TargetIDs:  []string{user.Id},
		TargetType: model.PropertyValueTargetTypeUser,
		PerPage:    model.AccessControlGroupFieldLimit + 5,
	})
	if appErr != nil {
		return appErr
	}
	existingByField := make(map[string]*model.PropertyValue, len(existing))
	for _, v := range existing {
		if v != nil && v.DeleteAt == 0 {
			existingByField[v.FieldID] = v
		}
	}

	var upserts []*model.PropertyValue
	var deletes []*model.PropertyValue
	for _, m := range mappings {
		value, ok := getAttribute(info, m.attribute)
		value = truncateRunes(strings.TrimSpace(value), model.CPAValueTypeTextMaxLength)
		current := existingByField[m.field.ID]

		if !ok || value == "" {
			if current != nil {
				deletes = append(deletes, current)
			}
			continue
		}

		if current != nil {
			var currentValue string
			if err := json.Unmarshal(current.Value, &currentValue); err == nil && currentValue == value {
				continue
			}
		}

		raw, err := json.Marshal(value)
		if err != nil {
			continue
		}
		upserts = append(upserts, &model.PropertyValue{
			GroupID:    group.ID,
			FieldID:    m.field.ID,
			TargetID:   user.Id,
			TargetType: model.PropertyValueTargetTypeUser,
			Value:      raw,
			CreatedBy:  user.Id,
			UpdatedBy:  user.Id,
		})
	}

	if len(upserts) > 0 {
		if _, appErr := s.b.UpsertPropertyValues(rctx, upserts, model.PropertyFieldObjectTypeUser, user.Id); appErr != nil {
			return appErr
		}
	}
	for _, v := range deletes {
		if appErr := s.b.DeletePropertyValue(rctx, group.ID, v.ID); appErr != nil {
			return appErr
		}
	}
	return nil
}

func hasAttributeStatement(info *saml2.AssertionInfo) bool {
	if info == nil {
		return false
	}
	if len(info.Values) > 0 {
		return true
	}
	for _, a := range info.Assertions {
		if a.AttributeStatement != nil {
			return true
		}
	}
	return false
}
