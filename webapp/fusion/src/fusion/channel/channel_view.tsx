// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useRef, useState} from 'react';
import {useIntl} from 'react-intl';
import {useSelector} from 'react-redux';
import {useRouteMatch} from 'react-router-dom';

import {getCurrentChannel, isDeactivatedDirectChannel} from 'mattermost-redux/selectors/entities/channels';
import {getCurrentUserId} from 'mattermost-redux/selectors/entities/users';

import {getChannelViewPanel} from 'selectors/channel_view_panel';

import ChannelBanner from 'components/channel_banner/channel_banner';
import {ChannelComposerBanner} from 'components/channel_view/channel_composer_banner';

import {useIsVoiceChannel} from 'fusion/calls/hooks';
import VoiceView from 'fusion/calls/voice_view';
import Icon from 'fusion/components/icon';
import Composer, {DROP_FILES_EVENT} from 'fusion/composer/composer';
import MessageList from 'fusion/messages/message_list';
import {useConversationName} from 'fusion/sidebar/group_dm';
import {am} from 'fusion/utils/class_names';

import type {GlobalState} from 'types/store';

import ChannelHeader from './channel_header';
import ChannelIntro from './channel_intro';
import ChannelViewPanel from './channel_view_panel';
import SharedBanner from './shared_banner';

// ChannelView is a conversation: header, messages and composer.
export default function ChannelView() {
    const {formatMessage} = useIntl();
    const channel = useSelector(getCurrentChannel);
    const deactivated = useSelector((state: GlobalState) => (channel ? isDeactivatedDirectChannel(state, channel.id) : false));
    const match = useRouteMatch<{postid?: string}>();
    const viewRef = useRef<HTMLDivElement>(null);
    const [dropping, setDropping] = useState(false);
    const voice = useIsVoiceChannel(channel?.id);
    const me = useSelector(getCurrentUserId);
    const conversationName = useConversationName(channel || {id: '', type: 'O', display_name: ''});
    const notes = channel?.type === 'D' && channel.name === `${me}__${me}`;

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

    // A voice channel is its call's stage, with its text chat in the right-hand panel; a permalink shows the post.
    if (voice && !match.params.postid) {
        return (
            <>
                <ChannelHeader channel={channel}/>
                <VoiceView channel={channel}/>
            </>
        );
    }

    const direct = channel.type === 'D' || channel.type === 'G';
    let placeholder = formatMessage({id: 'fusion.channel.messageChannel', defaultMessage: 'Message #{name}'}, {name: channel.display_name});
    if (notes) {
        placeholder = formatMessage({id: 'fusion.channel.messageNotes', defaultMessage: 'Write a note to self'});
    } else if (direct) {
        placeholder = formatMessage({id: 'fusion.channel.messageDirect', defaultMessage: 'Message {name}'}, {name: conversationName});
    }
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
            <ChannelBanner channelId={channel.id}/>
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
                            if (!archived && !deactivated && !dropping) {
                                setDropping(true);
                            }
                        }
                    }}
                    onDragLeave={(e) => {
                        if (!e.currentTarget.contains(e.relatedTarget as Node)) {
                            setDropping(false);
                        }
                    }}
                    onDrop={(e) => {
                        setDropping(false);
                        if (e.dataTransfer.files.length && !archived && !deactivated) {
                            e.preventDefault();
                            window.dispatchEvent(new CustomEvent(DROP_FILES_EVENT, {detail: Array.from(e.dataTransfer.files)}));
                        }
                    }}
                >
                    {dropping && (
                        <div
                            className={am('drop-overlay')}
                            aria-hidden='true'
                        >
                            <Icon name='upload'/>
                            <b>{formatMessage({id: 'fusion.channel.dropFiles', defaultMessage: 'Drop to upload'})}</b>
                            <span>{direct ? conversationName : `#${channel.display_name}`}</span>
                        </div>
                    )}
                    <MessageList
                        key={channel.id}
                        channelId={channel.id}
                        focusedPostId={match.params.postid}
                        scrollRef={viewRef}
                        intro={<ChannelIntro channel={channel}/>}
                    />
                </div>
            )}
            {messagesVisible && !archived && <ChannelComposerBanner channelId={channel.id}/>}
            {messagesVisible && composer}
        </>
    );
}
