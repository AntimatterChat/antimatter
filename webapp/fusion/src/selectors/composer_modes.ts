// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {createPluginErrorLog} from 'utils/plugin_error_log';

import type {GlobalState} from 'types/store';
import type {ComposerMenuItemContext, ComposerModeRegistration} from 'types/store/plugins';

const shouldRenderErrorLog = createPluginErrorLog('ComposerMode', {
    subject: 'shouldRender',
    outcome: 'hiding the mode',
});

export const clearLoggedComposerModeErrors = shouldRenderErrorLog.clear;

const EMPTY: ComposerModeRegistration[] = [];

/**
 * The composer modes plugins offer in this channel or thread (registerComposerMode): the registrations whose
 * shouldRender returns true, in registration order (alphabetical pluginId first). A throwing shouldRender hides its
 * mode. Compare results with shallowEqual: the array is new whenever some mode is hidden.
 */
export function getComposerModes(state: GlobalState, ctx: ComposerMenuItemContext): ComposerModeRegistration[] {
    const regs = state.plugins.components.ComposerMode;
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
