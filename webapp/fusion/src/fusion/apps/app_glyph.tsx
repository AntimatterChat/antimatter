// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useState} from 'react';
import {useSelector} from 'react-redux';

import {getAppBarPluginComponents, getChannelHeaderPluginComponents} from 'selectors/plugins';

import Icon from 'fusion/components/icon';
import type {IconName} from 'fusion/components/icon';
import {am} from 'fusion/utils/class_names';

import type {AppBarAction, ChannelHeaderButtonAction} from 'types/store/plugins';

import {getKnownApp} from './known_apps';

export type AppComponent = AppBarAction | ChannelHeaderButtonAction;

// AppGlyph is an app's icon: the mockup's, in its colour, for the apps it knows; else the plugin's own, or fallback
// for a plugin without one.
export function AppGlyph({component, fallback = 'plug'}: {component: AppComponent; fallback?: IconName}) {
    const [failed, setFailed] = useState(false);
    const known = getKnownApp(component.pluginId);
    const iconUrl = 'iconUrl' in component ? component.iconUrl : undefined;
    const icon = 'icon' in component ? component.icon : undefined;
    if (known) {
        return <Icon name={known.icon}/>;
    }
    if (iconUrl && !failed) {
        return (
            <img
                className={am('app-img')}
                src={iconUrl}
                alt=''
                onError={() => setFailed(true)}
            />
        );
    }
    return <>{icon || <Icon name={fallback}/>}</>;
}

// useAppComponent is the app bar (or channel header) entry of an app, where its icon comes from.
export function useAppComponent(pluginId: string): AppComponent | undefined {
    const appBarComponents = useSelector(getAppBarPluginComponents);
    const headerComponents = useSelector(getChannelHeaderPluginComponents);
    return appBarComponents.find((c) => c.pluginId === pluginId) || headerComponents.find((c) => c.pluginId === pluginId);
}

// MenuAppIcon is the icon of an app before one of its actions in a menu (a message's More actions), as Playbooks
// draws its own: the app's icon, in grey like the menu's own icons, so that no app stands out by its colours. The
// apps the mockup knows draw their Fusion icon; the others the icon they show in the app bar; an app without one a
// neutral app icon.
export function MenuAppIcon({pluginId}: {pluginId: string}) {
    const component = useAppComponent(pluginId);
    const known = getKnownApp(pluginId);
    let glyph: React.ReactNode;
    if (component) {
        glyph = (
            <AppGlyph
                component={component}
                fallback='apps'
            />
        );
    } else {
        glyph = <Icon name={known?.icon || 'apps'}/>;
    }
    return (
        <span
            className={am('mi-app')}
            data-plugin-id={pluginId}
            aria-hidden='true'
        >
            {glyph}
        </span>
    );
}
