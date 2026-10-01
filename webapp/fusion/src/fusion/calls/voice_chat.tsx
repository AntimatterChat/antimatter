// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useMemo, useRef} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector, useStore} from 'react-redux';

import type {Channel} from '@mattermost/types/channels';

import {getChannel, getCurrentChannelId} from 'mattermost-redux/selectors/entities/channels';

import {closeRightHandSide} from 'actions/views/rhs';
import {getIsRhsOpen} from 'selectors/rhs';

import Icon from 'fusion/components/icon';
import Composer from 'fusion/composer/composer';
import MessageList from 'fusion/messages/message_list';
import {useLayout} from 'fusion/shell/layout_context';
import {am} from 'fusion/utils/class_names';

import type {GlobalState} from 'types/store';

import {goToChannel} from './actions';
import {useIsVoiceChannel} from './hooks';
import {setVoiceChat, useCallsUI} from './ui_state';

// useOpenVoiceChat is the voice channel whose text chat shows in the right-hand panel, if any: the current channel's,
// while no other panel is open. Switching to another channel or opening another panel closes it.
export function useOpenVoiceChat(): string | null {
    const {voiceChat} = useCallsUI();
    const current = useSelector(getCurrentChannelId);
    const rhsOpen = useSelector(getIsRhsOpen);
    const previous = useRef({current, rhsOpen});

    useEffect(() => {
        const was = previous.current;
        previous.current = {current, rhsOpen};
        if (!voiceChat) {
            return;
        }
        if ((current !== was.current && current !== voiceChat) || (rhsOpen && !was.rhsOpen)) {
            setVoiceChat(null);
        }
    }, [voiceChat, current, rhsOpen]);

    return voiceChat && voiceChat === current && !rhsOpen ? voiceChat : null;
}

// useVoiceChatActions opens and closes a voice channel's text chat; opening it goes to the channel, without joining.
export function useVoiceChatActions() {
    const dispatch = useDispatch();
    const store = useStore<GlobalState>();
    const layout = useLayout();
    return useMemo(() => ({
        open: (channelId: string) => {
            if (getIsRhsOpen(store.getState())) {
                dispatch(closeRightHandSide());
            }
            setVoiceChat(channelId);
            if (getCurrentChannelId(store.getState()) !== channelId) {
                goToChannel(store.getState(), channelId);
            }
            layout.setRightOpen(true);
        },
        close: () => {
            setVoiceChat(null);
            layout.setRightOpen(false);
        },
    }), [dispatch, store, layout]);
}

// VoiceChatToggle is the header's "Text chat" button of a voice channel, which has no composer of its own.
export function VoiceChatToggle({channel}: {channel: Channel}) {
    const {formatMessage} = useIntl();
    const voice = useIsVoiceChannel(channel.id);
    const on = useOpenVoiceChat() === channel.id;
    const chat = useVoiceChatActions();

    if (!voice) {
        return null;
    }
    return (
        <button
            className={am('icon-btn', {on})}
            title={formatMessage({id: 'fusion.calls.textChat', defaultMessage: 'Text chat'})}
            aria-label={formatMessage({id: 'fusion.calls.textChat', defaultMessage: 'Text chat'})}
            aria-pressed={on}
            onClick={() => (on ? chat.close() : chat.open(channel.id))}
        >
            <Icon name='chat'/>
        </button>
    );
}

// VoiceChatPanel is a voice channel's text chat in the right-hand panel: its messages and a composer.
export function VoiceChatPanel({channelId}: {channelId: string}) {
    const {formatMessage} = useIntl();
    const channel = useSelector((state: GlobalState) => getChannel(state, channelId));
    const chat = useVoiceChatActions();
    const bodyRef = useRef<HTMLDivElement>(null);

    if (!channel) {
        return null;
    }
    return (
        <aside className={am('rhs')}>
            <div className={am('rhs-head')}>
                <div className={am('grow')}>
                    <h3>{formatMessage({id: 'fusion.calls.chat', defaultMessage: 'Chat'})}</h3>
                    <span className={am('where')}>{channel.display_name}</span>
                </div>
                <button
                    className={am('icon-btn')}
                    aria-label={formatMessage({id: 'fusion.calls.closeChat', defaultMessage: 'Close chat'})}
                    onClick={chat.close}
                >
                    <Icon name='x'/>
                </button>
            </div>
            <div
                ref={bodyRef}
                className={am('rhs-body')}
            >
                <MessageList
                    key={channelId}
                    channelId={channelId}
                    scrollRef={bodyRef}
                    emptyText={formatMessage({id: 'fusion.calls.chatEmpty', defaultMessage: 'No messages yet. The text chat lives alongside the call.'})}
                />
            </div>
            {channel.delete_at === 0 && (
                <Composer
                    key={channelId}
                    channelId={channelId}
                    placeholder={formatMessage({id: 'fusion.calls.chatPlaceholder', defaultMessage: 'Message {name}'}, {name: channel.display_name})}
                />
            )}
        </aside>
    );
}
