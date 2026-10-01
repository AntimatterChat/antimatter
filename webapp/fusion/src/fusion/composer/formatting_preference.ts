// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {useEffect, useState} from 'react';

// Whether composers show the formatting bar: one choice for every composer, kept in this browser, as the mockup's
// state.showFormatting ("am-formatting" in localStorage).
const KEY = 'am-formatting';
const EVENT = 'am-formatting-change';

function read(): boolean {
    try {
        return localStorage.getItem(KEY) !== 'false';
    } catch {
        return true;
    }
}

export function useShowFormatting(): [boolean, (show: boolean) => void] {
    const [show, setShow] = useState(read);

    useEffect(() => {
        const sync = () => setShow(read());
        window.addEventListener(EVENT, sync);
        window.addEventListener('storage', sync);
        return () => {
            window.removeEventListener(EVENT, sync);
            window.removeEventListener('storage', sync);
        };
    }, []);

    const update = (next: boolean) => {
        try {
            localStorage.setItem(KEY, String(next));
        } catch {
            // Without storage the choice lasts until the page reloads.
        }
        setShow(next);
        window.dispatchEvent(new Event(EVENT));
    };
    return [show, update];
}
