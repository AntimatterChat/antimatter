// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {createPluginErrorLog} from 'utils/plugin_error_log';

import type {GlobalState} from 'types/store';
import type {EmojiPickerTabContext, EmojiPickerTabRegistration} from 'types/store/plugins';

const shouldRenderErrorLog = createPluginErrorLog('EmojiPickerTab', {
    subject: 'shouldRender',
    outcome: 'hiding the tab',
});

export const clearLoggedEmojiPickerTabErrors = shouldRenderErrorLog.clear;

const EMPTY: EmojiPickerTabRegistration[] = [];

function renders(state: GlobalState, reg: EmojiPickerTabRegistration, ctx: EmojiPickerTabContext) {
    if (!reg.shouldRender) {
        return true;
    }
    try {
        return reg.shouldRender(state, ctx) === true;
    } catch (err) {
        shouldRenderErrorLog.logOnce(reg.pluginId, err);
        return false;
    }
}

/**
 * The plugin tabs of the emoji picker of this channel's or thread's message box: the
 * registrations whose shouldRender returns true, sorted by order, then alphabetically by
 * pluginId and in registration order. A throwing shouldRender hides its tab. Compare results
 * with shallowEqual: the array is new on every call that finds tabs.
 */
export function getEmojiPickerTabs(
    state: GlobalState,
    ctx: EmojiPickerTabContext,
): EmojiPickerTabRegistration[] {
    const regs = state.plugins.components.EmojiPickerTab;
    if (!ctx.channelId || !regs?.length) {
        return EMPTY;
    }
    const visible = regs.filter((reg) => renders(state, reg, ctx));

    // Array.prototype.sort is stable: equal orders keep the registrations' order.
    return visible.sort((a, b) => (a.order ?? 0) - (b.order ?? 0));
}

/**
 * Whether a plugin tab replaces the core GIF picker (Giphy) in this channel's or thread's emoji
 * picker: a registration with replacesGifPicker whose shouldRender returns true. Message boxes
 * that don't show plugin tabs (e.g. while editing) ask too, so the GIF picker is hidden everywhere
 * the plugin replaces it.
 */
export function isGifPickerReplaced(state: GlobalState, ctx: EmojiPickerTabContext): boolean {
    const regs = state.plugins.components.EmojiPickerTab;
    if (!regs?.length) {
        return false;
    }
    return regs.some((reg) => reg.replacesGifPicker && renders(state, reg, ctx));
}
