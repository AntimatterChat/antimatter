// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useIntl} from 'react-intl';
import {shallowEqual, useSelector} from 'react-redux';

import type {Channel} from '@mattermost/types/channels';

import {getCurrentUserId, getUserIdsInChannels} from 'mattermost-redux/selectors/entities/users';
import {getUserIdFromChannelName} from 'mattermost-redux/utils/channel_utils';

import Icon from 'fusion/components/icon';
import {am} from 'fusion/utils/class_names';

import type {GlobalState} from 'types/store';

import {channelLabel, firstName, useChannelLabel} from './actions';
import {useCall, useIsVoiceChannel, useParticipants, useTalkingMap, useUserTalk} from './hooks';

// CallLiveMarker is the mockup's green phone after the name of a sidebar channel with a call in progress. It counts
// calls, of which Mattermost has at most one per channel.
export function CallLiveMarker({channelId}: {channelId: string}) {
    const {formatMessage} = useIntl();
    const voice = useIsVoiceChannel(channelId);
    const call = useCall(channelId);
    const people = useParticipants(channelId).length;
    if (voice || !call) {
        return null;
    }
    const calls = 1;
    return (
        <span
            className={am('call-live')}
            title={formatMessage({id: 'fusion.calls.liveTitle', defaultMessage: '{calls, plural, one {# call} other {# calls}} · {people, plural, one {# person} other {# people}}'}, {calls, people})}
        >
            <Icon name='phone'/>
            {calls}
        </span>
    );
}

const NO_USERS: string[] = [];

// The other people of a direct or group message.
function useDirectUserIds(channel: Channel): string[] {
    return useSelector((state: GlobalState) => {
        const me = getCurrentUserId(state);
        if (channel.type === 'D') {
            const teammate = channel.teammate_id || getUserIdFromChannelName(me, channel.name);
            return teammate && teammate !== me ? [teammate] : NO_USERS;
        }
        if (channel.type === 'G') {
            const ids = getUserIdsInChannels(state)[channel.id];
            return ids ? [...ids].filter((id) => id !== me) : NO_USERS;
        }
        return NO_USERS;
    }, shallowEqual);
}

// DirectTalkMarker is the mockup's green phone (or speaker) on a conversation whose people are in a call (or a voice
// channel), titled with where each of them is.
export function DirectTalkMarker({channel}: {channel: Channel}) {
    const {formatMessage} = useIntl();
    const talking = useTalkingMap();
    const userIds = useDirectUserIds(channel);
    const who = userIds.filter((id) => talking.has(id));
    const title = useSelector((state: GlobalState) => who.map((id) => {
        const talk = talking.get(id)!;
        const name = firstName(state, id);
        if (talk.kind === 'voice') {
            return formatMessage({id: 'fusion.calls.isInVoice', defaultMessage: '{name} is in {where}'}, {name, where: channelLabel(state, talk.channelId)});
        }
        return formatMessage({id: 'fusion.calls.isInCall', defaultMessage: '{name} is in a call'}, {name});
    }).join(', '));

    if (!who.length) {
        return null;
    }
    const call = who.some((id) => talking.get(id)!.kind === 'call');
    return (
        <span
            className={am('dm-talk')}
            title={title}
            aria-label={title}
        >
            <Icon
                name={call ? 'phone' : 'speaker'}
                size='xs'
            />
        </span>
    );
}

// TalkNote is the second line of a member in the member list: where they are talking, else what they say about
// themselves.
export function TalkNote({userId, children}: {userId: string; children: React.ReactNode}) {
    const {formatMessage} = useIntl();
    const talk = useUserTalk(userId);
    const where = useChannelLabel(talk?.channelId);
    if (!talk || !where) {
        return <>{children}</>;
    }
    const voice = talk.kind === 'voice';
    return (
        <span
            className={am('talk')}
            title={voice ? formatMessage({id: 'fusion.calls.inVoiceChannel', defaultMessage: 'In voice channel {where}'}, {where}) : formatMessage({id: 'fusion.calls.inCallIn', defaultMessage: 'In a call in {where}'}, {where})}
        >
            <Icon
                name={voice ? 'speaker' : 'phone'}
                size='xs'
            />
            {` ${where}`}
        </span>
    );
}
