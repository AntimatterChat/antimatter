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
import {useDialogs} from 'fusion/shell/dialogs_context';
import {am} from 'fusion/utils/class_names';

import type {GlobalState} from 'types/store';

import ChannelRow from './channel_row';
import {canDropChannel, categoryDropIndex, endSidebarDrag, getSidebarDrag, isLowerHalf, moveSidebarChannel, startSidebarDrag} from './sidebar_drag';

// Where a dragged channel or category would land: before or after a channel row, or a category's header.
type DropMark = {target: string; after: boolean} | null;

// Category is a collapsible group of channels in the sidebar. Its channels and the category itself can be dragged to
// reorder them, as in the classic sidebar; its ⋯ button and right-click open its menu.
export default function Category({category}: {category: ChannelCategory}) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const store = useStore<GlobalState>();
    const dialogs = useDialogs();
    const getChannelIds = useMemo(() => makeGetFilteredChannelIdsForCategory(), []);
    const channelIds = useSelector((state: GlobalState) => getChannelIds(state, category));
    const menuButton = useRef<HTMLButtonElement>(null);
    const [menu, setMenu] = useState<{x: number; y: number} | 'button' | null>(null);
    const [mark, setMark] = useState<DropMark>(null);
    const collapsed = category.collapsed;
    const header = `category:${category.id}`;

    if (!channelIds.length && category.type === 'favorites') {
        return null;
    }

    const toggle = () => dispatch(setCategoryCollapsed(category.id, !collapsed));

    // A channel dropped on another goes before or after it; on the header, first in the category.
    const overChannel = (e: React.DragEvent, channelId: string) => {
        if (!canDropChannel(getSidebarDrag(), category)) {
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
        if (!canDropChannel(drag, category)) {
            return;
        }
        e.preventDefault();
        e.stopPropagation();
        if (drag.channelId !== channelId) {
            dispatch(moveSidebarChannel(category.id, drag.channelId, channelId, isLowerHalf(e)));
        }
        endSidebarDrag();
    };

    // On the header: a channel goes first in the category, a category before or after this one.
    const overHeader = (e: React.DragEvent) => {
        const drag = getSidebarDrag();
        if (drag?.kind === 'category' ? drag.categoryId === category.id : !canDropChannel(drag, category)) {
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
        } else if (canDropChannel(drag, category)) {
            e.preventDefault();
            dispatch(moveSidebarChannel(category.id, drag.channelId, null, false));
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
                draggable={true}
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
                {category.muted && (
                    <Icon
                        name='bell-off'
                        size='xs'
                    />
                )}
                <button
                    ref={menuButton}
                    className={am('add-ch', {open: menu !== null})}
                    title={formatMessage({id: 'fusion.sidebar.categoryOptions', defaultMessage: 'Category options'})}
                    aria-label={formatMessage({id: 'fusion.sidebar.categoryOptionsFor', defaultMessage: 'Category options for {name}'}, {name: category.display_name})}
                    aria-haspopup='menu'
                    aria-expanded={menu !== null}
                    onClick={(e) => {
                        e.stopPropagation();
                        setMenu(menu ? null : 'button');
                    }}
                >
                    <Icon
                        name='dots'
                        size='sm'
                    />
                </button>
                <button
                    className={am('add-ch')}
                    title={formatMessage({id: 'fusion.sidebar.createChannel', defaultMessage: 'Create channel'})}
                    aria-label={formatMessage({id: 'fusion.sidebar.createChannel', defaultMessage: 'Create channel'})}
                    onClick={(e) => {
                        e.stopPropagation();

                        // Favorites can't hold a channel that isn't one yet: it goes to the default category.
                        dialogs.createChannel(category.type === 'favorites' ? undefined : category.id);
                    }}
                >
                    <Icon
                        name='plus'
                        size='sm'
                    />
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
                    onClose={() => setMenu(null)}
                />
            )}
        </>
    );
}
