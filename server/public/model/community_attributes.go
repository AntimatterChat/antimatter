// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

// Antimatter: community attributes are the user attributes a community's admins define for its members (who they
// work for, their site…), shown on profiles within that community only. Each is a property field of the
// community_attributes group, of object type user, scoped to the community (target type team): the same model as
// the server's user attributes, one level down. They have their own routes, which check who may see and set what;
// the generic properties API doesn't serve the group.

const (
	CommunityAttributesPropertyGroupName = "community_attributes"

	// CommunityAttributesMaxFields is how many attributes a community may define.
	CommunityAttributesMaxFields = 50

	// CommunityAttributeNameMaxRunes is the longest name of an attribute: a label, as "Subcontractor".
	CommunityAttributeNameMaxRunes = 64

	// CommunityAttributeMaxOptions is how many choices a select attribute may offer.
	CommunityAttributeMaxOptions = 100

	// CommunityAttributeMaxUsers is how many people a multi-user attribute may name.
	CommunityAttributeMaxUsers = 20

	// CommunityAttributesMaxUserIDs is how many people's values one request may ask for.
	CommunityAttributesMaxUserIDs = 200

	// PropertyFieldAttrMemberList marks the attribute the member list shows under people's names (a bool).
	PropertyFieldAttrMemberList = "member_list"
)

// CommunityAttributeTypes are the kinds of community attributes: text (which may hold an email, a URL or a phone
// number, as its value type says), a choice, several choices, a date (YYYY-MM-DD), a person or several.
var CommunityAttributeTypes = []PropertyFieldType{
	PropertyFieldTypeText,
	PropertyFieldTypeSelect,
	PropertyFieldTypeMultiselect,
	PropertyFieldTypeDate,
	PropertyFieldTypeUser,
	PropertyFieldTypeMultiuser,
}

// CommunityAttributeValuesSearch asks for the attribute values of some of a community's members.
type CommunityAttributeValuesSearch struct {
	UserIDs []string `json:"user_ids"`
}
