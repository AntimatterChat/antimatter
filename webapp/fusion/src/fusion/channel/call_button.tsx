// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useRef, useState} from 'react';
import {useIntl} from 'react-intl';
import {useSelector, useStore} from 'react-redux';

import type {Channel} from '@mattermost/types/channels';

import {getMyChannelMembership} from 'mattermost-redux/selectors/entities/channels';

import {showCall, useCallActions} from 'fusion/calls/actions';
import {getCallsAPI} from 'fusion/calls/calls_api';
import {useCall, useCallsAvailable, useIsVoiceChannel, useLocalCall, useParticipants} from 'fusion/calls/hooks';
import {useCallsUI} from 'fusion/calls/ui_state';
import Icon from 'fusion/components/icon';
import {Popover} from 'fusion/components/layer';
import {am} from 'fusion/utils/class_names';

import type {GlobalState} from 'types/store';
import type {CallButtonAction} from 'types/store/plugins';

const CALLS_PLUGIN_ID = 'com.mattermost.calls';
const NO_PROVIDERS: CallButtonAction[] = [];

// OtherCallProviders are the call buttons of plugins other than Calls (registerCallButtonAction), e.g. Zoom or Jitsi:
// the only one shown as a button, several in a menu.
export function OtherCallProviders({channel}: {channel: Channel}) {
    const {formatMessage} = useIntl();
    const providers = useSelector((state: GlobalState) => state.plugins.components.CallButton) || NO_PROVIDERS;
    const member = useSelector((state: GlobalState) => getMyChannelMembership(state, channel.id));
    const others = providers.filter((p) => p.pluginId !== CALLS_PLUGIN_ID);
    const ref = useRef<HTMLButtonElement>(null);
    const [open, setOpen] = useState(false);
    if (!others.length || channel.delete_at !== 0) {
        return null;
    }
    if (others.length === 1) {
        return (
            <button
                className={am('icon-btn', 'plugin-call')}
                onClick={() => others[0].action?.(channel, member)}
            >
                {others[0].button}
            </button>
        );
    }
    return (
        <>
            <button
                ref={ref}
                className={am('icon-btn', {on: open})}
                title={formatMessage({id: 'fusion.calls.others', defaultMessage: 'Other ways to call'})}
                aria-label={formatMessage({id: 'fusion.calls.others', defaultMessage: 'Other ways to call'})}
                aria-haspopup='menu'
                onClick={() => setOpen(!open)}
            >
                <Icon name='video'/>
            </button>
            {open && (
                <Popover
                    anchor={ref.current}
                    placement='below'
                    className='plus-pop'
                    role='menu'
                    label={formatMessage({id: 'fusion.calls.others', defaultMessage: 'Other ways to call'})}
                    onClose={() => setOpen(false)}
                >
                    {others.map((p) => (
                        <button
                            key={p.id}
                            type='button'
                            role='menuitem'
                            className={am('mi')}
                            onClick={() => {
                                setOpen(false);
                                p.action?.(channel, member);
                            }}
                        >
                            {p.dropdownButton || p.button}
                        </button>
                    ))}
                </Popover>
            )}
        </>
    );
}

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
    const store = useStore<GlobalState>();
    const {callView} = useCallsUI();

    if (!available || voice) {
        return null;
    }

    // In the call: show it in place of the messages (while they're shown), and leave it.
    if (local?.channelId === channel.id) {
        const showing = callView === channel.id;
        const label = formatMessage({id: 'fusion.calls.showCall', defaultMessage: 'Show the call'});
        return (
            <>
                {!showing && (
                    <button
                        className={am('icon-btn')}
                        title={label}
                        aria-label={label}
                        onClick={() => showCall(store.getState(), channel.id)}
                    >
                        <Icon name='expand'/>
                    </button>
                )}
                <button
                    className={am('icon-btn', 'call-btn', 'leave')}
                    title={formatMessage({id: 'fusion.calls.leave', defaultMessage: 'Leave the call'})}
                    aria-label={formatMessage({id: 'fusion.calls.leave', defaultMessage: 'Leave the call'})}
                    onClick={actions.leaveCall}
                >
                    <Icon name='hangup'/>
                </button>
            </>
        );
    }
    if (call) {
        return (
            <button
                className={am('icon-btn', 'call-btn', 'join')}
                title={formatMessage({id: 'fusion.calls.join', defaultMessage: 'Join the call'})}
                aria-label={formatMessage({id: 'fusion.calls.joinPeople', defaultMessage: '{count, plural, one {Join the call, # person in it} other {Join the call, # people in it}}'}, {count: people})}
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
