// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

// Antimatter: a user attribute the System Console makes hidden or admin-only isn't only left off profile cards: its
// values are kept from everyone but the user it describes and those who administer users. See
// model.IsPropertyFieldValueRestricted. A bot's are also read by those who manage the bot: they set its attributes
// (see hasTargetAccess in api4) and act for it.

// SessionCanSeeRestrictedUserAttributes tells whether someone reads anyone's hidden and admin-only user attributes:
// those who administer the system or edit other users.
func (a *App) SessionCanSeeRestrictedUserAttributes(session model.Session) bool {
	return session.IsUnrestricted() ||
		a.SessionHasPermissionTo(session, model.PermissionManageSystem) ||
		a.SessionHasPermissionTo(session, model.PermissionEditOtherUsers)
}

// WithholdRestrictedUserAttributeValues drops, from a user's attribute values read by a session, those of hidden and
// admin-only attributes, unless the session is that user's, may see them, or manages that user as a bot. It fails
// closed: when the attributes can't be read, it keeps none.
func (a *App) WithholdRestrictedUserAttributeValues(rctx request.CTX, session model.Session, groupID, userID string, values []*model.PropertyValue) ([]*model.PropertyValue, *model.AppError) {
	if len(values) == 0 || userID == session.UserId || a.SessionCanSeeRestrictedUserAttributes(session) {
		return values, nil
	}
	fields, appErr := a.SearchPropertyFields(rctx, groupID, model.PropertyFieldSearchOpts{
		GroupID:    groupID,
		ObjectType: model.PropertyFieldObjectTypeUser,
		PerPage:    model.AccessControlGroupFieldLimit + 5,
	})
	if appErr != nil {
		return nil, appErr
	}
	restricted := make(map[string]bool, len(fields))
	for _, f := range fields {
		restricted[f.ID] = model.IsPropertyFieldValueRestricted(f)
	}
	kept := make([]*model.PropertyValue, 0, len(values))
	for _, v := range values {
		if restricted, known := restricted[v.FieldID]; known && !restricted {
			kept = append(kept, v)
		}
	}
	// Only when something would be withheld: the bot lookup costs a query.
	if len(kept) < len(values) && a.sessionManagesBotUser(rctx, session, userID) {
		return values, nil
	}
	return kept, nil
}

// sessionManagesBotUser tells whether a user ID is a bot's that the session manages: its owner with manage_bots, or
// anyone with manage_others_bots. False for anyone who isn't a bot. An agent's bot belongs to no user (see
// model.AIAgentBotOwnerId), so only manage_others_bots reaches it.
func (a *App) sessionManagesBotUser(rctx request.CTX, session model.Session, userID string) bool {
	return a.SessionHasPermissionToManageBot(rctx, session, userID) == nil
}
