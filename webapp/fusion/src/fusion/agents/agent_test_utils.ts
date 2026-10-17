// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {AgentActivity, AgentState} from '@mattermost/types/ai_agents';

// activity makes an agent's activity for the tests: busy states hold for a minute from now unless told otherwise.
export function activity(userId: string, state: AgentState, text = '', patch: Partial<AgentActivity> = {}): AgentActivity {
    const now = Date.now();
    const busy = state !== 'idle' && state !== 'offline';
    return {user_id: userId, state, text, thread_post_id: '', update_at: now, expires_at: busy ? now + 60000 : 0, ...patch};
}

// The layer the Fusion popovers and cards open in.
export function ensureLayer() {
    if (!document.getElementById('am-layer')) {
        const layer = document.createElement('div');
        layer.id = 'am-layer';
        document.body.appendChild(layer);
    }
}
