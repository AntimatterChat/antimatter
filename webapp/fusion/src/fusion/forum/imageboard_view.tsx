// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useRef} from 'react';
import {useIntl} from 'react-intl';
import {useSelector} from 'react-redux';

import type {Channel, ChannelKindMode} from '@mattermost/types/channels';
import type {ForumTopic, ForumTopicsQuery} from '@mattermost/types/posts';

import {getCurrentTeam} from 'mattermost-redux/selectors/entities/teams';

import {useDisplayName, useUser} from 'fusion/hooks/users';
import {isImage} from 'fusion/messages/content/files';
import {getThreadTitle} from 'fusion/messages/thread_title';
import {useWhen} from 'fusion/messages/time';
import {am} from 'fusion/utils/class_names';
import {topicPath} from 'fusion/utils/paths';
import {plainText} from 'fusion/utils/plain_text';
import {getHistory} from 'utils/browser_history';

import {firstImageFile, thumbSrc} from './topic_image';
import {useForumTopics} from './topics_data';
import VoteButton from './vote_button';

// How many pixels from the bottom of the board a scroll triggers the next page (design §4.4: "pages of 10 topics,
// with infinite scroll"), matching the threshold the search results panel and the member list use.
const SCROLL_LOAD_THRESHOLD = 400;

// The image board's own query (design §4.4): the same endpoint as the topics view ("same data as the topics view,
// just a different view", the owner's decision), but its own fixed page of 10 and the latest 3 replies of each
// topic, always in bump order (latest activity) — not the topics view's own sort, filter or search.
const IMAGEBOARD_QUERY: ForumTopicsQuery = {sort: 'activity', replies: 3, per_page: 10};

// The images a card counts (design §4.4: "N replies, M images"): the first post's and the latest 3 replies' — the
// only ones this query fetches. There's no separate image count from the server (the owner's decision is to reuse
// the topics view's data, not add one), so this undercounts a topic with more than 3 replies carrying images.
function countImages(topic: ForumTopic): number {
    const posts = [topic.post, ...(topic.replies || [])];
    return posts.reduce((n, post) => n + (post.metadata?.files?.filter((f) => isImage(f, false)).length || 0), 0);
}

function BoardCard({topic, channel, mode}: {topic: ForumTopic; channel: Channel; mode: ChannelKindMode}) {
    const {formatMessage} = useIntl();
    const team = useSelector(getCurrentTeam);
    const when = useWhen();
    const author = useUser(topic.post.user_id);
    const authorName = useDisplayName(author, channel.team_id);
    const title = getThreadTitle(topic.post) || topic.post.message;
    const body = plainText(topic.post.message, 600);
    const image = firstImageFile(topic.post);
    const images = countImages(topic);
    const replies = topic.replies || [];

    // A short, non-authoritative reference (design §4.4's mockup shows "#a1f3"): the first few characters of the
    // topic's own post id, not a separate id of its own.
    const shortId = topic.post.id.slice(0, 4);

    const open = () => team && getHistory().push(topicPath(team.name, channel, topic.post.id));

    return (
        <div
            className={am('board-card', {'no-image': !image})}
            role='listitem'
        >
            {image && (
                <img
                    className={am('board-thumb')}
                    src={thumbSrc(image)}
                    alt=''
                />
            )}
            <div className={am('board-main')}>
                <button
                    type='button'
                    className={am('board-title')}
                    onClick={open}
                >
                    {title}
                </button>
                <div className={am('board-meta')}>
                    <span>{authorName}</span>
                    <span>{when(topic.last_reply_at || topic.post.create_at)}</span>
                    <span
                        className={am('board-id')}
                        aria-label={formatMessage({id: 'fusion.forum.boardTopicId', defaultMessage: 'Topic {id}', description: 'An image board card: its topic\'s short id, shown for reference only (design §4.4\'s mockup: "#a1f3").'}, {id: shortId})}
                    >
                        {`#${shortId}`}
                    </span>
                    {(mode === 'qa' || mode === 'ideas') && (
                        <VoteButton
                            post={topic.post}
                            className='board-vote'
                        />
                    )}
                </div>
                {body && <div className={am('board-body')}>{body}</div>}
                <div className={am('board-footer')}>
                    <span>
                        {formatMessage({id: 'fusion.forum.boardCounts', defaultMessage: '{replies, plural, one {# reply} other {# replies}}, {images, plural, one {# image} other {# images}}', description: 'An image board card: its reply and image counts (design §4.4\'s mockup: "34 replies, 12 images").'}, {replies: topic.reply_count, images})}
                    </span>
                    <button
                        type='button'
                        className={am('btn')}
                        onClick={open}
                    >
                        {formatMessage({id: 'fusion.forum.boardOpenTopic', defaultMessage: 'Open topic', description: 'Button on an image board card that opens the topic page.'})}
                    </button>
                </div>
                {replies.length > 0 && (
                    <div className={am('board-replies')}>
                        {replies.map((reply) => {
                            const thumb = firstImageFile(reply);
                            return (
                                <div
                                    key={reply.id}
                                    className={am('board-reply')}
                                >
                                    {thumb && (
                                        <img
                                            className={am('board-reply-thumb')}
                                            src={thumbSrc(thumb)}
                                            alt=''
                                        />
                                    )}
                                    <span className={am('board-reply-text')}>{plainText(reply.message, 80)}</span>
                                </div>
                            );
                        })}
                    </div>
                )}
            </div>
        </div>
    );
}

// ImageboardView is the forum's image board (design §4.4): a page of topics at a time, newest activity first,
// each a card with its first image large, title, author, age and short id, the first post clipped to a few lines,
// the reply and image counts, an Open topic button, and its latest 3 replies, compact. Topics without an image show
// text only. It owns its own scroll area (fusion/forum/topics_view.tsx's forum-view already scrolls the view as a
// whole, but this one pages on its own scroll, not a "Load more" button), so it fetches and pages independently of
// the topics view's row list.
export default function ImageboardView({channel, mode}: {channel: Channel; mode: ChannelKindMode}) {
    const {formatMessage} = useIntl();
    const {topics, loading, error, hasMore, loadMore} = useForumTopics(channel.id, IMAGEBOARD_QUERY);
    const empty = !loading && topics.length === 0;
    const boardRef = useRef<HTMLDivElement>(null);

    const onScroll = () => {
        const el = boardRef.current;
        if (!el || !hasMore || loading) {
            return;
        }
        if (el.scrollHeight - el.scrollTop - el.clientHeight < SCROLL_LOAD_THRESHOLD) {
            loadMore();
        }
    };

    return (
        <div
            ref={boardRef}
            className={am('board-view')}
            role='list'
            onScroll={onScroll}
        >
            {error && <div className={am('note-box')}>{error}</div>}
            {empty && !error && (
                <div className={am('empty')}>
                    {formatMessage({id: 'fusion.forum.noTopicsBoard', defaultMessage: 'No topics yet.', description: 'The image board view\'s empty state.'})}
                </div>
            )}
            {topics.map((topic) => (
                <BoardCard
                    key={topic.post.id}
                    topic={topic}
                    channel={channel}
                    mode={mode}
                />
            ))}
        </div>
    );
}
