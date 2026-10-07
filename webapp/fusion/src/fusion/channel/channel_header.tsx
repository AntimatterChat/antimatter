// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useRef, useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import type {Channel} from '@mattermost/types/channels';

import {getDirectTeammate} from 'mattermost-redux/selectors/entities/channels';

import {closeRightHandSide, showSearchResults, updateSearchTerms} from 'actions/views/rhs';
import {getIsRhsOpen} from 'selectors/rhs';

import {VoiceChatToggle} from 'fusion/calls/voice_chat';
import Avatar from 'fusion/components/avatar';
import Icon from 'fusion/components/icon';
import {useUnreadMentions} from 'fusion/hooks/mentions';
import CollectionsPopover from 'fusion/popovers/collections_popover';
import StatusPopover from 'fusion/popovers/status_popover';
import {addRecentSearch, getRecentSearches} from 'fusion/search/recent_searches';
import {isPhoneLayout, useLayout} from 'fusion/shell/layout_context';
import ChannelIcon from 'fusion/sidebar/channel_icon';
import {useConversationName} from 'fusion/sidebar/group_dm';
import {am} from 'fusion/utils/class_names';

import type {GlobalState} from 'types/store';

import CallButton from './call_button';
import HeaderTopic from './header_topic';

const FILTERS: Array<[string, {id: string; defaultMessage: string}]> = [
    ['from:', {id: 'fusion.search.from', defaultMessage: 'a person, e.g. from:@marie'}],
    ['in:', {id: 'fusion.search.in', defaultMessage: 'a channel or conversation'}],
    ['before:', {id: 'fusion.search.dates', defaultMessage: 'a date: before: after: on:'}],
    ['ext:', {id: 'fusion.search.ext', defaultMessage: 'files of a type, e.g. ext:pdf'}],
];

function scopeOf(channel: Channel, teammateUsername?: string) {
    if (channel.type === 'D' && teammateUsername) {
        return `in:@${teammateUsername}`;
    }
    return `in:${channel.name}`;
}

// ChannelHeader is the bar above a conversation: its name and topic, a search scoped to it, and its actions.
export default function ChannelHeader({channel}: {channel: Channel}) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const layout = useLayout();
    const teammate = useSelector((state: GlobalState) => (channel.type === 'D' ? getDirectTeammate(state, channel.id) : undefined));
    const rhsOpen = useSelector(getIsRhsOpen);
    const mentions = useUnreadMentions();
    const [query, setQuery] = useState('');
    const [searchFocused, setSearchFocused] = useState(false);
    const [collections, setCollections] = useState(false);
    const [status, setStatus] = useState(false);
    const inputRef = useRef<HTMLInputElement>(null);
    const collectionsRef = useRef<HTMLButtonElement>(null);
    const meRef = useRef<HTMLButtonElement>(null);

    const direct = channel.type === 'D' || channel.type === 'G';
    const title = useConversationName(channel);
    const placeholder = direct ? formatMessage({id: 'fusion.header.searchConversation', defaultMessage: 'Search this conversation'}) : formatMessage({id: 'fusion.header.searchChannel', defaultMessage: 'Search #{name}'}, {name: channel.display_name});

    const search = (e: React.FormEvent) => {
        e.preventDefault();
        const q = query.trim();
        if (!q) {
            return;
        }
        const scoped = (/(^|\s)(in|from):/).test(q) ? q : `${scopeOf(channel, teammate?.username)} ${q}`;
        addRecentSearch(q);
        dispatch(updateSearchTerms(scoped));
        dispatch(showSearchResults());
        setSearchFocused(false);
        inputRef.current?.blur();
    };

    return (
        <header className={am('chat-head')}>
            <div className={am('ch-title')}>
                <button
                    className={am('icon-btn', 'menu-btn')}
                    aria-label={formatMessage({id: 'fusion.header.nav', defaultMessage: 'Open navigation'})}
                    onClick={() => layout.setNavOpen(true)}
                >
                    <Icon name='menu'/>
                </button>
                {channel.type === 'D' && teammate ? (
                    <Avatar
                        userId={teammate.id}
                        size='sm'
                        status={true}
                    />
                ) : null}
                {channel.type === 'G' && <span className={am('av', 'group', 'sm')}>{channel.display_name.split(',').length}</span>}
                {!direct && <ChannelIcon channel={channel}/>}
                <h1>{title}</h1>
                <HeaderTopic channel={channel}/>
            </div>
            <form
                className={am('search')}
                role='search'
                onSubmit={search}
            >
                <label>
                    <Icon
                        name='search'
                        size='sm'
                    />
                    <input
                        ref={inputRef}
                        value={query}
                        placeholder={placeholder}
                        aria-label={formatMessage({id: 'fusion.header.searchLabel', defaultMessage: 'Search messages'})}
                        autoComplete='off'
                        onChange={(e) => setQuery(e.target.value)}
                        onFocus={() => setSearchFocused(true)}
                        onBlur={() => setTimeout(() => setSearchFocused(false), 150)}
                        onKeyDown={(e) => {
                            if (e.key === 'Escape') {
                                setQuery('');
                                inputRef.current?.blur();
                            }
                        }}
                    />
                </label>
                {searchFocused && !query && (
                    <div className={am('search-pop')}>
                        <h4>{formatMessage({id: 'fusion.search.filters', defaultMessage: 'Filters'})}</h4>
                        {FILTERS.map(([filter, hint]) => (
                            <button
                                key={filter}
                                type='button'
                                onMouseDown={(e) => {
                                    e.preventDefault();
                                    setQuery(filter);
                                    inputRef.current?.focus();
                                }}
                            >
                                <code>{filter}</code>
                                <span>{formatMessage(hint)}</span>
                            </button>
                        ))}
                        {getRecentSearches().length > 0 && <h4>{formatMessage({id: 'fusion.search.recent', defaultMessage: 'Recent'})}</h4>}
                        {getRecentSearches().map((recent) => (
                            <button
                                key={recent}
                                type='button'
                                onMouseDown={(e) => {
                                    e.preventDefault();
                                    setQuery(recent);
                                    inputRef.current?.focus();
                                }}
                            >
                                <Icon
                                    name='clock'
                                    size='xs'
                                />
                                <span>{recent}</span>
                            </button>
                        ))}
                    </div>
                )}
            </form>
            <div className={am('head-actions')}>
                <CallButton channel={channel}/>
                <button
                    ref={collectionsRef}
                    className={am('icon-btn', {on: collections})}
                    title={formatMessage({id: 'fusion.header.collections', defaultMessage: 'Mentions, saved, pinned and threads'})}
                    aria-label={formatMessage({id: 'fusion.header.collections', defaultMessage: 'Mentions, saved, pinned and threads'})}
                    aria-haspopup='dialog'
                    onClick={() => setCollections(!collections)}
                >
                    <Icon name='inbox'/>
                    {mentions > 0 && <span className={am('dot')}/>}
                </button>
                <VoiceChatToggle channel={channel}/>
                <button
                    className={am('icon-btn', {on: layout.showMembers && !rhsOpen})}
                    title={formatMessage({id: 'fusion.header.members', defaultMessage: 'Member list'})}
                    aria-label={formatMessage({id: 'fusion.header.membersToggle', defaultMessage: 'Toggle member list'})}
                    onClick={() => {
                        // On a phone the member list lives in the right-hand drawer, with the app rail.
                        if (isPhoneLayout()) {
                            if (layout.rightOpen) {
                                layout.setRightOpen(false);
                            } else {
                                layout.openRightDrawer();
                            }
                        } else if (rhsOpen) {
                            dispatch(closeRightHandSide());
                            if (!layout.showMembers) {
                                layout.toggleMembers();
                            }
                        } else {
                            layout.toggleMembers();
                        }
                    }}
                >
                    <Icon name='users'/>
                </button>
                <MeButton
                    buttonRef={meRef}
                    onClick={() => setStatus(!status)}
                />
            </div>
            {collections && (
                <CollectionsPopover
                    anchor={collectionsRef.current}
                    channel={channel}
                    onClose={() => setCollections(false)}
                />
            )}
            {status && (
                <StatusPopover
                    anchor={meRef.current}
                    onClose={() => setStatus(false)}
                />
            )}
        </header>
    );
}

// On narrow screens, where the app rail is hidden, your avatar sits at the end of the header.
function MeButton({buttonRef, onClick}: {buttonRef: React.RefObject<HTMLButtonElement | null>; onClick: () => void}) {
    const {formatMessage} = useIntl();
    const me = useSelector((state: GlobalState) => state.entities.users.currentUserId);
    return (
        <button
            ref={buttonRef}
            className={am('icon-btn', 'me-btn')}
            aria-label={formatMessage({id: 'fusion.header.me', defaultMessage: 'Your status, profile and settings'})}
            onClick={onClick}
        >
            <Avatar
                userId={me}
                size='sm'
                status={true}
            />
        </button>
    );
}
