// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {ChannelForumView} from '@mattermost/types/channels';

import {savePreferences} from 'mattermost-redux/actions/preferences';
import {get as getPreference} from 'mattermost-redux/selectors/entities/preferences';
import {getCurrentUserId} from 'mattermost-redux/selectors/entities/users';

import type {ActionFuncAsync, GlobalState} from 'types/store';

// Antimatter forums: each person's own choice of the forum view (Topics or Image board), one preference per
// channel, over the channel's own am_forum_view default (design §4.4: "the setting is the channel's default, the
// switch is the person's own" — not redundant UI, since the channel's default is set elsewhere, in channel settings).
export const CATEGORY_FORUM_VIEW = 'fusion_forum_view';

export function getForumView(state: GlobalState, channelId: string, channelDefault: ChannelForumView): ChannelForumView {
    const value = getPreference(state, CATEGORY_FORUM_VIEW, channelId, '');
    return value === 'topics' || value === 'imageboard' ? value : channelDefault;
}

export function setForumView(channelId: string, view: ChannelForumView): ActionFuncAsync {
    return (dispatch, getState) => {
        const userId = getCurrentUserId(getState());
        return dispatch(savePreferences(userId, [{user_id: userId, category: CATEGORY_FORUM_VIEW, name: channelId, value: view}]));
    };
}
