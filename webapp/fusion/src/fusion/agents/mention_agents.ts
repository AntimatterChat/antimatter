// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {useEffect, useSyncExternalStore} from 'react';

import type {WebSocketMessage} from '@mattermost/client';
import type {Agent} from '@mattermost/types/agents';

import {Client4} from 'mattermost-redux/client';

import webSocketClient from 'client/web_websocket_client';

import {AGENT_ACTIVITY_EVENT} from './activity';

// The agents a person may talk to (Antimatter), for the composer's @ suggestions: a server-wide agent belongs to no
// community, so it's never among a channel's or a community's members the usual suggestions come from. The list comes
// from GET /api/v4/agents once per session, and again when an agent changes or the websocket reconnects.

let agents: Agent[] | undefined;
let loading = false;
let stale = false;
let listening = false;
let version = 0;
const listeners = new Set<() => void>();

function changed() {
    version++;
    listeners.forEach((listener) => listener());
}

function markStale() {
    stale = true;
    changed();
}

// A new agent says it's idle when it's made: an activity of an agent not listed means the list changed.
function onMessage(message: WebSocketMessage) {
    const msg = message as unknown as {event: string; data?: {user_id?: string}};
    if (agents && msg.event === AGENT_ACTIVITY_EVENT && msg.data?.user_id && !agents.some((a) => a.id === msg.data!.user_id)) {
        markStale();
    }
}

function subscribe(listener: () => void) {
    if (!listening) {
        listening = true;
        webSocketClient.addMessageListener(onMessage);
        webSocketClient.addReconnectListener(markStale);
    }
    listeners.add(listener);
    return () => {
        listeners.delete(listener);
    };
}

async function load() {
    if (loading) {
        return;
    }
    loading = true;
    stale = false;
    try {
        agents = await Client4.getAgents();
    } catch {
        agents = agents || [];
    }
    loading = false;
    changed();
}

// useMentionableAgents gives the agents the person may talk to (empty until they're known).
export function useMentionableAgents(): Agent[] {
    useSyncExternalStore(subscribe, () => version);
    useEffect(() => {
        if (!agents || stale) {
            load();
        }
    }, [version]); // eslint-disable-line react-hooks/exhaustive-deps
    return agents || NO_AGENTS;
}

const NO_AGENTS: Agent[] = [];

export type AgentMentionItem = {
    id: string;
    username: string;
    first_name: string;
    last_name: string;
    nickname: string;
    is_bot: true;
    is_agent: true;
};

// agentMentionRows adds to the @ suggestions the agents whose username or name starts with what's typed after the @
// (matchedPretext, as "@co"), leaving out those already suggested.
export function agentMentionRows<R extends {term: string; item: unknown}>(rows: R[], matchedPretext: string, list: Agent[]): Array<R | {term: string; item: AgentMentionItem}> {
    if (!(/^[@＠]/).test(matchedPretext) || !list.length) {
        return rows;
    }
    const prefix = matchedPretext.slice(1).toLowerCase().trim();
    const shown = new Set(rows.map((r) => r.term.toLowerCase()));
    const extra = list.
        filter((a) => !shown.has(`@${a.username}`.toLowerCase())).
        filter((a) => a.username.toLowerCase().startsWith(prefix) || (a.displayName || '').toLowerCase().split(/\s+/).some((word) => word.startsWith(prefix))).
        map((a) => ({
            term: `@${a.username}`,
            item: {id: a.id, username: a.username, first_name: a.displayName || '', last_name: '', nickname: '', is_bot: true as const, is_agent: true as const},
        }));
    return [...rows, ...extra];
}

// resetMentionableAgents forgets the list, for the tests.
export function resetMentionableAgents() {
    agents = undefined;
    loading = false;
    stale = false;
    listening = false;
    changed();
}
