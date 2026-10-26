// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {getCurrentUserId} from 'mattermost-redux/selectors/entities/users';

import store from 'stores/redux_store';

// The last searches, kept in this browser for the header's search and the global search, as the mockup does. They're
// the signed-in user's own (one list per user), and signing out forgets them: on a shared computer the next person
// doesn't see what you searched for.
const PREFIX = 'am-recent-searches';
const MAX = 5;

function keyFor(userId: string): string {
    return `${PREFIX}:${userId}`;
}

function currentKey(): string | null {
    let userId = '';
    try {
        userId = getCurrentUserId(store.getState());
    } catch {
        // No store yet (or not a full one): nobody is known to be signed in.
    }
    return userId ? keyFor(userId) : null;
}

// The list kept for everyone before it was per user: whose it was isn't known, so it's dropped.
function dropShared() {
    try {
        localStorage.removeItem(PREFIX);
    } catch {
        // Without storage, there's nothing to drop.
    }
}

export function getRecentSearches(): string[] {
    dropShared();
    const key = currentKey();
    if (!key) {
        return [];
    }
    try {
        const list = JSON.parse(localStorage.getItem(key) || '[]');
        return Array.isArray(list) ? list.filter((q) => typeof q === 'string').slice(0, MAX) : [];
    } catch {
        return [];
    }
}

export function addRecentSearch(query: string) {
    const q = query.trim();
    const key = currentKey();
    if (!q || !key) {
        return;
    }
    try {
        localStorage.setItem(key, JSON.stringify([q, ...getRecentSearches().filter((x) => x !== q)].slice(0, MAX)));
    } catch {
        // Without storage, searches aren't remembered.
    }
}

// clearRecentSearches forgets the signed-in user's searches, on signing out.
export function clearRecentSearches() {
    dropShared();
    const key = currentKey();
    if (!key) {
        return;
    }
    try {
        localStorage.removeItem(key);
    } catch {
        // Without storage, there's nothing to forget.
    }
}
