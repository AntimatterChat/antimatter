// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useMemo, useRef, useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector, useStore} from 'react-redux';

import type {ChannelCategory} from '@mattermost/types/channel_categories';

import {moveCategory, setCategoryCollapsed} from 'mattermost-redux/actions/channel_categories';
import {getNonManagedCategoryOrderForTeam} from 'mattermost-redux/selectors/entities/channel_categories';
import {getChannel} from 'mattermost-redux/selectors/entities/channels';

import {makeGetFilteredChannelIdsForCategory} from 'selectors/views/channel_sidebar';

import Icon from 'fusion/components/icon';
import CategoryMenu from 'fusion/popovers/category_menu';
import {am} from 'fusion/utils/class_names';

import type {GlobalState} from 'types/store';

import ChannelRow from './channel_row';
import {getHiddenChannelIds} from './hidden';
import {canFileChannel, fileChannel, fileChannelHere, findServerCategory, getServerCategories, isServerCategory, isTeamAdmin, SERVER_CATEGORY_PREFIX, setServerCategoryCollapsed} from './server_categories';
import {canDropChannel, categoryDropIndex, endSidebarDrag, getSidebarDrag, isLowerHalf, moveSidebarChannel, startSidebarDrag} from './sidebar_drag';
import type {SidebarDrag} from './sidebar_drag';

type ChannelDrag = Extract<SidebarDrag, {kind: 'channel'}>;

// Where a dragged channel or category would land: before or after a channel row, or a category's header.
type DropMark = {target: string; after: boolean} | null;

// Category is a collapsible group of channels in the sidebar. Its channels and the category itself can be dragged to
// reorder them, as in the classic sidebar; its ⋯ button and right-click open its menu.
export default function Category({category}: {category: ChannelCategory}) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const store = useStore<GlobalState>();
    const getChannelIds = useMemo(() => makeGetFilteredChannelIdsForCategory(), []);
    const allChannelIds = useSelector((state: GlobalState) => getChannelIds(state, category));

    // Hidden conversations stay out until something new is said in them.
    const hidden = useSelector(getHiddenChannelIds);
    const channelIds = useMemo(() => (hidden.size ? allChannelIds.filter((id) => !hidden.has(id)) : allChannelIds), [allChannelIds, hidden]);
    const menuButton = useRef<HTMLButtonElement>(null);
    const [menu, setMenu] = useState<{x: number; y: number} | 'button' | null>(null);
    const [mark, setMark] = useState<DropMark>(null);
    const collapsed = category.collapsed;

    // Server categories (set up by the team's admins for everyone) fold for each person, take channels from those allowed
    // to file them, and keep their channels when locked.
    const server = isServerCategory(category);
    const locked = useSelector((state: GlobalState) => Boolean(server && findServerCategory(getServerCategories(state, category.team_id), category.display_name)?.locked));
    const canCreateHere = useSelector((state: GlobalState) => !locked || isTeamAdmin(state, category.team_id));
    const header = `category:${category.id}`;

    if (!channelIds.length && category.type === 'favorites') {
        return null;
    }

    const toggle = () => {
        if (server) {
            dispatch(setServerCategoryCollapsed(category.team_id, category.display_name, !collapsed));
        } else {
            dispatch(setCategoryCollapsed(category.id, !collapsed));
        }
    };

    // Where a dragged channel may go. Into a server category: filed there for everyone, by those allowed to. Out of an
    // open one: into one of the person's own categories or favourites (theirs alone), or back to Channels, unfiled,
    // by those allowed to. A locked server category keeps its channels.
    const accepts = (drag: SidebarDrag | null): drag is ChannelDrag => {
        if (drag?.kind !== 'channel') {
            return false;
        }
        const state = store.getState();
        const channel = getChannel(state, drag.channelId);
        if (!channel) {
            return false;
        }
        if (server) {
            return !drag.directMessage && drag.categoryId !== category.id && canFileChannel(state, channel, category.display_name);
        }
        if (drag.categoryId.startsWith(SERVER_CATEGORY_PREFIX)) {
            const from = findServerCategory(getServerCategories(state, category.team_id), drag.categoryId.slice(SERVER_CATEGORY_PREFIX.length));
            if (from?.locked) {
                return category.type === 'channels' && canFileChannel(state, channel, '');
            }
            if (category.type === 'channels') {
                return canFileChannel(state, channel, '');
            }
            return category.type === 'custom' || category.type === 'favorites';
        }
        return canDropChannel(drag, category);
    };
    const place = (drag: ChannelDrag, targetId: string | null, after: boolean) => {
        const channel = getChannel(store.getState(), drag.channelId);
        if (!channel) {
            return;
        }
        if (server) {
            dispatch(fileChannelHere(channel, category.display_name));
            return;
        }
        if (drag.categoryId.startsWith(SERVER_CATEGORY_PREFIX) && category.type === 'channels') {
            dispatch(fileChannel(channel.id, ''));
        }
        dispatch(moveSidebarChannel(category.id, drag.channelId, targetId, after));
    };

    // A channel dropped on another goes before or after it; on the header, first in the category.
    const overChannel = (e: React.DragEvent, channelId: string) => {
        if (!accepts(getSidebarDrag())) {
            return;
        }
        e.preventDefault();
        e.stopPropagation();
        const after = isLowerHalf(e);
        if (mark?.target !== channelId || mark.after !== after) {
            setMark({target: channelId, after});
        }
    };
    const dropOnChannel = (e: React.DragEvent, channelId: string) => {
        const drag = getSidebarDrag();
        setMark(null);
        if (!accepts(drag)) {
            return;
        }
        e.preventDefault();
        e.stopPropagation();
        if (drag.channelId !== channelId) {
            place(drag, channelId, isLowerHalf(e));
        }
        endSidebarDrag();
    };

    // On the header: a channel goes first in the category, a category before or after this one.
    const overHeader = (e: React.DragEvent) => {
        const drag = getSidebarDrag();
        if (drag?.kind === 'category' ? server || drag.categoryId === category.id : !accepts(drag)) {
            return;
        }
        e.preventDefault();
        const after = drag?.kind === 'category' && isLowerHalf(e);
        if (mark?.target !== header || mark.after !== after) {
            setMark({target: header, after});
        }
    };
    const dropOnHeader = (e: React.DragEvent) => {
        const drag = getSidebarDrag();
        setMark(null);
        if (drag?.kind === 'category') {
            e.preventDefault();
            const order = getNonManagedCategoryOrderForTeam(store.getState(), category.team_id) || [];
            const index = categoryDropIndex(order, drag.categoryId, category.id, isLowerHalf(e));
            if (index !== -1) {
                dispatch(moveCategory(category.team_id, drag.categoryId, index));
            }
        } else if (accepts(drag)) {
            e.preventDefault();
            place(drag, null, false);
        }
        endSidebarDrag();
    };
    const leave = (e: React.DragEvent) => {
        if (!e.currentTarget.contains(e.relatedTarget as Node | null)) {
            setMark(null);
        }
    };
    const markClass = (target: string) => (mark?.target === target ? am(mark.after ? 'drop-after' : 'drop-before') : '');

    return (
        <>
            <div
                className={am('cat', {collapsed}) + ' ' + markClass(header)}
                role='button'
                tabIndex={0}
                aria-expanded={!collapsed}
                draggable={!server}
                onClick={toggle}
                onKeyDown={(e) => {
                    if (e.key === 'Enter' || e.key === ' ') {
                        e.preventDefault();
                        toggle();
                    }
                }}
                onContextMenu={(e) => {
                    e.preventDefault();
                    setMenu({x: e.clientX, y: e.clientY});
                }}
                onDragStart={(e) => startSidebarDrag({kind: 'category', categoryId: category.id}, e.dataTransfer)}
                onDragEnd={() => {
                    endSidebarDrag();
                    setMark(null);
                }}
                onDragOver={overHeader}
                onDragLeave={leave}
                onDrop={dropOnHeader}
            >
                <Icon
                    name='chev'
                    size='xs'
                />
                <span className={am('grow')}>{category.display_name}</span>
                {server && (
                    <span
                        className={am('server-mark')}
                        title={locked ? formatMessage({id: 'fusion.sidebar.serverLocked', defaultMessage: 'Set up for everyone by the team\'s admins. Its channels stay here.'}) : formatMessage({id: 'fusion.sidebar.serverOpen', defaultMessage: 'Set up for everyone by the team\'s admins. You can move its channels to your own categories.'})}
                    >
                        <Icon
                            name={locked ? 'lock' : 'server'}
                            size='xs'
                        />
                    </span>
                )}
                {category.muted && (
                    <Icon
                        name='bell-off'
                        size='xs'
                    />
                )}
                <button
                    ref={menuButton}
                    className={am('ch-act', 'cat-act', {open: menu !== null})}
                    title={formatMessage({id: 'fusion.sidebar.categoryOptions', defaultMessage: 'Category options'})}
                    aria-label={formatMessage({id: 'fusion.sidebar.categoryOptionsFor', defaultMessage: 'Category options for {name}'}, {name: category.display_name})}
                    aria-haspopup='menu'
                    aria-expanded={menu !== null}
                    onClick={(e) => {
                        e.stopPropagation();
                        setMenu(menu ? null : 'button');
                    }}
                >
                    <Icon name='dots'/>
                </button>
            </div>
            {channelIds.map((id) => (
                <div
                    key={id}
                    className={am('drag-row') + ' ' + markClass(id)}
                    draggable={true}
                    onDragStart={(e) => {
                        const channel = getChannel(store.getState(), id);
                        startSidebarDrag({kind: 'channel', channelId: id, categoryId: category.id, directMessage: channel?.type === 'D' || channel?.type === 'G'}, e.dataTransfer);
                    }}
                    onDragEnd={() => {
                        endSidebarDrag();
                        setMark(null);
                    }}
                    onDragOver={(e) => overChannel(e, id)}
                    onDragLeave={leave}
                    onDrop={(e) => dropOnChannel(e, id)}
                >
                    <ChannelRow
                        channelId={id}
                        collapsed={collapsed}
                    />
                </div>
            ))}
            {menu && (
                <CategoryMenu
                    category={category}
                    anchor={menu === 'button' ? menuButton.current : null}
                    point={menu === 'button' ? undefined : menu}
                    canCreate={canCreateHere}
                    onClose={() => setMenu(null)}
                />
            )}
        </>
    );
}
