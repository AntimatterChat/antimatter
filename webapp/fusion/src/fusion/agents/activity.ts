// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {useEffect, useSyncExternalStore} from 'react';

import type {WebSocketMessage} from '@mattermost/client';
import type {AgentActivity, AgentState} from '@mattermost/types/ai_agents';
import {AGENT_BUSY_STATES} from '@mattermost/types/ai_agents';
import type {UserProfile} from '@mattermost/types/users';

import {Client4} from 'mattermost-redux/client';

import webSocketClient from 'client/web_websocket_client';

// Agents' activity (Antimatter): what an agent is doing now (working on "Running the poll tests", waiting for
// someone...). It's kept here rather than in Redux, as the community attributes are: the activities of the agents the UI
// showed, asked for in batches as they come into view, and kept current by the server's agent_activity_changed events.
// A busy state holds until its expires_at: past it, the agent is idle again, without waiting for the server to say so.

export const AGENT_ACTIVITY_EVENT = 'agent_activity_changed';

// The batch the server answers at most, and how long the UI gathers agents before asking.
const BATCH = 200;
const GATHER_MS = 50;

const activities = new Map<string, AgentActivity>();
const requested = new Set<string>();
const queue = new Set<string>();
const listeners = new Set<() => void>();
let version = 0;
let listening = false;
let gatherTimer: ReturnType<typeof setTimeout> | undefined;

function changed() {
    version++;
    listeners.forEach((listener) => listener());
}

// isBusy tells whether a state is one an agent works in (and that expires).
export function isBusy(state: AgentState | undefined): boolean {
    return Boolean(state) && AGENT_BUSY_STATES.includes(state!);
}

// isWorking: thinking or working, shown in yellow. Waiting and blocked need someone, shown in the attention colour.
export function isWorking(state: AgentState | undefined): boolean {
    return state === 'thinking' || state === 'working';
}

export function needsSomeone(state: AgentState | undefined): boolean {
    return state === 'waiting' || state === 'blocked';
}

// effectiveActivity is the activity as it holds at now: an expired busy state is idle.
export function effectiveActivity(activity: AgentActivity | undefined, now = Date.now()): AgentActivity | undefined {
    if (!activity) {
        return undefined;
    }
    if (activity.expires_at && now >= activity.expires_at) {
        return {user_id: activity.user_id, state: 'idle', text: '', thread_post_id: '', update_at: activity.expires_at, expires_at: 0};
    }
    return activity;
}

// receivedActivity keeps an activity unless a newer one is known.
export function receivedActivity(activity: AgentActivity) {
    const known = activities.get(activity.user_id);
    if (known && known.update_at > activity.update_at) {
        return;
    }

    // The event never carries the thread: keep the one the server gave this person, for the same state.
    const thread = !activity.thread_post_id && known && known.state === activity.state && known.update_at === activity.update_at ? known.thread_post_id : activity.thread_post_id;
    activities.set(activity.user_id, {...activity, thread_post_id: thread || ''});
    requested.add(activity.user_id);
    changed();
}

function onMessage(message: WebSocketMessage) {
    const msg = message as unknown as {event: string; data?: {user_id?: string; activity?: AgentActivity}};
    if (msg.event !== AGENT_ACTIVITY_EVENT || !msg.data?.activity) {
        return;
    }
    receivedActivity({...msg.data.activity, user_id: msg.data.activity.user_id || msg.data.user_id || ''});
}

// The websocket reconnects without replaying what was missed: ask again for every agent shown.
function onReconnect() {
    requested.forEach((id) => queue.add(id));
    requested.clear();
    scheduleLoad();
}

function subscribe(listener: () => void) {
    if (!listening) {
        listening = true;
        webSocketClient.addMessageListener(onMessage);
        webSocketClient.addReconnectListener(onReconnect);
    }
    listeners.add(listener);
    return () => {
        listeners.delete(listener);
    };
}

function useVersion() {
    return useSyncExternalStore(subscribe, () => version);
}

async function loadQueued() {
    gatherTimer = undefined;
    const ids = [...queue];
    queue.clear();
    for (let i = 0; i < ids.length; i += BATCH) {
        const batch = ids.slice(i, i + BATCH);
        try {
            // eslint-disable-next-line no-await-in-loop
            const found = await Client4.getAgentActivitiesByIds(batch);
            for (const activity of found) {
                receivedActivity(activity);
            }
        } catch {
            // Without its activity, an agent shows its plain status; it's asked again after a reconnection.
        }
    }
}

function scheduleLoad() {
    if (!gatherTimer && queue.size) {
        gatherTimer = setTimeout(loadQueued, GATHER_MS);
    }
}

// requestAgentActivity asks for an agent's activity once, gathered with the others shown at the same time.
export function requestAgentActivity(userId: string) {
    if (!userId || requested.has(userId)) {
        return;
    }
    requested.add(userId);
    queue.add(userId);
    scheduleLoad();
}

// useAgentActivity gives an agent's activity as it holds now (undefined for someone who isn't an agent, or until it's
// known), and renders again when a busy state expires.
export function useAgentActivity(user: UserProfile | undefined): AgentActivity | undefined {
    useVersion();
    const userId = user?.is_agent ? user.id : '';
    useEffect(() => {
        if (userId) {
            requestAgentActivity(userId);
        }
    }, [userId]);

    const stored = userId ? activities.get(userId) : undefined;
    const expiresAt = stored && isBusy(stored.state) ? stored.expires_at : 0;
    useEffect(() => {
        if (!expiresAt) {
            return undefined;
        }
        const timer = setTimeout(changed, Math.max(0, expiresAt - Date.now()) + 10);
        return () => clearTimeout(timer);
    }, [expiresAt]);

    return effectiveActivity(stored);
}

// resetAgentActivities forgets everything, for the tests.
export function resetAgentActivities() {
    activities.clear();
    requested.clear();
    queue.clear();
    if (gatherTimer) {
        clearTimeout(gatherTimer);
        gatherTimer = undefined;
    }
    listening = false;
    changed();
}
