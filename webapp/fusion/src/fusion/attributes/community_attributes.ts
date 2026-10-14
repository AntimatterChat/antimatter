// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {useEffect, useSyncExternalStore} from 'react';

import type {WebSocketMessage} from '@mattermost/client';
import type {CommunityAttributeField, CommunityAttributeFieldPatch, UserAttributeValue} from '@mattermost/types/properties_user';

import {Client4} from 'mattermost-redux/client';

import webSocketClient from 'client/web_websocket_client';

// A community's attributes (the user attributes its admins define, as "Subcontractor" or "Site") and the values of
// the members the UI showed, as the server lets this person see them. They're kept here rather than in Redux; when the
// server says they changed, they're marked stale (and still shown) until whoever shows them has asked again.

export type AttributeValues = Record<string, UserAttributeValue>;

type Community = {
    fields?: CommunityAttributeField[];
    fieldsStale?: boolean;
    fieldsLoading?: boolean;

    // By user: their values, once loaded.
    values: Map<string, AttributeValues>;
    stale: Set<string>;
    loading: Set<string>;
};

const communities = new Map<string, Community>();
const listeners = new Set<() => void>();
let version = 0;
let listening = false;

const EVENT = 'community_attributes_updated';
const BATCH = 200;
const NO_VALUES: AttributeValues = {};

function community(teamId: string): Community {
    let c = communities.get(teamId);
    if (!c) {
        c = {values: new Map(), stale: new Set(), loading: new Set()};
        communities.set(teamId, c);
    }
    return c;
}

function changed() {
    version++;
    listeners.forEach((listener) => listener());
}

// The server tells a community's members when its attributes change, or someone's values.
function onMessage(message: WebSocketMessage) {
    const msg = message as unknown as {event: string; data: {team_id: string; user_id?: string}};
    if (msg.event !== EVENT) {
        return;
    }
    const {team_id: teamId, user_id: userId} = msg.data;
    const c = communities.get(teamId);
    if (!c) {
        return;
    }
    if (userId) {
        c.stale.add(userId);
    } else {
        markStale(c);
    }
    changed();
}

function markStale(c: Community) {
    c.fieldsStale = true;
    c.values.forEach((_, id) => c.stale.add(id));
}

function subscribe(listener: () => void) {
    if (!listening) {
        listening = true;
        webSocketClient.addMessageListener(onMessage);
    }
    listeners.add(listener);
    return () => {
        listeners.delete(listener);
    };
}

function useVersion() {
    return useSyncExternalStore(subscribe, () => version);
}

async function loadFields(teamId: string) {
    const c = community(teamId);
    if ((c.fields && !c.fieldsStale) || c.fieldsLoading) {
        return;
    }
    c.fieldsLoading = true;
    c.fieldsStale = false;
    try {
        c.fields = await Client4.getCommunityAttributes(teamId);
    } catch {
        c.fields = c.fields || [];
    }
    c.fieldsLoading = false;
    changed();
}

async function loadValues(teamId: string, userIds: string[]) {
    const c = community(teamId);
    const missing = userIds.filter((id) => (!c.values.has(id) || c.stale.has(id)) && !c.loading.has(id));
    if (!missing.length) {
        return;
    }
    missing.forEach((id) => {
        c.loading.add(id);
        c.stale.delete(id);
    });
    for (let i = 0; i < missing.length; i += BATCH) {
        const batch = missing.slice(i, i + BATCH);
        try {
            // eslint-disable-next-line no-await-in-loop
            const found = await Client4.getCommunityAttributeValues(teamId, batch);
            batch.forEach((id) => c.values.set(id, found[id] || NO_VALUES));
        } catch {
            batch.forEach((id) => c.values.set(id, c.values.get(id) || NO_VALUES));
        }
        batch.forEach((id) => c.loading.delete(id));
    }
    changed();
}

// useCommunityAttributes gives a community's attributes, in their order (undefined until they're known).
export function useCommunityAttributes(teamId: string | undefined): CommunityAttributeField[] | undefined {
    useVersion();
    const c = teamId ? communities.get(teamId) : undefined;
    const fields = c?.fields;
    const stale = c?.fieldsStale;
    useEffect(() => {
        if (teamId && (!fields || stale)) {
            loadFields(teamId);
        }
    }, [teamId, fields, stale]);
    return fields;
}

// useCommunityAttributeValues gives some members' values of a community's attributes, by user, asking the server
// for those not known yet (when the community has attributes at all).
export function useCommunityAttributeValues(teamId: string | undefined, userIds: string[]): Record<string, AttributeValues> {
    const fields = useCommunityAttributes(teamId);
    useVersion();
    const c = teamId ? communities.get(teamId) : undefined;
    const key = userIds.join(',');
    useEffect(() => {
        if (teamId && fields?.length) {
            loadValues(teamId, userIds);
        }
    }, [teamId, fields, key, version]); // eslint-disable-line react-hooks/exhaustive-deps

    const values: Record<string, AttributeValues> = {};
    if (c && fields?.length) {
        for (const id of userIds) {
            const v = c.values.get(id);
            if (v) {
                values[id] = v;
            }
        }
    }
    return values;
}

// useMemberAttributeValues is one member's values.
export function useMemberAttributeValues(teamId: string | undefined, userId: string): AttributeValues | undefined {
    return useCommunityAttributeValues(teamId, [userId])[userId];
}

// listedAttribute is the attribute the member list shows under names (the first one marked so).
export function listedAttribute(fields: CommunityAttributeField[] | undefined): CommunityAttributeField | undefined {
    return fields?.find((field) => field.attrs?.member_list);
}

async function reloadFields(teamId: string) {
    markStale(community(teamId));
    await loadFields(teamId);
}

export async function createCommunityAttribute(teamId: string, field: CommunityAttributeFieldPatch) {
    const created = await Client4.createCommunityAttribute(teamId, field);
    await reloadFields(teamId);
    return created;
}

export async function patchCommunityAttribute(teamId: string, fieldId: string, patch: CommunityAttributeFieldPatch) {
    const patched = await Client4.patchCommunityAttribute(teamId, fieldId, patch);
    await reloadFields(teamId);
    return patched;
}

export async function deleteCommunityAttribute(teamId: string, fieldId: string) {
    await Client4.deleteCommunityAttribute(teamId, fieldId);
    await reloadFields(teamId);
}

// saveMemberAttributeValues sets some of a member's values (an empty one clears it).
export async function saveMemberAttributeValues(teamId: string, userId: string, values: Record<string, UserAttributeValue>) {
    const saved = await Client4.patchCommunityAttributeValues(teamId, userId, values);
    const c = community(teamId);
    c.values.set(userId, {...(c.values.get(userId) || NO_VALUES), ...saved});
    changed();
    return saved;
}
