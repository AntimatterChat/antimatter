// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useMemo, useRef, useState} from 'react';
import {defineMessage, useIntl} from 'react-intl';
import type {MessageDescriptor} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import type {Channel} from '@mattermost/types/channels';

import {getArchivedChannels, getChannels, getChannelsMemberCount, getMyChannelJoinRequests, joinChannel, requestJoinChannel, searchAllChannels, withdrawMyChannelJoinRequest} from 'mattermost-redux/actions/channels';
import {getChannelsInCurrentTeam, getChannelsMemberCount as selectMemberCounts, getMyChannelMemberships, getMyPendingJoinRequestsByChannel} from 'mattermost-redux/selectors/entities/channels';
import {isDiscoverableChannelsEnabled} from 'mattermost-redux/selectors/entities/general';
import {getCurrentTeam} from 'mattermost-redux/selectors/entities/teams';
import {getCurrentUserId} from 'mattermost-redux/selectors/entities/users';

import {getVoiceAPI} from 'fusion/calls/calls_api';
import {isAllChannelsCommunity} from 'fusion/channel/community_channels';
import Icon from 'fusion/components/icon';
import {Dialog} from 'fusion/components/layer';
import {useToast} from 'fusion/shell/toast_context';
import ChannelIcon from 'fusion/sidebar/channel_icon';
import {am} from 'fusion/utils/class_names';
import {getHistory} from 'utils/browser_history';
import Constants from 'utils/constants';

import type {GlobalState} from 'types/store';

export type BrowseFilter = 'all' | 'public' | 'private' | 'archived' | 'requestable';

const FILTERS: Array<{key: BrowseFilter; label: MessageDescriptor}> = [
    {key: 'all', label: defineMessage({id: 'fusion.browse.all', defaultMessage: 'All'})},
    {key: 'public', label: defineMessage({id: 'fusion.browse.public', defaultMessage: 'Public'})},
    {key: 'private', label: defineMessage({id: 'fusion.browse.private', defaultMessage: 'Private'})},
    {key: 'archived', label: defineMessage({id: 'fusion.browse.archived', defaultMessage: 'Archived'})},
    {key: 'requestable', label: defineMessage({id: 'fusion.browse.requestable', defaultMessage: 'Ask to join'})},
];

const PAGE = 50;

// isRequestable tells whether a channel is a private one someone may ask to join (Discoverable Private Channels).
export function isRequestable(channel: Channel, member: boolean): boolean {
    return channel.type === Constants.PRIVATE_CHANNEL && channel.discoverable === true && !member && channel.delete_at === 0;
}

// browseList is what the dialog lists: the team's channels of a kind, those joined left out when asked, by name.
export function browseList(channels: Channel[], filter: BrowseFilter, isMember: (id: string) => boolean, hideJoined: boolean): Channel[] {
    const kept = channels.filter((c) => {
        const member = isMember(c.id);
        if (hideJoined && member) {
            return false;
        }
        switch (filter) {
        case 'public':
            return c.type === Constants.OPEN_CHANNEL && c.delete_at === 0;
        case 'private':
            return c.type === Constants.PRIVATE_CHANNEL && c.delete_at === 0 && member;
        case 'archived':
            return c.delete_at !== 0;
        case 'requestable':
            return isRequestable(c, member);
        default:
            return c.delete_at === 0 && (c.type === Constants.OPEN_CHANNEL || member || isRequestable(c, member));
        }
    });
    const seen = new Set<string>();
    return kept.filter((c) => !seen.has(c.id) && seen.add(c.id)).sort((a, b) => a.display_name.localeCompare(b.display_name));
}

// BrowseChannelsDialog lists the team's channels to find and join, as Mattermost's "Browse channels": searched,
// filtered by kind, the joined ones hidden or not, and private ones open to requests asked to join.
export default function BrowseChannelsDialog({onClose}: {onClose: () => void}) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const toast = useToast();
    const team = useSelector(getCurrentTeam);
    const me = useSelector(getCurrentUserId);
    const teamChannels = useSelector(getChannelsInCurrentTeam);
    const memberships = useSelector(getMyChannelMemberships);
    const memberCounts = useSelector(selectMemberCounts);
    const pending = useSelector(getMyPendingJoinRequestsByChannel);
    const discoverable = useSelector(isDiscoverableChannelsEnabled);

    // In a community that has everyone in all its channels, those joined are all of them: what's left to find is
    // archived, or asked for.
    const allChannels = useSelector((state: GlobalState) => Boolean(team) && isAllChannelsCommunity(state, team!.id));
    const [filter, setFilter] = useState<BrowseFilter>('all');
    const [hideJoined, setHideJoined] = useState(allChannels);
    const [query, setQuery] = useState('');
    const [found, setFound] = useState<Channel[] | null>(null);
    const [extra, setExtra] = useState<Channel[]>([]);
    const [busy, setBusy] = useState<string | null>(null);
    const [pages, setPages] = useState({public: 0, archived: 0, done: false});
    const endRef = useRef<HTMLDivElement>(null);
    const teamId = team?.id || '';

    // The first pages, and what search doesn't return otherwise: private channels open to requests, and requests.
    useEffect(() => {
        if (!teamId) {
            return;
        }
        dispatch(getChannels(teamId, 0, PAGE));
        dispatch(getArchivedChannels(teamId, 0, PAGE));
        if (discoverable) {
            dispatch(getMyChannelJoinRequests({status: 'pending'}));
            dispatch(searchAllChannels('', {team_ids: [teamId], nonAdminSearch: true})).then((result) => {
                if ('data' in result && Array.isArray(result.data)) {
                    setExtra(result.data.filter((c) => c.team_id === teamId && c.discoverable === true));
                }
            });
        }
    }, [teamId, discoverable, dispatch]);

    // Searching asks the server, archived channels included, once typing stops.
    useEffect(() => {
        const term = query.trim();
        if (!term || !teamId) {
            setFound(null);
            return undefined;
        }
        let stale = false;
        const timer = setTimeout(async () => {
            const result = await dispatch(searchAllChannels(term, {team_ids: [teamId], nonAdminSearch: true, include_deleted: true}));
            if (!stale && 'data' in result && Array.isArray(result.data)) {
                setFound(result.data.filter((c) => c.team_id === teamId));
            }
        }, 250);
        return () => {
            stale = true;
            clearTimeout(timer);
        };
    }, [query, teamId, dispatch]);

    const isMember = (id: string) => Boolean(memberships[id]);
    const source = found ?? [...teamChannels, ...extra];
    const list = useMemo(() => browseList(source, filter, isMember, hideJoined), [source, filter, memberships, hideJoined]); // eslint-disable-line react-hooks/exhaustive-deps

    // Which of the channels listed that the person isn't in are voice channels: the voice plugin knows the others.
    const notJoined = useMemo(() => list.filter((c) => !memberships[c.id] && c.delete_at === 0).map((c) => c.id).sort().join(','), [list, memberships]);
    useEffect(() => {
        if (notJoined) {
            getVoiceAPI()?.load?.(notJoined.split(',')).catch(() => {});
        }
    }, [notJoined]);

    // Member counts of the channels listed.
    const missingCounts = useMemo(() => list.filter((c) => memberCounts[c.id] === undefined).map((c) => c.id).sort().join(','), [list, memberCounts]);
    useEffect(() => {
        if (missingCounts) {
            dispatch(getChannelsMemberCount(missingCounts.split(',')));
        }
    }, [missingCounts, dispatch]);

    // More pages as the end of the list comes into view.
    useEffect(() => {
        const end = endRef.current;
        if (!end || found || pages.done || !teamId || typeof IntersectionObserver === 'undefined') {
            return undefined;
        }
        const observer = new IntersectionObserver(async ([entry]) => {
            if (!entry.isIntersecting) {
                return;
            }
            const next = {public: pages.public + 1, archived: pages.archived + 1};
            const [open, archived] = await Promise.all([dispatch(getChannels(teamId, next.public, PAGE)), dispatch(getArchivedChannels(teamId, next.archived, PAGE))]);
            const count = (r: unknown) => ((r as {data?: unknown[]}).data?.length || 0);
            setPages({...next, done: count(open) < PAGE && count(archived) < PAGE});
        });
        observer.observe(end);
        return () => observer.disconnect();
    }, [found, pages, teamId, dispatch]);

    const open = (channel: Channel) => {
        if (team) {
            getHistory().push(`/${team.name}/channels/${channel.name}`);
        }
        onClose();
    };
    const join = async (channel: Channel) => {
        setBusy(channel.id);
        const result = await dispatch(joinChannel(me, teamId, channel.id));
        setBusy(null);
        if ('error' in result && result.error) {
            toast((result.error as {message?: string}).message || formatMessage({id: 'fusion.browse.joinFailed', defaultMessage: 'Couldn\'t join #{channel}'}, {channel: channel.display_name}));
            return;
        }
        open(channel);
    };
    const ask = async (channel: Channel) => {
        setBusy(channel.id);
        const result = await dispatch(pending[channel.id] ? withdrawMyChannelJoinRequest(channel.id) : requestJoinChannel(channel.id));
        setBusy(null);
        if ('error' in result && result.error) {
            toast((result.error as {message?: string}).message || formatMessage({id: 'fusion.browse.requestFailed', defaultMessage: 'The request couldn\'t be sent.'}));
        }
    };

    return (
        <Dialog
            label={formatMessage({id: 'fusion.browse.title', defaultMessage: 'Browse channels'})}
            onClose={onClose}
            small={false}
            className={am('wide')}
        >
            <div className={am('pane', 'browse')}>
                <h2>{formatMessage({id: 'fusion.browse.title', defaultMessage: 'Browse channels'})}</h2>
                {allChannels && <p className={am('lead')}>{formatMessage({id: 'fusion.browse.allChannels', defaultMessage: 'Everyone in {community} is in all the channels they may see. Find its archived channels here, and the private ones you can ask to join.'}, {community: team?.display_name})}</p>}
                <div className={am('browse-search')}>
                    <Icon
                        name='search'
                        size='sm'
                    />
                    <input
                        type='text'
                        value={query}
                        autoFocus={true}
                        placeholder={formatMessage({id: 'fusion.browse.search', defaultMessage: 'Search channels'})}
                        aria-label={formatMessage({id: 'fusion.browse.search', defaultMessage: 'Search channels'})}
                        onChange={(e) => setQuery(e.target.value)}
                    />
                </div>
                <div className={am('browse-bar')}>
                    <div
                        className={am('browse-filters')}
                        role='radiogroup'
                        aria-label={formatMessage({id: 'fusion.browse.kind', defaultMessage: 'Kind of channels'})}
                    >
                        {FILTERS.filter((f) => f.key !== 'requestable' || discoverable).map((f) => (
                            <button
                                key={f.key}
                                type='button'
                                role='radio'
                                aria-checked={filter === f.key}
                                className={am('chip-pick', {on: filter === f.key})}
                                onClick={() => setFilter(f.key)}
                            >
                                {formatMessage(f.label)}
                            </button>
                        ))}
                    </div>
                    <label className={am('check-row')}>
                        <input
                            type='checkbox'
                            checked={hideJoined}
                            onChange={(e) => setHideJoined(e.target.checked)}
                        />
                        {formatMessage({id: 'fusion.browse.hideJoined', defaultMessage: 'Hide joined'})}
                    </label>
                </div>
                <div
                    className={am('browse-list')}
                    role='list'
                    aria-label={formatMessage({id: 'fusion.browse.list', defaultMessage: 'Channels'})}
                >
                    {list.map((c) => {
                        const member = isMember(c.id);
                        const requestable = isRequestable(c, member);
                        const archived = c.delete_at !== 0;
                        return (
                            <div
                                key={c.id}
                                role='listitem'
                                className={am('browse-row')}
                            >
                                <button
                                    type='button'
                                    className={am('browse-open')}
                                    disabled={requestable}
                                    onClick={() => open(c)}
                                >
                                    {archived ? <Icon name='folder'/> : <ChannelIcon channel={c}/>}
                                    <span className={am('grow')}>
                                        <b>{c.display_name}</b>
                                        <small>
                                            {[
                                                memberCounts[c.id] === undefined ? '' : formatMessage({id: 'fusion.browse.members', defaultMessage: '{count, plural, one {# member} other {# members}}'}, {count: memberCounts[c.id]}),
                                                archived ? formatMessage({id: 'fusion.browse.isArchived', defaultMessage: 'Archived'}) : '',
                                                c.purpose,
                                            ].filter(Boolean).join(' · ')}
                                        </small>
                                    </span>
                                </button>
                                {member && <span className={am('browse-joined')}>{formatMessage({id: 'fusion.browse.joined', defaultMessage: 'Joined'})}</span>}
                                {!member && !archived && !requestable && (
                                    <button
                                        type='button'
                                        className={am('btn', 'small', 'primary')}
                                        disabled={busy === c.id}
                                        onClick={() => join(c)}
                                    >
                                        {formatMessage({id: 'fusion.browse.join', defaultMessage: 'Join'})}
                                    </button>
                                )}
                                {requestable && (
                                    <button
                                        type='button'
                                        className={am('btn', 'small', {primary: !pending[c.id]})}
                                        disabled={busy === c.id}
                                        title={pending[c.id] ? formatMessage({id: 'fusion.browse.withdrawTitle', defaultMessage: 'Withdraw your request'}) : undefined}
                                        onClick={() => ask(c)}
                                    >
                                        {pending[c.id] ? formatMessage({id: 'fusion.browse.requested', defaultMessage: 'Requested'}) : formatMessage({id: 'fusion.browse.ask', defaultMessage: 'Ask to join'})}
                                    </button>
                                )}
                            </div>
                        );
                    })}
                    {!list.length && <p className={am('lead')}>{query.trim() ? formatMessage({id: 'fusion.browse.noMatch', defaultMessage: 'No channel matches “{query}”.'}, {query: query.trim()}) : formatMessage({id: 'fusion.browse.none', defaultMessage: 'No channels here.'})}</p>}
                    <div ref={endRef}/>
                </div>
            </div>
        </Dialog>
    );
}
