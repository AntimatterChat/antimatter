// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import type {Channel} from '@mattermost/types/channels';
import type {FileInfo} from '@mattermost/types/files';
import type {Post} from '@mattermost/types/posts';

import {markAllThreadsInTeamRead} from 'mattermost-redux/actions/threads';
import {Client4} from 'mattermost-redux/client';
import {getAllChannels} from 'mattermost-redux/selectors/entities/channels';
import {getConfig} from 'mattermost-redux/selectors/entities/general';
import {getCurrentTeam} from 'mattermost-redux/selectors/entities/teams';
import {getCurrentUserId, getCurrentUserMentionKeys} from 'mattermost-redux/selectors/entities/users';
import {getFormattedFileSize} from 'mattermost-redux/utils/file_utils';

import {selectPostById} from 'actions/views/rhs';

import FilePreviewModal from 'components/file_preview_modal';

import Avatar from 'fusion/components/avatar';
import Icon from 'fusion/components/icon';
import type {IconName} from 'fusion/components/icon';
import {Popover} from 'fusion/components/layer';
import {useDisplayName, useUser} from 'fusion/hooks/users';
import {useWhen} from 'fusion/messages/time';
import {useDialogs} from 'fusion/shell/dialogs_context';
import {useToast} from 'fusion/shell/toast_context';
import {am} from 'fusion/utils/class_names';
import {openDialog} from 'fusion/utils/modals';
import {permalinkPath} from 'fusion/utils/paths';
import {getHistory} from 'utils/browser_history';
import {ModalIdentifiers} from 'utils/constants';
import {canSearchFiles} from 'utils/file_utils';

import type {GlobalState} from 'types/store';

export type Tab = 'mentions' | 'saved' | 'pinned' | 'files' | 'threads';

const TABS: Array<[Tab, IconName, {id: string; defaultMessage: string}, {id: string; defaultMessage: string}]> = [
    ['mentions', 'at', {id: 'fusion.collections.mentions', defaultMessage: 'Mentions'}, {id: 'fusion.collections.mentionsDesc', defaultMessage: 'Messages that mention you'}],
    ['saved', 'bookmark', {id: 'fusion.collections.saved', defaultMessage: 'Saved'}, {id: 'fusion.collections.savedDesc', defaultMessage: 'Messages you saved for later'}],
    ['pinned', 'pin', {id: 'fusion.collections.pinned', defaultMessage: 'Pinned'}, {id: 'fusion.collections.pinnedDesc', defaultMessage: 'Messages pinned in this conversation'}],
    ['files', 'file', {id: 'fusion.collections.files', defaultMessage: 'Files'}, {id: 'fusion.collections.filesDesc', defaultMessage: 'Files shared in this conversation'}],
    ['threads', 'thread', {id: 'fusion.collections.threads', defaultMessage: 'Threads'}, {id: 'fusion.collections.threadsDesc', defaultMessage: 'Threads you follow'}],
];

// The descriptions in a conversation, where every tab only lists what's in it.
const HERE: Partial<Record<Tab, {id: string; defaultMessage: string}>> = {
    mentions: {id: 'fusion.collections.mentionsHereDesc', defaultMessage: 'Messages that mention you in this conversation'},
    saved: {id: 'fusion.collections.savedHereDesc', defaultMessage: 'Messages you saved from this conversation'},
    threads: {id: 'fusion.collections.threadsHereDesc', defaultMessage: 'Threads you follow in this conversation'},
};

const EMPTY_HERE: Partial<Record<Tab, {id: string; defaultMessage: string}>> = {
    mentions: {id: 'fusion.collections.noMentionsHere', defaultMessage: 'No one has mentioned you here yet.'},
    saved: {id: 'fusion.collections.noSavedHere', defaultMessage: 'Nothing saved from this conversation yet. Save messages from their ⋯ menu.'},
    threads: {id: 'fusion.collections.noThreadsHere', defaultMessage: 'You follow no thread here yet.'},
};

const EMPTY: Record<Tab, {id: string; defaultMessage: string}> = {
    mentions: {id: 'fusion.collections.noMentions', defaultMessage: 'No one has mentioned you yet.'},
    saved: {id: 'fusion.collections.noSaved', defaultMessage: 'Save messages from their ⋯ menu to find them here.'},
    pinned: {id: 'fusion.collections.noPinned', defaultMessage: 'Nothing pinned yet. Pin a message from its ⋯ menu.'},
    files: {id: 'fusion.collections.noFiles', defaultMessage: 'No files shared here yet.'},
    threads: {id: 'fusion.collections.noThreads', defaultMessage: 'Follow a thread to keep track of new replies.'},
};

type Item = {post: Post; replies?: number; lastReplyAt?: number; file?: FileInfo};

// What the threads tab shows: every thread you follow, or only those with unread replies.
export type ThreadsFilter = 'all' | 'unread';

// The threads tab's cache has one list per filter; every other tab has one of its own.
function cacheKey(tab: Tab, threadsFilter: ThreadsFilter): string {
    return tab === 'threads' ? `threads:${threadsFilter}` : tab;
}

// FileRow is a file shared in the conversation: it opens in the file preview.
function FileRow({file, onOpen}: {file: FileInfo; onOpen: () => void}) {
    const when = useWhen();
    const user = useUser(file.user_id);
    const name = useDisplayName(user);
    const ext = (file.extension || '').toUpperCase().slice(0, 4) || 'FILE';
    return (
        <button
            className={am('coll-item', 'coll-file')}
            onClick={onOpen}
        >
            <span className={am('file')}>{ext}</span>
            <span style={{minWidth: 0}}>
                <span className={am('snip')}>{file.name}</span>
                <span className={am('where')}>{[getFormattedFileSize(file.size), name, when(file.create_at)].filter(Boolean).join(' · ')}</span>
            </span>
        </button>
    );
}

function Row({item, channels, thread, onOpen}: {item: Item; channels: Record<string, Channel>; thread: boolean; onOpen: () => void}) {
    const {formatMessage} = useIntl();
    const when = useWhen();
    const user = useUser(item.post.user_id);
    const name = useDisplayName(user);
    const channel = channels[item.post.channel_id];
    const where = channel ? (channel.type === 'O' || channel.type === 'P' ? '#' : '') + channel.display_name : '';
    return (
        <button
            className={am('coll-item')}
            onClick={onOpen}
        >
            <Avatar userId={item.post.user_id}/>
            <span style={{minWidth: 0}}>
                <span className={am('where')}>
                    {thread ? <Icon name='thread'/> : <span className={am('who')}>{name}</span>}
                    {thread ? formatMessage({id: 'fusion.collections.threadIn', defaultMessage: 'Thread · {where}'}, {where}) : `${where} · ${when(item.post.create_at)}`}
                </span>
                <span className={am('snip')}>{item.post.message}</span>
                {thread && (
                    <span className={am('where')}>
                        {formatMessage({id: 'fusion.collections.replies', defaultMessage: '{count, plural, one {# reply} other {# replies}}'}, {count: item.replies || 0})}
                        {item.lastReplyAt ? ` · ${formatMessage({id: 'fusion.collections.last', defaultMessage: 'last reply {when}'}, {when: when(item.lastReplyAt)})}` : ''}
                    </span>
                )}
            </span>
        </button>
    );
}

type Props = {
    anchor: HTMLElement | null;
    channel?: Channel;
    initialTab?: Tab;

    // Under the header's button, or to the right of the home sidebar's links.
    placement?: 'below' | 'right';
    onClose: () => void;
};

// CollectionsPopover gathers mentions, saved and pinned messages, files and followed threads, in one panel. Opened
// from a conversation, every tab lists what's in that conversation; from the home, mentions, saved messages and
// threads are the whole team's.
export default function CollectionsPopover({anchor, channel: conversation, initialTab = 'mentions', placement = 'below', onClose}: Props) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const toast = useToast();
    const dialogs = useDialogs();
    const team = useSelector(getCurrentTeam);
    const userId = useSelector(getCurrentUserId);
    const mentionKeys = useSelector(getCurrentUserMentionKeys);
    const channels = useSelector(getAllChannels);
    const filesSearchAvailable = useSelector((state: GlobalState) => canSearchFiles(getConfig(state)));
    const [tab, setTab] = useState<Tab>(initialTab === 'files' && !filesSearchAvailable ? 'mentions' : initialTab);
    const [items, setItems] = useState<Record<string, Item[] | undefined>>({});

    // The threads tab's own All/Unreads tabs (the mockup's "Followed threads" list only had one view).
    const [threadsFilter, setThreadsFilter] = useState<ThreadsFilter>('all');

    // Opened from a conversation, the panel lists that conversation's items, or the whole community's on demand.
    // Pinned messages and files only exist within a conversation.
    const [scope, setScope] = useState<'here' | 'all'>('here');
    const channel = scope === 'here' ? conversation : undefined;
    const tabs = (conversation && scope === 'all' ? TABS.filter(([k]) => k !== 'pinned' && k !== 'files') : TABS).filter(([k]) => k !== 'files' || filesSearchAvailable);
    const changeScope = (next: 'here' | 'all') => {
        if (next === scope) {
            return;
        }
        setScope(next);
        setItems({});
        if (next === 'all' && (tab === 'pinned' || tab === 'files')) {
            setTab('mentions');
        }
    };

    const key = cacheKey(tab, threadsFilter);
    useEffect(() => {
        if (!team || items[key]) {
            return undefined;
        }
        let cancelled = false;
        const set = (list: Item[]) => !cancelled && setItems((prev) => ({...prev, [key]: list}));
        const fromPostList = (list: {order: string[]; posts: Record<string, Post>}) => list.order.map((id) => ({post: list.posts[id]})).filter((i) => i.post);
        (async () => {
            try {
                if (tab === 'mentions') {
                    const terms = mentionKeys.filter(({key: mentionKey}) => !['@channel', '@all', '@here'].includes(mentionKey)).map(({key: mentionKey}) => mentionKey).join(' ');

                    // In a conversation, searched there; the filter on the results also covers direct messages, whose
                    // names "in:" may not find.
                    const where = channel ? ` in:${channel.name}` : '';
                    const found = fromPostList(await Client4.searchPostsWithParams(team.id, {terms: terms + where, is_or_search: true, include_deleted_channels: false, page: 0, per_page: 30}));
                    set(channel ? found.filter((i) => i.post.channel_id === channel.id) : found);
                } else if (tab === 'saved') {
                    set(fromPostList(await Client4.getFlaggedPosts(userId, channel?.id || '', channel ? '' : team.id, 0, 30)));
                } else if (tab === 'pinned') {
                    set(channel ? fromPostList(await Client4.getPinnedPosts(channel.id)) : []);
                } else if (tab === 'files') {
                    if (!channel) {
                        set([]);
                        return;
                    }
                    const found = await Client4.searchFilesWithParams(channel.team_id || '', {terms: `channel:${channel.id}`, is_or_search: false, include_deleted_channels: false, page: 0, per_page: 60});
                    const infos = found.file_infos as unknown as Record<string, FileInfo>;
                    set(found.order.map((id: string) => infos[id]).filter(Boolean).map((file: FileInfo) => ({post: {id: file.post_id} as Post, file})));
                } else {
                    // The team's followed threads (all, or only those with unread replies), then, in a conversation,
                    // those started there.
                    const threads = await Client4.getUserThreads(userId, team.id, {perPage: channel ? 200 : 30, extended: false, unread: threadsFilter === 'unread'});
                    set(threads.threads.
                        filter((t) => !channel || (t.post as Post).channel_id === channel.id).
                        map((t) => ({post: t.post as Post, replies: t.reply_count, lastReplyAt: t.last_reply_at})));
                }
            } catch {
                set([]);
            }
        })();
        return () => {
            cancelled = true;
        };
    }, [tab, team, userId, channel, mentionKeys, items, key, threadsFilter]);

    // Mark all as read: the server's one API for the whole community (there's no per-channel bulk mark-read).
    const teamId = team?.id;
    const markAllRead = () => {
        if (!teamId) {
            return;
        }
        dialogs.confirm({
            title: formatMessage({id: 'fusion.collections.markAllReadTitle', defaultMessage: 'Mark all threads as read?'}),
            message: formatMessage({id: 'fusion.collections.markAllReadMessage', defaultMessage: 'Every thread you follow in this community will show as read.'}),
            confirmLabel: formatMessage({id: 'fusion.collections.markAllRead', defaultMessage: 'Mark all as read'}),
            onConfirm: async () => {
                const result = await dispatch(markAllThreadsInTeamRead(userId, teamId));
                if (result && 'error' in result && result.error) {
                    toast((result.error as {message?: string}).message || formatMessage({id: 'fusion.collections.markAllReadFailed', defaultMessage: 'Threads couldn\'t be marked as read'}));
                    return;
                }
                setItems((prev) => ({...prev, 'threads:all': undefined, 'threads:unread': []}));
                toast(formatMessage({id: 'fusion.toast.allThreadsRead', defaultMessage: 'All threads marked as read'}));
            },
        });
    };

    const open = (item: Item) => {
        onClose();
        if (item.file) {
            dispatch(openDialog(ModalIdentifiers.FILE_PREVIEW_MODAL, FilePreviewModal, {postId: item.file.post_id, fileInfos: [item.file], startIndex: 0}));
        } else if (tab === 'threads') {
            dispatch(selectPostById(item.post.id));
        } else if (team) {
            getHistory().push(permalinkPath(team.name, item.post.id));
        }
    };

    const [, , label, desc] = TABS.find(([k]) => k === tab)!;
    const list = items[key];
    return (
        <Popover
            anchor={anchor}
            placement={placement}
            className='coll'
            label={formatMessage({id: 'fusion.collections.label', defaultMessage: 'Mentions, saved, pinned and threads'})}
            onClose={onClose}
        >
            <nav
                className={am('coll-nav')}
                role='tablist'
                aria-orientation='vertical'
            >
                {tabs.map(([k, icon, l]) => {
                    const navItems = items[cacheKey(k, 'all')];
                    return (
                        <button
                            key={k}
                            role='tab'
                            aria-selected={tab === k}
                            className={am({on: tab === k})}
                            onClick={() => setTab(k)}
                        >
                            <Icon
                                name={icon}
                                size='sm'
                            />
                            {formatMessage(l)}
                            {navItems && <span className={am('count', {hot: k === 'mentions' && navItems.length > 0})}>{navItems.length}</span>}
                        </button>
                    );
                })}
            </nav>
            <div className={am('coll-main')}>
                <div className={am('coll-head')}>
                    <h3>{formatMessage(label)}</h3>
                    {tab === 'threads' && (
                        <span
                            className={am('seg', 'coll-threads-filter')}
                            role='radiogroup'
                            aria-label={formatMessage({id: 'fusion.collections.threadsFilter', defaultMessage: 'Show threads'})}
                        >
                            {([['all', formatMessage({id: 'fusion.collections.threadsAll', defaultMessage: 'All'})], ['unread', formatMessage({id: 'fusion.collections.threadsUnread', defaultMessage: 'Unreads'})]] as const).map(([k, l]) => (
                                <button
                                    key={k}
                                    type='button'
                                    role='radio'
                                    aria-checked={threadsFilter === k}
                                    className={am({on: threadsFilter === k})}
                                    onClick={() => setThreadsFilter(k)}
                                >
                                    {l}
                                </button>
                            ))}
                        </span>
                    )}
                    {(conversation || tab === 'threads') && (
                        <span className={am('coll-scope')}>
                            {tab === 'threads' && (
                                <button
                                    type='button'
                                    className={am('icon-btn')}
                                    title={formatMessage({id: 'fusion.collections.markAllRead', defaultMessage: 'Mark all as read'})}
                                    aria-label={formatMessage({id: 'fusion.collections.markAllRead', defaultMessage: 'Mark all as read'})}
                                    onClick={markAllRead}
                                >
                                    <Icon
                                        name='check'
                                        size='sm'
                                    />
                                </button>
                            )}
                            {conversation && (
                                <span
                                    role='radiogroup'
                                    aria-label={formatMessage({id: 'fusion.collections.scope', defaultMessage: 'Show items from'})}
                                >
                                    {([['here', 'hash', formatMessage({id: 'fusion.collections.scopeHere', defaultMessage: 'This conversation'})], ['all', 'globe', formatMessage({id: 'fusion.collections.scopeAll', defaultMessage: 'The whole community'})]] as const).map(([k, icon, l]) => (
                                        <button
                                            key={k}
                                            type='button'
                                            role='radio'
                                            aria-checked={scope === k}
                                            aria-label={l}
                                            title={l}
                                            className={am('icon-btn', {on: scope === k})}
                                            onClick={() => changeScope(k)}
                                        >
                                            <Icon
                                                name={icon}
                                                size='sm'
                                            />
                                        </button>
                                    ))}
                                </span>
                            )}
                        </span>
                    )}
                    <p>{formatMessage((channel && HERE[tab]) || desc)}</p>
                </div>
                <div className={am('coll-list')}>
                    {!list && <div className={am('empty')}>{formatMessage({id: 'fusion.collections.loading', defaultMessage: 'Loading…'})}</div>}
                    {list && !list.length && <div className={am('empty')}>{formatMessage((channel && EMPTY_HERE[tab]) || EMPTY[tab])}</div>}
                    {list?.map((item) => (item.file ? (
                        <FileRow
                            key={item.file.id}
                            file={item.file}
                            onOpen={() => open(item)}
                        />
                    ) : (
                        <Row
                            key={item.post.id}
                            item={item}
                            channels={channels}
                            thread={tab === 'threads'}
                            onOpen={() => open(item)}
                        />
                    )))}
                </div>
            </div>
        </Popover>
    );
}
