// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useRef} from 'react';
import {useIntl} from 'react-intl';
import {shallowEqual, useDispatch, useSelector} from 'react-redux';

import type {Post} from '@mattermost/types/posts';

import {getMorePostsForSearch} from 'mattermost-redux/actions/search';
import {getAllChannels} from 'mattermost-redux/selectors/entities/channels';
import {getCurrentTeam} from 'mattermost-redux/selectors/entities/teams';

import {closeRightHandSide} from 'actions/views/rhs';
import {getSearchTeam, getSearchTerms} from 'selectors/rhs';

import Icon from 'fusion/components/icon';
import {useDisplayName, useUser} from 'fusion/hooks/users';
import {useWhen} from 'fusion/messages/time';
import {am} from 'fusion/utils/class_names';
import {permalinkPath} from 'fusion/utils/paths';
import {plainText} from 'fusion/utils/plain_text';
import {getHistory} from 'utils/browser_history';
import {RHSStates} from 'utils/constants';

import type {GlobalState} from 'types/store';

function highlight(text: string, terms: string) {
    const words = terms.split(/\s+/).filter((w) => w && !w.includes(':') && w.length > 1).map((w) => w.replace(/^["@#]|"$/g, ''));
    if (!words.length) {
        return text;
    }
    const re = new RegExp('(' + words.map((w) => w.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')).join('|') + ')', 'ig');
    return text.split(re).map((part, i) => (i % 2 ? <mark key={i}>{part}</mark> : part));
}

function Result({post, terms}: {post: Post; terms: string}) {
    const when = useWhen();
    const user = useUser(post.user_id);
    const name = useDisplayName(user);
    const channel = useSelector((state: GlobalState) => getAllChannels(state)[post.channel_id]);
    const team = useSelector(getCurrentTeam);
    const direct = channel && (channel.type === 'D' || channel.type === 'G');
    let icon: 'chat' | 'lock' | 'hash' = 'hash';
    if (direct) {
        icon = 'chat';
    } else if (channel?.type === 'P') {
        icon = 'lock';
    }
    return (
        <button
            className={am('result')}
            onClick={() => team && getHistory().push(permalinkPath(team.name, post.id))}
        >
            <span className={am('where')}>
                <Icon
                    name={icon}
                    size='xs'
                />
                {`${channel?.display_name || ''}${post.root_id ? ' · thread' : ''} · ${when(post.create_at)}`}
            </span>
            <b>{name}</b>
            <div>{highlight(plainText(post.message), terms)}</div>
        </button>
    );
}

// ResultsPanel lists search results, mentions, and saved or pinned messages in the right-hand panel.
export default function ResultsPanel({rhsState}: {rhsState: string}) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const terms = useSelector(getSearchTerms);
    const channelId = useSelector((state: GlobalState) => state.views.rhs.selectedChannelId || state.entities.channels.currentChannelId);
    const ids = useSelector((state: GlobalState) => {
        const search = state.entities.search;
        if (rhsState === RHSStates.FLAG) {
            return search.flagged;
        }
        if (rhsState === RHSStates.PIN) {
            return search.pinned[channelId] || [];
        }
        return search.results;
    });
    const posts = useSelector((state: GlobalState) => ids.map((id) => state.entities.posts.posts[id]).filter(Boolean), shallowEqual);
    const searching = useSelector((state: GlobalState) => state.entities.search.isSearchingTerm || state.views.rhs.isSearchingFlaggedPost || state.views.rhs.isSearchingPinnedPost);

    // Search results and mentions come a page at a time: the next page loads near the bottom.
    const pagedTeam = useSelector((state: GlobalState) => (rhsState === RHSStates.MENTION ? '' : getSearchTeam(state)));
    const paged = rhsState === RHSStates.SEARCH || rhsState === RHSStates.MENTION;
    const atEnd = useSelector((state: GlobalState) => state.entities.search.current[pagedTeam || 'ALL_TEAMS']?.isEnd ?? true);
    const gettingMore = useSelector((state: GlobalState) => state.entities.search.isSearchGettingMore);
    const bodyRef = useRef<HTMLDivElement>(null);
    const onScroll = () => {
        const body = bodyRef.current;
        if (!paged || atEnd || gettingMore || searching || !body) {
            return;
        }
        if (body.scrollHeight - body.scrollTop - body.clientHeight < 300) {
            dispatch(getMorePostsForSearch(pagedTeam));
        }
    };

    const title = {
        [RHSStates.SEARCH]: formatMessage({id: 'fusion.results.search', defaultMessage: 'Search results'}),
        [RHSStates.MENTION]: formatMessage({id: 'fusion.results.mentions', defaultMessage: 'Recent mentions'}),
        [RHSStates.FLAG]: formatMessage({id: 'fusion.results.saved', defaultMessage: 'Saved messages'}),
        [RHSStates.PIN]: formatMessage({id: 'fusion.results.pinned', defaultMessage: 'Pinned messages'}),
    }[rhsState];
    const sub = rhsState === RHSStates.SEARCH ? `“${terms.trim()}” · ${formatMessage({id: 'fusion.results.count', defaultMessage: '{count, plural, one {# match} other {# matches}}'}, {count: posts.length})}` : formatMessage({id: 'fusion.results.countOnly', defaultMessage: '{count, plural, one {# message} other {# messages}}'}, {count: posts.length});

    return (
        <>
            <div className={am('rhs-head')}>
                <div className={am('grow')}>
                    <h3>{title}</h3>
                    <span className={am('where')}>{sub}</span>
                </div>
                <button
                    className={am('icon-btn')}
                    aria-label={formatMessage({id: 'fusion.rhs.close', defaultMessage: 'Close'})}
                    onClick={() => dispatch(closeRightHandSide())}
                >
                    <Icon name='x'/>
                </button>
            </div>
            <div
                ref={bodyRef}
                className={am('rhs-body')}
                onScroll={onScroll}
            >
                {searching && !posts.length && <div className={am('empty')}>{formatMessage({id: 'fusion.results.searching', defaultMessage: 'Searching…'})}</div>}
                {!searching && !posts.length && <div className={am('empty')}>{formatMessage({id: 'fusion.results.none', defaultMessage: 'No messages match. Try another word or a filter like from: or in:.'})}</div>}
                {posts.map((post) => (
                    <Result
                        key={post.id}
                        post={post}
                        terms={rhsState === RHSStates.SEARCH ? terms : ''}
                    />
                ))}
                {paged && gettingMore && <div className={am('empty')}>{formatMessage({id: 'fusion.results.more', defaultMessage: 'Loading more…'})}</div>}
            </div>
        </>
    );
}
