// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useIntl} from 'react-intl';
import {useSelector} from 'react-redux';

import {AppGlyph, useAppComponent} from 'fusion/apps/app_glyph';
import {getKnownApp} from 'fusion/apps/known_apps';
import Icon from 'fusion/components/icon';
import {am} from 'fusion/utils/class_names';

import type {GlobalState} from 'types/store';

import {RestoreWidthButton} from './panel_width';

// The header of an app's side panel is the same for every app, drawn by the web app alone: the app's icon and name,
// then the panel's own buttons (restore its size once resized, open it in a new window, close it). An app's controls
// (its views, its + buttons, the open item's actions) are in its panel, under the header: there is no slot or portal
// through which an app puts anything in it, and the title an app registers only names it when it is plain text.

// The text of a label, title or tooltip an app gave: React content (an element, a formatted message) is not text.
function text(value: unknown): string | undefined {
    return typeof value === 'string' && value.trim() ? value.trim() : undefined;
}

/**
 * useAppName is the name the header of an app's panel shows: the title the app registered for its panel when it is
 * text, else the label of its icon in the app rail (or channel header), else the name in its manifest, else its id.
 */
export function useAppName(pluginId: string, title?: React.ReactNode): string {
    const component = useAppComponent(pluginId);
    const manifestName = useSelector((state: GlobalState) => state.plugins.plugins[pluginId]?.name);
    return text(title) ||
        text(component?.tooltipText) ||
        text(component && 'dropdownText' in component ? component.dropdownText : undefined) ||
        text(manifestName) ||
        pluginId;
}

// AppPanelIcon is the icon of the app whose panel is open, before its name, as in the app rail: an app without one
// in the rail gets a neutral app icon.
export function AppPanelIcon({pluginId}: {pluginId: string}) {
    const component = useAppComponent(pluginId);
    return (
        <span
            className={am('rhs-app-ic', getKnownApp(pluginId)?.tone)}
            aria-hidden='true'
        >
            {component ? <AppGlyph component={component}/> : <Icon name='plug'/>}
        </span>
    );
}

export type AppPanelHeaderProps = {
    pluginId: string;

    // The title the app registered for its panel: its name when it is text, ignored otherwise.
    title?: React.ReactNode;

    // Whether the person resized the panel away from its default width, and how to give it back.
    sized?: boolean;
    onRestoreSize?: () => void;

    // Each button shows when its action is given: a panel in its own window has neither.
    onNewWindow?: () => void;
    onClose?: () => void;
};

/**
 * AppPanelHeader is the header of an app's side panel, left to right: the app's icon and name, "Restore the panel's
 * size" (only once resized), "Open in a new window" and "Close".
 */
export default function AppPanelHeader({pluginId, title, sized = false, onRestoreSize, onNewWindow, onClose}: AppPanelHeaderProps) {
    const {formatMessage} = useIntl();
    const name = useAppName(pluginId, title);
    const newWindow = formatMessage({id: 'fusion.rhs.newWindow', defaultMessage: 'Open in a new window'});
    const close = formatMessage({id: 'fusion.rhs.close', defaultMessage: 'Close'});

    return (
        <div
            className={am('rhs-head', 'app-head')}
            data-testid='app-panel-header'
            data-plugin-id={pluginId}
        >
            <AppPanelIcon pluginId={pluginId}/>
            <h3
                id='rhsPanelTitle'
                className={am('app-name', 'grow')}
            >
                <span className={am('tname')}>{name}</span>
            </h3>
            {onRestoreSize && (
                <RestoreWidthButton
                    sized={sized}
                    onRestore={onRestoreSize}
                />
            )}
            {onNewWindow && (
                <button
                    type='button'
                    className={am('icon-btn', 'rhs-new-window')}
                    data-am-tip={newWindow}
                    aria-label={newWindow}
                    onClick={onNewWindow}
                >
                    <Icon
                        name='popout'
                        size='sm'
                    />
                </button>
            )}
            {onClose && (
                <button
                    type='button'
                    id='searchResultsCloseButton'
                    className={am('icon-btn', 'rhs-close')}
                    data-am-tip={close}
                    aria-label={close}
                    onClick={onClose}
                >
                    <Icon name='x'/>
                </button>
            )}
        </div>
    );
}
