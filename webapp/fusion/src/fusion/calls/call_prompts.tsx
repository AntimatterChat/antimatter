// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useRef, useState} from 'react';
import {useIntl} from 'react-intl';
import {shallowEqual, useSelector} from 'react-redux';

import {getChannel} from 'mattermost-redux/selectors/entities/channels';

import Avatar from 'fusion/components/avatar';
import Icon from 'fusion/components/icon';
import {Layer} from 'fusion/components/layer';
import {useDisplayName, useUser} from 'fusion/hooks/users';
import {getBlockLevel, isChannelMessageRequest} from 'fusion/privacy/blocking';
import {useConversationName} from 'fusion/sidebar/group_dm';
import {am} from 'fusion/utils/class_names';

import type {GlobalState} from 'types/store';

import {firstName, useCallActions, useChannelLabel} from './actions';
import {getCallsAPI, useCallsAPIs} from './calls_api';
import type {CallsIncomingCall, CallsUnmuteRequest} from './calls_api';
import {useLocalCall, useMyCall} from './hooks';

// How long a call's card is shown: Calls rings for 30 seconds, the card stays a little longer. The conversation then
// stays on top of the direct messages, with a Join banner, while the call lasts.
export const INCOMING_CALL_TIMEOUT = 40 * 1000;

const NO_CALLS: CallsIncomingCall[] = [];

// pickIncomingCall is the call to show a card for: the newest one ringing in a conversation you aren't in the call
// of, whose card hasn't been answered, declined or timed out.
export function pickIncomingCall(calls: CallsIncomingCall[], inChannelId: string | undefined, done: Set<string>): CallsIncomingCall | undefined {
    return calls.find((c) => c.channelId !== inChannelId && !done.has(c.callId));
}

function IncomingCallCard({call, onDone}: {call: CallsIncomingCall; onDone: (callId: string) => void}) {
    const {formatMessage} = useIntl();
    const actions = useCallActions();
    const caller = useDisplayName(useUser(call.callerId));
    const channel = useSelector((state: GlobalState) => getChannel(state, call.channelId));
    const group = useConversationName(channel || {id: call.channelId, type: 'G', display_name: ''});
    const others = call.type === 'gm' && channel ? Math.max(0, channel.display_name.split(',').length - 1) : 0;

    // The ring stops with the card: Calls stops it on its own after 30 seconds, and when the call is joined or ends.
    useEffect(() => {
        const timer = setTimeout(() => {
            getCallsAPI()?.stopRinging?.(call.callId);
            onDone(call.callId);
        }, INCOMING_CALL_TIMEOUT);
        return () => clearTimeout(timer);
    }, [call.callId, onDone]);

    const join = () => {
        onDone(call.callId);
        actions.joinAndShowCall(call.channelId);
    };
    const decline = () => {
        onDone(call.callId);
        getCallsAPI()?.dismissIncomingCall(call.channelId, call.callId);
    };

    const title = call.type === 'dm' ? caller : group;
    const sub = call.type === 'dm' ? formatMessage({id: 'fusion.calls.incoming.dm', defaultMessage: 'is calling you'}) : formatMessage({id: 'fusion.calls.incoming.gm', defaultMessage: '{name} is calling the group'}, {name: caller});

    return (
        <div
            className={am('call-prompt', 'incoming')}
            role='alertdialog'
            aria-label={call.type === 'dm' ? formatMessage({id: 'fusion.calls.incoming.dmLabel', defaultMessage: '{name} is calling you'}, {name: caller}) : formatMessage({id: 'fusion.calls.incoming.gmLabel', defaultMessage: '{name} is calling {group}'}, {name: caller, group})}
        >
            <div className={am('call-prompt-top')}>
                <span className={am('ring-av', {ringing: call.ringing})}>
                    <Avatar
                        userId={call.callerId}
                        size='lg'
                    />
                    {others > 0 && <span className={am('ring-more')}>{`+${others}`}</span>}
                </span>
                <div className={am('grow')}>
                    <b>{title}</b>
                    <span className={am('sub')}>
                        <Icon
                            name='phone'
                            size='xs'
                        />
                        {sub}
                    </span>
                </div>
                {call.ringing && (
                    <button
                        className={am('icon-btn')}
                        title={formatMessage({id: 'fusion.calls.incoming.muteRing', defaultMessage: 'Mute the ring'})}
                        aria-label={formatMessage({id: 'fusion.calls.incoming.muteRing', defaultMessage: 'Mute the ring'})}
                        onClick={() => getCallsAPI()?.stopRinging?.(call.callId)}
                    >
                        <Icon
                            name='bell-off'
                            size='sm'
                        />
                    </button>
                )}
            </div>
            <div className={am('call-prompt-actions')}>
                <button
                    className={am('btn', 'decline')}
                    onClick={decline}
                >
                    <Icon
                        name='hangup'
                        size='sm'
                    />
                    {formatMessage({id: 'fusion.calls.incoming.decline', defaultMessage: 'Decline'})}
                </button>
                <button
                    className={am('btn', 'primary', 'accept')}
                    onClick={join}
                >
                    <Icon
                        name='phone'
                        size='sm'
                    />
                    {formatMessage({id: 'fusion.calls.incoming.join', defaultMessage: 'Join'})}
                </button>
            </div>
        </div>
    );
}

// IncomingCalls is the card of a call ringing in a direct or group message, wherever you are in the Fusion UI: join
// it, decline it, or mute its ring. Calls rings (unless you're in another call, on do not disturb, or have turned
// the ring off) and sends the desktop notification; it doesn't ring the caller nor those already in the call.
export function IncomingCalls() {
    const {calls} = useCallsAPIs();
    const incoming = useSelector((state: GlobalState) => calls?.selectors.getIncomingCalls(state) || NO_CALLS);

    // No card for a call from someone you ignore or block, nor in a direct message waiting as a message request.
    const silenced = useSelector((state: GlobalState) => incoming.filter((c) => getBlockLevel(state, c.callerId) !== '' || isChannelMessageRequest(state, c.channelId)).map((c) => c.callId), shallowEqual);
    const local = useLocalCall();
    const [done, setDone] = useState<Set<string>>(() => new Set());
    const onDone = useRef((callId: string) => setDone((d) => new Set(d).add(callId))).current;

    const call = pickIncomingCall(silenced.length ? incoming.filter((c) => !silenced.includes(c.callId)) : incoming, local?.channelId, done);
    if (!call) {
        return null;
    }
    return (
        <IncomingCallCard
            key={call.callId}
            call={call}
            onDone={onDone}
        />
    );
}

// How long the prompt to unmute stays up unanswered.
const UNMUTE_REQUEST_TIMEOUT = 20 * 1000;

function UnmuteRequestCard({request, onClose}: {request: CallsUnmuteRequest; onClose: () => void}) {
    const {formatMessage} = useIntl();
    const actions = useCallActions();
    const my = useMyCall();
    const name = useSelector((state: GlobalState) => firstName(state, request.byUserId));
    const where = useChannelLabel(request.channelId);

    // Answered elsewhere (unmuted, or the call left), the prompt goes.
    const stale = !my || my.call.sessionId !== request.sessionId || !my.muted || my.modMuted;
    useEffect(() => {
        if (stale) {
            onClose();
            return undefined;
        }
        const timer = setTimeout(onClose, UNMUTE_REQUEST_TIMEOUT);
        return () => clearTimeout(timer);
    }, [stale, onClose]);
    if (stale || !my) {
        return null;
    }

    return (
        <div
            className={am('call-prompt')}
            role='alertdialog'
            aria-label={formatMessage({id: 'fusion.calls.askUnmute.label', defaultMessage: '{name} asks you to unmute'}, {name})}
        >
            <div className={am('call-prompt-top')}>
                <span className={am('ring-av')}>
                    <Avatar
                        userId={request.byUserId}
                        size='lg'
                    />
                </span>
                <div className={am('grow')}>
                    <b>{formatMessage({id: 'fusion.calls.askUnmute.title', defaultMessage: '{name} asks you to unmute'}, {name})}</b>
                    <span className={am('sub')}>
                        <Icon
                            name='mic-off'
                            size='xs'
                        />
                        {formatMessage({id: 'fusion.calls.askUnmute.sub', defaultMessage: 'You are muted in {where}'}, {where})}
                    </span>
                </div>
            </div>
            <div className={am('call-prompt-actions')}>
                <button
                    className={am('btn')}
                    onClick={onClose}
                >
                    {formatMessage({id: 'fusion.calls.askUnmute.notNow', defaultMessage: 'Not now'})}
                </button>
                <button
                    className={am('btn', 'primary')}
                    onClick={() => {
                        onClose();
                        actions.toggleMute(my);
                    }}
                >
                    <Icon
                        name='mic'
                        size='sm'
                    />
                    {formatMessage({id: 'fusion.calls.askUnmute.unmute', defaultMessage: 'Unmute'})}
                </button>
            </div>
        </div>
    );
}

// UnmuteRequest prompts you to unmute when the call's host or a moderator asks (in case you forgot): unmute, or not
// now. A new request replaces the one shown.
export function UnmuteRequest() {
    const {calls} = useCallsAPIs();
    const [request, setRequest] = useState<CallsUnmuteRequest | null>(null);
    const close = useRef(() => setRequest(null)).current;

    useEffect(() => {
        if (!calls) {
            return undefined;
        }

        // Older Calls versions have no such event, and ignore the listener.
        return calls.on('unmute-request', (r: CallsUnmuteRequest) => setRequest(r));
    }, [calls]);

    if (!request) {
        return null;
    }
    return (
        <UnmuteRequestCard
            key={`${request.sessionId}:${request.byUserId}`}
            request={request}
            onClose={close}
        />
    );
}

// CallPrompts are the call's cards over the whole UI, in the top right corner: incoming calls and requests to unmute.
export default function CallPrompts() {
    return (
        <Layer>
            <div className={am('call-prompts')}>
                <UnmuteRequest/>
                <IncomingCalls/>
            </div>
        </Layer>
    );
}
