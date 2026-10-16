// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

// A window.matchMedia whose answers the tests set, telling its listeners when they change (a phone turned sideways).
export function fakeMatchMedia(matching: (query: string) => boolean) {
    let matches = matching;
    const lists: Array<{query: string; listeners: Set<() => void>; list: MediaQueryList}> = [];
    window.matchMedia = jest.fn((query: string) => {
        const listeners = new Set<() => void>();
        const list = {
            media: query,
            get matches() {
                return matches(query);
            },
            addEventListener: (type: string, cb: () => void) => listeners.add(cb),
            removeEventListener: (type: string, cb: () => void) => listeners.delete(cb),
        } as unknown as MediaQueryList;
        lists.push({query, listeners, list});
        return list;
    }) as unknown as typeof window.matchMedia;
    return {
        set(next: (query: string) => boolean) {
            matches = next;
            lists.forEach(({listeners}) => listeners.forEach((cb) => cb()));
        },
        listenerCount: () => lists.reduce((n, {listeners}) => n + listeners.size, 0),
    };
}
