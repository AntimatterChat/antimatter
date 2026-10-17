// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useLayoutEffect, useRef, useState} from 'react';
import {useIntl} from 'react-intl';
import {useSelector} from 'react-redux';

import type {Post} from '@mattermost/types/posts';

import {getConfig} from 'mattermost-redux/selectors/entities/general';

import {MenuAppIcon} from 'fusion/apps/app_glyph';
import Icon from 'fusion/components/icon';
import {Flyout, MenuHeading, MenuSeparator} from 'fusion/components/menu';
import {useToast} from 'fusion/shell/toast_context';
import {am} from 'fusion/utils/class_names';
import {getImageSrc, isSystemMessage} from 'utils/post_utils';

import type {GlobalState} from 'types/store';
import type {PostDropdownMenuAction} from 'types/store/plugins';

import {messageActionIconSrc, MessageActionError, runMessageAction, useMessageActions} from './message_actions';
import type {MessageAction} from './message_actions';

const NO_ACTIONS: PostDropdownMenuAction[] = [];

type Props = {
    post: Post;

    // The flyout holding the apps' and integrations' actions ("More actions"), and its menu's accessible name.
    label: string;
    menuLabel: string;

    // Runs an action, then closes the menu holding the entries.
    run: (action: () => void) => () => void;
};

// Whether integrations may be sent the message: one the server has, and not a system message.
function canSendToIntegrations(post: Post): boolean {
    return Boolean(post.id) && !isSystemMessage(post) && !post.failed && post.pending_post_id !== post.id;
}

// usePluginMenuActions are the actions apps and integrations add to the message: the apps' single actions, their
// submenus (with their entries), and the integrations' message actions.
export function usePluginMenuActions(post: Post) {
    const all = useSelector((state: GlobalState) => state.plugins.components.PostDropdownMenu) || NO_ACTIONS;
    const outgoingWebhooks = useSelector((state: GlobalState) => getConfig(state).EnableOutgoingWebhooks === 'true');
    const integrations = useMessageActions(post.channel_id, outgoingWebhooks && canSendToIntegrations(post));
    const shown = (a: PostDropdownMenuAction) => !a.filter || a.filter(post.id);
    const visible = all.filter(shown);
    const single = visible.filter((a) => !a.subMenu?.length);
    const groups = visible.
        filter((a) => a.subMenu?.length).
        map((group) => ({group, items: (group.subMenu || []).filter(shown)})).
        filter(({items}) => items.length > 0);
    return {single, groups, integrations, any: single.length > 0 || groups.length > 0 || integrations.length > 0};
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

// IntegrationIcon is an integration's icon, in grey, or a neutral one when it has none (or it doesn't load).
function IntegrationIcon({action}: {action: MessageAction}) {
    const hasImageProxy = useSelector((state: GlobalState) => getConfig(state).HasImageProxy === 'true');
    const [failed, setFailed] = useState(false);
    const src = messageActionIconSrc(action.icon_url, (url) => getImageSrc(url, hasImageProxy));
    return (
        <span
            className={am('mi-app')}
            aria-hidden='true'
        >
            {src && !failed ? (
                <img
                    src={src}
                    alt=''
                    onError={() => setFailed(true)}
                />
            ) : (
                <Icon name='plug'/>
            )}
        </span>
    );
}

// PluginMenuActions are the entries apps and integrations add to a message's ⋯ menu, all under one "More actions"
// flyout: no app gets an entry of its own in the menu. The apps' single actions come first, then the entries of each
// submenu (registerPostDropdownSubMenuAction, such as the AI actions), set apart by a line but without the submenu's
// name, then the integrations' message actions under "Integrations". Each entry shows its app's or integration's
// icon, in grey.
export default function PluginMenuActions({post, label, menuLabel, run}: Props) {
    const {formatMessage} = useIntl();
    const toast = useToast();
    const {single, groups, integrations, any} = usePluginMenuActions(post);

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

    // Sending the message to an integration: a short notice tells it was sent (or the integration's own text, or
    // why it wasn't).
    const send = async (action: MessageAction) => {
        try {
            const result = await runMessageAction(post.id, action.id);
            toast(result.ephemeral_text || formatMessage({id: 'fusion.messageActions.sent', defaultMessage: 'Sent to {name}'}, {name: action.display_name}));
        } catch (e) {
            const message = e instanceof MessageActionError ? e.message : '';
            toast(message || formatMessage({id: 'fusion.messageActions.failed', defaultMessage: 'Couldn\'t send to {name}'}, {name: action.display_name}));
        }
    };

    return (
        <Flyout
            icon='plug'
            label={label}
            menuLabel={menuLabel}
        >
            {single.map(item)}

            {/* A submenu's entries (the AI actions) without its name as a heading: their app's icon and the menu
                say where they come from. A line sets them apart. */}
            {groups.map(({group, items}, i) => (
                <React.Fragment key={group.id}>
                    {(single.length > 0 || i > 0) && <MenuSeparator/>}
                    {items.map(item)}
                </React.Fragment>
            ))}
            {integrations.length > 0 && (
                <>
                    <MenuHeading>{formatMessage({id: 'fusion.messageActions.integrations', defaultMessage: 'Integrations'})}</MenuHeading>
                    {integrations.map((action) => (
                        <button
                            key={action.id}
                            type='button'
                            role='menuitem'
                            title={action.description || undefined}
                            onClick={run(() => send(action))}
                        >
                            <IntegrationIcon action={action}/>
                            <span className={am('mi-label')}>{action.display_name}</span>
                        </button>
                    ))}
                </>
            )}
        </Flyout>
    );
}
