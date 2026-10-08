// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {useEffect, useRef} from 'react';

// Cards about someone or something (a profile card, a team card) show one at a time: opening one closes the other.
const CARD_OPENED = 'am-card-opened';

let next = 0;

export function useExclusiveCard(onClose: () => void) {
    const close = useRef(onClose);
    close.current = onClose;
    useEffect(() => {
        next += 1;
        const me = next;
        const onOpened = (e: Event) => {
            if ((e as CustomEvent<number>).detail !== me) {
                close.current();
            }
        };
        window.dispatchEvent(new CustomEvent(CARD_OPENED, {detail: me}));
        window.addEventListener(CARD_OPENED, onOpened);
        return () => window.removeEventListener(CARD_OPENED, onOpened);
    }, []);
}
