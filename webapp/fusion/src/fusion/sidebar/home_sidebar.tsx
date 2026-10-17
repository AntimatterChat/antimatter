// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useMemo, useRef, useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';
import {Link, useLocation} from 'react-router-dom';

import {Client4} from 'mattermost-redux/client';
import {getDirectAndGroupChannels} from 'mattermost-redux/selectors/entities/channels';
import {getFeatureFlagValue} from 'mattermost-redux/selectors/entities/general';
import {getScheduledPostsByTeamCount, isScheduledPostsEnabled} from 'mattermost-redux/selectors/entities/scheduled_posts';
import {getCurrentTeam} from 'mattermost-redux/selectors/entities/teams';
import {getCurrentUserId, getUser, getUsers} from 'mattermost-redux/selectors/entities/users';
import {getUserIdFromChannelName} from 'mattermost-redux/utils/channel_utils';

import {openDirectChannelToUserId} from 'actions/channel_actions';
import {makeGetDraftsCount} from 'selectors/drafts';

import {useRecapsEntryShown} from 'components/common/hooks/useAIFeatures';

import {useCallsOnTop} from 'fusion/calls/calls_on_top';
import Icon from 'fusion/components/icon';
import {useUnreadMentions} from 'fusion/hooks/mentions';
import CollectionsPopover from 'fusion/popovers/collections_popover';
import {getHiddenDirectIds} from 'fusion/privacy/blocking';
import {RequestsSection} from 'fusion/privacy/message_requests';
import {useLayout} from 'fusion/shell/layout_context';
import {a11yRegion} from 'fusion/utils/a11y_regions';
import {am} from 'fusion/utils/class_names';
import {openNewDirectMessage} from 'fusion/utils/modals';
import {channelPath} from 'fusion/utils/paths';
import {getHistory} from 'utils/browser_history';
import {SCHEDULED_POST_URL_SUFFIX} from 'utils/constants';

import type {GlobalState} from 'types/store';

import DirectRow, {useTeammateId} from './direct_row';
import {useNotesToSelf} from './group_dm';
import {getHiddenChannelIds} from './hidden';
import HiddenSection from './hidden_section';
import {useLastMessage} from './last_message';
import VoicePanel from './voice_panel';

// The "Find or start a conversation" field, which the home icon focuses.
export const DM_FIND_ID = 'am-dm-find';

type Collection = 'mentions' | 'threads' | 'saved';

// The second line of a conversation without messages: the person's custom status or position.
function useDirectMeta(teammateId?: string): string {
    return useSelector((state: GlobalState) => {
        if (!teammateId) {
            return '';
        }
        const user = getUser(state, teammateId);
        const custom = user?.props?.customStatus ? JSON.parse(user.props.customStatus as string) : null;
        return custom?.text || user?.position || '';
    });
}

function HomeRow({channel}: {channel: Parameters<typeof DirectRow>[0]['channel']}) {
    const last = useLastMessage(channel);
    const status = useDirectMeta(useTeammateId(channel));
    const meta = last || status;
    return (
        <DirectRow
            channel={channel}
            meta={meta || undefined}
        />
    );
}

// HomeSidebar lists every direct and group message, from the home icon of the team rail.
export default function HomeSidebar() {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const channels = useSelector(getDirectAndGroupChannels);
    const [query, setQuery] = useState('');
    const [collection, setCollection] = useState<Collection | null>(null);
    const linkRefs = useRef<Partial<Record<Collection, HTMLButtonElement | null>>>({});

    const mentions = useUnreadMentions();
    const team = useSelector(getCurrentTeam);
    const users = useSelector(getUsers);
    const hidden = useSelector(getHiddenChannelIds);

    // Message requests are listed apart, and conversations with people you block left out; a search finds them.
    const requestsAndBlocked = useSelector(getHiddenDirectIds);
    const me = useSelector(getCurrentUserId);
    const layout = useLayout();
    const openNotes = useNotesToSelf();
    const getDraftsCount = useMemo(() => makeGetDraftsCount(), []);
    const draftCount = useSelector(getDraftsCount);
    const scheduledEnabled = useSelector(isScheduledPostsEnabled);
    const scheduledCount = useSelector((state: GlobalState) => (team ? getScheduledPostsByTeamCount(state, team.id, true) : 0));

    // Antimatter: recaps once they're on and an admin picked a model connection for them, or while the person has
    // recaps to read.
    const recapsEnabled = useRecapsEntryShown(useSelector((state: GlobalState) => getFeatureFlagValue(state, 'EnableAIRecaps') === 'true'));
    const pathname = useLocation().pathname;

    // The pages of the home: drafts, scheduled messages and AI recaps, in the main column.
    const page = (path: string, icon: 'pen' | 'clock' | 'sparkle', label: string, count = 0) => (
        <Link
            to={`/${team!.name}/${path}`}
            className={am('ch', {active: pathname.startsWith(`/${team!.name}/${path}`)})}
            onClick={() => layout.setNavOpen(false)}
        >
            <Icon name={icon}/>
            <span className={am('name')}>{label}</span>
            {count > 0 && <span className={am('count')}>{count}</span>}
        </Link>
    );

    // Enter opens the first match, else starts a conversation with the first person matching, as in the mockup.
    const findOrStart = async () => {
        if (!team) {
            return;
        }
        if (list.length) {
            layout.setHome(false);
            getHistory().push(channelPath(team.name, list[0]));
            return;
        }
        if (!q) {
            dispatch(openNewDirectMessage());
            return;
        }
        try {
            const {users} = await Client4.autocompleteUsers(q, '', '', {limit: 1});
            const user = users.find((u) => !u.delete_at);
            if (!user) {
                dispatch(openNewDirectMessage());
                return;
            }
            const result = await dispatch(openDirectChannelToUserId(user.id));
            if ('data' in result && result.data) {
                setQuery('');
                layout.setHome(false);
                getHistory().push(channelPath(team.name, result.data));
            }
        } catch {
            dispatch(openNewDirectMessage());
        }
    };
    const link = (key: Collection, icon: 'at' | 'thread' | 'pin', label: string, badge = 0) => (
        <button
            ref={(el) => {
                linkRefs.current[key] = el;
            }}
            className={am('ch', {active: collection === key})}
            aria-haspopup='dialog'
            aria-expanded={collection === key}
            onClick={() => setCollection(collection === key ? null : key)}
        >
            <Icon name={icon}/>
            <span className={am('name')}>{label}</span>
            {badge > 0 && <span className={am('badge')}>{badge}</span>}
        </button>
    );

    const q = query.trim().toLowerCase().replace(/^@/, '');
    const matches = channels.
        filter((c) => c.delete_at === 0 && (Boolean(q) || !requestsAndBlocked.has(c.id)) && (!q || c.display_name.toLowerCase().includes(q) || (c.type === 'D' && Boolean(users[c.teammate_id || getUserIdFromChannelName(me, c.name)]?.username.includes(q))))).

        sort((a, b) => b.last_post_at - a.last_post_at);

    // Hidden conversations wait at the end until they get new messages, but a search finds them. A conversation
    // with a call in progress stays on top while it lasts.
    const visible = q ? matches : matches.filter((c) => !hidden.has(c.id));
    const onTop = useCallsOnTop(visible);
    const list = q ? visible : onTop;
    const hiddenList = q ? [] : matches.filter((c) => hidden.has(c.id) && !list.includes(c));

    return (
        <aside
            {...a11yRegion(am('sidebar'), 'sidebar')}
            aria-label={formatMessage({id: 'fusion.home.label', defaultMessage: 'Direct messages'})}
        >
            <div className={am('side-head')}>
                <h2>{formatMessage({id: 'fusion.home.title', defaultMessage: 'Direct messages'})}</h2>
                <button
                    className={am('icon-btn')}
                    data-am-tip={formatMessage({id: 'fusion.home.new', defaultMessage: 'New message'})}
                    aria-label={formatMessage({id: 'fusion.home.newLabel', defaultMessage: 'New direct message'})}
                    onClick={() => dispatch(openNewDirectMessage())}
                >
                    <Icon
                        name='plus'
                        size='sm'
                    />
                </button>
            </div>
            <label className={am('dm-find')}>
                <Icon
                    name='search'
                    size='sm'
                />
                <input
                    id={DM_FIND_ID}
                    value={query}
                    placeholder={formatMessage({id: 'fusion.home.find', defaultMessage: 'Find or start a conversation'})}
                    aria-label={formatMessage({id: 'fusion.home.find', defaultMessage: 'Find or start a conversation'})}
                    autoComplete='off'
                    onChange={(e) => setQuery(e.target.value)}
                    onKeyDown={(e) => {
                        if (e.key === 'Enter') {
                            e.preventDefault();
                            findOrStart();
                        }
                    }}
                />
            </label>
            <div className={am('chan-scroll')}>
                <div className={am('home-links')}>
                    {link('mentions', 'at', formatMessage({id: 'fusion.home.mentions', defaultMessage: 'Mentions'}), mentions)}
                    {link('threads', 'thread', formatMessage({id: 'fusion.home.threads', defaultMessage: 'Followed threads'}))}
                    {link('saved', 'pin', formatMessage({id: 'fusion.home.saved', defaultMessage: 'Saved messages'}))}
                    {team && page('drafts', 'pen', formatMessage({id: 'fusion.home.drafts', defaultMessage: 'Drafts'}), draftCount)}
                    {team && scheduledEnabled && page(SCHEDULED_POST_URL_SUFFIX, 'clock', formatMessage({id: 'fusion.home.scheduled', defaultMessage: 'Scheduled'}), scheduledCount)}
                    {team && recapsEnabled && page('recaps', 'sparkle', formatMessage({id: 'fusion.home.recaps', defaultMessage: 'Recaps'}))}
                    <button
                        className={am('ch')}
                        onClick={async () => {
                            const notes = await openNotes();
                            if (notes && team) {
                                layout.setHome(false);
                                getHistory().push(channelPath(team.name, notes));
                            }
                        }}
                    >
                        <Icon name='pad'/>
                        <span className={am('name')}>{formatMessage({id: 'fusion.notes.name', defaultMessage: 'Notes to self'})}</span>
                    </button>
                </div>
                <div
                    className={am('cat')}
                    style={{cursor: 'default'}}
                >
                    <span className={am('grow')}>{formatMessage({id: 'fusion.home.conversations', defaultMessage: 'Conversations'})}</span>
                    <button
                        type='button'
                        className={am('add-ch')}
                        style={{opacity: 1}}
                        data-am-tip={formatMessage({id: 'fusion.home.newGroup', defaultMessage: 'New conversation with one or more people'})}
                        aria-label={formatMessage({id: 'fusion.home.newGroup', defaultMessage: 'New conversation with one or more people'})}
                        onClick={() => dispatch(openNewDirectMessage())}
                    >
                        <Icon
                            name='plus'
                            size='sm'
                        />
                    </button>
                </div>
                <div className={am('dm-list')}>
                    {!q && <RequestsSection/>}
                    {list.map((channel) => (
                        <HomeRow
                            key={channel.id}
                            channel={channel}
                        />
                    ))}
                    <HiddenSection
                        items={hiddenList}
                        render={(channel) => (
                            <HomeRow
                                key={channel.id}
                                channel={channel}
                            />
                        )}
                    />
                    {!list.length && !hiddenList.length && (
                        <div className={am('empty')}>
                            {formatMessage({id: 'fusion.home.none', defaultMessage: 'No conversation matches “{query}”. Press Enter to start one.'}, {query})}
                        </div>
                    )}
                </div>
            </div>
            <VoicePanel/>
            {collection && (
                <CollectionsPopover
                    anchor={linkRefs.current[collection] || null}
                    initialTab={collection}
                    placement='right'
                    onClose={() => setCollection(null)}
                />
            )}
        </aside>
    );
}
