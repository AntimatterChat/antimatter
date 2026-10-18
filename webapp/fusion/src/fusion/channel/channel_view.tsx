// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useRef, useState} from 'react';
import {useIntl} from 'react-intl';
import {useSelector} from 'react-redux';

import {Permissions} from 'mattermost-redux/constants';
import {getChannelMessageCount, getCurrentChannel, isDeactivatedDirectChannel} from 'mattermost-redux/selectors/entities/channels';
import {getMemberTimeout, haveIChannelPermission} from 'mattermost-redux/selectors/entities/roles';
import {getCurrentUserId} from 'mattermost-redux/selectors/entities/users';
import {getUserIdFromChannelName} from 'mattermost-redux/utils/channel_utils';

import {getChannelViewPanel} from 'selectors/channel_view_panel';

import ChannelBanner from 'components/channel_banner/channel_banner';
import {ChannelComposerBanner} from 'components/channel_view/channel_composer_banner';

import CallBanner from 'fusion/calls/call_banner';
import {useIsVoiceChannel, useLocalCall} from 'fusion/calls/hooks';
import {useCallsUI} from 'fusion/calls/ui_state';
import VoiceView from 'fusion/calls/voice_view';
import Icon from 'fusion/components/icon';
import Composer, {DROP_FILES_EVENT} from 'fusion/composer/composer';
import MessageList from 'fusion/messages/message_list';
import {getBlockLevel, isChannelMessageRequest} from 'fusion/privacy/blocking';
import {BlockedNote, RequestBar} from 'fusion/privacy/message_requests';
import {useConversationName} from 'fusion/sidebar/group_dm';
import {am} from 'fusion/utils/class_names';

import type {GlobalState} from 'types/store';

import ChannelHeader from './channel_header';
import ChannelIntro from './channel_intro';
import ChannelViewPanel from './channel_view_panel';
import OnTopStrip from './on_top_strip';
import {usePermalinkAnchor} from './permalink_anchor';
import SharedBanner from './shared_banner';

type Props = {

    // A permalink is being resolved to its conversation: the conversation shown stays as it is meanwhile.
    resolvingPermalink?: boolean;
};

// ChannelView is a conversation: header, messages and composer.
export default function ChannelView({resolvingPermalink = false}: Props) {
    const {formatMessage, formatTime} = useIntl();
    const channel = useSelector(getCurrentChannel);
    const deactivated = useSelector((state: GlobalState) => (channel ? isDeactivatedDirectChannel(state, channel.id) : false));

    // A team channel someone is in but can't post in: read-only by its moderation, or by an override, or they're timed
    // out of the team (Antimatter). A timeout shows until it ends, when the composer comes back.
    const timeout = useSelector((state: GlobalState) => (channel?.team_id ? getMemberTimeout(state, channel.team_id, getCurrentUserId(state)) : undefined));
    const [, rerender] = useState(0);
    useEffect(() => {
        if (!timeout) {
            return undefined;
        }
        const timer = setTimeout(() => rerender((n) => n + 1), Math.max(0, timeout.until - Date.now()) + 500);
        return () => clearTimeout(timer);
    }, [timeout]);
    const readOnly = useSelector((state: GlobalState) => Boolean(channel?.team_id) && Boolean(state.entities.channels.myMembers[channel!.id]) &&
        !haveIChannelPermission(state, channel!.team_id, channel!.id, Permissions.CREATE_POST));

    // The message a link opened: highlighted while the address names it, and the one the list stays around.
    const {linkedPostId, anchorPostId, leaveAnchor} = usePermalinkAnchor(channel?.id, resolvingPermalink);
    const viewRef = useRef<HTMLDivElement>(null);
    const [dropping, setDropping] = useState(false);
    const voice = useIsVoiceChannel(channel?.id);
    const {callView} = useCallsUI();
    const inCallHere = useLocalCall()?.channelId === channel?.id;
    const me = useSelector(getCurrentUserId);
    const conversationName = useConversationName(channel || {id: '', type: 'O', display_name: ''});
    const notes = channel?.type === 'D' && channel.name === `${me}__${me}`;

    // A direct message waiting as a message request, or with someone you block, has no message box. One without
    // messages yet has: writing first in it accepts it.
    const teammateId = channel?.type === 'D' && !notes ? getUserIdFromChannelName(me, channel.name) : '';
    const request = useSelector((state: GlobalState) => Boolean(teammateId) && (getChannelMessageCount(state, channel!.id)?.total ?? 0) > 0 && isChannelMessageRequest(state, channel!.id));
    const blocked = useSelector((state: GlobalState) => Boolean(teammateId) && getBlockLevel(state, teammateId) === 'block');

    // A plugin's panel (e.g. a call) takes the place of the messages until it shows them; they're
    // hidden again on channel switch, and shown for a permalink so that it shows its post.
    const panel = useSelector((state: GlobalState) => (channel ? getChannelViewPanel(state, channel.id) : null));
    const [panelMessagesVisible, setPanelMessagesVisible] = useState(Boolean(anchorPostId));
    useEffect(() => {
        setPanelMessagesVisible(Boolean(anchorPostId));
    }, [channel?.id, anchorPostId]);

    if (!channel) {
        return <div className={am('empty')}>{formatMessage({id: 'fusion.channel.loading', defaultMessage: 'Loading…'})}</div>;
    }

    // A voice channel is its call's stage, with its text chat in the right-hand panel; so is a conversation whose
    // call is shown (from the call panel, or once joined from its card). A permalink shows the post.
    if ((voice || (callView === channel.id && inCallHere)) && !anchorPostId) {
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
    if (archived || deactivated || readOnly) {
        composer = (
            <div className={am('composer')}>
                <div
                    className={am('note-box')}
                    style={{margin: 0}}
                >
                    {archived && formatMessage({id: 'fusion.channel.archived', defaultMessage: 'You are viewing an archived channel. New messages cannot be posted.'})}
                    {!archived && deactivated && formatMessage({id: 'fusion.channel.deactivated', defaultMessage: 'You are viewing a conversation with a deactivated user. New messages cannot be posted.'})}
                    {!archived && !deactivated && timeout && formatMessage({id: 'fusion.channel.timedOut', defaultMessage: 'You\'re timed out until {time}: you can read, but not send messages or react.'}, {time: formatTime(timeout.until)})}
                    {!archived && !deactivated && !timeout && formatMessage({id: 'fusion.channel.readOnly', defaultMessage: 'You can\'t send messages in this channel.'})}
                </div>
            </div>
        );
    } else if (blocked) {
        composer = <BlockedNote userId={teammateId}/>;
    } else if (request) {
        composer = (
            <RequestBar
                channel={channel}
                userId={teammateId}
            />
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
            <CallBanner channel={channel}/>
            <SharedBanner channel={channel}/>
            <ChannelBanner channelId={channel.id}/>
            <OnTopStrip channel={channel}/>
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
                        focusedPostId={anchorPostId}
                        highlight={Boolean(linkedPostId)}
                        onJumpToPresent={leaveAnchor}
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
