// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {Permissions} from 'mattermost-redux/constants';
import {getCustomEmojisEnabled} from 'mattermost-redux/selectors/entities/emojis';
import {haveISystemPermission, haveITeamPermission} from 'mattermost-redux/selectors/entities/roles';
import {getMyTeams} from 'mattermost-redux/selectors/entities/teams';

import type {GlobalState} from 'types/store';

// Who may add and delete custom emoji, exactly as the server checks it (api4/emoji.go): custom emoji are on
// (EnableCustomEmoji), and the permission is the person's system role's or that of any community they're in.
// Deleting someone else's emoji takes both delete_emojis and delete_others_emojis.

// haveIEmojiPermission tells whether the person has a permission on the server or in any of their communities.
export function haveIEmojiPermission(state: GlobalState, permission: string): boolean {
    if (haveISystemPermission(state, {permission})) {
        return true;
    }
    return getMyTeams(state).some((team) => haveITeamPermission(state, team.id, permission));
}

export function canCreateCustomEmoji(state: GlobalState): boolean {
    return getCustomEmojisEnabled(state) && haveIEmojiPermission(state, Permissions.CREATE_EMOJIS);
}

export function canDeleteOwnCustomEmoji(state: GlobalState): boolean {
    return getCustomEmojisEnabled(state) && haveIEmojiPermission(state, Permissions.DELETE_EMOJIS);
}

export function canDeleteOthersCustomEmoji(state: GlobalState): boolean {
    return canDeleteOwnCustomEmoji(state) && haveIEmojiPermission(state, Permissions.DELETE_OTHERS_EMOJIS);
}

// canManageCustomEmoji: whether the picker has the "Custom emoji" (manage) button, as the integrations page shows
// custom emoji to those who may add or delete some.
export function canManageCustomEmoji(state: GlobalState): boolean {
    return canCreateCustomEmoji(state) || canDeleteOwnCustomEmoji(state);
}
