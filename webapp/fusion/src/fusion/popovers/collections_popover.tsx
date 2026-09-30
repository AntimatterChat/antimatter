// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import type {Channel} from '@mattermost/types/channels';
import type {Post} from '@mattermost/types/posts';

import {Client4} from 'mattermost-redux/client';
import {getAllChannels} from 'mattermost-redux/selectors/entities/channels';
import {getCurrentTeam} from 'mattermost-redux/selectors/entities/teams';
import {getCurrentUserId, getCurrentUserMentionKeys} from 'mattermost-redux/selectors/entities/users';

import {selectPostById} from 'actions/views/rhs';

import Avatar from 'fusion/components/avatar';
import Icon from 'fusion/components/icon';
import type {IconName} from 'fusion/components/icon';
import {Popover} from 'fusion/components/layer';
import {useDisplayName, useUser} from 'fusion/hooks/users';
import {useWhen} from 'fusion/messages/time';
import {am} from 'fusion/utils/class_names';
import {permalinkPath} from 'fusion/utils/paths';
import {getHistory} from 'utils/browser_history';

type Tab = 'mentions' | 'saved' | 'pinned' | 'threads';

const TABS: Array<[Tab, IconName, {id: string; defaultMessage: string}, {id: string; defaultMessage: string}]> = [
    ['mentions', 'at', {id: 'fusion.collections.mentions', defaultMessage: 'Mentions'}, {id: 'fusion.collections.mentionsDesc', defaultMessage: 'Messages that mention you'}],
    ['saved', 'bookmark', {id: 'fusion.collections.saved', defaultMessage: 'Saved'}, {id: 'fusion.collections.savedDesc', defaultMessage: 'Messages you saved for later'}],
    ['pinned', 'pin', {id: 'fusion.collections.pinned', defaultMessage: 'Pinned'}, {id: 'fusion.collections.pinnedDesc', defaultMessage: 'Messages pinned in this conversation'}],
    ['threads', 'thread', {id: 'fusion.collections.threads', defaultMessage: 'Threads'}, {id: 'fusion.collections.threadsDesc', defaultMessage: 'Threads you follow'}],
];

const EMPTY: Record<Tab, {id: string; defaultMessage: string}> = {
    mentions: {id: 'fusion.collections.noMentions', defaultMessage: 'No one has mentioned you yet.'},
    saved: {id: 'fusion.collections.noSaved', defaultMessage: 'Save messages from their ⋯ menu to find them here.'},
    pinned: {id: 'fusion.collections.noPinned', defaultMessage: 'Nothing pinned yet. Pin a message from its ⋯ menu.'},
    threads: {id: 'fusion.collections.noThreads', defaultMessage: 'Follow a thread to keep track of new replies.'},
};

type Item = {post: Post; replies?: number; lastReplyAt?: number};

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
                        {item.lastReplyAt ? ` · ${formatMessage({id: 'fusion.collections.last', defaultMessage: 'last {when}'}, {when: when(item.lastReplyAt)})}` : ''}
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
    onClose: () => void;
};

// CollectionsPopover gathers mentions, saved and pinned messages, and followed threads, in one panel.
export default function CollectionsPopover({anchor, channel, initialTab = 'mentions', onClose}: Props) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const team = useSelector(getCurrentTeam);
    const userId = useSelector(getCurrentUserId);
    const mentionKeys = useSelector(getCurrentUserMentionKeys);
    const channels = useSelector(getAllChannels);
    const [tab, setTab] = useState<Tab>(initialTab);
    const [items, setItems] = useState<Record<Tab, Item[] | undefined>>({mentions: undefined, saved: undefined, pinned: undefined, threads: undefined});

    useEffect(() => {
        if (!team || items[tab]) {
            return undefined;
        }
        let cancelled = false;
        const set = (list: Item[]) => !cancelled && setItems((prev) => ({...prev, [tab]: list}));
        const fromPostList = (list: {order: string[]; posts: Record<string, Post>}) => list.order.map((id) => ({post: list.posts[id]})).filter((i) => i.post);
        (async () => {
            try {
                if (tab === 'mentions') {
                    const terms = mentionKeys.filter(({key}) => !['@channel', '@all', '@here'].includes(key)).map(({key}) => key).join(' ');
                    set(fromPostList(await Client4.searchPostsWithParams(team.id, {terms, is_or_search: true, include_deleted_channels: false, page: 0, per_page: 30})));
                } else if (tab === 'saved') {
                    set(fromPostList(await Client4.getFlaggedPosts(userId, '', '', 0, 30)));
                } else if (tab === 'pinned') {
                    set(channel ? fromPostList(await Client4.getPinnedPosts(channel.id)) : []);
                } else {
                    const threads = await Client4.getUserThreads(userId, team.id, {perPage: 30, extended: false});
                    set(threads.threads.map((t) => ({post: t.post as Post, replies: t.reply_count, lastReplyAt: t.last_reply_at})));
                }
            } catch {
                set([]);
            }
        })();
        return () => {
            cancelled = true;
        };
    }, [tab, team, userId, channel, mentionKeys, items]);

    const open = (item: Item) => {
        onClose();
        if (tab === 'threads') {
            dispatch(selectPostById(item.post.id));
        } else if (team) {
            getHistory().push(permalinkPath(team.name, item.post.id));
        }
    };

    const [, , label, desc] = TABS.find(([k]) => k === tab)!;
    const list = items[tab];
    return (
        <Popover
            anchor={anchor}
            placement='below'
            className='coll'
            label={formatMessage({id: 'fusion.collections.label', defaultMessage: 'Mentions, saved, pinned and threads'})}
            onClose={onClose}
        >
            <nav
                className={am('coll-nav')}
                role='tablist'
                aria-orientation='vertical'
            >
                {TABS.map(([k, icon, l]) => (
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
                        {items[k] && <span className={am('count', {hot: k === 'mentions' && items[k]!.length > 0})}>{items[k]!.length}</span>}
                    </button>
                ))}
            </nav>
            <div className={am('coll-main')}>
                <div className={am('coll-head')}>
                    <h3>{formatMessage(label)}</h3>
                    <p>{formatMessage(desc)}</p>
                </div>
                <div className={am('coll-list')}>
                    {!list && <div className={am('empty')}>{formatMessage({id: 'fusion.collections.loading', defaultMessage: 'Loading…'})}</div>}
                    {list && !list.length && <div className={am('empty')}>{formatMessage(EMPTY[tab])}</div>}
                    {list?.map((item) => (
                        <Row
                            key={item.post.id}
                            item={item}
                            channels={channels}
                            thread={tab === 'threads'}
                            onOpen={() => open(item)}
                        />
                    ))}
                </div>
            </div>
        </Popover>
    );
}
