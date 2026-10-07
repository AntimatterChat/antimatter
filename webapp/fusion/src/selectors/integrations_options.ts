// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {createPluginErrorLog} from 'utils/plugin_error_log';

import type {GlobalState} from 'types/store';
import type {IntegrationsOptionRegistration} from 'types/store/plugins';

const shouldRenderErrorLog = createPluginErrorLog('IntegrationsOption', {
    subject: 'shouldRender',
    outcome: 'hiding the entry',
});

export const clearLoggedIntegrationsOptionErrors = shouldRenderErrorLog.clear;

const EMPTY: IntegrationsOptionRegistration[] = [];

/**
 * The plugin entries of the integrations pages: the registrations whose shouldRender returns
 * true, in registration order. A throwing shouldRender hides its entry. Compare results with
 * shallowEqual: the array is new whenever some entry is hidden.
 */
export function getIntegrationsOptions(state: GlobalState): IntegrationsOptionRegistration[] {
    const regs = state.plugins.components.IntegrationsOption;
    if (!regs?.length) {
        return EMPTY;
    }
    const visible = regs.filter((reg) => {
        if (!reg.shouldRender) {
            return true;
        }
        try {
            return reg.shouldRender(state) === true;
        } catch (err) {
            shouldRenderErrorLog.logOnce(reg.pluginId, err);
            return false;
        }
    });
    return visible.length === regs.length ? regs : visible;
}
