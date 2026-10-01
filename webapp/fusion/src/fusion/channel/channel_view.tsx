// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useRef, useState} from 'react';
import {useIntl} from 'react-intl';
import {useSelector} from 'react-redux';
import {useRouteMatch} from 'react-router-dom';

import {getCurrentChannel, isDeactivatedDirectChannel} from 'mattermost-redux/selectors/entities/channels';

import {getChannelViewPanel} from 'selectors/channel_view_panel';

import Composer, {DROP_FILES_EVENT} from 'fusion/composer/composer';
import MessageList from 'fusion/messages/message_list';
import {am} from 'fusion/utils/class_names';

import type {GlobalState} from 'types/store';

import ChannelHeader from './channel_header';
import ChannelViewPanel from './channel_view_panel';
import SharedBanner from './shared_banner';

// ChannelView is a conversation: header, messages and composer.
export default function ChannelView() {
    const {formatMessage} = useIntl();
    const channel = useSelector(getCurrentChannel);
    const deactivated = useSelector((state: GlobalState) => (channel ? isDeactivatedDirectChannel(state, channel.id) : false));
    const match = useRouteMatch<{postid?: string}>();
    const viewRef = useRef<HTMLDivElement>(null);

    // A plugin's panel (e.g. a call) takes the place of the messages until it shows them; they're
    // hidden again on channel switch, and shown for a permalink so that it shows its post.
    const panel = useSelector((state: GlobalState) => (channel ? getChannelViewPanel(state, channel.id) : null));
    const [panelMessagesVisible, setPanelMessagesVisible] = useState(Boolean(match.params.postid));
    useEffect(() => {
        setPanelMessagesVisible(Boolean(match.params.postid));
    }, [channel?.id, match.params.postid]);

    if (!channel) {
        return <div className={am('empty')}>{formatMessage({id: 'fusion.channel.loading', defaultMessage: 'Loading…'})}</div>;
    }

    const direct = channel.type === 'D' || channel.type === 'G';
    const placeholder = direct ? formatMessage({id: 'fusion.channel.messageDirect', defaultMessage: 'Message {name}'}, {name: channel.display_name}) : formatMessage({id: 'fusion.channel.messageChannel', defaultMessage: 'Message #{name}'}, {name: channel.display_name});
    const archived = channel.delete_at !== 0;
    const messagesVisible = !panel || panelMessagesVisible;

    let composer;
    if (archived || deactivated) {
        composer = (
            <div className={am('composer')}>
                <div
                    className={am('note-box')}
                    style={{margin: 0}}
                >
                    {archived ? formatMessage({id: 'fusion.channel.archived', defaultMessage: 'You are viewing an archived channel. New messages cannot be posted.'}) : formatMessage({id: 'fusion.channel.deactivated', defaultMessage: 'You are viewing a conversation with a deactivated user. New messages cannot be posted.'})}
                </div>
            </div>
        );
    } else {
        composer = (
            <Composer
                key={channel.id}
                channelId={channel.id}
                placeholder={placeholder}
            />
        );
    }

    return (
        <>
            <ChannelHeader channel={channel}/>
            <SharedBanner channel={channel}/>
            {panel && (
                <ChannelViewPanel
                    registration={panel}
                    channel={channel}
                    messagesVisible={messagesVisible}
                    setMessagesVisible={setPanelMessagesVisible}
                />
            )}
            {messagesVisible && (
                <div
                    ref={viewRef}
                    className={am('view')}
                    id='am-view'
                    onDragOver={(e) => {
                        if (e.dataTransfer.types.includes('Files')) {
                            e.preventDefault();
                        }
                    }}
                    onDrop={(e) => {
                        if (e.dataTransfer.files.length && !archived && !deactivated) {
                            e.preventDefault();
                            window.dispatchEvent(new CustomEvent(DROP_FILES_EVENT, {detail: Array.from(e.dataTransfer.files)}));
                        }
                    }}
                >
                    <MessageList
                        key={channel.id}
                        channelId={channel.id}
                        focusedPostId={match.params.postid}
                        scrollRef={viewRef}
                    />
                </div>
            )}
            {messagesVisible && composer}
        </>
    );
}
