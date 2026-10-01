// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {createPluginErrorLog} from 'utils/plugin_error_log';

import type {GlobalState} from 'types/store';
import type {ComposerMenuItemContext, ComposerMenuItemRegistration} from 'types/store/plugins';

const shouldRenderErrorLog = createPluginErrorLog('ComposerMenuItem', {
    subject: 'shouldRender',
    outcome: 'hiding the item',
});

export const clearLoggedComposerMenuItemErrors = shouldRenderErrorLog.clear;

const EMPTY: ComposerMenuItemRegistration[] = [];

/**
 * The plugin items of the message box's "+" menu for this channel or thread: the registrations
 * whose shouldRender returns true, in registration order (alphabetical pluginId first). A
 * throwing shouldRender hides its item. Compare results with shallowEqual: the array is new
 * whenever some item is hidden.
 */
export function getComposerMenuItems(
    state: GlobalState,
    ctx: ComposerMenuItemContext,
): ComposerMenuItemRegistration[] {
    const regs = state.plugins.components.ComposerMenuItem;
    if (!ctx.channelId || !regs?.length) {
        return EMPTY;
    }
    const visible = regs.filter((reg) => {
        if (!reg.shouldRender) {
            return true;
        }
        try {
            return reg.shouldRender(state, ctx) === true;
        } catch (err) {
            shouldRenderErrorLog.logOnce(reg.pluginId, err);
            return false;
        }
    });
    return visible.length === regs.length ? regs : visible;
}
