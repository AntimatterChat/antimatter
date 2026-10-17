// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useLayoutEffect, useRef} from 'react';
import {useSelector} from 'react-redux';

import type {Post} from '@mattermost/types/posts';

import {MenuAppIcon} from 'fusion/apps/app_glyph';
import {Flyout, MenuHeading} from 'fusion/components/menu';
import {am} from 'fusion/utils/class_names';

import type {GlobalState} from 'types/store';
import type {PostDropdownMenuAction} from 'types/store/plugins';

const NO_ACTIONS: PostDropdownMenuAction[] = [];

type Props = {
    post: Post;

    // The flyout holding the apps' actions ("More actions"), and its menu's accessible name.
    label: string;
    menuLabel: string;

    // Runs an action, then closes the menu holding the entries.
    run: (action: () => void) => () => void;
};

// usePluginMenuActions are the actions apps add to the message: their single actions, and their submenus (with their
// entries).
export function usePluginMenuActions(post: Post) {
    const all = useSelector((state: GlobalState) => state.plugins.components.PostDropdownMenu) || NO_ACTIONS;
    const shown = (a: PostDropdownMenuAction) => !a.filter || a.filter(post.id);
    const visible = all.filter(shown);
    const single = visible.filter((a) => !a.subMenu?.length);
    const groups = visible.
        filter((a) => a.subMenu?.length).
        map((group) => ({group, items: (group.subMenu || []).filter(shown)})).
        filter(({items}) => items.length > 0);
    return {single, groups, any: single.length > 0 || groups.length > 0};
}

// ActionLabel is an app's text for its action. Apps that draw their own icon in it (Playbooks, GitHub, the AI chat)
// start it with an element without text: that one is hidden, the app's icon being drawn before the label, in grey.
function ActionLabel({children}: {children: React.ReactNode}) {
    const ref = useRef<HTMLSpanElement>(null);
    useLayoutEffect(() => {
        const first = ref.current?.firstChild;
        if (first instanceof Element && !first.textContent?.trim()) {
            first.classList.add(am('mi-own-ic'));
        }
    });
    return (
        <span
            ref={ref}
            className={am('mi-label')}
        >
            {children}
        </span>
    );
}

// PluginMenuActions are the entries apps add to a message's ⋯ menu, all under one "More actions" flyout: no app gets
// an entry of its own in the menu. The single actions come first, then each submenu (registerPostDropdownSubMenuAction,
// such as the AI actions) as a headed group. Each entry shows its app's icon, in grey.
export default function PluginMenuActions({post, label, menuLabel, run}: Props) {
    const {single, groups, any} = usePluginMenuActions(post);

    if (!any) {
        return null;
    }

    const item = (action: PostDropdownMenuAction) => (
        <button
            key={action.id}
            type='button'
            role='menuitem'
            onClick={run(() => action.action?.(post.id))}
        >
            <MenuAppIcon pluginId={action.pluginId}/>
            <ActionLabel>{action.text}</ActionLabel>
        </button>
    );

    return (
        <Flyout
            icon='plug'
            label={label}
            menuLabel={menuLabel}
        >
            {single.map(item)}
            {groups.map(({group, items}) => (
                <React.Fragment key={group.id}>
                    <MenuHeading>{group.text}</MenuHeading>
                    {items.map(item)}
                </React.Fragment>
            ))}
        </Flyout>
    );
}
