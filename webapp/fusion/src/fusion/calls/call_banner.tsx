// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useState} from 'react';
import {useIntl} from 'react-intl';
import {useSelector} from 'react-redux';

import type {Channel} from '@mattermost/types/channels';

import Avatar from 'fusion/components/avatar';
import Icon from 'fusion/components/icon';
import {am} from 'fusion/utils/class_names';

import type {GlobalState} from 'types/store';

import {firstName, useCallActions} from './actions';
import {useCall, useIsVoiceChannel, useLocalCall, useParticipants} from './hooks';

// callDuration writes how long a call has lasted, as its timer: 4:05, or 1:02:03.
export function callDuration(ms: number): string {
    const total = Math.max(0, Math.floor(ms / 1000));
    const h = Math.floor(total / 3600);
    const m = Math.floor((total % 3600) / 60);
    const s = String(total % 60).padStart(2, '0');
    return h ? `${h}:${String(m).padStart(2, '0')}:${s}` : `${m}:${s}`;
}

function CallTimer({startAt}: {startAt: number}) {
    const [now, setNow] = useState(Date.now);
    useEffect(() => {
        const timer = setInterval(() => setNow(Date.now()), 1000);
        return () => clearInterval(timer);
    }, []);
    return <span className={am('call-timer')}>{callDuration(now - startAt)}</span>;
}

// CallBanner is the banner under a conversation's header while a call is in progress there and you're not in it:
// who started it, who's in it, how long it has lasted, and a button to join it.
export default function CallBanner({channel}: {channel: Channel}) {
    const {formatMessage} = useIntl();
    const actions = useCallActions();
    const voice = useIsVoiceChannel(channel.id);
    const call = useCall(channel.id);
    const participants = useParticipants(channel.id);
    const local = useLocalCall();
    const owner = useSelector((state: GlobalState) => (call ? firstName(state, call.ownerId) : ''));

    if (voice || !call || local?.channelId === channel.id || channel.delete_at !== 0) {
        return null;
    }
    const who = call.title ? formatMessage({id: 'fusion.calls.banner.titled', defaultMessage: '“{title}” is in progress'}, {title: call.title}) : formatMessage({id: 'fusion.calls.banner.started', defaultMessage: '{name} started a call'}, {name: owner});

    return (
        <div
            className={am('call-banner')}
            role='status'
        >
            <span className={am('ph')}>
                <Icon
                    name='phone'
                    size='sm'
                />
            </span>
            <div className={am('grow')}>
                <b>{who}</b>
                <span className={am('sub')}>
                    {formatMessage({id: 'fusion.calls.banner.people', defaultMessage: '{count, plural, =0 {Nobody in it yet} one {# person in it} other {# people in it}}'}, {count: participants.length})}
                    {' · '}
                    <CallTimer startAt={call.startAt}/>
                </span>
            </div>
            <span className={am('faces')}>
                {participants.slice(0, 4).map((p) => (
                    <Avatar
                        key={p.sessionId}
                        userId={p.userId}
                        size='sm'
                    />
                ))}
            </span>
            <button
                className={am('btn', 'primary')}
                onClick={() => actions.joinAndShowCall(channel.id)}
            >
                <Icon
                    name='phone'
                    size='sm'
                />
                {formatMessage({id: 'fusion.calls.banner.join', defaultMessage: 'Join call'})}
            </button>
        </div>
    );
}
