// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {useEffect, useState} from 'react';

import {Client4} from 'mattermost-redux/client';

// Message actions are integrations in a message's More actions: outgoing webhooks set to trigger "from the message
// menu". Picking one sends the message to the integration. The server lists the ones the person may pick in a
// channel; they are kept for a minute per channel, so that opening menus doesn't ask each time.

export type MessageAction = {
    id: string;
    team_id: string;
    channel_id: string;
    display_name: string;
    description: string;

    // The uploaded icon (a path of the server) or an http(s) image URL; empty for none.
    icon_url: string;
};

export type MessageActionResult = {
    ephemeral_text?: string;
};

const FRESH_MS = 60 * 1000;
const NONE: MessageAction[] = [];

type Entry = {at: number; actions: MessageAction[]; loading?: Promise<MessageAction[]>};
const cache = new Map<string, Entry>();

// MessageActionError is a refusal of the server, with its message for the person.
export class MessageActionError extends Error {
    status: number;
    constructor(message: string, status: number) {
        super(message);
        this.status = status;
    }
}

async function failure(res: Response): Promise<MessageActionError> {
    let message = '';
    try {
        message = ((await res.json()) as {message?: string}).message || '';
    } catch {
        // Not a JSON error: no message for the person.
    }
    return new MessageActionError(message, res.status);
}

export async function fetchMessageActions(channelId: string): Promise<MessageAction[]> {
    const res = await fetch(`${Client4.getChannelRoute(channelId)}/message_actions`, Client4.getOptions({method: 'get'}));
    if (!res.ok) {
        throw await failure(res);
    }
    const actions = await res.json();
    return Array.isArray(actions) ? actions : [];
}

// loadMessageActions is the channel's message actions, from the cache while fresh.
export function loadMessageActions(channelId: string): Promise<MessageAction[]> {
    const entry = cache.get(channelId);
    if (entry?.loading) {
        return entry.loading;
    }
    if (entry && Date.now() - entry.at < FRESH_MS) {
        return Promise.resolve(entry.actions);
    }
    const loading = fetchMessageActions(channelId).
        catch(() => NONE).
        then((actions) => {
            cache.set(channelId, {at: Date.now(), actions});
            return actions;
        });
    cache.set(channelId, {at: entry?.at ?? 0, actions: entry?.actions ?? NONE, loading});
    return loading;
}

// useMessageActions is the message actions of a channel: what is known at once, then the server's list.
export function useMessageActions(channelId: string, enabled: boolean): MessageAction[] {
    const [actions, setActions] = useState<MessageAction[]>(() => (enabled && channelId ? cache.get(channelId)?.actions ?? NONE : NONE));
    useEffect(() => {
        if (!enabled || !channelId) {
            setActions(NONE);
            return undefined;
        }
        let live = true;
        loadMessageActions(channelId).then((loaded) => {
            if (live) {
                setActions(loaded);
            }
        });
        return () => {
            live = false;
        };
    }, [channelId, enabled]);
    return actions;
}

// runMessageAction sends a message to an integration, as the person.
export async function runMessageAction(postId: string, hookId: string): Promise<MessageActionResult> {
    const res = await fetch(`${Client4.getPostRoute(postId)}/message_actions/${encodeURIComponent(hookId)}`, Client4.getOptions({method: 'post'}));
    if (!res.ok) {
        throw await failure(res);
    }
    try {
        return (await res.json()) as MessageActionResult;
    } catch {
        return {};
    }
}

// messageActionIconSrc is where the menu loads an action's icon from: the server's own path for an uploaded icon
// (under the server's subpath), else the URL through the image proxy when there is one.
export function messageActionIconSrc(iconUrl: string, imageProxy: (url: string) => string): string {
    if (!iconUrl) {
        return '';
    }
    if (iconUrl.startsWith('/api/v4/hooks/outgoing/')) {
        return Client4.getUrl() + iconUrl;
    }
    if ((/^https?:\/\//i).test(iconUrl)) {
        return imageProxy(iconUrl);
    }
    return '';
}

// clearMessageActions forgets the lists (for tests, and when an integration changes).
export function clearMessageActions() {
    cache.clear();
}
