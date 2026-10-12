// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useRef, useState} from 'react';
import {useIntl} from 'react-intl';
import {shallowEqual, useDispatch, useSelector} from 'react-redux';

import type {Channel} from '@mattermost/types/channels';
import type {UserProfile} from '@mattermost/types/users';

import {Client4} from 'mattermost-redux/client';
import {getAllChannelStats, getDirectTeammate, getMyChannelMembership, getMyChannels, isMyChannelAutotranslated} from 'mattermost-redux/selectors/entities/channels';
import {getCurrentTeam} from 'mattermost-redux/selectors/entities/teams';

import {closeRightHandSide, showSearchResults, updateSearchTeam, updateSearchTerms} from 'actions/views/rhs';
import {getSearchBoxHints, getSearchButtons, getSearchPluginSuggestions} from 'selectors/plugins';
import {getIsRhsOpen} from 'selectors/rhs';

import ChannelAttributeLabels from 'components/channel_attributes/channel_attribute_labels';
import {ChannelLabelSurface} from 'components/common/hooks/useChannelLabels';

import {VoiceChatToggle} from 'fusion/calls/voice_chat';
import Avatar from 'fusion/components/avatar';
import Icon from 'fusion/components/icon';
import {Popover} from 'fusion/components/layer';
import {MenuItem} from 'fusion/components/menu';
import {useIsPhone} from 'fusion/hooks/media';
import {useUnreadMentions} from 'fusion/hooks/mentions';
import CollectionsPopover from 'fusion/popovers/collections_popover';
import type {Tab as CollectionsTab} from 'fusion/popovers/collections_popover';
import StatusPopover from 'fusion/popovers/status_popover';
import {addRecentSearch, getRecentSearches} from 'fusion/search/recent_searches';
import {isPhoneLayout, useLayout} from 'fusion/shell/layout_context';
import ChannelIcon from 'fusion/sidebar/channel_icon';
import {useConversationName} from 'fusion/sidebar/group_dm';
import {a11yRegion} from 'fusion/utils/a11y_regions';
import {am} from 'fusion/utils/class_names';
import PluggableErrorBoundary from 'plugins/pluggable/error_boundary';

import type {GlobalState} from 'types/store';
import type {PluginComponent, SearchButtonsComponent, SearchSuggestionsComponent} from 'types/store/plugins';

import CallButton, {OtherCallProviders} from './call_button';
import ChannelNameMenu from './channel_name_menu';
import HeaderTopic from './header_topic';

const NO_CHANNELS: Channel[] = [];
const NO_ICONS: PluginComponent[] = [];
const AGENTS_PLUGIN_IDS = ['mattermost-ai', 'com.antimatterchat.ai'];

const FILTERS: Array<[string, {id: string; defaultMessage: string}]> = [
    ['from:', {id: 'fusion.search.from', defaultMessage: 'a person, e.g. from:@marie'}],
    ['in:', {id: 'fusion.search.in', defaultMessage: 'a channel or conversation'}],
    ['on:', {id: 'fusion.search.on', defaultMessage: 'a day; also before: and after:'}],
    ['ext:', {id: 'fusion.search.ext', defaultMessage: 'files of a type, e.g. ext:pdf'}],
    ['"', {id: 'fusion.search.phrase', defaultMessage: '"an exact phrase"'}],
    ['-', {id: 'fusion.search.exclude', defaultMessage: 'leave a word out, e.g. -draft'}],
    ['*', {id: 'fusion.search.wildcard', defaultMessage: 'word beginnings, e.g. deploy*'}],
];

function scopeOf(channel: Channel, teammateUsername?: string) {
    if (channel.type === 'D' && teammateUsername) {
        return `in:@${teammateUsername}`;
    }
    return `in:${channel.name}`;
}

// The filter being typed at the end of the search, to suggest its values: from: people, in: channels, on:, before:
// and after: a date.
function typedFilter(query: string): {key: 'from' | 'in' | 'date'; value: string} | null {
    const match = (/(?:^|\s)(from|in|on|before|after):(\S*)$/i).exec(query);
    if (!match) {
        return null;
    }
    const key = match[1].toLowerCase();
    return {key: key === 'from' || key === 'in' ? key : 'date', value: match[2].replace(/^[@~]/, '').toLowerCase()};
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
    const [info, setInfo] = useState(false);
    const infoRef = useRef<HTMLButtonElement>(null);
    const [collectionsTab, setCollectionsTab] = useState<CollectionsTab>('mentions');
    const member = useSelector((state: GlobalState) => getMyChannelMembership(state, channel.id));
    const translated = useSelector((state: GlobalState) => isMyChannelAutotranslated(state, channel.id));

    // Plugins' header icons, but Agents': what it asks about the channel is offered in its own panel.
    const headerIcons = useSelector((state: GlobalState) => (state.plugins.components.ChannelHeaderIcon || NO_ICONS).filter((c) => !AGENTS_PLUGIN_IDS.includes(c.pluginId)), shallowEqual);
    const inputRef = useRef<HTMLInputElement>(null);
    const collectionsRef = useRef<HTMLButtonElement>(null);
    const meRef = useRef<HTMLButtonElement>(null);

    // On a phone the header keeps the search and the call; the other actions are in its "More" menu.
    const phone = useIsPhone();
    const moreRef = useRef<HTMLButtonElement>(null);
    const [more, setMore] = useState(false);

    const direct = channel.type === 'D' || channel.type === 'G';

    // As the classic header's "This channel has guests".
    const hasGuests = useSelector((state: GlobalState) => !direct && (getAllChannelStats(state)[channel.id]?.guest_count || 0) > 0);
    const title = useConversationName(channel);
    const team = useSelector(getCurrentTeam);

    // The header's search looks in the whole team (the mockup's server); "in:" narrows it to this conversation.
    const placeholder = formatMessage({id: 'fusion.header.searchTeam', defaultMessage: 'Search {team}'}, {team: team?.display_name || ''});
    const here = scopeOf(channel, teammate?.username);

    // Plugins can add their own search (registerSearchComponents): picked in the suggestions, it runs instead.
    const pluginSearches = useSelector(getSearchButtons);
    const pluginSuggestions = useSelector(getSearchPluginSuggestions);
    const pluginHints = useSelector(getSearchBoxHints);
    const [pluginScope, setPluginScope] = useState('');
    const scopedPlugin = pluginSearches.find((p: SearchButtonsComponent) => p.pluginId === pluginScope);

    const search = (e?: React.FormEvent) => {
        e?.preventDefault();
        const q = query.trim();
        if (!q) {
            return;
        }
        if (scopedPlugin) {
            scopedPlugin.action(q);
            setSearchFocused(false);
            inputRef.current?.blur();
            return;
        }
        addRecentSearch(q);
        dispatch(updateSearchTeam(team?.id || null));
        dispatch(updateSearchTerms(q));
        dispatch(showSearchResults());
        setSearchFocused(false);
        inputRef.current?.blur();
    };

    // Suggestions for the filter being typed: people for from:, channels for in:, a day for the dates.
    const typing = typedFilter(query);
    const [people, setPeople] = useState<UserProfile[]>([]);
    const myChannels = useSelector((state: GlobalState) => (typing?.key === 'in' ? getMyChannels(state) : NO_CHANNELS));
    useEffect(() => {
        if (typing?.key !== 'from') {
            setPeople([]);
            return undefined;
        }
        let stale = false;
        const timer = setTimeout(async () => {
            try {
                const {users} = await Client4.autocompleteUsers(typing.value, team?.id || '', '', {limit: 6});
                if (!stale) {
                    setPeople(users);
                }
            } catch {
                if (!stale) {
                    setPeople([]);
                }
            }
        }, 150);
        return () => {
            stale = true;
            clearTimeout(timer);
        };
    }, [typing?.key, typing?.value, team?.id]);
    const complete = (value: string) => {
        setQuery(query.replace(/(\S*)$/, (token) => token.slice(0, token.indexOf(':') + 1) + value) + ' ');
        inputRef.current?.focus();
    };
    const channelMatches = typing?.key === 'in' ? myChannels.filter((c) => c.delete_at === 0 && c.type !== 'D' && c.type !== 'G' && (c.name.includes(typing.value) || c.display_name.toLowerCase().includes(typing.value))).slice(0, 6) : [];

    return (
        <header {...a11yRegion(am('chat-head'), 'header')}>
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
                <h1>
                    <button
                        ref={infoRef}
                        type='button'
                        className={am('title-btn', {on: info})}
                        title={formatMessage({id: 'fusion.header.info', defaultMessage: 'About this conversation'})}
                        aria-haspopup='menu'
                        aria-expanded={info}
                        onClick={() => setInfo(!info)}
                    >
                        {title}
                        <Icon
                            name='chev'
                            size='xs'
                        />
                    </button>
                </h1>
                {hasGuests && (
                    <span
                        className={am('guest-chip')}
                        title={formatMessage({id: 'fusion.header.guestsTitle', defaultMessage: 'Guests can read and post in this channel'})}
                    >
                        {formatMessage({id: 'fusion.header.guests', defaultMessage: 'Has guests'})}
                    </span>
                )}
                {translated && (
                    <span
                        className={am('guest-chip', 'policy-chip', 'translated-chip')}
                        title={formatMessage({id: 'fusion.header.translatedTitle', defaultMessage: 'Messages here are translated into your language'})}
                    >
                        <Icon
                            name='globe'
                            size='xs'
                        />
                        {formatMessage({id: 'fusion.header.translated', defaultMessage: 'Translated'})}
                    </span>
                )}
                {channel.policy_enforced && (
                    <span
                        className={am('guest-chip', 'policy-chip')}
                        title={formatMessage({id: 'fusion.header.policyTitle', defaultMessage: 'Who can join this channel is set by an access policy'})}
                    >
                        <Icon
                            name='shield'
                            size='xs'
                        />
                        {formatMessage({id: 'fusion.header.policy', defaultMessage: 'Restricted'})}
                    </span>
                )}
                {!direct && (
                    <span className={am('ch-labels')}>
                        <ChannelAttributeLabels
                            channelId={channel.id}
                            surface={[ChannelLabelSurface.INFO, ChannelLabelSurface.HEADER]}
                        />
                    </span>
                )}
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
                    {scopedPlugin && (
                        <button
                            type='button'
                            className={am('search-scope-chip')}
                            title={formatMessage({id: 'fusion.search.leaveScope', defaultMessage: 'Back to searching messages'})}
                            onClick={() => setPluginScope('')}
                        >
                            <PluggableErrorBoundary pluginId={scopedPlugin.pluginId}>
                                <scopedPlugin.component/>
                            </PluggableErrorBoundary>
                            <Icon
                                name='x'
                                size='xs'
                            />
                        </button>
                    )}
                    <input
                        ref={inputRef}
                        value={query}
                        placeholder={placeholder}
                        aria-label={formatMessage({id: 'fusion.header.searchLabel', defaultMessage: 'Search messages'})}
                        autoComplete='off'
                        onChange={(e) => setQuery(e.target.value)}
                        onFocus={() => setSearchFocused(true)}
                        onBlur={(e) => {
                            // Focus moving into the suggestions (the day picker) keeps them open.
                            if (!e.currentTarget.form?.contains(e.relatedTarget as Node)) {
                                setTimeout(() => setSearchFocused(false), 150);
                            }
                        }}
                        onKeyDown={(e) => {
                            if (e.key === 'Escape') {
                                setQuery('');
                                inputRef.current?.blur();
                            }
                        }}
                    />
                </label>
                {searchFocused && scopedPlugin && (
                    <div className={am('search-pop')}>
                        {[...pluginSuggestions, ...(query ? [] : pluginHints)].filter((c: {pluginId: string}) => c.pluginId === pluginScope).map((c) => {
                            const Component = c.component as SearchSuggestionsComponent['component'];
                            return (
                                <PluggableErrorBoundary
                                    key={c.id}
                                    pluginId={c.pluginId}
                                >
                                    <Component
                                        searchTerms={query}
                                        onChangeSearch={(value: string, pretext: string) => setQuery(query.slice(0, query.length - pretext.length) + value)}
                                        onRunSearch={(terms: string) => {
                                            scopedPlugin.action(terms);
                                            setSearchFocused(false);
                                        }}
                                    />
                                </PluggableErrorBoundary>
                            );
                        })}
                    </div>
                )}
                {searchFocused && !query && !scopedPlugin && (
                    <div className={am('search-pop')}>
                        {pluginSearches.length > 0 && (
                            <div className={am('search-scopes')}>
                                {pluginSearches.map((p: SearchButtonsComponent) => {
                                    const Button = p.component;
                                    return (
                                        <button
                                            key={p.id}
                                            type='button'
                                            onMouseDown={(e) => {
                                                e.preventDefault();
                                                setPluginScope(p.pluginId);
                                                inputRef.current?.focus();
                                            }}
                                        >
                                            <PluggableErrorBoundary pluginId={p.pluginId}>
                                                <Button/>
                                            </PluggableErrorBoundary>
                                        </button>
                                    );
                                })}
                            </div>
                        )}
                        <button
                            type='button'
                            onMouseDown={(e) => {
                                e.preventDefault();
                                setQuery(`${here} `);
                                inputRef.current?.focus();
                            }}
                        >
                            <Icon
                                name='search'
                                size='xs'
                            />
                            <span>{direct ? formatMessage({id: 'fusion.search.inThisConversation', defaultMessage: 'Search in this conversation'}) : formatMessage({id: 'fusion.search.inThisChannel', defaultMessage: 'Search in #{name}'}, {name: channel.display_name})}</span>
                        </button>
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
                {searchFocused && typing && (people.length > 0 || channelMatches.length > 0 || typing.key === 'date') && (
                    <div className={am('search-pop')}>
                        {people.map((user) => (
                            <button
                                key={user.id}
                                type='button'
                                onMouseDown={(e) => {
                                    e.preventDefault();
                                    complete(user.username);
                                }}
                            >
                                <Avatar
                                    userId={user.id}
                                    size='xs'
                                />
                                <span>{`@${user.username}`}</span>
                            </button>
                        ))}
                        {channelMatches.map((c) => (
                            <button
                                key={c.id}
                                type='button'
                                onMouseDown={(e) => {
                                    e.preventDefault();
                                    complete(c.name);
                                }}
                            >
                                <ChannelIcon channel={c}/>
                                <span>{c.display_name}</span>
                            </button>
                        ))}
                        {typing.key === 'date' && (
                            <label className={am('search-date')}>
                                <span>{formatMessage({id: 'fusion.search.pickDay', defaultMessage: 'Pick a day'})}</span>
                                <input
                                    type='date'
                                    onMouseDown={(e) => e.stopPropagation()}
                                    onChange={(e) => e.target.value && complete(e.target.value)}
                                />
                            </label>
                        )}
                    </div>
                )}
            </form>
            <div className={am('head-actions')}>
                <CallButton channel={channel}/>
                <OtherCallProviders channel={channel}/>
                {member && headerIcons.length > 0 && !phone && (
                    <span className={am('head-plugins')}>
                        {headerIcons.map((c) => {
                            const Component = c.component as React.ComponentType<{channel: Channel; channelMember: typeof member}>;
                            return (
                                <PluggableErrorBoundary
                                    key={c.id}
                                    pluginId={c.pluginId}
                                >
                                    <Component
                                        channel={channel}
                                        channelMember={member}
                                    />
                                </PluggableErrorBoundary>
                            );
                        })}
                    </span>
                )}
                <button
                    ref={collectionsRef}
                    className={am('icon-btn', 'head-collections', {on: collections})}
                    title={formatMessage({id: 'fusion.header.collections', defaultMessage: 'Mentions, saved, pinned, files and threads here'})}
                    aria-label={formatMessage({id: 'fusion.header.collections', defaultMessage: 'Mentions, saved, pinned, files and threads here'})}
                    aria-haspopup='dialog'
                    onClick={() => {
                        setCollectionsTab('mentions');
                        setCollections(!collections);
                    }}
                >
                    <Icon name='inbox'/>
                    {mentions > 0 && <span className={am('dot')}/>}
                </button>
                <VoiceChatToggle channel={channel}/>
                <button
                    className={am('icon-btn', 'head-members', {on: layout.showMembers && !rhsOpen})}
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
                {phone && (
                    <button
                        ref={moreRef}
                        className={am('icon-btn', 'head-more', {on: more})}
                        title={formatMessage({id: 'fusion.header.more', defaultMessage: 'More'})}
                        aria-label={formatMessage({id: 'fusion.header.more', defaultMessage: 'More'})}
                        aria-haspopup='menu'
                        aria-expanded={more}
                        onClick={() => setMore(!more)}
                    >
                        <Icon name='dots'/>
                        {mentions > 0 && <span className={am('dot')}/>}
                    </button>
                )}
            </div>
            {more && (
                <Popover
                    anchor={moreRef.current}
                    className='plus-pop head-more-menu'
                    role='menu'
                    label={formatMessage({id: 'fusion.header.more', defaultMessage: 'More'})}
                    onClose={() => setMore(false)}
                >
                    {member && headerIcons.length > 0 && (
                        <span className={am('head-plugins')}>
                            {headerIcons.map((c) => {
                                const Component = c.component as React.ComponentType<{channel: Channel; channelMember: typeof member}>;
                                return (
                                    <PluggableErrorBoundary
                                        key={c.id}
                                        pluginId={c.pluginId}
                                    >
                                        <Component
                                            channel={channel}
                                            channelMember={member}
                                        />
                                    </PluggableErrorBoundary>
                                );
                            })}
                        </span>
                    )}
                    <MenuItem
                        icon='users'
                        label={formatMessage({id: 'fusion.header.membersAndApps', defaultMessage: 'Members and apps'})}
                        onClick={() => {
                            setMore(false);
                            layout.openRightDrawer();
                        }}
                    />
                    <MenuItem
                        icon='inbox'
                        label={formatMessage({id: 'fusion.header.collectionsShort', defaultMessage: 'Mentions, saved, pinned and files'})}
                        onClick={() => {
                            setMore(false);
                            setCollectionsTab('mentions');
                            setCollections(true);
                        }}
                    >
                        {mentions > 0 && <span className={am('end', 'count')}>{mentions}</span>}
                    </MenuItem>
                    <MenuItem
                        icon='cog'
                        label={formatMessage({id: 'fusion.header.statusAndSettings', defaultMessage: 'Your status, profile and settings'})}
                        onClick={() => {
                            setMore(false);
                            setStatus(true);
                        }}
                    />
                </Popover>
            )}
            {info && (
                <ChannelNameMenu
                    channel={channel}
                    title={title}
                    anchor={infoRef.current}
                    onClose={() => setInfo(false)}
                />
            )}
            {collections && (
                <CollectionsPopover
                    anchor={collectionsRef.current}
                    channel={channel}
                    initialTab={collectionsTab}
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
