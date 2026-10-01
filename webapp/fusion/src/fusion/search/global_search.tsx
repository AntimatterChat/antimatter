// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useMemo, useRef, useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import type {Channel} from '@mattermost/types/channels';
import type {Post} from '@mattermost/types/posts';
import type {UserProfile} from '@mattermost/types/users';

import {Client4} from 'mattermost-redux/client';
import {getAllChannels, getMyChannelMemberships} from 'mattermost-redux/selectors/entities/channels';
import {getCurrentTeam, getTeam} from 'mattermost-redux/selectors/entities/teams';
import {getCurrentUserId, getUsers} from 'mattermost-redux/selectors/entities/users';
import {getUserIdFromChannelName} from 'mattermost-redux/utils/channel_utils';

import {openDirectChannelToUserId} from 'actions/channel_actions';
import {showSearchResults, updateSearchTerms} from 'actions/views/rhs';

import Avatar from 'fusion/components/avatar';
import Icon from 'fusion/components/icon';
import {Dialog} from 'fusion/components/layer';
import {useWhen} from 'fusion/messages/time';
import {useGlobalSearch} from 'fusion/shell/global_search_context';
import ChannelIcon from 'fusion/sidebar/channel_icon';
import {am} from 'fusion/utils/class_names';
import {channelPath, permalinkPath} from 'fusion/utils/paths';
import {plainText} from 'fusion/utils/plain_text';
import {getHistory} from 'utils/browser_history';

import type {GlobalState} from 'types/store';

import {addRecentSearch, getRecentSearches} from './recent_searches';

type Filter = 'all' | 'messages' | 'channels' | 'people';
type Result =
    | {kind: 'channel'; channel: Channel} |
    {kind: 'user'; user: UserProfile} |
    {kind: 'message'; post: Post} |
    {kind: 'recent'; q: string} |
    {kind: 'all'};

function Mark({text, q}: {text: string; q: string}) {
    if (!q) {
        return <>{text}</>;
    }
    const i = text.toLowerCase().indexOf(q.toLowerCase());
    if (i < 0) {
        return <>{text}</>;
    }
    return <>{text.slice(0, i)}<mark>{text.slice(i, i + q.length)}</mark>{text.slice(i + q.length)}</>;
}

// GlobalSearch is the Ctrl/⌘+K overlay: jump to a channel, a conversation or a person, or search every message.
export default function GlobalSearch() {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const search = useGlobalSearch();
    const when = useWhen();
    const team = useSelector(getCurrentTeam);
    const me = useSelector(getCurrentUserId);
    const channels = useSelector(getAllChannels);
    const memberships = useSelector(getMyChannelMemberships);
    const users = useSelector(getUsers);
    const teamsById = useSelector((state: GlobalState) => (id: string) => getTeam(state, id));
    const [query, setQuery] = useState('');
    const [filter, setFilter] = useState<Filter>('all');
    const [selected, setSelected] = useState(0);
    const [people, setPeople] = useState<UserProfile[]>([]);
    const [messages, setMessages] = useState<Post[]>([]);
    const inputRef = useRef<HTMLInputElement>(null);

    useEffect(() => inputRef.current?.focus(), []);

    const q = query.trim();
    const myChannels = useMemo(() => Object.values(channels).filter((c) => memberships[c.id] && c.delete_at === 0), [channels, memberships]);

    // People and messages come from the server, a moment after typing stops.
    useEffect(() => {
        if (!q || !team) {
            setPeople([]);
            setMessages([]);
            return undefined;
        }
        const timer = setTimeout(async () => {
            try {
                const [found, posts] = await Promise.all([
                    filter === 'all' || filter === 'people' ? Client4.autocompleteUsers(q, '', '', {limit: 6}) : Promise.resolve({users: []}),
                    filter === 'all' || filter === 'messages' ? Client4.searchPostsWithParams(team.id, {terms: q, is_or_search: false, include_deleted_channels: false, page: 0, per_page: filter === 'messages' ? 20 : 5}) : Promise.resolve({order: [], posts: {}}),
                ]);
                setPeople(found.users.filter((u: UserProfile) => u.id !== me && !u.delete_at));
                setMessages(posts.order.map((id: string) => (posts.posts as Record<string, Post>)[id]).filter(Boolean));
            } catch {
                setPeople([]);
                setMessages([]);
            }
        }, 250);
        return () => clearTimeout(timer);
    }, [q, filter, team, me]);

    const groups: Array<[string, Result[]]> = [];
    if (q) {
        if (filter === 'all' || filter === 'channels') {
            const matched = myChannels.filter((c) => c.display_name.toLowerCase().includes(q.toLowerCase()) || c.name.includes(q.toLowerCase())).slice(0, 8);
            groups.push([formatMessage({id: 'fusion.gs.channels', defaultMessage: 'Channels & conversations'}), matched.map((channel) => ({kind: 'channel', channel}))]);
        }
        if (filter === 'all' || filter === 'people') {
            groups.push([formatMessage({id: 'fusion.gs.people', defaultMessage: 'People'}), people.map((user) => ({kind: 'user', user}))]);
        }
        if (filter === 'all' || filter === 'messages') {
            groups.push([formatMessage({id: 'fusion.gs.messages', defaultMessage: 'Messages'}), messages.map((post) => ({kind: 'message', post}))]);
        }
        groups.push(['', [{kind: 'all'}]]);
    } else {
        groups.push([formatMessage({id: 'fusion.gs.recentSearches', defaultMessage: 'Recent searches'}), getRecentSearches().map((recentQ) => ({kind: 'recent', q: recentQ}))]);
        const recent = [...myChannels].sort((a, b) => (memberships[b.id]?.last_viewed_at || 0) - (memberships[a.id]?.last_viewed_at || 0)).slice(0, 8);
        groups.push([formatMessage({id: 'fusion.gs.recent', defaultMessage: 'Recent'}), recent.map((channel) => ({kind: 'channel', channel}))]);
    }
    const visible = groups.filter(([, rows]) => rows.length);
    const flat = visible.flatMap(([, rows]) => rows);
    const sel = Math.min(selected, Math.max(0, flat.length - 1));

    const activate = async (r: Result) => {
        search.close();
        if (r.kind === 'channel') {
            const t = r.channel.team_id ? teamsById(r.channel.team_id) : team;
            if (t || team) {
                getHistory().push(channelPath((t || team)!.name, r.channel));
            }
        } else if (r.kind === 'user') {
            const result = await dispatch(openDirectChannelToUserId(r.user.id));
            if ('data' in result && result.data && team) {
                getHistory().push(channelPath(team.name, result.data));
            }
        } else if (r.kind === 'message') {
            if (team) {
                getHistory().push(permalinkPath(team.name, r.post.id));
            }
        } else {
            const terms = r.kind === 'recent' ? r.q : q;
            addRecentSearch(terms);
            dispatch(updateSearchTerms(terms));
            dispatch(showSearchResults());
        }
    };

    const row = (r: Result) => {
        if (r.kind === 'channel') {
            const c = r.channel;
            const direct = c.type === 'D' || c.type === 'G';
            const mate = c.type === 'D' ? (c.teammate_id || getUserIdFromChannelName(me, c.name)) : undefined;
            return (
                <>
                    {mate ? (
                        <Avatar
                            userId={mate}
                            status={true}
                        />
                    ) : (
                        <span className={am('gi')}>{direct ? <Icon name='users'/> : <ChannelIcon channel={c}/>}</span>
                    )}
                    <span style={{minWidth: 0}}>
                        <b>
                            <Mark
                                text={c.display_name}
                                q={q}
                            />
                        </b>
                        <span className={am('sub')}>{direct ? formatMessage({id: 'fusion.gs.direct', defaultMessage: 'Direct message'}) : (c.purpose || c.header || '')}</span>
                    </span>
                    <span className={am('kind')}>{direct ? formatMessage({id: 'fusion.gs.kindDM', defaultMessage: 'DM'}) : formatMessage({id: 'fusion.gs.kindChannel', defaultMessage: 'Channel'})}</span>
                </>
            );
        }
        if (r.kind === 'user') {
            const u = users[r.user.id] || r.user;
            const name = [u.first_name, u.last_name].filter(Boolean).join(' ') || u.username;
            return (
                <>
                    <Avatar
                        userId={u.id}
                        status={true}
                    />
                    <span style={{minWidth: 0}}>
                        <b>
                            <Mark
                                text={name}
                                q={q}
                            />
                        </b>
                        <span className={am('sub')}>
                            {'@'}
                            <Mark
                                text={u.username}
                                q={q}
                            />
                            {u.position ? ` · ${u.position}` : ''}
                        </span>
                    </span>
                    <span className={am('kind')}>{formatMessage({id: 'fusion.gs.kindPerson', defaultMessage: 'Person'})}</span>
                </>
            );
        }
        if (r.kind === 'message') {
            const author = users[r.post.user_id];
            const channel = channels[r.post.channel_id];
            const direct = channel && (channel.type === 'D' || channel.type === 'G');
            const where = channel ? (direct ? '' : '#') + channel.display_name : '';

            // The snippet starts near the match, as the mockup's.
            const text = plainText(r.post.message, 2000);
            const at = text.toLowerCase().indexOf(q.toLowerCase());
            const snippet = at > 40 ? '…' + text.slice(at - 30, at + 130) : text.slice(0, 160);
            return (
                <>
                    <Avatar userId={r.post.user_id}/>
                    <span style={{minWidth: 0}}>
                        <b>
                            {author ? ([author.first_name, author.last_name].filter(Boolean).join(' ') || author.username) : ''}
                            <span
                                className={am('sub')}
                                style={{display: 'inline'}}
                            >
                                {' ' + formatMessage({id: 'fusion.gs.messageWhere', defaultMessage: 'in {where}'}, {where}) + (r.post.root_id ? ' · ' + formatMessage({id: 'fusion.gs.thread', defaultMessage: 'thread'}) : '') + ` · ${when(r.post.create_at)}`}
                            </span>
                        </b>
                        <span className={am('sub')}>
                            <Mark
                                text={snippet}
                                q={q}
                            />
                        </span>
                    </span>
                    <span className={am('kind')}>{formatMessage({id: 'fusion.gs.kindMessage', defaultMessage: 'Message'})}</span>
                </>
            );
        }
        if (r.kind === 'recent') {
            return (
                <>
                    <span className={am('gi')}><Icon name='clock'/></span>
                    <span style={{minWidth: 0}}>
                        <b>{r.q}</b>
                        <span className={am('sub')}>{formatMessage({id: 'fusion.gs.recentSub', defaultMessage: 'Search again in the side panel'})}</span>
                    </span>
                    <span className={am('kind')}>{formatMessage({id: 'fusion.gs.kindSearch', defaultMessage: 'Search'})}</span>
                </>
            );
        }
        return (
            <>
                <span className={am('gi')}><Icon name='search'/></span>
                <span>
                    <b>{formatMessage({id: 'fusion.gs.all', defaultMessage: 'Search all messages for “{q}”'}, {q})}</b>
                    <span className={am('sub')}>{formatMessage({id: 'fusion.gs.allSub', defaultMessage: 'See every match in the side panel'})}</span>
                </span>
                <span className={am('kind')}>{'Enter'}</span>
            </>
        );
    };

    let index = 0;
    return (
        <Dialog
            label={formatMessage({id: 'fusion.gs.label', defaultMessage: 'Search everything'})}
            top={true}
            onClose={search.close}
            className={am('gsearch')}
        >
            <label className={am('gs-input')}>
                <Icon name='search'/>
                <input
                    ref={inputRef}
                    value={query}
                    placeholder={formatMessage({id: 'fusion.gs.placeholder', defaultMessage: 'Search messages, channels, people and more'})}
                    aria-label={formatMessage({id: 'fusion.gs.label', defaultMessage: 'Search everything'})}
                    autoComplete='off'
                    role='combobox'
                    aria-expanded='true'
                    onChange={(e) => {
                        setQuery(e.target.value);
                        setSelected(0);
                    }}
                    onKeyDown={(e) => {
                        if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
                            e.preventDefault();
                            const n = Math.max(1, flat.length);
                            setSelected((sel + (e.key === 'ArrowDown' ? 1 : n - 1)) % n);
                        } else if (e.key === 'Enter' && flat[sel]) {
                            e.preventDefault();
                            activate(flat[sel]);
                        }
                    }}
                />
                <kbd>{'Esc'}</kbd>
            </label>
            <div
                className={am('gs-filters')}
                role='group'
            >
                {(['all', 'messages', 'channels', 'people'] as Filter[]).map((f) => (
                    <button
                        key={f}
                        type='button'
                        className={am({on: filter === f})}
                        onClick={() => setFilter(f)}
                    >
                        {{
                            all: formatMessage({id: 'fusion.gs.fAll', defaultMessage: 'All'}),
                            messages: formatMessage({id: 'fusion.gs.fMessages', defaultMessage: 'Messages'}),
                            channels: formatMessage({id: 'fusion.gs.fChannels', defaultMessage: 'Channels'}),
                            people: formatMessage({id: 'fusion.gs.fPeople', defaultMessage: 'People'}),
                        }[f]}
                    </button>
                ))}
            </div>
            <div
                className={am('gs-results')}
                role='listbox'
            >
                {visible.map(([title, rows]) => (
                    <React.Fragment key={title || 'all'}>
                        {title && <h5>{title}</h5>}
                        {rows.map((r) => {
                            const i = index++;
                            return (
                                <button
                                    key={i}
                                    type='button'
                                    className={am('gs-row', {sel: i === sel, all: r.kind === 'all'})}
                                    role='option'
                                    aria-selected={i === sel}
                                    onMouseEnter={() => setSelected(i)}
                                    onClick={() => activate(r)}
                                >
                                    {row(r)}
                                </button>
                            );
                        })}
                    </React.Fragment>
                ))}
                {!visible.length && <div className={am('empty')}>{formatMessage({id: 'fusion.gs.start', defaultMessage: 'Start typing to search.'})}</div>}
            </div>
            <div className={am('gs-foot')}>
                <span><kbd>{'↑'}</kbd><kbd>{'↓'}</kbd>{formatMessage({id: 'fusion.gs.navigate', defaultMessage: 'navigate'})}</span>
                <span><kbd>{'Enter'}</kbd>{formatMessage({id: 'fusion.gs.open', defaultMessage: 'open'})}</span>
                <span><kbd>{'Esc'}</kbd>{formatMessage({id: 'fusion.gs.close', defaultMessage: 'close'})}</span>
            </div>
        </Dialog>
    );
}
