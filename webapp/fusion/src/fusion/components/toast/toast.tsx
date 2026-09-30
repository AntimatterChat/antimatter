// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useState} from 'react';

import {Layer} from 'fusion/components/layer';
import {am} from 'fusion/utils/class_names';

const DURATION = 2600;

type Toast = {id: number; text: string};

let nextId = 1;
const listeners = new Set<(toast: Toast) => void>();

// showToast briefly shows a short confirmation at the bottom of the window, like the mockup's toast(): a new
// one replaces the one on screen.
export function showToast(text: string) {
    const toast = {id: nextId++, text};
    listeners.forEach((listener) => listener(toast));
}

// Toasts shows the toasts of showToast; the app renders it once.
export default function Toasts() {
    const [toast, setToast] = useState<Toast | null>(null);

    useEffect(() => {
        listeners.add(setToast);
        return () => {
            listeners.delete(setToast);
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
                role='status'
            >
                {toast.text}
            </div>
        </Layer>
    );
}
