// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

// The last searches, kept in this browser for the header's search and the global search, as the mockup does.
const KEY = 'am-recent-searches';
const MAX = 5;

export function getRecentSearches(): string[] {
    try {
        const list = JSON.parse(localStorage.getItem(KEY) || '[]');
        return Array.isArray(list) ? list.filter((q) => typeof q === 'string').slice(0, MAX) : [];
    } catch {
        return [];
    }
}

export function addRecentSearch(query: string) {
    const q = query.trim();
    if (!q) {
        return;
    }
    try {
        localStorage.setItem(KEY, JSON.stringify([q, ...getRecentSearches().filter((x) => x !== q)].slice(0, MAX)));
    } catch {
        // Without storage, searches aren't remembered.
    }
}
