// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useMemo, useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import type {Channel, ChannelKindMode} from '@mattermost/types/channels';
import type {FileInfo} from '@mattermost/types/files';
import type {ForumTopic, ForumTopicSort, Post} from '@mattermost/types/posts';

import {getChannelTypeInfo} from 'mattermost-redux/selectors/entities/channel_types';
import {canStartConversations} from 'mattermost-redux/selectors/entities/roles';
import {getCurrentTeam} from 'mattermost-redux/selectors/entities/teams';
import {getThread} from 'mattermost-redux/selectors/entities/threads';
import {getFilePreviewUrl, getFileUrl} from 'mattermost-redux/utils/file_utils';

import {FORUM_VIEW_OPTIONS} from 'fusion/channel/channel_type_presets';
import Avatar from 'fusion/components/avatar';
import Icon from 'fusion/components/icon';
import {useDisplayName, useUser} from 'fusion/hooks/users';
import {isImage} from 'fusion/messages/content/files';
import {getThreadTitle} from 'fusion/messages/thread_title';
import {useWhen} from 'fusion/messages/time';
import {am} from 'fusion/utils/class_names';
import {topicPath} from 'fusion/utils/paths';
import {plainText} from 'fusion/utils/plain_text';
import {getHistory} from 'utils/browser_history';

import type {GlobalState} from 'types/store';

import {openTopicLabel} from './labels';
import NewTopicForm from './new_topic_form';
import {buildTopicsQuery, defaultSortForMode, filtersForMode, sortsForMode} from './query';
import type {TopicFilter} from './query';
import {topicStatusLabel} from './status';
import {useForumTopics} from './topics_data';
import {isTopicUnread} from './unread';
import {getForumView, setForumView} from './view_preference';

function firstImageFile(post: Post): FileInfo | undefined {
    return post.metadata?.files?.find((f) => isImage(f, false));
}

function thumbSrc(file: FileInfo): string {
    const ext = (file.extension || '').toLowerCase();
    return file.has_preview_image && ext !== 'gif' ? getFilePreviewUrl(file.id) : getFileUrl(file.id);
}

function TopicRow({topic, channel, mode}: {topic: ForumTopic; channel: Channel; mode: ChannelKindMode}) {
    const {formatMessage} = useIntl();
    const team = useSelector(getCurrentTeam);
    const when = useWhen();
    const lastViewedAt = useSelector((state: GlobalState) => state.entities.channels.myMembers[channel.id]?.last_viewed_at || 0);
    const thread = useSelector((state: GlobalState) => getThread(state, topic.post.id));
    const unread = isTopicUnread(topic, lastViewedAt, thread || undefined);
    const author = useUser(topic.post.user_id);
    const authorName = useDisplayName(author, channel.team_id);
    const title = getThreadTitle(topic.post) || topic.post.message;
    const firstLine = plainText(topic.post.message, 140);
    const image = firstImageFile(topic.post);
    const others = topic.participant_ids.filter((id) => id !== topic.post.user_id).slice(0, 3);

    const open = () => team && getHistory().push(topicPath(team.name, channel, topic.post.id));

    return (
        <div
            className={am('topic-row', {unread})}
            role='listitem'
        >
            {topic.on_top && (
                <Icon
                    name='pin-top'
                    size='xs'
                />
            )}
            {image && (
                <img
                    className={am('topic-thumb')}
                    src={thumbSrc(image)}
                    alt=''
                />
            )}
            <div className={am('topic-main')}>
                <button
                    type='button'
                    className={am('topic-title')}
                    onClick={open}
                >
                    {title}
                </button>
                {firstLine && <div className={am('topic-line')}>{firstLine}</div>}
                <div className={am('topic-people')}>
                    <Avatar
                        userId={topic.post.user_id}
                        size='sm'
                    />
                    <span>{authorName}</span>
                    {others.map((id) => (
                        <Avatar
                            key={id}
                            userId={id}
                            size='sm'
                        />
                    ))}
                </div>
            </div>
            <div className={am('topic-meta')}>
                <span>{formatMessage({id: 'fusion.forum.replyCount', defaultMessage: '{count, plural, one {# reply} other {# replies}}', description: 'A forum topic row: how many replies it has.'}, {count: topic.reply_count})}</span>
                <span>{when(topic.last_reply_at || topic.post.create_at)}</span>
            </div>
            {mode === 'qa' && (
                <div className={am('topic-answered', {on: Boolean(topic.accepted_answer_id)})}>
                    {topic.accepted_answer_id && (
                        <Icon
                            name='check'
                            size='xs'
                        />
                    )}
                    {topic.accepted_answer_id ? formatMessage({id: 'fusion.forum.answered', defaultMessage: 'Answered', description: 'A Q&A topic row: it has an accepted answer.'}) : formatMessage({id: 'fusion.forum.unansweredMark', defaultMessage: 'Unanswered', description: 'A Q&A topic row: it has no accepted answer yet.'})}
                </div>
            )}
            {mode === 'ideas' && (
                <div className={am('topic-status', topic.status || 'open')}>
                    {formatMessage(topicStatusLabel(topic.status || 'open'))}
                </div>
            )}
            {(mode === 'qa' || mode === 'ideas') && (
                <div
                    className={am('topic-votes')}
                    aria-label={formatMessage({id: 'fusion.forum.voteCount', defaultMessage: '{count} votes', description: 'A forum topic row: its 👍 vote count.'}, {count: topic.votes})}
                >
                    <span aria-hidden='true'>{'👍'}</span>
                    {topic.votes}
                </div>
            )}
        </div>
    );
}

// TopicsView replaces the message list in a forum channel (design §4.4): no bottom composer, its own search and
// sort/filter bar, and the New topic form opening at the top of the list.
export default function TopicsView({channel}: {channel: Channel}) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const typeInfo = useSelector((state: GlobalState) => getChannelTypeInfo(state, channel.id));
    const mode = typeInfo.mode;
    const canOpen = useSelector((state: GlobalState) => canStartConversations(state, channel));
    const view = useSelector((state: GlobalState) => getForumView(state, channel.id, typeInfo.view || 'topics'));

    const [sort, setSort] = useState<ForumTopicSort>(() => defaultSortForMode(mode));
    const [filter, setFilter] = useState<TopicFilter>('all');
    const [search, setSearch] = useState('');
    const [composing, setComposing] = useState(false);
    const lastViewedAt = useSelector((state: GlobalState) => state.entities.channels.myMembers[channel.id]?.last_viewed_at || 0);
    const threads = useSelector((state: GlobalState) => state.entities.threads.threads);

    const query = useMemo(() => buildTopicsQuery(sort, filter, search), [sort, filter, search]);
    const {topics: fetched, loading, error, hasMore, loadMore} = useForumTopics(channel.id, query);

    // The server has no "unread" filter (GET /channels/{id}/topics's parameters, design §3.3): it's the one filter
    // applied to the fetched page here, on the client.
    const topics = filter === 'unread' ? fetched.filter((t) => isTopicUnread(t, lastViewedAt, threads[t.post.id])) : fetched;

    const sorts = sortsForMode(mode);
    const filters = filtersForMode(mode);
    const empty = !loading && topics.length === 0;

    return (
        <div className={am('forum-view')}>
            <div className={am('forum-bar')}>
                <div
                    className={am('seg')}
                    role='radiogroup'
                    aria-label={formatMessage({id: 'fusion.forum.viewSwitch', defaultMessage: 'Forum view', description: 'The Topics/Image board view switch in a forum channel.'})}
                >
                    <button
                        type='button'
                        role='radio'
                        aria-checked={view === 'topics'}
                        className={am({on: view === 'topics'})}
                        onClick={() => dispatch(setForumView(channel.id, 'topics'))}
                    >
                        {formatMessage(FORUM_VIEW_OPTIONS.find((o) => o.id === 'topics')!.label)}
                    </button>
                    <button
                        type='button'
                        role='radio'
                        aria-checked={view === 'imageboard'}
                        className={am({on: view === 'imageboard'})}
                        onClick={() => dispatch(setForumView(channel.id, 'imageboard'))}
                    >
                        {formatMessage(FORUM_VIEW_OPTIONS.find((o) => o.id === 'imageboard')!.label)}
                    </button>
                </div>
                <input
                    type='search'
                    className={am('forum-search')}
                    value={search}
                    onChange={(e) => setSearch(e.target.value)}
                    placeholder={formatMessage({id: 'fusion.forum.searchPlaceholder', defaultMessage: 'Search topics…', description: 'The forum topics view\'s own search box.'})}
                    aria-label={formatMessage({id: 'fusion.forum.searchPlaceholder', defaultMessage: 'Search topics…', description: 'The forum topics view\'s own search box.'})}
                />
                {canOpen && (
                    <button
                        type='button'
                        className={am('btn', 'primary')}
                        onClick={() => setComposing(true)}
                    >
                        {formatMessage(openTopicLabel(mode))}
                    </button>
                )}
            </div>
            {channel.purpose && (
                <div className={am('forum-guidelines')}>
                    {formatMessage({id: 'fusion.forum.guidelines', defaultMessage: 'Guidelines: {purpose}', description: 'The forum\'s own guidelines, shown from the channel\'s purpose.'}, {purpose: channel.purpose})}
                </div>
            )}
            <div className={am('forum-filters')}>
                <div
                    className={am('chips')}
                    role='radiogroup'
                    aria-label={formatMessage({id: 'fusion.forum.filterLabel', defaultMessage: 'Filter topics', description: 'The forum topics view\'s filter chips (All, Unread, Mine, Following, and the mode\'s own).'})}
                >
                    {filters.map((f) => (
                        <button
                            key={f.id}
                            type='button'
                            role='radio'
                            aria-checked={filter === f.id}
                            className={am('chip-btn', {on: filter === f.id})}
                            onClick={() => setFilter(f.id)}
                        >
                            {formatMessage(f.label)}
                        </button>
                    ))}
                </div>
                <label className={am('forum-sort')}>
                    <span>{formatMessage({id: 'fusion.forum.sortLabel', defaultMessage: 'Sort', description: 'Label of the forum topics view\'s sort dropdown.'})}</span>
                    <select
                        value={sort}
                        onChange={(e) => setSort(e.target.value as ForumTopicSort)}
                    >
                        {sorts.map((s) => (
                            <option
                                key={s.id}
                                value={s.id}
                            >
                                {formatMessage(s.label)}
                            </option>
                        ))}
                    </select>
                </label>
            </div>
            <div
                className={am('topic-list')}
                role='list'
            >
                {composing && (
                    <NewTopicForm
                        channel={channel}
                        mode={mode}
                        onClose={() => setComposing(false)}
                    />
                )}
                {error && <div className={am('note-box')}>{error}</div>}
                {empty && !error && (
                    <div className={am('empty')}>
                        {search.trim() ? formatMessage({id: 'fusion.forum.noResults', defaultMessage: 'No topics found for "{search}".', description: 'The forum topics view\'s empty state while searching.'}, {search: search.trim()}) : formatMessage({id: 'fusion.forum.noTopics', defaultMessage: 'No topics yet.', description: 'The forum topics view\'s empty state, no search in progress.'})}
                    </div>
                )}
                {topics.map((topic) => (
                    <TopicRow
                        key={topic.post.id}
                        topic={topic}
                        channel={channel}
                        mode={mode}
                    />
                ))}
                {hasMore && !loading && (
                    <button
                        type='button'
                        className={am('load-more')}
                        onClick={loadMore}
                    >
                        {formatMessage({id: 'fusion.forum.loadMore', defaultMessage: 'Load more topics', description: 'Button at the end of the forum topics view\'s list, loading the next page.'})}
                    </button>
                )}
            </div>
        </div>
    );
}
