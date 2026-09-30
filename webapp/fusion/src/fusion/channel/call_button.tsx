// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useIntl} from 'react-intl';
import {useSelector} from 'react-redux';

import type {Channel} from '@mattermost/types/channels';

import {getMyChannelMembership} from 'mattermost-redux/selectors/entities/channels';

import Icon from 'fusion/components/icon';
import {am} from 'fusion/utils/class_names';

import type {GlobalState} from 'types/store';

const CALLS_STATE = 'plugins-com.mattermost.calls';

type CallsState = {
    calls?: Record<string, unknown>;
    sessions?: Record<string, Record<string, unknown>>;
};

declare global {
    interface Window {
        callsClient?: {channelID: string; disconnect: () => void};
    }
}

// CallButton is the mockup's call button, driving the Calls plugin: start a call, join the one in progress, or leave.
export default function CallButton({channel}: {channel: Channel}) {
    const {formatMessage} = useIntl();
    const actions = useSelector((state: GlobalState) => state.plugins.components.CallButton || []);
    const member = useSelector((state: GlobalState) => getMyChannelMembership(state, channel.id));
    const calls = useSelector((state: GlobalState) => (state as unknown as Record<string, CallsState>)[CALLS_STATE]);

    if (!actions.length) {
        return null;
    }
    const active = Boolean(calls?.calls?.[channel.id]);
    const people = Object.keys(calls?.sessions?.[channel.id] || {}).length;
    const inThis = window.callsClient?.channelID === channel.id;
    const start = () => actions[0].action(channel, member);

    if (inThis) {
        return (
            <button
                className={am('icon-btn', 'call-btn', 'leave')}
                title={formatMessage({id: 'fusion.call.leave', defaultMessage: 'Leave the call'})}
                aria-label={formatMessage({id: 'fusion.call.leave', defaultMessage: 'Leave the call'})}
                onClick={() => window.callsClient?.disconnect()}
            >
                <Icon name='hangup'/>
            </button>
        );
    }
    if (active) {
        return (
            <button
                className={am('icon-btn', 'call-btn', 'join')}
                title={formatMessage({id: 'fusion.call.join', defaultMessage: 'Join the call'})}
                aria-label={formatMessage({id: 'fusion.call.joinCount', defaultMessage: 'Join the call, {count} in it'}, {count: people})}
                onClick={start}
            >
                <Icon name='phone'/>
                {formatMessage({id: 'fusion.call.joinShort', defaultMessage: 'Join · {count}'}, {count: people})}
            </button>
        );
    }
    return (
        <button
            className={am('icon-btn', 'call-btn', 'start')}
            title={formatMessage({id: 'fusion.call.start', defaultMessage: 'Start a call'})}
            aria-label={formatMessage({id: 'fusion.call.start', defaultMessage: 'Start a call'})}
            onClick={start}
        >
            <Icon name='phone'/>
        </button>
    );
}
