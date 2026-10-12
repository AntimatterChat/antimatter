// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useLayoutEffect, useRef, useState} from 'react';
import {useIntl} from 'react-intl';
import {shallowEqual, useDispatch, useSelector} from 'react-redux';

import type {Channel} from '@mattermost/types/channels';
import type {OnTopPost, Post} from '@mattermost/types/posts';

import {fetchOnTopPosts, removePostFromTop} from 'mattermost-redux/actions/channels';
import {getPostsByIds} from 'mattermost-redux/actions/posts';
import {getMissingProfilesByIds} from 'mattermost-redux/actions/users';
import {Posts} from 'mattermost-redux/constants';
import {getOnTopPosts} from 'mattermost-redux/selectors/entities/on_top_posts';
import {isPostPriorityEnabled} from 'mattermost-redux/selectors/entities/posts';
import {getCurrentTeam} from 'mattermost-redux/selectors/entities/teams';
import {getCurrentUserId, getUser} from 'mattermost-redux/selectors/entities/users';

import Icon from 'fusion/components/icon';
import {useDisplayName} from 'fusion/hooks/users';
import MessageContent from 'fusion/messages/content/message_content';
import {jumpToMessage} from 'fusion/messages/inline_reply';
import {activeOnTop, nextExpiry, timeLeft, useNow} from 'fusion/messages/on_top';
import {useToast} from 'fusion/shell/toast_context';
import {useRoleColor} from 'fusion/teams/roles';
import {am} from 'fusion/utils/class_names';
import {permalinkPath} from 'fusion/utils/paths';
import {getHistory} from 'utils/browser_history';
import {isFromBot} from 'utils/post_utils';

import type {GlobalState} from 'types/store';

// onTopShown gives the messages on top of a channel that can be shown: still on top, and loaded (not deleted).
export function onTopShown(list: OnTopPost[], posts: Record<string, Post | undefined>, now: number): OnTopPost[] {
    return activeOnTop(list, now).filter((o) => {
        const post = posts[o.post_id];
        return post && post.state !== Posts.POST_DELETED && post.delete_at === 0;
    });
}

// OnTopStrip shows the messages put on top of a channel above its messages, in the channel banner's slot: one at a
// time, the latest put there first, with previous/next between them, its author, who put it there and for how long
// still, its first lines (all of it on Show more), a jump to it and a button taking it off the top.
export default function OnTopStrip({channel}: {channel: Channel}) {
    const intl = useIntl();
    const {formatMessage} = intl;
    const dispatch = useDispatch();
    const toast = useToast();
    const list = useSelector((state: GlobalState) => getOnTopPosts(state, channel.id));
    const posts = useSelector((state: GlobalState) => {
        const found: Record<string, Post | undefined> = {};
        for (const o of getOnTopPosts(state, channel.id)) {
            found[o.post_id] = state.entities.posts.posts[o.post_id];
        }
        return found;
    }, shallowEqual);
    const now = useNow(nextExpiry(list, Date.now()));
    const shown = onTopShown(list, posts, now);
    const [index, setIndex] = useState(0);
    const [open, setOpen] = useState(false);

    // What's on top is loaded on opening the channel; the websocket keeps it up to date.
    useEffect(() => {
        dispatch(fetchOnTopPosts(channel.id));
        setIndex(0);
        setOpen(false);
    }, [channel.id, dispatch]);

    // The messages on top that aren't loaded yet are fetched, once.
    const asked = useRef(new Set<string>());
    useEffect(() => {
        const missing = activeOnTop(list, Date.now()).map((o) => o.post_id).filter((id) => !posts[id] && !asked.current.has(id));
        if (missing.length) {
            missing.forEach((id) => asked.current.add(id));
            dispatch(getPostsByIds(missing));
        }
    }, [list, posts, dispatch]);

    // A message just put on top shows first.
    const latest = list[0] ? list[0].post_id + ':' + list[0].create_at : '';
    useEffect(() => {
        setIndex(0);
        setOpen(false);
    }, [latest]);

    if (!shown.length) {
        return null;
    }
    const current = Math.min(index, shown.length - 1);
    const entry = shown[current];
    const go = (step: number) => {
        setIndex((current + step + shown.length) % shown.length);
        setOpen(false);
    };
    const remove = async () => {
        const result = await dispatch(removePostFromTop(channel.id, entry.post_id));
        if (result && 'error' in result && result.error) {
            toast((result.error as {message?: string}).message || formatMessage({id: 'fusion.ontop.removeFailed', defaultMessage: 'The message couldn\'t be taken off the top'}));
            return;
        }
        toast(formatMessage({id: 'fusion.ontop.toast.removed', defaultMessage: 'Removed from the top of the channel'}));
    };

    return (
        <OnTopBanner
            entry={entry}
            post={posts[entry.post_id]!}
            now={now}
            position={shown.length > 1 ? {current, count: shown.length, go} : undefined}
            open={open}
            setOpen={setOpen}
            canRemove={channel.delete_at === 0}
            onRemove={remove}
        />
    );
}

type BannerProps = {
    entry: OnTopPost;
    post: Post;
    now: number;
    position?: {current: number; count: number; go: (step: number) => void};
    open: boolean;
    setOpen: (open: boolean) => void;
    canRemove: boolean;
    onRemove: () => void;
};

function OnTopBanner({entry, post, now, position, open, setOpen, canRemove, onRemove}: BannerProps) {
    const intl = useIntl();
    const {formatMessage} = intl;
    const team = useSelector(getCurrentTeam);
    const me = useSelector(getCurrentUserId);
    const author = useSelector((state: GlobalState) => getUser(state, post.user_id));
    const by = useSelector((state: GlobalState) => getUser(state, entry.created_by));
    const authorName = useDisplayName(author);
    const byName = useDisplayName(by);
    const roleColor = useRoleColor(post.user_id);
    const priorityEnabled = useSelector(isPostPriorityEnabled);
    const urgent = priorityEnabled && post.metadata?.priority?.priority === 'urgent';
    const bot = isFromBot(post) || Boolean(author?.is_bot);

    // The author and who put it on top may not be loaded yet.
    const dispatch = useDispatch();
    useEffect(() => {
        dispatch(getMissingProfilesByIds([post.user_id, entry.created_by]));
    }, [post.user_id, entry.created_by, dispatch]);

    // Up to five lines show; Show more (or a click on the text) shows all of it.
    const textRef = useRef<HTMLDivElement>(null);
    const [long, setLong] = useState(false);
    useLayoutEffect(() => {
        const el = textRef.current;
        if (!el) {
            return undefined;
        }
        const measure = () => setLong(open || el.scrollHeight > el.clientHeight + 1);
        measure();
        if (typeof ResizeObserver === 'undefined') {
            return undefined;
        }

        // The text may grow without its box growing, e.g. as images load.
        const observer = new ResizeObserver(measure);
        observer.observe(el);
        if (el.firstElementChild) {
            observer.observe(el.firstElementChild);
        }
        return () => observer.disconnect();
    }, [open, post.id]);

    const jump = () => {
        const messages = document.querySelector<HTMLElement>(`#am-view .${am('msgs')}`);
        if (messages && jumpToMessage(messages, post.id)) {
            return;
        }
        if (team) {
            getHistory().push(permalinkPath(team.name, post.id));
        }
    };

    return (
        <div
            className={am('ontop', {urgent, open})}
            role='region'
            aria-label={formatMessage({id: 'fusion.ontop.region', defaultMessage: 'On top of this channel'})}
        >
            <span className={am('ot-ic')}>
                <Icon
                    name={urgent ? 'flag' : 'pin-top'}
                    size='sm'
                />
            </span>
            <div className={am('ot-body')}>
                <div className={am('ot-meta')}>
                    <b style={roleColor && !bot ? {color: roleColor} : undefined}>{authorName}</b>
                    {bot && <span className={am('bot-tag')}>{'BOT'}</span>}
                    <span>
                        {entry.created_by === me ? formatMessage({id: 'fusion.ontop.byYou', defaultMessage: '· put on top by you'}) : formatMessage({id: 'fusion.ontop.by', defaultMessage: '· put on top by {name}'}, {name: byName})}
                    </span>
                    <span className={am('left')}>
                        <Icon name='clock'/>
                        {timeLeft(intl, entry.expires_at, now)}
                    </span>
                </div>
                <div
                    ref={textRef}
                    className={am('ot-rich')}
                    dir='auto'
                    onClick={(e) => {
                        if (long && !(e.target as HTMLElement).closest('a, button, input, img, video')) {
                            setOpen(!open);
                        }
                    }}
                >
                    <MessageContent
                        post={post}
                        inThread={false}
                        autotranslated={false}
                    />
                </div>
                {long && (
                    <button
                        type='button'
                        className={am('ot-more')}
                        aria-expanded={open}
                        onClick={() => setOpen(!open)}
                    >
                        {open ? formatMessage({id: 'fusion.ontop.showLess', defaultMessage: 'Show less'}) : formatMessage({id: 'fusion.ontop.showMore', defaultMessage: 'Show more'})}
                    </button>
                )}
            </div>
            <div className={am('ot-actions')}>
                {position && (
                    <>
                        <button
                            type='button'
                            className={am('icon-btn')}
                            data-ontop-nav='-1'
                            aria-label={formatMessage({id: 'fusion.ontop.previous', defaultMessage: 'Previous'})}
                            title={formatMessage({id: 'fusion.ontop.previous', defaultMessage: 'Previous'})}
                            onClick={() => position.go(-1)}
                        >
                            <Icon
                                name='chev'
                                size='xs'
                            />
                        </button>
                        <span className={am('nav')}>{`${position.current + 1}/${position.count}`}</span>
                        <button
                            type='button'
                            className={am('icon-btn')}
                            data-ontop-nav='1'
                            aria-label={formatMessage({id: 'fusion.ontop.next', defaultMessage: 'Next'})}
                            title={formatMessage({id: 'fusion.ontop.next', defaultMessage: 'Next'})}
                            onClick={() => position.go(1)}
                        >
                            <Icon
                                name='chev'
                                size='xs'
                            />
                        </button>
                    </>
                )}
                <button
                    type='button'
                    className={am('jump')}
                    title={formatMessage({id: 'fusion.ontop.jumpTitle', defaultMessage: 'Jump to the message'})}
                    onClick={jump}
                >
                    {formatMessage({id: 'fusion.ontop.jump', defaultMessage: 'Jump'})}
                </button>
                {canRemove && (
                    <button
                        type='button'
                        className={am('icon-btn')}
                        title={formatMessage({id: 'fusion.ontop.remove', defaultMessage: 'Remove from top'})}
                        aria-label={formatMessage({id: 'fusion.ontop.remove', defaultMessage: 'Remove from top'})}
                        onClick={onRemove}
                    >
                        <Icon
                            name='x'
                            size='sm'
                        />
                    </button>
                )}
            </div>
        </div>
    );
}
