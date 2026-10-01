// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {createPluginErrorLog} from 'utils/plugin_error_log';

import type {GlobalState} from 'types/store';
import type {ChannelViewPanelRegistration} from 'types/store/plugins';

const matcherErrorLog = createPluginErrorLog('ChannelViewPanel');

export const clearLoggedChannelViewPanelErrors = matcherErrorLog.clear;

/** First channel view panel registration whose matcher returns === true for this channel, or null. */
export function getChannelViewPanel(
    state: GlobalState,
    channelId: string,
): ChannelViewPanelRegistration | null {
    const regs = state.plugins.components.ChannelViewPanel;
    if (!channelId || !regs?.length) {
        return null;
    }
    const channel = state.entities?.channels?.channels?.[channelId];
    if (!channel) {
        return null;
    }
    for (const reg of regs) {
        try {
            if (reg.matcher(state, channel) === true) {
                return reg;
            }
        } catch (err) {
            matcherErrorLog.logOnce(reg.pluginId, err);
        }
    }
    return null;
}
