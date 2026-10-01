// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useIntl} from 'react-intl';
import {useSelector} from 'react-redux';

import type {Channel} from '@mattermost/types/channels';

import {useCallActions} from 'fusion/calls/actions';
import {getCallsAPI} from 'fusion/calls/calls_api';
import {useCall, useCallsAvailable, useIsVoiceChannel, useLocalCall, useParticipants} from 'fusion/calls/hooks';
import Icon from 'fusion/components/icon';
import {am} from 'fusion/utils/class_names';

import type {GlobalState} from 'types/store';

// CallButton is the mockup's call button of text channels and direct messages: start a call, join the one in
// progress, or leave it. Voice channels are always-open rooms and have none.
export default function CallButton({channel}: {channel: Channel}) {
    const {formatMessage} = useIntl();
    const actions = useCallActions();
    const available = useCallsAvailable();
    const voice = useIsVoiceChannel(channel.id);
    const enabled = useSelector((state: GlobalState) => Boolean(getCallsAPI()?.selectors.isCallsEnabled(state, channel.id)));
    const call = useCall(channel.id);
    const people = useParticipants(channel.id).length;
    const local = useLocalCall();

    if (!available || voice) {
        return null;
    }

    if (local?.channelId === channel.id) {
        return (
            <button
                className={am('icon-btn', 'call-btn', 'leave')}
                title={formatMessage({id: 'fusion.calls.leave', defaultMessage: 'Leave the call'})}
                aria-label={formatMessage({id: 'fusion.calls.leave', defaultMessage: 'Leave the call'})}
                onClick={actions.leaveCall}
            >
                <Icon name='hangup'/>
            </button>
        );
    }
    if (call) {
        return (
            <button
                className={am('icon-btn', 'call-btn', 'join')}
                title={formatMessage({id: 'fusion.calls.join', defaultMessage: 'Join the call'})}
                aria-label={formatMessage({id: 'fusion.calls.joinCount', defaultMessage: 'Join the call, {count} in it'}, {count: people})}
                onClick={() => actions.startOrJoinCall(channel.id)}
            >
                <Icon name='phone'/>
                {formatMessage({id: 'fusion.calls.joinShort', defaultMessage: 'Join · {count}'}, {count: people})}
            </button>
        );
    }
    if (!enabled) {
        return null;
    }
    return (
        <button
            className={am('icon-btn', 'call-btn', 'start')}
            title={formatMessage({id: 'fusion.calls.start', defaultMessage: 'Start a call'})}
            aria-label={formatMessage({id: 'fusion.calls.start', defaultMessage: 'Start a call'})}
            onClick={() => actions.startOrJoinCall(channel.id)}
        >
            <Icon name='phone'/>
        </button>
    );
}
