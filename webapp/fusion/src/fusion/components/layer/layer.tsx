// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useLayoutEffect, useRef, useState} from 'react';
import ReactDOM from 'react-dom';

import {am} from 'fusion/utils/class_names';

export const LAYER_ID = 'am-layer';

// Layer renders popovers, menus and dialogs above the app, like the mockup's #layer.
export function Layer({children}: {children: React.ReactNode}) {
    const root = document.getElementById(LAYER_ID);
    return root ? ReactDOM.createPortal(children, root) : null;
}

export type Placement =

    // Under the anchor, right-aligned (flips above when there's no room): the mockup's placeNear.
    'below' |

    // Above the anchor, left-aligned: popAbove, for the composer's menus.
    'above' |

    // Beside the anchor, on its left when there's room: placePop, for profile cards.
    'beside' |

    // To the right of the anchor, top-aligned: the channel menu.
    'right' |

    // At a point, for context menus.
    'point';

type PopoverProps = {
    anchor?: HTMLElement | null;
    point?: {x: number; y: number};
    placement?: Placement;
    className: string;
    role?: string;
    label: string;
    onClose: () => void;
    children: React.ReactNode;
    style?: React.CSSProperties;
};

const MARGIN = 8;
const clamp = (v: number, size: number, max: number) => Math.max(MARGIN, Math.min(v, max - size - MARGIN));

function place(pop: HTMLElement, placement: Placement, anchor?: HTMLElement | null, point?: {x: number; y: number}) {
    const w = pop.offsetWidth;
    const h = pop.offsetHeight;
    const vw = window.innerWidth;
    const vh = window.innerHeight;
    let left = 0;
    let top = 0;
    if (placement === 'point' || !anchor) {
        left = point?.x ?? vw / 2;
        top = point?.y ?? vh / 3;
    } else {
        const r = anchor.getBoundingClientRect();
        if (placement === 'below') {
            left = r.right - w;
            top = r.bottom + 4;
            if (top + h > vh - MARGIN) {
                top = r.top - h - 4;
            }
        } else if (placement === 'above') {
            left = r.left;
            top = r.top - h - 8;
            if (top < MARGIN) {
                top = r.bottom + 8;
            }
        } else if (placement === 'beside') {
            left = r.left - w - 10;
            if (left < MARGIN) {
                left = r.right + 10;
            }
            top = r.top;
        } else {
            left = r.right + 6;
            top = r.top - 6;
        }
    }
    pop.style.left = clamp(left, w, vw) + 'px';
    pop.style.top = clamp(top, h, vh) + 'px';
}

// Popover is a floating .am-pop: placed next to its anchor, closed by Escape or a click elsewhere.
export function Popover({anchor, point, placement = 'below', className, role = 'dialog', label, onClose, children, style}: PopoverProps) {
    const ref = useRef<HTMLDivElement>(null);
    const [, setPlaced] = useState(false);

    useLayoutEffect(() => {
        if (ref.current) {
            place(ref.current, placement, anchor, point);
            setPlaced(true);
        }
    });

    useEffect(() => {
        const onPointerDown = (e: PointerEvent) => {
            const target = e.target as Node;
            if (ref.current?.contains(target) || anchor?.contains(target)) {
                return;
            }
            onClose();
        };
        const onKeyDown = (e: KeyboardEvent) => {
            if (e.key === 'Escape') {
                e.stopPropagation();
                onClose();
            }
        };
        document.addEventListener('pointerdown', onPointerDown, true);
        document.addEventListener('keydown', onKeyDown, true);
        return () => {
            document.removeEventListener('pointerdown', onPointerDown, true);
            document.removeEventListener('keydown', onKeyDown, true);
        };
    }, [anchor, onClose]);

    return (
        <Layer>
            <div
                ref={ref}
                className={am('pop', className)}
                role={role}
                aria-label={label}
                style={{left: -9999, top: -9999, ...style}}
            >
                {children}
            </div>
        </Layer>
    );
}

type DialogProps = {
    label: string;
    onClose: () => void;
    small?: boolean;
    top?: boolean;
    children: React.ReactNode;
    className?: string;
    onSubmit?: (e: React.FormEvent) => void;
};

// Dialog is a modal on a scrim (.am-scrim > .am-modal), closed by Escape or a click on the scrim.
export function Dialog({label, onClose, small = true, top = false, children, className, onSubmit}: DialogProps) {
    useEffect(() => {
        const onKeyDown = (e: KeyboardEvent) => {
            if (e.key === 'Escape') {
                e.stopPropagation();
                onClose();
            }
        };
        document.addEventListener('keydown', onKeyDown, true);
        return () => document.removeEventListener('keydown', onKeyDown, true);
    }, [onClose]);

    const content = onSubmit ? (
        <form
            className={am('modal', {small}) + (className ? ' ' + className : '')}
            role='dialog'
            aria-modal='true'
            aria-label={label}
            onSubmit={(e) => {
                e.preventDefault();
                onSubmit(e);
            }}
        >
            {children}
        </form>
    ) : (
        <div
            className={am('modal', {small}) + (className ? ' ' + className : '')}
            role='dialog'
            aria-modal='true'
            aria-label={label}
        >
            {children}
        </div>
    );

    return (
        <Layer>
            <div
                className={am('scrim', {top})}
                onMouseDown={(e) => {
                    if (e.target === e.currentTarget) {
                        onClose();
                    }
                }}
            >
                {content}
            </div>
        </Layer>
    );
}
