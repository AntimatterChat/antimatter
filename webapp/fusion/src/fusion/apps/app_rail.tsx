// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useRef, useState} from 'react';
import {useIntl} from 'react-intl';
import {useSelector} from 'react-redux';

import {getCurrentChannel, getMyCurrentChannelMembership} from 'mattermost-redux/selectors/entities/channels';
import {getCurrentUserId} from 'mattermost-redux/selectors/entities/users';

import {getAppBarPluginComponents, getChannelHeaderPluginComponents, shouldShowAppBar} from 'selectors/plugins';
import {getActiveRhsComponent} from 'selectors/rhs';

import Avatar from 'fusion/components/avatar';
import Icon from 'fusion/components/icon';
import AboutPopover from 'fusion/popovers/about_popover';
import StatusPopover from 'fusion/popovers/status_popover';
import {am} from 'fusion/utils/class_names';

import type {AppBarAction, ChannelHeaderButtonAction} from 'types/store/plugins';

function AppIcon({component}: {component: AppBarAction | ChannelHeaderButtonAction}) {
    const channel = useSelector(getCurrentChannel);
    const member = useSelector(getMyCurrentChannelMembership);
    const active = useSelector(getActiveRhsComponent);
    const [failed, setFailed] = useState(false);
    const iconUrl = 'iconUrl' in component ? component.iconUrl : undefined;
    const icon = 'icon' in component ? component.icon : undefined;
    const rhsComponentId = 'rhsComponentId' in component ? component.rhsComponentId : undefined;
    const label = String(('tooltipText' in component && component.tooltipText) || ('dropdownText' in component && component.dropdownText) || component.pluginId);
    const on = rhsComponentId ? active?.id === rhsComponentId : active?.pluginId === component.pluginId;

    return (
        <button
            className={am('app-ic', {on})}
            title={label}
            aria-label={label}
            aria-pressed={on}
            onClick={() => {
                if (rhsComponentId) {
                    (component as AppBarAction & {action: () => void}).action();
                } else {
                    component.action?.(channel, member);
                }
            }}
        >
            {iconUrl && !failed ? (
                <img
                    className={am('app-img')}
                    src={iconUrl}
                    alt=''
                    onError={() => setFailed(true)}
                />
            ) : (icon || <Icon name='plug'/>)}
        </button>
    );
}

// AppRail is the right-hand column: your avatar on top, the apps plugins add, and About at the bottom.
export default function AppRail() {
    const {formatMessage} = useIntl();
    const me = useSelector(getCurrentUserId);
    const enabled = useSelector(shouldShowAppBar);
    const appBarComponents = useSelector(getAppBarPluginComponents);
    const headerComponents = useSelector(getChannelHeaderPluginComponents);
    const meRef = useRef<HTMLButtonElement>(null);
    const aboutRef = useRef<HTMLButtonElement>(null);
    const [open, setOpen] = useState<'status' | 'about' | null>(null);

    return (
        <div className={am('appbar')}>
            <div className={am('me-slot')}>
                <button
                    ref={meRef}
                    className={am('me-tile', 'me-btn')}
                    aria-haspopup='dialog'
                    aria-expanded={open === 'status'}
                    aria-label={formatMessage({id: 'fusion.apps.me', defaultMessage: 'Your status, profile and settings'})}
                    onClick={() => setOpen(open === 'status' ? null : 'status')}
                >
                    <Avatar
                        userId={me}
                        size='me'
                        status={true}
                    />
                </button>
            </div>
            {enabled && [...appBarComponents, ...headerComponents].map((c) => (
                <AppIcon
                    key={c.id}
                    component={c}
                />
            ))}
            <span className={am('grow')}/>
            <button
                ref={aboutRef}
                className={am('app-ic', 'about-btn')}
                title={formatMessage({id: 'fusion.apps.about', defaultMessage: 'About Antimatter'})}
                aria-label={formatMessage({id: 'fusion.apps.about', defaultMessage: 'About Antimatter'})}
                aria-haspopup='dialog'
                onClick={() => setOpen(open === 'about' ? null : 'about')}
            >
                <svg
                    className={am('ic')}
                    viewBox='0 0 64 64'
                    aria-hidden='true'
                >
                    <use href='#am-i-mark'/>
                </svg>
            </button>
            {open === 'status' && (
                <StatusPopover
                    anchor={meRef.current}
                    onClose={() => setOpen(null)}
                />
            )}
            {open === 'about' && (
                <AboutPopover
                    anchor={aboutRef.current}
                    onClose={() => setOpen(null)}
                />
            )}
        </div>
    );
}
