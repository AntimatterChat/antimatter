// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useState} from 'react';

import {Layer} from 'fusion/components/layer';
import {am} from 'fusion/utils/class_names';

const DURATION = 2600;

// How far below the top of the main column a toast shows: under its 52px header.
const TOP_OFFSET = 52 + 12;

type Toast = {id: number; text: string};
type ShownToast = Toast & {style?: React.CSSProperties};

let nextId = 1;
const listeners = new Set<(toast: Toast) => void>();

// showToast briefly shows a short confirmation, like the mockup's toast(): a new one replaces the one on screen.
// The mockup shows it at the bottom of the window, over the message box: it shows at the top of the main column
// instead, under its header.
export function showToast(text: string) {
    const toast = {id: nextId++, text};
    listeners.forEach((listener) => listener(toast));
}

// placeToast centres a toast under the main column's header; without a main column, the styles place it.
export function placeToast(): React.CSSProperties | undefined {
    const main = document.querySelector('.am-app .am-main');
    if (!main) {
        return undefined;
    }
    const rect = main.getBoundingClientRect();
    return {top: rect.top + TOP_OFFSET, left: rect.left + (rect.width / 2)};
}

// Toasts shows the toasts of showToast; the app renders it once.
export default function Toasts() {
    const [toast, setToast] = useState<ShownToast | null>(null);

    useEffect(() => {
        const listener = (shown: Toast) => setToast({...shown, style: placeToast()});
        listeners.add(listener);
        return () => {
            listeners.delete(listener);
        };
    }, []);

    useEffect(() => {
        if (!toast) {
            return undefined;
        }
        const timer = setTimeout(() => setToast(null), DURATION);
        return () => clearTimeout(timer);
    }, [toast]);

    if (!toast) {
        return null;
    }
    return (
        <Layer>
            <div
                key={toast.id}
                className={am('toast')}
                style={toast.style}
                role='status'
            >
                {toast.text}
            </div>
        </Layer>
    );
}
