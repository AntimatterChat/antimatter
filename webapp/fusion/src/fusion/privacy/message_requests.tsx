// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector, useStore} from 'react-redux';

import type {Channel} from '@mattermost/types/channels';

import {getMissingProfilesByIds} from 'mattermost-redux/actions/users';
import {get as getPreference} from 'mattermost-redux/selectors/entities/preferences';
import {getCurrentTeam} from 'mattermost-redux/selectors/entities/teams';
import type {MessageRequestProps} from 'mattermost-redux/utils/channel_utils';

import Icon from 'fusion/components/icon';
import {useDisplayName, useUser} from 'fusion/hooks/users';
import {useDialogs} from 'fusion/shell/dialogs_context';
import {useToast} from 'fusion/shell/toast_context';
import DirectRow from 'fusion/sidebar/direct_row';
import {useLastMessage} from 'fusion/sidebar/last_message';
import {am} from 'fusion/utils/class_names';
import {getHistory} from 'utils/browser_history';

import type {GlobalState} from 'types/store';

import type {BlockLevel, MessageRequest} from './blocking';
import {BLOCK_PROMPT_CATEGORY, getMessageRequests, isUserBlockingEnabled, setBlockLevel, updateMessageRequest, useBlockLevel} from './blocking';

// Ignoring someone after this many dismissals of their requests, or with this many of their messages left unopened,
// offers to block them.
const PROMPT_DISMISSALS = 2;
const PROMPT_UNOPENED = 3;

// useBlockActions ignores, blocks, and stops either, for someone's menu: blocking asks first; each tells what it did.
export function useBlockActions(userId: string) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const dialogs = useDialogs();
    const toast = useToast();
    const enabled = useSelector(isUserBlockingEnabled);
    const level = useBlockLevel(userId);
    const name = useDisplayName(useUser(userId));

    const set = async (next: BlockLevel, done: string) => {
        const result = await dispatch(setBlockLevel(userId, next));
        if ('error' in result && result.error) {
            toast((result.error as {message?: string}).message || formatMessage({id: 'fusion.block.failed', defaultMessage: 'That didn\'t work. Try again.'}));
            return;
        }
        toast(done);
    };

    const block = () => dialogs.confirm({
        title: formatMessage({id: 'fusion.block.confirmTitle', defaultMessage: 'Block {name}?'}, {name}),
        message: formatMessage({id: 'fusion.block.confirmText', defaultMessage: 'They won\'t be able to message you or start a group message with you, and you won\'t be notified of anything they write. Their messages in the channels you share are folded. They aren\'t told.'}),
        confirmLabel: formatMessage({id: 'fusion.block.confirm', defaultMessage: 'Block'}),
        danger: true,
        onConfirm: () => set('block', formatMessage({id: 'fusion.block.blocked', defaultMessage: '{name} is blocked'}, {name})),
    });

    return {
        enabled,
        level,
        block,
        toggleIgnore: () => (level === 'ignore' ? set('', formatMessage({id: 'fusion.block.unignored', defaultMessage: 'You no longer ignore {name}'}, {name})) : set('ignore', formatMessage({id: 'fusion.block.ignored', defaultMessage: 'You ignore {name}: no more notifications from them'}, {name}))),
        toggleBlock: () => (level === 'block' ? set('', formatMessage({id: 'fusion.block.unblocked', defaultMessage: '{name} is unblocked'}, {name})) : block()),
    };
}

// useDismissRequest ignores a message request: it's hidden until its sender writes again. After a few of their
// requests ignored, or with several of their messages never opened, it offers to block them, unless you asked not to.
export function useDismissRequest() {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const store = useStore<GlobalState>();
    const dialogs = useDialogs();
    const toast = useToast();

    return async (channel: Channel, userId: string, name: string) => {
        const {myMembers, messageCounts} = store.getState().entities.channels;
        const member = myMembers[channel.id];
        const unopened = member ? (messageCounts[channel.id]?.total ?? 0) - member.msg_count : 0;
        const result = await dispatch(updateMessageRequest(channel.id, 'dismiss'));
        if ('error' in result && result.error) {
            toast((result.error as {message?: string}).message || formatMessage({id: 'fusion.block.failed', defaultMessage: 'That didn\'t work. Try again.'}));
            return;
        }
        toast(formatMessage({id: 'fusion.requests.dismissed', defaultMessage: 'Request from {name} ignored'}, {name}));
        const dismissals = parseInt((result.data?.notify_props as MessageRequestProps | undefined)?.dm_request_dismissals || '0', 10);
        const neverAsk = getPreference(store.getState(), BLOCK_PROMPT_CATEGORY, userId, '') === 'off';
        if (!neverAsk && (dismissals >= PROMPT_DISMISSALS || unopened >= PROMPT_UNOPENED)) {
            dialogs.blockPrompt(userId);
        }
    };
}

function RequestRow({request}: {request: MessageRequest}) {
    const {formatMessage} = useIntl();
    const dismiss = useDismissRequest();
    const name = useDisplayName(useUser(request.userId));
    const last = useLastMessage(request.channel);
    return (
        <div className={am('dock-row', 'request-row')}>
            <DirectRow
                channel={request.channel}
                meta={last || undefined}
            />
            <button
                className={am('dock-x')}
                title={formatMessage({id: 'fusion.requests.ignore', defaultMessage: 'Ignore'})}
                aria-label={formatMessage({id: 'fusion.requests.ignoreLabel', defaultMessage: 'Ignore the request from {name}'}, {name})}
                onClick={() => dismiss(request.channel, request.userId, name)}
            >
                <Icon
                    name='x'
                    size='xs'
                />
            </button>
        </div>
    );
}

// RequestsSection lists the message requests in the home list, above your conversations: the direct messages of
// people you don't talk with yet, or ignore, which don't notify you until you accept them.
export function RequestsSection() {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const {pending} = useSelector(getMessageRequests);
    const [open, setOpen] = useState(true);

    // Their senders may be strangers to this session.
    const senders = pending.map((request) => request.userId).join(',');
    useEffect(() => {
        if (senders) {
            dispatch(getMissingProfilesByIds(senders.split(',')));
        }
    }, [dispatch, senders]);

    if (!pending.length) {
        return null;
    }
    return (
        <>
            <button
                type='button'
                className={am('cat', 'requests-cat', {collapsed: !open})}
                aria-expanded={open}
                title={formatMessage({id: 'fusion.requests.title', defaultMessage: 'Direct messages from people you don\'t talk with yet. They don\'t notify you until you accept them.'})}
                onClick={() => setOpen(!open)}
            >
                <Icon
                    name='chev'
                    size='xs'
                />
                <span className={am('grow')}>{formatMessage({id: 'fusion.requests.label', defaultMessage: 'Message requests'})}</span>
                <span className={am('badge')}>{pending.length}</span>
            </button>
            {open && pending.map((request) => (
                <RequestRow
                    key={request.channel.id}
                    request={request}
                />
            ))}
        </>
    );
}

// RequestBar takes the place of the message box in a message request: accept it to reply, ignore it, or block its
// sender. Nothing of it is told to them.
export function RequestBar({channel, userId}: {channel: Channel; userId: string}) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const toast = useToast();
    const team = useSelector(getCurrentTeam);
    const user = useUser(userId);
    const name = useDisplayName(user);
    const {level, block} = useBlockActions(userId);
    const dismiss = useDismissRequest();
    const [busy, setBusy] = useState(false);

    // Accepting someone you ignore stops ignoring them, or their next message would make a request again.
    const accept = async () => {
        setBusy(true);
        const result = level ? await dispatch(setBlockLevel(userId, '')) : await dispatch(updateMessageRequest(channel.id, 'accept'));
        setBusy(false);
        if ('error' in result && result.error) {
            toast((result.error as {message?: string}).message || formatMessage({id: 'fusion.block.failed', defaultMessage: 'That didn\'t work. Try again.'}));
            return;
        }
        toast(formatMessage({id: 'fusion.requests.accepted', defaultMessage: 'You can now talk with {name}'}, {name}));
    };
    const ignore = async () => {
        await dismiss(channel, userId, name);
        if (team) {
            getHistory().push(`/${team.name}`);
        }
    };

    return (
        <div className={am('composer')}>
            <div className={am('note-box', 'request-bar')}>
                <Icon name='inbox'/>
                <div className={am('grow')}>
                    <b>{formatMessage({id: 'fusion.requests.barTitle', defaultMessage: 'Message request from {name}'}, {name})}</b>
                    <span>
                        {level === 'ignore' ? formatMessage({id: 'fusion.requests.barIgnored', defaultMessage: 'You ignore {name}. Accepting stops ignoring them, and their messages notify you again.'}, {name}) : formatMessage({id: 'fusion.requests.barText', defaultMessage: 'Accept to reply. Until then their messages don\'t notify you, and they aren\'t told whether you read, ignore or block them.'})}
                    </span>
                </div>
                <div className={am('request-actions')}>
                    <button
                        type='button'
                        className={am('btn', 'primary')}
                        disabled={busy}
                        onClick={accept}
                    >
                        {formatMessage({id: 'fusion.requests.accept', defaultMessage: 'Accept'})}
                    </button>
                    <button
                        type='button'
                        className={am('btn')}
                        disabled={busy}
                        onClick={ignore}
                    >
                        {formatMessage({id: 'fusion.requests.ignore', defaultMessage: 'Ignore'})}
                    </button>
                    <button
                        type='button'
                        className={am('btn', 'danger')}
                        disabled={busy}
                        onClick={block}
                    >
                        {formatMessage({id: 'fusion.requests.block', defaultMessage: 'Block'})}
                    </button>
                </div>
            </div>
        </div>
    );
}

// BlockedNote takes the place of the message box in a direct message with someone you block.
export function BlockedNote({userId}: {userId: string}) {
    const {formatMessage} = useIntl();
    const name = useDisplayName(useUser(userId));
    const {toggleBlock} = useBlockActions(userId);
    return (
        <div className={am('composer')}>
            <div className={am('note-box', 'request-bar')}>
                <Icon name='ban'/>
                <div className={am('grow')}>
                    <b>{formatMessage({id: 'fusion.blocked.title', defaultMessage: 'You blocked {name}'}, {name})}</b>
                    <span>{formatMessage({id: 'fusion.blocked.text', defaultMessage: 'Neither of you can write here until you unblock them.'})}</span>
                </div>
                <div className={am('request-actions')}>
                    <button
                        type='button'
                        className={am('btn')}
                        onClick={toggleBlock}
                    >
                        {formatMessage({id: 'fusion.blocked.unblock', defaultMessage: 'Unblock'})}
                    </button>
                </div>
            </div>
        </div>
    );
}
