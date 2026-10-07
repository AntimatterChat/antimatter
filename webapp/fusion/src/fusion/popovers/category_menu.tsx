// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useMemo} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import {CategorySorting} from '@mattermost/types/channel_categories';
import type {ChannelCategory} from '@mattermost/types/channel_categories';

import {setCategoryMuted, setCategorySorting} from 'mattermost-redux/actions/channel_categories';
import {readMultipleChannels} from 'mattermost-redux/actions/channels';

import {makeGetUnreadIdsForCategory} from 'selectors/views/channel_sidebar';

import {Popover} from 'fusion/components/layer';
import {Flyout, MenuItem, MenuSeparator} from 'fusion/components/menu';
import {useToast} from 'fusion/shell/toast_context';
import {openCreateCategory, openDeleteCategory, openRenameCategory} from 'fusion/utils/modals';

import type {GlobalState} from 'types/store';

type Props = {
    category: ChannelCategory;
    anchor?: HTMLElement | null;
    point?: {x: number; y: number};
    onClose: () => void;
};

// CategoryMenu is the ⋯ menu of a sidebar category (also its right-click menu), with what the classic sidebar's
// category menu offers.
export default function CategoryMenu({category, anchor, point, onClose}: Props) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const toast = useToast();
    const getUnreadIds = useMemo(() => makeGetUnreadIdsForCategory(), []);
    const unreadIds = useSelector((state: GlobalState) => getUnreadIds(state, category));
    const custom = category.type === 'custom';

    const run = (action: () => void) => () => {
        action();
        onClose();
    };
    const sortBy = (sorting: CategorySorting) => run(() => dispatch(setCategorySorting(category.id, sorting)));

    return (
        <Popover
            anchor={anchor}
            point={point}
            placement={point ? 'point' : 'right'}
            className='plus-pop ch-menu'
            role='menu'
            label={formatMessage({id: 'fusion.categoryMenu.label', defaultMessage: 'Options for {name}'}, {name: category.display_name})}
            onClose={onClose}
        >
            {unreadIds.length > 0 && (
                <MenuItem
                    icon='check'
                    label={formatMessage({id: 'fusion.categoryMenu.markRead', defaultMessage: 'Mark category as read'})}
                    onClick={run(() => {
                        dispatch(readMultipleChannels(unreadIds));
                        toast(formatMessage({id: 'fusion.toast.markedRead', defaultMessage: 'Marked {name} as read'}, {name: category.display_name}));
                    })}
                />
            )}
            {category.type !== 'direct_messages' && (
                <MenuItem
                    icon={category.muted ? 'bell' : 'bell-off'}
                    label={category.muted ? formatMessage({id: 'fusion.categoryMenu.unmute', defaultMessage: 'Unmute category'}) : formatMessage({id: 'fusion.categoryMenu.mute', defaultMessage: 'Mute category'})}
                    onClick={run(() => dispatch(setCategoryMuted(category.id, !category.muted)))}
                />
            )}
            <Flyout
                icon='ul'
                label={formatMessage({id: 'fusion.categoryMenu.sort', defaultMessage: 'Sort'})}
                menuLabel={formatMessage({id: 'fusion.categoryMenu.sortLabel', defaultMessage: 'Sort channels'})}
            >
                <MenuItem
                    role='menuitemradio'
                    checked={category.sorting === CategorySorting.Alphabetical}
                    label={formatMessage({id: 'fusion.categoryMenu.sortAlpha', defaultMessage: 'Alphabetically'})}
                    onClick={sortBy(CategorySorting.Alphabetical)}
                />
                <MenuItem
                    role='menuitemradio'
                    checked={category.sorting === CategorySorting.Recency}
                    label={formatMessage({id: 'fusion.categoryMenu.sortRecent', defaultMessage: 'By recent activity'})}
                    onClick={sortBy(CategorySorting.Recency)}
                />
                <MenuItem
                    role='menuitemradio'
                    checked={category.sorting === CategorySorting.Manual || category.sorting === CategorySorting.Default}
                    label={formatMessage({id: 'fusion.categoryMenu.sortManual', defaultMessage: 'Manually'})}
                    onClick={sortBy(CategorySorting.Manual)}
                />
            </Flyout>
            {custom && (
                <>
                    <MenuSeparator/>
                    <MenuItem
                        icon='pen'
                        label={formatMessage({id: 'fusion.categoryMenu.rename', defaultMessage: 'Rename category'})}
                        onClick={run(() => dispatch(openRenameCategory(category)))}
                    />
                    <MenuItem
                        icon='trash'
                        danger={true}
                        label={formatMessage({id: 'fusion.categoryMenu.delete', defaultMessage: 'Delete category'})}
                        onClick={run(() => dispatch(openDeleteCategory(category)))}
                    />
                </>
            )}
            <MenuSeparator/>
            <MenuItem
                icon='folder'
                label={formatMessage({id: 'fusion.categoryMenu.create', defaultMessage: 'Create new category'})}
                onClick={run(() => dispatch(openCreateCategory()))}
            />
        </Popover>
    );
}
