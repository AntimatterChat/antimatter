// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useRef, useState} from 'react';
import {useIntl} from 'react-intl';
import {shallowEqual, useDispatch, useSelector} from 'react-redux';

import type {FileSearchResultItem} from '@mattermost/types/files';
import type {Post} from '@mattermost/types/posts';

import {getMorePostsForSearch} from 'mattermost-redux/actions/search';
import {getChannelTypeInfo} from 'mattermost-redux/selectors/entities/channel_types';
import {getAllChannels} from 'mattermost-redux/selectors/entities/channels';
import {getSearchFilesResults} from 'mattermost-redux/selectors/entities/files';
import {getConfig} from 'mattermost-redux/selectors/entities/general';
import {getPost, isPostFlagged} from 'mattermost-redux/selectors/entities/posts';
import {getCurrentTeam} from 'mattermost-redux/selectors/entities/teams';
import {getFormattedFileSize} from 'mattermost-redux/utils/file_utils';

import {flagPost, pinPost, unflagPost, unpinPost} from 'actions/post_actions';
import {closeRightHandSide, selectPostFromRightHandSideSearch, toggleRhsExpanded} from 'actions/views/rhs';
import {getIsRhsExpanded, getSearchTeam, getSearchTerms, getSearchType} from 'selectors/rhs';

import useSearchResultsActions from 'components/common/hooks/use_search_results_actions';
import FilePreviewModal from 'components/file_preview_modal';
import {getSearchPopoutTitle} from 'components/rhs_search_popout/title';
import type {SearchFilterType} from 'components/search/types';

import Icon from 'fusion/components/icon';
import {useDisplayName, useUser} from 'fusion/hooks/users';
import FileMenu from 'fusion/messages/content/file_menu';
import {getThreadTitle} from 'fusion/messages/thread_title';
import {useWhen} from 'fusion/messages/time';
import {scopeOfChannel} from 'fusion/profiles/community_profiles';
import {useToast} from 'fusion/shell/toast_context';
import {am} from 'fusion/utils/class_names';
import {openDialog} from 'fusion/utils/modals';
import {permalinkPath, topicPath} from 'fusion/utils/paths';
import {plainText} from 'fusion/utils/plain_text';
import {getHistory} from 'utils/browser_history';
import {ModalIdentifiers, RHSStates} from 'utils/constants';
import {canSearchFiles} from 'utils/file_utils';
import {canPopout, popoutRhsSearch} from 'utils/popouts/popout_windows';
import {getSiteURL} from 'utils/url';
import {copyToClipboard} from 'utils/utils';

import type {GlobalState} from 'types/store';
import type {RhsState, SearchType} from 'types/store/rhs';

function highlight(text: string, terms: string) {
    const words = terms.split(/\s+/).filter((w) => w && !w.includes(':') && w.length > 1).map((w) => w.replace(/^["@#]|"$/g, ''));
    if (!words.length) {
        return text;
    }
    const re = new RegExp('(' + words.map((w) => w.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')).join('|') + ')', 'ig');
    return text.split(re).map((part, i) => (i % 2 ? <mark key={i}>{part}</mark> : part));
}

function Result({post, terms}: {post: Post; terms: string}) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const toast = useToast();
    const when = useWhen();
    const user = useUser(post.user_id);
    const channel = useSelector((state: GlobalState) => getAllChannels(state)[post.channel_id]);

    // As the message's community shows its author.
    const name = useDisplayName(user, scopeOfChannel(channel));
    const team = useSelector(getCurrentTeam);
    const saved = useSelector((state: GlobalState) => isPostFlagged(state, post.id));
    const canPin = Boolean(channel) && channel.delete_at === 0;
    const direct = channel && (channel.type === 'D' || channel.type === 'G');
    let icon: 'chat' | 'lock' | 'hash' = 'hash';
    if (direct) {
        icon = 'chat';
    } else if (channel?.type === 'P') {
        icon = 'lock';
    }

    // Antimatter forums (design §4.4): a hit in a forum shows its topic's title above it, and opens the topic
    // page rather than a permalink. Indexing titles in global search comes later; this reads whichever root the
    // hit's own thread already has in the store.
    const forum = useSelector((state: GlobalState) => getChannelTypeInfo(state, post.channel_id).type === 'forum');
    const topicTitle = useSelector((state: GlobalState) => (forum ? getThreadTitle(getPost(state, post.root_id || post.id)) : ''));
    const jump = () => {
        if (!team) {
            return;
        }
        if (forum && channel) {
            getHistory().push(topicPath(team.name, channel, post.root_id || post.id));
        } else {
            getHistory().push(permalinkPath(team.name, post.id));
        }
    };
    const action = (label: string, iconName: 'thread' | 'bookmark' | 'pin' | 'link', onClick: () => void, on = false) => (
        <button
            type='button'
            className={am({on})}
            title={label}
            aria-label={label}
            onClick={(e) => {
                e.stopPropagation();
                onClick();
            }}
        >
            <Icon
                name={iconName}
                size='sm'
            />
        </button>
    );
    return (
        <div className={am('result-wrap')}>
            <button
                className={am('result')}
                onClick={jump}
            >
                {forum && topicTitle && <div className={am('result-topic')}>{topicTitle}</div>}
                <span className={am('where')}>
                    <Icon
                        name={icon}
                        size='xs'
                    />
                    {[channel?.display_name, post.root_id && formatMessage({id: 'fusion.results.inThread', defaultMessage: 'thread'}), when(post.create_at)].filter(Boolean).join(' · ')}
                </span>
                <b>{name}</b>
                <div>{highlight(plainText(post.message), terms)}</div>
            </button>
            <div className={am('result-tools')}>
                {action(formatMessage({id: 'fusion.results.openThread', defaultMessage: 'Open the thread'}), 'thread', () => dispatch(selectPostFromRightHandSideSearch(post)))}
                {action(saved ? formatMessage({id: 'fusion.results.unsave', defaultMessage: 'Remove from saved'}) : formatMessage({id: 'fusion.results.save', defaultMessage: 'Save'}), 'bookmark', () => dispatch(saved ? unflagPost(post.id) : flagPost(post.id)), saved)}
                {canPin && action(post.is_pinned ? formatMessage({id: 'fusion.results.unpin', defaultMessage: 'Unpin from channel'}) : formatMessage({id: 'fusion.results.pin', defaultMessage: 'Pin to channel'}), 'pin', () => dispatch(post.is_pinned ? unpinPost(post.id) : pinPost(post.id)), post.is_pinned)}
                {action(formatMessage({id: 'fusion.results.copyLink', defaultMessage: 'Copy link'}), 'link', () => {
                    if (team) {
                        copyToClipboard(getSiteURL() + permalinkPath(team.name, post.id));
                        toast(formatMessage({id: 'fusion.toast.linkCopied', defaultMessage: 'Link copied'}));
                    }
                })}
            </div>
        </div>
    );
}

// The file types to filter file results by, as the classic search's.
const FILE_FILTERS: Array<[SearchFilterType, {id: string; defaultMessage: string}]> = [
    ['all', {id: 'fusion.results.fAll', defaultMessage: 'All'}],
    ['documents', {id: 'fusion.results.fDocuments', defaultMessage: 'Documents'}],
    ['spreadsheets', {id: 'fusion.results.fSpreadsheets', defaultMessage: 'Spreadsheets'}],
    ['presentations', {id: 'fusion.results.fPresentations', defaultMessage: 'Presentations'}],
    ['code', {id: 'fusion.results.fCode', defaultMessage: 'Code'}],
    ['images', {id: 'fusion.results.fImages', defaultMessage: 'Images'}],
    ['audio', {id: 'fusion.results.fAudio', defaultMessage: 'Audio'}],
    ['video', {id: 'fusion.results.fVideo', defaultMessage: 'Video'}],
];

function FileResult({file}: {file: FileSearchResultItem}) {
    const dispatch = useDispatch();
    const when = useWhen();
    const channel = useSelector((state: GlobalState) => getAllChannels(state)[file.channel_id]);
    const post = useSelector((state: GlobalState) => (file.post_id ? state.entities.posts.posts[file.post_id] : undefined));
    const [menu, setMenu] = useState<HTMLElement | null>(null);
    const ext = (file.extension || '').toUpperCase().slice(0, 4) || 'FILE';
    return (
        <div className={am('result-wrap')}>
            <button
                type='button'
                className={am('result', 'file-result')}
                onClick={() => dispatch(openDialog(ModalIdentifiers.FILE_PREVIEW_MODAL, FilePreviewModal, {postId: file.post_id, fileInfos: [file], startIndex: 0}))}
            >
                <span className={am('file')}>{ext}</span>
                <span className={am('attach-meta')}>
                    <b>{file.name}</b>
                    <span>{[getFormattedFileSize(file.size), channel?.display_name, when(file.create_at)].filter(Boolean).join(' · ')}</span>
                </span>
            </button>
            {post && (
                <div className={am('result-tools')}>
                    <button
                        type='button'
                        aria-label={file.name}
                        onClick={(e) => setMenu(e.currentTarget)}
                    >
                        <Icon
                            name='dots'
                            size='sm'
                        />
                    </button>
                </div>
            )}
            {menu && post && (
                <FileMenu
                    file={file}
                    post={post}
                    anchor={menu}
                    onClose={() => setMenu(null)}
                />
            )}
        </div>
    );
}

// ResultsPanel lists search results, mentions, and saved or pinned messages in the right-hand panel.
export default function ResultsPanel({rhsState}: {rhsState: string}) {
    const intl = useIntl();
    const {formatMessage} = intl;
    const dispatch = useDispatch();
    const terms = useSelector(getSearchTerms);
    const channelId = useSelector((state: GlobalState) => state.views.rhs.selectedChannelId || state.entities.channels.currentChannelId);
    const searchActions = useSearchResultsActions();
    const searchType = useSelector(getSearchType);
    const filesSearchAvailable = useSelector((state: GlobalState) => canSearchFiles(getConfig(state)));
    const showFiles = rhsState === RHSStates.SEARCH && searchType === 'files' && filesSearchAvailable;
    const files = useSelector(getSearchFilesResults);
    const filesAtEnd = useSelector((state: GlobalState) => state.entities.search.current[getSearchTeam(state) || 'ALL_TEAMS']?.isFilesEnd ?? true);
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
        if (!paged || (atEnd && !showFiles) || gettingMore || searching || !body) {
            return;
        }
        if (body.scrollHeight - body.scrollTop - body.clientHeight < 300) {
            if (showFiles) {
                if (!filesAtEnd) {
                    searchActions.getMoreFilesForSearch();
                }
            } else {
                dispatch(getMorePostsForSearch(pagedTeam));
            }
        }
    };

    // The results can open in a window of their own, as the classic search results. There's no expand button: a
    // panel left expanded (by an older version) goes back to its width, which the person can still drag.
    const expanded = useSelector(getIsRhsExpanded);
    useEffect(() => {
        if (expanded) {
            dispatch(toggleRhsExpanded());
        }
    }, [expanded, dispatch]);
    const team = useSelector(getCurrentTeam);
    const channelName = useSelector((state: GlobalState) => state.entities.channels.channels[channelId]?.name);
    const searchTeamId = useSelector(getSearchTeam);
    const popout = () => {
        if (!team) {
            return;
        }
        const mode = rhsState as NonNullable<RhsState>;
        popoutRhsSearch(
            intl.formatMessage(getSearchPopoutTitle(mode), {serverName: '{serverName}', channelName: '{channelName}', searchTerms: terms}),
            team.name,
            terms,
            mode,
            searchType as SearchType,
            rhsState === RHSStates.PIN ? channelName : undefined,
            searchTeamId,
        );
        dispatch(closeRightHandSide());
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
                {canPopout() && team && (
                    <button
                        className={am('icon-btn')}
                        title={formatMessage({id: 'fusion.results.popout', defaultMessage: 'Open in a new window'})}
                        aria-label={formatMessage({id: 'fusion.results.popout', defaultMessage: 'Open in a new window'})}
                        onClick={popout}
                    >
                        <Icon
                            name='popout'
                            size='sm'
                        />
                    </button>
                )}
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
                {rhsState === RHSStates.SEARCH && (
                    <div
                        className={am('results-tabs')}
                        role='tablist'
                    >
                        <button
                            type='button'
                            role='tab'
                            aria-selected={!showFiles}
                            className={am({on: !showFiles})}
                            onClick={() => searchActions.setSearchType('messages')}
                        >
                            {formatMessage({id: 'fusion.results.messages', defaultMessage: 'Messages'})}
                            <span>{posts.length}</span>
                        </button>
                        {filesSearchAvailable && (
                            <button
                                type='button'
                                role='tab'
                                aria-selected={showFiles}
                                className={am({on: showFiles})}
                                onClick={() => searchActions.setSearchType('files')}
                            >
                                {formatMessage({id: 'fusion.results.files', defaultMessage: 'Files'})}
                                <span>{files.length}</span>
                            </button>
                        )}
                    </div>
                )}
                {showFiles && (
                    <div className={am('file-filters')}>
                        {FILE_FILTERS.map(([type, label]) => (
                            <button
                                key={type}
                                type='button'
                                className={am({on: searchActions.searchFilterType === type})}
                                onClick={() => searchActions.setSearchFilterType(type)}
                            >
                                {formatMessage(label)}
                            </button>
                        ))}
                    </div>
                )}
                {showFiles && files.map((file) => (
                    <FileResult
                        key={file.id}
                        file={file}
                    />
                ))}
                {showFiles && !searching && !files.length && <div className={am('empty')}>{formatMessage({id: 'fusion.results.noFiles', defaultMessage: 'No files match.'})}</div>}
                {!showFiles && searching && !posts.length && <div className={am('empty')}>{formatMessage({id: 'fusion.results.searching', defaultMessage: 'Searching…'})}</div>}
                {!showFiles && !searching && !posts.length && <div className={am('empty')}>{formatMessage({id: 'fusion.results.none', defaultMessage: 'No messages match. Try another word or a filter like from:, in: or has:link.'})}</div>}
                {!showFiles && posts.map((post) => (
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
