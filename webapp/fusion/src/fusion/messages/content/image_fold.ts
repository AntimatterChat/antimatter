// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {useSyncExternalStore} from 'react';

// Whether a message's images are shown, as its reader chose from the message's ⋯ menu (or its "Show images" line):
// shared by the message and its menu, and kept while the app runs, so the choice holds as the message scrolls out of
// view and back. Messages nobody chose for follow the "previews collapsed" setting (/expand, /collapse).
const chosen = new Map<string, boolean>();
const listeners = new Set<() => void>();

const subscribe = (listener: () => void) => {
    listeners.add(listener);
    return () => {
        listeners.delete(listener);
    };
};

export function setImagesShown(postId: string, shown: boolean) {
    chosen.set(postId, shown);
    listeners.forEach((listener) => listener());
}

// useImagesShown is the reader's choice for the message, undefined when they made none.
export function useImagesShown(postId: string): boolean | undefined {
    return useSyncExternalStore(subscribe, () => chosen.get(postId));
}

// For the tests: forget every choice.
export function clearImagesShown() {
    chosen.clear();
    listeners.forEach((listener) => listener());
}
