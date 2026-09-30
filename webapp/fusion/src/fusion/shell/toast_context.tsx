// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {createContext, useCallback, useContext, useEffect, useRef, useState} from 'react';

import {Layer} from 'fusion/components/layer';
import {am} from 'fusion/utils/class_names';

// How long a toast stays, as in the mockup.
const TOAST_MS = 2600;

type ShowToast = (text: string) => void;

const ToastContext = createContext<ShowToast>(() => {});

// ToastProvider shows the mockup's toasts: one short confirmation at a time at the bottom of the screen, gone after
// a few seconds, announced to screen readers (role="status").
export function ToastProvider({children}: {children: React.ReactNode}) {
    const [toast, setToast] = useState<{text: string; key: number} | null>(null);
    const timer = useRef<ReturnType<typeof setTimeout>>(undefined);

    const show = useCallback((text: string) => {
        clearTimeout(timer.current);
        setToast({text, key: Date.now()});
        timer.current = setTimeout(() => setToast(null), TOAST_MS);
    }, []);

    useEffect(() => () => clearTimeout(timer.current), []);

    return (
        <ToastContext.Provider value={show}>
            {children}
            {toast && (
                <Layer>
                    <div
                        key={toast.key}
                        className={am('toast')}
                        role='status'
                    >
                        {toast.text}
                    </div>
                </Layer>
            )}
        </ToastContext.Provider>
    );
}

// useToast returns the function that shows a toast.
export function useToast(): ShowToast {
    return useContext(ToastContext);
}
