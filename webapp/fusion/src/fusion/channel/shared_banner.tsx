// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect} from 'react';
import {useIntl} from 'react-intl';
import {shallowEqual, useDispatch, useSelector} from 'react-redux';

import type {Channel} from '@mattermost/types/channels';

import {fetchChannelRemotes} from 'mattermost-redux/actions/shared_channels';
import {getRemoteNamesForChannel} from 'mattermost-redux/selectors/entities/shared_channels';

import Icon from 'fusion/components/icon';
import {am} from 'fusion/utils/class_names';

import type {GlobalState} from 'types/store';

// SharedBanner tells, above a shared channel's messages, which servers it's shared with: the mockup's .fed-banner.
export default function SharedBanner({channel}: {channel: Channel}) {
    const {formatMessage, formatList} = useIntl();
    const dispatch = useDispatch();
    const remotes = useSelector((state: GlobalState) => getRemoteNamesForChannel(state, channel.id), shallowEqual);
    const shared = Boolean(channel.shared) && channel.type !== 'D' && channel.type !== 'G';

    useEffect(() => {
        if (shared) {
            dispatch(fetchChannelRemotes(channel.id));
        }
    }, [shared, channel.id, dispatch]);

    if (!shared) {
        return null;
    }
    const hosts = formatList(remotes, {type: 'conjunction'});
    return (
        <div
            className={am('fed-banner')}
            role='note'
        >
            <Icon
                name='globe'
                size='sm'
            />
            <span>
                {remotes.length ? formatMessage(
                    {id: 'fusion.shared.banner', defaultMessage: 'Shared channel with <b>{hosts}</b>. Messages are synced both ways; members from both servers can take part.'},
                    {hosts, b: (chunks: React.ReactNode) => <b>{chunks}</b>},
                ) : formatMessage({id: 'fusion.shared.bannerUnknown', defaultMessage: 'Shared channel. Messages are synced with other servers; their members can take part.'})}
            </span>
        </div>
    );
}
