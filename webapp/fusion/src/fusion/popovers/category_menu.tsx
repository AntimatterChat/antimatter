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
import {useDialogs} from 'fusion/shell/dialogs_context';
import {useToast} from 'fusion/shell/toast_context';
import {deleteServerCategory, findServerCategory, getServerCategories, isServerCategory, isTeamAdmin, moveServerCategory, setServerCategoryLocked} from 'fusion/sidebar/server_categories';
import {openDeleteCategory, openRenameCategory} from 'fusion/utils/modals';

import type {GlobalState} from 'types/store';

type Props = {
    category: ChannelCategory;
    anchor?: HTMLElement | null;
    point?: {x: number; y: number};
    onClose: () => void;

    // Whether channels can be created in it, its menu's first item.
    canCreate?: boolean;
};

// CategoryMenu is the ⋯ menu of a sidebar category (also its right-click menu), with what the classic sidebar's
// category menu offers.
export default function CategoryMenu({category, anchor, point, onClose, canCreate = false}: Props) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const toast = useToast();
    const getUnreadIds = useMemo(() => makeGetUnreadIdsForCategory(), []);
    const unreadIds = useSelector((state: GlobalState) => getUnreadIds(state, category));
    const custom = category.type === 'custom';
    const dialogs = useDialogs();

    // A server category (set up by the team's admins for everyone): its admins rename, lock, order and delete it.
    const server = isServerCategory(category);
    const name = category.display_name;
    const admin = useSelector((state: GlobalState) => isTeamAdmin(state, category.team_id));
    const listed = useSelector((state: GlobalState) => getServerCategories(state, category.team_id));
    const known = server ? findServerCategory(listed, name) : undefined;
    const at = known ? listed.indexOf(known) : -1;

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
            {canCreate && (
                <>
                    <MenuItem
                        icon='plus'
                        label={formatMessage({id: 'fusion.sidebar.createChannel', defaultMessage: 'Create channel'})}

                        // Favorites can't hold a channel that isn't one yet: it goes to the default category.
                        onClick={run(() => dialogs.createChannel(category.type === 'favorites' ? undefined : category.id))}
                    />
                    <MenuSeparator/>
                </>
            )}
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
            {server && admin && (
                <>
                    <MenuItem
                        icon='shield'
                        label={formatMessage({id: 'fusion.categoryMenu.permissions', defaultMessage: 'Permissions…'})}
                        sub={formatMessage({id: 'fusion.categoryMenu.permissionsSub', defaultMessage: 'Its channels inherit them'})}
                        onClick={run(() => dialogs.categoryPermissions(category.team_id, name))}
                    />
                    <MenuItem
                        icon='pen'
                        label={formatMessage({id: 'fusion.categoryMenu.renameServer', defaultMessage: 'Rename for everyone…'})}
                        onClick={run(() => dialogs.renameServerCategory(name))}
                    />
                    <MenuItem
                        icon='lock'
                        role='menuitemcheckbox'
                        checked={Boolean(known?.locked)}
                        label={formatMessage({id: 'fusion.categoryMenu.lock', defaultMessage: 'Locked'})}
                        sub={formatMessage({id: 'fusion.categoryMenu.lockSub', defaultMessage: 'Only admins file channels here, and they stay here'})}
                        onClick={run(() => dispatch(setServerCategoryLocked(category.team_id, name, !known?.locked)))}
                    />
                    {at > 0 && (
                        <MenuItem
                            icon='chev'
                            label={formatMessage({id: 'fusion.categoryMenu.moveUp', defaultMessage: 'Move up'})}
                            onClick={run(() => dispatch(moveServerCategory(category.team_id, name, -1)))}
                        />
                    )}
                    {at !== -1 && at < listed.length - 1 && (
                        <MenuItem
                            icon='chev'
                            label={formatMessage({id: 'fusion.categoryMenu.moveDown', defaultMessage: 'Move down'})}
                            onClick={run(() => dispatch(moveServerCategory(category.team_id, name, 1)))}
                        />
                    )}
                    <MenuItem
                        icon='trash'
                        danger={true}
                        label={formatMessage({id: 'fusion.categoryMenu.deleteServer', defaultMessage: 'Delete for everyone'})}
                        sub={formatMessage({id: 'fusion.categoryMenu.deleteServerSub', defaultMessage: 'Its channels go back to everyone\'s own categories'})}
                        onClick={run(() => dialogs.confirm({
                            title: formatMessage({id: 'fusion.categoryMenu.deleteServerTitle', defaultMessage: 'Delete “{name}” for everyone?'}, {name}),
                            message: formatMessage({id: 'fusion.categoryMenu.deleteServerMessage', defaultMessage: 'Everyone in the team loses this category; its channels go back to their own categories.'}),
                            confirmLabel: formatMessage({id: 'fusion.categoryMenu.deleteServerConfirm', defaultMessage: 'Delete for everyone'}),
                            danger: true,
                            onConfirm: async () => {
                                const result = await dispatch(deleteServerCategory(category.team_id, name));
                                if ('error' in result && result.error) {
                                    toast((result.error as {message?: string}).message || formatMessage({id: 'fusion.toast.serverCategoryFailed', defaultMessage: 'The categories couldn\'t be saved.'}));
                                } else {
                                    toast(formatMessage({id: 'fusion.toast.serverCategoryDeleted', defaultMessage: 'Deleted “{name}” for everyone'}, {name}));
                                }
                            },
                        }))}
                    />
                </>
            )}
            {!server && category.type !== 'direct_messages' && (
                <MenuItem
                    icon={category.muted ? 'bell' : 'bell-off'}
                    label={category.muted ? formatMessage({id: 'fusion.categoryMenu.unmute', defaultMessage: 'Unmute category'}) : formatMessage({id: 'fusion.categoryMenu.mute', defaultMessage: 'Mute category'})}
                    onClick={run(() => dispatch(setCategoryMuted(category.id, !category.muted)))}
                />
            )}
            {!server && (
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
            )}
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
                onClick={run(() => dialogs.createCategory())}
            />
        </Popover>
    );
}
