// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {OutgoingWebhook} from '@mattermost/types/integrations';

import {Client4} from 'mattermost-redux/client';

// uploadOutgoingHookIcon uploads the icon of an outgoing webhook (Antimatter): its message action's icon in the
// message menu. The server stores it as a 128 px PNG and points the webhook's icon_url at it.
export async function uploadOutgoingHookIcon(hookId: string, file: File): Promise<OutgoingWebhook> {
    const body = new FormData();
    body.append('image', file);
    const res = await fetch(`${Client4.getOutgoingHookRoute(hookId)}/icon`, Client4.getOptions({method: 'post', body}));
    if (!res.ok) {
        let message = '';
        try {
            message = ((await res.json()) as {message?: string}).message || '';
        } catch {
            // Not a JSON error.
        }
        throw new Error(message || `status ${res.status}`);
    }
    return res.json();
}
