// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useIntl} from 'react-intl';
import {shallowEqual, useSelector} from 'react-redux';

import {createSelector} from 'mattermost-redux/selectors/create_selector';
import {shouldShowUnreadsCategory} from 'mattermost-redux/selectors/entities/preferences';

import {getUnreadChannels} from 'selectors/views/channel_sidebar';

import {am} from 'fusion/utils/class_names';

import type {GlobalState} from 'types/store';

import ChannelRow from './channel_row';

const NONE: string[] = [];

// The team's unread channels, those with mentions first: direct messages stay in the dock of open conversations.
const getUnreadTeamChannelIds = createSelector(
    'fusionGetUnreadTeamChannelIds',
    getUnreadChannels,
    (channels) => channels.filter((channel) => channel.type !== 'D' && channel.type !== 'G').map((channel) => channel.id),
);

// UnreadCategory gathers the unread channels above the categories, when the person groups unread channels
// separately (the "Group unread channels" setting, off by default). They leave their categories meanwhile.
export default function UnreadCategory() {
    const {formatMessage} = useIntl();
    const channelIds = useSelector((state: GlobalState) => (shouldShowUnreadsCategory(state) ? getUnreadTeamChannelIds(state) : NONE), shallowEqual);

    if (!channelIds.length) {
        return null;
    }
    return (
        <>
            <div className={am('cat', 'unreads')}>
                <span className={am('grow')}>{formatMessage({id: 'fusion.sidebar.unreads', defaultMessage: 'Unreads'})}</span>
            </div>
            {channelIds.map((id) => (
                <ChannelRow
                    key={id}
                    channelId={id}
                />
            ))}
        </>
    );
}
