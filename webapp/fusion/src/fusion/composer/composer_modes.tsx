// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useIntl} from 'react-intl';
import type {IntlShape} from 'react-intl';
import {shallowEqual, useSelector} from 'react-redux';

import {getComposerModes} from 'selectors/composer_modes';

import Icon, {isIconName} from 'fusion/components/icon';
import {MenuItem} from 'fusion/components/menu';
import {am} from 'fusion/utils/class_names';
import PluggableErrorBoundary from 'plugins/pluggable/error_boundary';

import type {GlobalState} from 'types/store';
import type {ComposerModeProps, ComposerModeRegistration} from 'types/store/plugins';

// Composer modes are other ways of writing a message than plain text, which apps add (registerComposerMode): a poll,
// for one, is written in the composer, its question the composer's text and its options the mode's fields under it.
// The "+" menu offers them; in a mode, a bar names it with Cancel (back to a plain message), and Send runs the mode's
// submit.

// An app opens its mode from elsewhere (e.g. a slash command without arguments) with this window event.
export const COMPOSER_MODE_EVENT = 'am-composer-mode';
export type ComposerModeEventDetail = {id: string; channelId: string; rootId?: string};

// useComposerModes are the modes offered in a composer.
export function useComposerModes(channelId: string, rootId: string): ComposerModeRegistration[] {
    return useSelector((state: GlobalState) => getComposerModes(state, {channelId, rootId: rootId || undefined}), shallowEqual);
}

// modeText reads a mode's placeholder or send label: a string, or a function of the intl.
export function modeText(value: ComposerModeRegistration['placeholder'], intl: IntlShape): string | undefined {
    if (typeof value === 'function') {
        try {
            return value(intl) || undefined;
        } catch {
            return undefined;
        }
    }
    return value || undefined;
}

const iconOf = (mode: ComposerModeRegistration) => (mode.fusionIcon && isIconName(mode.fusionIcon) ? mode.fusionIcon : undefined);

// ComposerModeItems are the modes in the "+" menu.
export function ComposerModeItems({modes, onPick}: {modes: ComposerModeRegistration[]; onPick: (id: string) => void}) {
    return (
        <>
            {modes.map((mode) => (
                <MenuItem
                    key={mode.id}
                    icon={iconOf(mode)}
                    label={mode.label}
                    onClick={() => onPick(mode.id)}
                />
            ))}
        </>
    );
}

// ComposerModeBar names the mode the composer is in, at its top, with the way back to a plain message.
export function ComposerModeBar({mode, onCancel}: {mode: ComposerModeRegistration; onCancel: () => void}) {
    const {formatMessage} = useIntl();
    const icon = iconOf(mode);
    return (
        <div className={am('cmode-bar')}>
            {icon && (
                <Icon
                    name={icon}
                    size='sm'
                />
            )}
            <span className={am('cmode-name')}>{mode.label}</span>
            <button
                type='button'
                className={am('cmode-cancel')}
                aria-label={formatMessage({id: 'fusion.composer.modeCancel', defaultMessage: 'Cancel: back to a plain message'})}
                onClick={onCancel}
            >
                {formatMessage({id: 'fusion.composer.modeCancelShort', defaultMessage: 'Cancel'})}
            </button>
        </div>
    );
}

// ComposerModeBody is the mode's fields, under the text box: an app's component, kept from breaking the composer.
export function ComposerModeBody({mode, ...props}: ComposerModeProps & {mode: ComposerModeRegistration}) {
    const Component = mode.component;
    return (
        <div className={am('cmode-body')}>
            <PluggableErrorBoundary pluginId={mode.pluginId}>
                <Component {...props}/>
            </PluggableErrorBoundary>
        </div>
    );
}
