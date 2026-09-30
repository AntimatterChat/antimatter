// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useMemo} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import type {ChannelCategory} from '@mattermost/types/channel_categories';

import {setCategoryCollapsed} from 'mattermost-redux/actions/channel_categories';

import {makeGetFilteredChannelIdsForCategory} from 'selectors/views/channel_sidebar';

import Icon from 'fusion/components/icon';
import {am} from 'fusion/utils/class_names';
import {openNewChannel} from 'fusion/utils/modals';

import type {GlobalState} from 'types/store';

import ChannelRow from './channel_row';

// Category is a collapsible group of channels in the sidebar.
export default function Category({category}: {category: ChannelCategory}) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const getChannelIds = useMemo(() => makeGetFilteredChannelIdsForCategory(), []);
    const channelIds = useSelector((state: GlobalState) => getChannelIds(state, category));
    const collapsed = category.collapsed;

    if (!channelIds.length && category.type === 'favorites') {
        return null;
    }

    return (
        <>
            <div
                className={am('cat', {collapsed})}
                role='button'
                tabIndex={0}
                aria-expanded={!collapsed}
                onClick={() => dispatch(setCategoryCollapsed(category.id, !collapsed))}
                onKeyDown={(e) => {
                    if (e.key === 'Enter' || e.key === ' ') {
                        e.preventDefault();
                        dispatch(setCategoryCollapsed(category.id, !collapsed));
                    }
                }}
            >
                <Icon
                    name='chev'
                    size='xs'
                />
                <span className={am('grow')}>{category.display_name}</span>
                <button
                    className={am('add-ch')}
                    title={formatMessage({id: 'fusion.sidebar.createChannel', defaultMessage: 'Create channel'})}
                    aria-label={formatMessage({id: 'fusion.sidebar.createChannel', defaultMessage: 'Create channel'})}
                    onClick={(e) => {
                        e.stopPropagation();
                        dispatch(openNewChannel());
                    }}
                >
                    <Icon
                        name='plus'
                        size='sm'
                    />
                </button>
            </div>
            {channelIds.map((id) => (
                <ChannelRow
                    key={id}
                    channelId={id}
                    collapsed={collapsed}
                />
            ))}
        </>
    );
}
