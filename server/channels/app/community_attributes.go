// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"cmp"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

// Antimatter community attributes: the user attributes a community's admins define for its members, as
// "Subcontractor" or "Site". The server's own user attributes (the access_control group) are defined in the System
// Console; these are one level down, scoped to a community, and only its members see them. See
// model.CommunityAttributesPropertyGroupName.
//
// Who may do what:
//   - the community's admins (manage_team) define its attributes and set anyone's values;
//   - a member sets their own values, except for attributes managed by the admins (managed=admin);
//   - members see each other's values, except for hidden and admin-only attributes, which only the admins and the
//     person they describe see.

func (a *App) communityAttributesGroupID(rctx request.CTX) (string, *model.AppError) {
	group, appErr := a.GetPropertyGroup(rctx, model.CommunityAttributesPropertyGroupName)
	if appErr != nil {
		return "", appErr
	}
	return group.ID, nil
}

// SessionCanManageCommunityAttributes tells whether someone administers a community's attributes and its members'
// values: its admins, and the system's.
func (a *App) SessionCanManageCommunityAttributes(session model.Session, teamID string) bool {
	return a.SessionHasPermissionToTeam(session, teamID, model.PermissionManageTeam)
}

// GetCommunityAttributes gives a community's attributes, in their order.
func (a *App) GetCommunityAttributes(rctx request.CTX, teamID string) ([]*model.PropertyField, *model.AppError) {
	groupID, appErr := a.communityAttributesGroupID(rctx)
	if appErr != nil {
		return nil, appErr
	}
	fields, appErr := a.SearchPropertyFields(rctx, groupID, model.PropertyFieldSearchOpts{
		GroupID:    groupID,
		ObjectType: model.PropertyFieldObjectTypeUser,
		TargetType: string(model.PropertyFieldTargetLevelTeam),
		TargetIDs:  []string{teamID},
		PerPage:    model.CommunityAttributesMaxFields + 5,
	})
	if appErr != nil {
		return nil, appErr
	}
	slices.SortStableFunc(fields, func(x, y *model.PropertyField) int {
		if c := cmp.Compare(communityAttributeSortOrder(x), communityAttributeSortOrder(y)); c != 0 {
			return c
		}
		return cmp.Compare(x.CreateAt, y.CreateAt)
	})
	return fields, nil
}

func communityAttributeSortOrder(field *model.PropertyField) float64 {
	order, _ := field.Attrs[model.PropertyFieldAttrSortOrder].(float64)
	return order
}

// getCommunityAttribute gives one of a community's attributes, or a 404 when it isn't one.
func (a *App) getCommunityAttribute(rctx request.CTX, groupID, teamID, fieldID string) (*model.PropertyField, *model.AppError) {
	field, appErr := a.GetPropertyField(rctx, groupID, fieldID)
	if appErr != nil || field.DeleteAt != 0 || field.TargetType != string(model.PropertyFieldTargetLevelTeam) || field.TargetID != teamID {
		return nil, model.NewAppError("getCommunityAttribute", "app.community_attributes.not_found.app_error", nil, "", http.StatusNotFound)
	}
	return field, nil
}

// CreateCommunityAttribute defines a new attribute for a community's members.
func (a *App) CreateCommunityAttribute(rctx request.CTX, teamID, userID string, field *model.PropertyField) (*model.PropertyField, *model.AppError) {
	groupID, appErr := a.communityAttributesGroupID(rctx)
	if appErr != nil {
		return nil, appErr
	}
	existing, appErr := a.GetCommunityAttributes(rctx, teamID)
	if appErr != nil {
		return nil, appErr
	}
	if len(existing) >= model.CommunityAttributesMaxFields {
		return nil, model.NewAppError("CreateCommunityAttribute", "app.community_attributes.too_many.app_error", map[string]any{"Max": model.CommunityAttributesMaxFields}, "", http.StatusBadRequest)
	}

	field.ID = ""
	field.GroupID = groupID
	field.ObjectType = model.PropertyFieldObjectTypeUser
	field.TargetType = string(model.PropertyFieldTargetLevelTeam)
	field.TargetID = teamID
	field.Protected = false
	field.LinkedFieldID = nil
	field.CreatedBy = userID
	field.UpdatedBy = userID
	if field.Attrs == nil {
		field.Attrs = model.StringInterface{}
	}
	if _, ok := field.Attrs[model.PropertyFieldAttrSortOrder]; !ok {
		order := float64(0)
		for _, f := range existing {
			order = max(order, communityAttributeSortOrder(f)+1)
		}
		field.Attrs[model.PropertyFieldAttrSortOrder] = order
	}
	if appErr = normalizeCommunityAttribute(field); appErr != nil {
		return nil, appErr
	}

	created, appErr := a.CreatePropertyField(rctx, field, false, "")
	if appErr != nil {
		return nil, appErr
	}
	a.publishCommunityAttributesUpdated(teamID, "")
	return created, nil
}

// PatchCommunityAttribute changes one of a community's attributes: its name and attrs (its type stays: its values
// wouldn't fit another).
func (a *App) PatchCommunityAttribute(rctx request.CTX, teamID, fieldID, userID string, patch *model.PropertyFieldPatch) (*model.PropertyField, *model.AppError) {
	groupID, appErr := a.communityAttributesGroupID(rctx)
	if appErr != nil {
		return nil, appErr
	}
	field, appErr := a.getCommunityAttribute(rctx, groupID, teamID, fieldID)
	if appErr != nil {
		return nil, appErr
	}
	if patch.Type != nil && *patch.Type != field.Type {
		return nil, model.NewAppError("PatchCommunityAttribute", "app.community_attributes.type_change.app_error", nil, "", http.StatusBadRequest)
	}
	if patch.Name != nil {
		field.Name = *patch.Name
	}
	if patch.Attrs != nil {
		if field.Attrs == nil {
			field.Attrs = model.StringInterface{}
		}
		for key, value := range *patch.Attrs {
			if value == nil {
				delete(field.Attrs, key)
			} else {
				field.Attrs[key] = value
			}
		}
	}
	if appErr = normalizeCommunityAttribute(field); appErr != nil {
		return nil, appErr
	}
	field.UpdatedBy = userID

	updated, _, appErr := a.UpdatePropertyField(rctx, groupID, field, false, "")
	if appErr != nil {
		return nil, appErr
	}
	a.publishCommunityAttributesUpdated(teamID, "")
	return updated, nil
}

// DeleteCommunityAttribute deletes one of a community's attributes, and with it its values.
func (a *App) DeleteCommunityAttribute(rctx request.CTX, teamID, fieldID string) *model.AppError {
	groupID, appErr := a.communityAttributesGroupID(rctx)
	if appErr != nil {
		return appErr
	}
	if _, appErr := a.getCommunityAttribute(rctx, groupID, teamID, fieldID); appErr != nil {
		return appErr
	}
	if appErr := a.DeletePropertyField(rctx, groupID, fieldID, false, ""); appErr != nil {
		return appErr
	}
	a.publishCommunityAttributesUpdated(teamID, "")
	return nil
}

// normalizeCommunityAttribute checks an attribute's name and type, and keeps only the attrs a community attribute
// has, checked: visibility, managed, value_type (text), options (select and multiselect), sort_order, member_list.
// It also sets the permission levels the attrs stand for, so that the field reads the same as any other: the
// community's admins edit it, and set its values when it's admin-managed.
func normalizeCommunityAttribute(field *model.PropertyField) *model.AppError {
	invalid := func(reason string) *model.AppError {
		return model.NewAppError("normalizeCommunityAttribute", "app.community_attributes.invalid.app_error", map[string]any{"Reason": reason}, "", http.StatusBadRequest)
	}

	field.Name = strings.TrimSpace(field.Name)
	if field.Name == "" || utf8.RuneCountInString(field.Name) > model.CommunityAttributeNameMaxRunes {
		return invalid("name")
	}
	if !slices.Contains(model.CommunityAttributeTypes, field.Type) {
		return invalid("type")
	}

	in := field.Attrs
	attrs := model.StringInterface{}

	visibility, _ := in[model.PropertyFieldAttrVisibility].(string)
	visibility = strings.TrimSpace(visibility)
	if visibility == "" {
		visibility = model.PropertyFieldVisibilityWhenSet
	}
	if !model.IsValidPropertyFieldVisibility(visibility) {
		return invalid("visibility")
	}
	attrs[model.PropertyFieldAttrVisibility] = visibility

	managed, _ := in[model.PropertyFieldAttrManaged].(string)
	if managed != "" && managed != "admin" {
		return invalid("managed")
	}
	attrs[model.PropertyFieldAttrManaged] = managed

	if field.Type == model.PropertyFieldTypeText {
		valueType, _ := in[model.PropertyFieldAttrValueType].(string)
		if valueType != "" && !model.IsValidPropertyFieldValueType(valueType) {
			return invalid("value_type")
		}
		attrs[model.PropertyFieldAttrValueType] = valueType
	}

	switch order := in[model.PropertyFieldAttrSortOrder].(type) {
	case nil:
		attrs[model.PropertyFieldAttrSortOrder] = float64(0)
	case float64:
		attrs[model.PropertyFieldAttrSortOrder] = order
	case int:
		attrs[model.PropertyFieldAttrSortOrder] = float64(order)
	case json.Number:
		value, err := order.Float64()
		if err != nil {
			return invalid("sort_order")
		}
		attrs[model.PropertyFieldAttrSortOrder] = value
	default:
		return invalid("sort_order")
	}

	if listed, ok := in[model.PropertyFieldAttrMemberList].(bool); ok && listed {
		attrs[model.PropertyFieldAttrMemberList] = true
	}

	if field.Type == model.PropertyFieldTypeSelect || field.Type == model.PropertyFieldTypeMultiselect {
		options, appErr := normalizeCommunityAttributeOptions(in[model.PropertyFieldAttributeOptions])
		if appErr != nil {
			return appErr
		}
		attrs[model.PropertyFieldAttributeOptions] = options
	}
	field.Attrs = attrs

	admin := model.PermissionLevelAdmin
	values := model.PermissionLevelMember
	if managed == "admin" {
		values = model.PermissionLevelAdmin
	}
	field.PermissionField = &admin
	field.PermissionOptions = &admin
	field.PermissionValues = &values
	return nil
}

// normalizeCommunityAttributeOptions checks a select attribute's choices, gives new ones an ID, and returns them in
// the canonical attrs form (a list of maps).
func normalizeCommunityAttributeOptions(raw any) ([]any, *model.AppError) {
	invalid := func(reason string) *model.AppError {
		return model.NewAppError("normalizeCommunityAttribute", "app.community_attributes.invalid.app_error", map[string]any{"Reason": reason}, "", http.StatusBadRequest)
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, invalid("options")
	}
	var options []*model.CustomProfileAttributesSelectOption
	if err := json.Unmarshal(data, &options); err != nil {
		return nil, invalid("options")
	}
	if len(options) == 0 || len(options) > model.CommunityAttributeMaxOptions {
		return nil, invalid("options")
	}
	seen := map[string]bool{}
	out := make([]any, 0, len(options))
	for _, option := range options {
		if option == nil {
			return nil, invalid("options")
		}
		option.Name = strings.TrimSpace(option.Name)
		if option.ID == "" {
			option.ID = model.NewId()
		}
		key := strings.ToLower(option.Name)
		if option.IsValid() != nil || seen[key] || seen[option.ID] {
			return nil, invalid("options")
		}
		seen[key] = true
		seen[option.ID] = true
		out = append(out, map[string]any{"id": option.ID, "name": option.Name, "color": option.Color})
	}
	return out, nil
}

// canSeeCommunityAttributeValue tells whether someone sees a member's value of an attribute: everyone in the
// community does, but only the admins and the member themselves see hidden and admin-only ones.
func canSeeCommunityAttributeValue(field *model.PropertyField, viewerID, userID string, admin bool) bool {
	return admin || viewerID == userID || !model.IsPropertyFieldValueRestricted(field)
}

// GetCommunityAttributeValues gives the attribute values of some of a community's members, as the session may see
// them: by user, then by field. People who aren't in the community have none.
func (a *App) GetCommunityAttributeValues(rctx request.CTX, session model.Session, teamID string, userIDs []string) (map[string]map[string]json.RawMessage, *model.AppError) {
	result := map[string]map[string]json.RawMessage{}
	if len(userIDs) == 0 {
		return result, nil
	}
	if len(userIDs) > model.CommunityAttributesMaxUserIDs {
		return nil, model.NewAppError("GetCommunityAttributeValues", "app.community_attributes.too_many_users.app_error", map[string]any{"Max": model.CommunityAttributesMaxUserIDs}, "", http.StatusBadRequest)
	}
	fields, appErr := a.GetCommunityAttributes(rctx, teamID)
	if appErr != nil || len(fields) == 0 {
		return result, appErr
	}
	fieldByID := make(map[string]*model.PropertyField, len(fields))
	for _, f := range fields {
		fieldByID[f.ID] = f
	}

	members, err := a.Srv().Store().Team().GetMembersByIds(teamID, userIDs, nil)
	if err != nil {
		return nil, model.NewAppError("GetCommunityAttributeValues", "app.team.get_members.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	memberIDs := make([]string, 0, len(members))
	for _, m := range members {
		if m.DeleteAt == 0 {
			memberIDs = append(memberIDs, m.UserId)
		}
	}
	if len(memberIDs) == 0 {
		return result, nil
	}

	values, appErr := a.searchUserAttributeValues(rctx, fields[0].GroupID, memberIDs)
	if appErr != nil {
		return nil, appErr
	}
	admin := a.SessionCanManageCommunityAttributes(session, teamID)
	for _, v := range values {
		field := fieldByID[v.FieldID]
		if field == nil || !canSeeCommunityAttributeValue(field, session.UserId, v.TargetID, admin) {
			continue
		}
		if result[v.TargetID] == nil {
			result[v.TargetID] = map[string]json.RawMessage{}
		}
		result[v.TargetID][v.FieldID] = v.Value
	}
	return result, nil
}

// searchUserAttributeValues gives all the values a group holds for some people, a page at a time: a person has
// values in each community they're in.
func (a *App) searchUserAttributeValues(rctx request.CTX, groupID string, userIDs []string) ([]*model.PropertyValue, *model.AppError) {
	const perPage = 1000
	var values []*model.PropertyValue
	opts := model.PropertyValueSearchOpts{
		GroupID:    groupID,
		TargetType: model.PropertyFieldObjectTypeUser,
		TargetIDs:  userIDs,
		PerPage:    perPage,
	}
	for {
		page, appErr := a.SearchPropertyValues(rctx, groupID, opts)
		if appErr != nil {
			return nil, appErr
		}
		values = append(values, page...)
		if len(page) < perPage {
			return values, nil
		}
		last := page[len(page)-1]
		opts.Cursor = model.PropertyValueSearchCursor{PropertyValueID: last.ID, CreateAt: last.CreateAt}
	}
}

// PatchCommunityAttributeValues sets some of a community member's attribute values (an empty value clears one),
// and gives the values set. The community's admins set any; the member sets their own, but for admin-managed ones.
func (a *App) PatchCommunityAttributeValues(rctx request.CTX, session model.Session, teamID, userID string, updates map[string]json.RawMessage) (map[string]json.RawMessage, *model.AppError) {
	if len(updates) == 0 || len(updates) > model.CommunityAttributesMaxFields {
		return nil, model.NewAppError("PatchCommunityAttributeValues", "app.community_attributes.values.count.app_error", map[string]any{"Max": model.CommunityAttributesMaxFields}, "", http.StatusBadRequest)
	}
	member, err := a.Srv().Store().Team().GetMember(rctx, teamID, userID)
	if err != nil || member.DeleteAt != 0 {
		return nil, model.NewAppError("PatchCommunityAttributeValues", "app.community_attributes.not_member.app_error", nil, "", http.StatusNotFound)
	}
	fields, appErr := a.GetCommunityAttributes(rctx, teamID)
	if appErr != nil {
		return nil, appErr
	}
	fieldByID := make(map[string]*model.PropertyField, len(fields))
	for _, f := range fields {
		fieldByID[f.ID] = f
	}

	admin := a.SessionCanManageCommunityAttributes(session, teamID)
	values := make([]*model.PropertyValue, 0, len(updates))
	for fieldID, raw := range updates {
		field := fieldByID[fieldID]
		if field == nil {
			return nil, model.NewAppError("PatchCommunityAttributeValues", "app.community_attributes.not_found.app_error", nil, "", http.StatusNotFound)
		}
		if !admin && (session.UserId != userID || field.Attrs[model.PropertyFieldAttrManaged] == "admin") {
			return nil, model.NewAppError("PatchCommunityAttributeValues", "app.community_attributes.values.forbidden.app_error", nil, "", http.StatusForbidden)
		}
		value, appErr := a.normalizeCommunityAttributeValue(rctx, field, raw)
		if appErr != nil {
			return nil, appErr
		}
		values = append(values, &model.PropertyValue{
			TargetID:   userID,
			TargetType: model.PropertyFieldObjectTypeUser,
			GroupID:    field.GroupID,
			FieldID:    fieldID,
			Value:      value,
			CreatedBy:  session.UserId,
			UpdatedBy:  session.UserId,
		})
	}

	upserted, err := a.Srv().propertyService.UpsertPropertyValues(rctx, values)
	if err != nil {
		if appErr := mapPropertyServiceError("PatchCommunityAttributeValues", err); appErr != nil {
			return nil, appErr
		}
		return nil, model.NewAppError("PatchCommunityAttributeValues", "app.property_value.upsert_many.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	result := make(map[string]json.RawMessage, len(upserted))
	for _, v := range upserted {
		result[v.FieldID] = v.Value
	}
	a.publishCommunityAttributesUpdated(teamID, userID)
	return result, nil
}

// normalizeCommunityAttributeValue checks a value against its attribute and gives it in its stored form: a string
// for text, a choice, a date or a person, a list for several choices or people. null clears a value, as an empty
// one does.
func (a *App) normalizeCommunityAttributeValue(rctx request.CTX, field *model.PropertyField, raw json.RawMessage) (json.RawMessage, *model.AppError) {
	invalid := func() *model.AppError {
		return model.NewAppError("normalizeCommunityAttributeValue", "app.community_attributes.values.invalid.app_error", map[string]any{"Name": field.Name}, "", http.StatusBadRequest)
	}
	isNull := len(raw) == 0 || string(raw) == "null"
	optionIDs := map[string]bool{}
	if options, ok := field.Attrs[model.PropertyFieldAttributeOptions].([]any); ok {
		for _, o := range options {
			if option, ok := o.(map[string]any); ok {
				if id, ok := option["id"].(string); ok {
					optionIDs[id] = true
				}
			}
		}
	}

	switch field.Type {
	case model.PropertyFieldTypeText, model.PropertyFieldTypeSelect, model.PropertyFieldTypeDate, model.PropertyFieldTypeUser:
		var text string
		if !isNull {
			if err := json.Unmarshal(raw, &text); err != nil {
				return nil, invalid()
			}
		}
		text = strings.TrimSpace(text)
		if text != "" {
			switch field.Type {
			case model.PropertyFieldTypeText:
				if utf8.RuneCountInString(text) > model.PropertyFieldValueTypeTextMaxLength {
					return nil, invalid()
				}
				if valueType := model.GetPropertyFieldValueType(field); valueType != "" {
					quoted, _ := json.Marshal(text)
					if model.ValidatePropertyValueForValueType(valueType, quoted) != nil {
						return nil, invalid()
					}
				}
			case model.PropertyFieldTypeSelect:
				if !optionIDs[text] {
					return nil, invalid()
				}
			case model.PropertyFieldTypeDate:
				if _, err := time.Parse(time.DateOnly, text); err != nil {
					return nil, invalid()
				}
			case model.PropertyFieldTypeUser:
				if !model.IsValidId(text) {
					return nil, invalid()
				}
				if _, appErr := a.GetUser(rctx, text); appErr != nil {
					return nil, invalid()
				}
			}
		}
		value, _ := json.Marshal(text)
		return value, nil

	case model.PropertyFieldTypeMultiselect, model.PropertyFieldTypeMultiuser:
		var list []string
		if !isNull {
			if err := json.Unmarshal(raw, &list); err != nil {
				return nil, invalid()
			}
		}
		ids := make([]string, 0, len(list))
		for _, id := range list {
			if !slices.Contains(ids, id) {
				ids = append(ids, id)
			}
		}
		if field.Type == model.PropertyFieldTypeMultiselect {
			for _, id := range ids {
				if !optionIDs[id] {
					return nil, invalid()
				}
			}
		} else if len(ids) > 0 {
			if len(ids) > model.CommunityAttributeMaxUsers {
				return nil, invalid()
			}
			for _, id := range ids {
				if !model.IsValidId(id) {
					return nil, invalid()
				}
			}
			users, appErr := a.GetUsers(rctx, ids)
			if appErr != nil || len(users) != len(ids) {
				return nil, invalid()
			}
		}
		value, _ := json.Marshal(ids)
		return value, nil
	}
	return nil, invalid()
}

// publishCommunityAttributesUpdated tells a community's members that its attributes changed (userID empty) or a
// member's values did. It carries no values: clients ask for those they may see.
func (a *App) publishCommunityAttributesUpdated(teamID, userID string) {
	message := model.NewWebSocketEvent(model.WebsocketEventCommunityAttributesUpdated, teamID, "", "", nil, "")
	message.Add("team_id", teamID)
	if userID != "" {
		message.Add("user_id", userID)
	}
	a.Publish(message)
}
