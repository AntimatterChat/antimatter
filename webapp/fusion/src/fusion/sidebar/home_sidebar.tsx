// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useRef, useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import {getDirectAndGroupChannels} from 'mattermost-redux/selectors/entities/channels';
import {getUser} from 'mattermost-redux/selectors/entities/users';

import Icon from 'fusion/components/icon';
import {useUnreadMentions} from 'fusion/hooks/mentions';
import CollectionsPopover from 'fusion/popovers/collections_popover';
import {am} from 'fusion/utils/class_names';
import {openNewDirectMessage} from 'fusion/utils/modals';

import type {GlobalState} from 'types/store';

import DirectRow, {useTeammateId} from './direct_row';
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

    const q = query.trim().toLowerCase();
    const list = channels.
        filter((c) => c.delete_at === 0 && (!q || c.display_name.toLowerCase().includes(q))).
        sort((a, b) => b.last_post_at - a.last_post_at);

    return (
        <aside
            className={am('sidebar')}
            aria-label={formatMessage({id: 'fusion.home.label', defaultMessage: 'Direct messages'})}
        >
            <div className={am('side-head')}>
                <h2>{formatMessage({id: 'fusion.home.title', defaultMessage: 'Direct messages'})}</h2>
                <button
                    className={am('icon-btn')}
                    title={formatMessage({id: 'fusion.home.new', defaultMessage: 'New message'})}
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
                        if (e.key === 'Enter' && !list.length) {
                            dispatch(openNewDirectMessage());
                        }
                    }}
                />
            </label>
            <div className={am('chan-scroll')}>
                <div className={am('home-links')}>
                    {link('mentions', 'at', formatMessage({id: 'fusion.home.mentions', defaultMessage: 'Mentions'}), mentions)}
                    {link('threads', 'thread', formatMessage({id: 'fusion.home.threads', defaultMessage: 'Followed threads'}))}
                    {link('saved', 'pin', formatMessage({id: 'fusion.home.saved', defaultMessage: 'Saved messages'}))}
                </div>
                <div
                    className={am('cat')}
                    style={{cursor: 'default'}}
                >
                    <span className={am('grow')}>{formatMessage({id: 'fusion.home.conversations', defaultMessage: 'Conversations'})}</span>
                </div>
                <div className={am('dm-list')}>
                    {list.map((channel) => (
                        <HomeRow
                            key={channel.id}
                            channel={channel}
                        />
                    ))}
                    {!list.length && (
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
