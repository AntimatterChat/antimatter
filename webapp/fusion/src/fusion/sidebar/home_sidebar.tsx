// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import {getDirectAndGroupChannels} from 'mattermost-redux/selectors/entities/channels';
import {getCurrentTeam} from 'mattermost-redux/selectors/entities/teams';
import {getUser} from 'mattermost-redux/selectors/entities/users';

import {showFlaggedPosts, showMentions} from 'actions/views/rhs';

import Icon from 'fusion/components/icon';
import {useLayout} from 'fusion/shell/layout_context';
import {am} from 'fusion/utils/class_names';
import {openNewDirectMessage} from 'fusion/utils/modals';
import {getHistory} from 'utils/browser_history';

import type {GlobalState} from 'types/store';

import DirectRow, {useTeammateId} from './direct_row';
import VoicePanel from './voice_panel';

// The second line of a conversation: the person's custom status or position.
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
    const meta = useDirectMeta(useTeammateId(channel));
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
    const layout = useLayout();
    const team = useSelector(getCurrentTeam);
    const channels = useSelector(getDirectAndGroupChannels);
    const [query, setQuery] = useState('');

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
                    <button
                        className={am('ch')}
                        onClick={() => dispatch(showMentions())}
                    >
                        <Icon name='at'/>
                        <span className={am('name')}>{formatMessage({id: 'fusion.home.mentions', defaultMessage: 'Mentions'})}</span>
                    </button>
                    <button
                        className={am('ch')}
                        onClick={() => {
                            if (team) {
                                layout.setNavOpen(false);
                                getHistory().push(`/${team.name}/threads`);
                            }
                        }}
                    >
                        <Icon name='thread'/>
                        <span className={am('name')}>{formatMessage({id: 'fusion.home.threads', defaultMessage: 'Followed threads'})}</span>
                    </button>
                    <button
                        className={am('ch')}
                        onClick={() => dispatch(showFlaggedPosts())}
                    >
                        <Icon name='bookmark'/>
                        <span className={am('name')}>{formatMessage({id: 'fusion.home.saved', defaultMessage: 'Saved messages'})}</span>
                    </button>
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
        </aside>
    );
}
