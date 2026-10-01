// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useMemo} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';
import {Link} from 'react-router-dom';

import type {Channel} from '@mattermost/types/channels';

import {savePreferences} from 'mattermost-redux/actions/preferences';
import {Preferences} from 'mattermost-redux/constants';
import {getCurrentChannelId, makeGetChannelUnreadCount} from 'mattermost-redux/selectors/entities/channels';
import {getCurrentTeam} from 'mattermost-redux/selectors/entities/teams';
import {getCurrentUserId} from 'mattermost-redux/selectors/entities/users';
import {getUserIdFromChannelName} from 'mattermost-redux/utils/channel_utils';

import {leaveDirectChannel} from 'actions/views/channel';

import Avatar from 'fusion/components/avatar';
import Icon from 'fusion/components/icon';
import {useLayout} from 'fusion/shell/layout_context';
import {am} from 'fusion/utils/class_names';
import {channelPath} from 'fusion/utils/paths';
import {getHistory} from 'utils/browser_history';

import type {GlobalState} from 'types/store';

type Props = {
    channel: Channel;

    // In the dock, conversations can be closed; in the home list they show a second line.
    dock?: boolean;
    meta?: React.ReactNode;
};

// The other person of a direct message.
export function useTeammateId(channel: Channel): string | undefined {
    const me = useSelector(getCurrentUserId);
    if (channel.type !== 'D') {
        return undefined;
    }
    return channel.teammate_id || getUserIdFromChannelName(me, channel.name);
}

export function DirectFace({channel, size = ''}: {channel: Channel; size?: '' | 'sm'}) {
    const teammateId = useTeammateId(channel);
    if (teammateId) {
        return (
            <Avatar
                userId={teammateId}
                size={size}
                status={true}
            />
        );
    }
    return <span className={am('av', 'group', size)}>{channel.display_name.split(',').length}</span>;
}

// DirectRow is a direct or group message in the dock or the home list.
export default function DirectRow({channel, dock, meta}: Props) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const layout = useLayout();
    const getUnreadCount = useMemo(() => makeGetChannelUnreadCount(), []);
    const team = useSelector(getCurrentTeam);
    const userId = useSelector(getCurrentUserId);
    const active = useSelector(getCurrentChannelId) === channel.id;
    const unread = useSelector((state: GlobalState) => getUnreadCount(state, channel.id));
    const badge = unread.mentions || (unread.showUnread ? unread.messages : 0);

    if (!team) {
        return null;
    }

    const close = () => {
        const category = channel.type === 'D' ? Preferences.CATEGORY_DIRECT_CHANNEL_SHOW : Preferences.CATEGORY_GROUP_CHANNEL_SHOW;
        const name = channel.type === 'D' ? channel.teammate_id! : channel.id;
        dispatch(savePreferences(userId, [{user_id: userId, category, name, value: 'false'}]));
        dispatch(leaveDirectChannel(channel.name));
        if (active) {
            getHistory().push(`/${team.name}`);
        }
    };

    const row = (
        <Link
            to={channelPath(team.name, channel)}
            className={am('dm', {active, unread: unread.showUnread})}
            aria-current={active ? 'page' : undefined}
            onClick={() => {
                layout.setNavOpen(false);

                // Opened from the home list, it joins the dock and the team you were in comes back.
                if (!dock) {
                    layout.setHome(false);
                }
            }}
        >
            <DirectFace
                channel={channel}
                size={dock ? 'sm' : ''}
            />
            <span className={am('name')}>
                {channel.display_name}
                {meta && <span className={am('meta')}>{meta}</span>}
            </span>
            {badge > 0 && (
                <span
                    className={am('badge')}
                    style={{boxShadow: 'none'}}
                >
                    {badge}
                </span>
            )}
        </Link>
    );

    if (!dock) {
        return row;
    }
    return (
        <div className={am('dock-row')}>
            {row}
            <button
                className={am('dock-x')}
                title={formatMessage({id: 'fusion.dock.close', defaultMessage: 'Close'})}
                aria-label={formatMessage({id: 'fusion.dock.closeLabel', defaultMessage: 'Close conversation with {name}'}, {name: channel.display_name})}
                onClick={close}
            >
                <Icon
                    name='x'
                    size='xs'
                />
            </button>
        </div>
    );
}
