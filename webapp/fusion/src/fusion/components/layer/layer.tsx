// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {createContext, useContext, useEffect, useLayoutEffect, useRef, useState} from 'react';
import ReactDOM from 'react-dom';
import {useIntl} from 'react-intl';

import Icon from 'fusion/components/icon';
import {isTouch, useIsPhone} from 'fusion/hooks/media';
import {am} from 'fusion/utils/class_names';

export const LAYER_ID = 'am-layer';

// Layer renders popovers, menus and dialogs above the app, like the mockup's #layer.
// LayerRoot is the layer of another window the Fusion UI draws in (the call's window), for its popovers.
export const LayerRoot = createContext<HTMLElement | null>(null);

export function Layer({children}: {children: React.ReactNode}) {
    const own = useContext(LayerRoot);
    const root = own || document.getElementById(LAYER_ID);
    return root ? ReactDOM.createPortal(children, root) : null;
}

export type Placement =

    // Under the anchor, right-aligned (flips above when there's no room): the mockup's placeNear.
    'below' |

    // Above the anchor, left-aligned: popAbove, for the composer's menus.
    'above' |

    // Above the anchor, centred on it (below it when there's no room): a call's reactions, over their button.
    'above-center' |

    // Above the anchor, right-aligned with it (below it when there's no room): the send button's "Send later" menu,
    // which ends where the button does instead of running on past it.
    'above-end' |

    // Under the anchor, left-aligned (above when there's no room): the channel info under its name.
    'below-start' |

    // Above the anchor, right-aligned (below it when there's no room): openPicker, for the emoji picker.
    'picker' |

    // Beside the anchor, on its left when there's room: placePop, for profile cards.
    'beside' |

    // To the right of the anchor, top-aligned: the channel menu.
    'right' |

    // To the left of the anchor, top-aligned with its toolbar (or with it): a message's menu, over the toolbar's other
    // buttons (which it repeats) but not the one that opened it.
    'left' |

    // At a point, for context menus.
    'point';

type PopoverProps = {
    anchor?: HTMLElement | null;
    point?: {x: number; y: number};
    placement?: Placement;
    className: string;
    role?: string;
    label: string;

    // The id of what describes it, read as it opens (a menu's header: the channel's purpose).
    describedBy?: string;
    onClose: () => void;
    children: React.ReactNode;
    style?: React.CSSProperties;

    // Elements other than the anchor that open and close the popover themselves (a message's author name, for its
    // avatar's profile card): pressing them isn't a click elsewhere, or the popover would close and their click
    // reopen it at once.
    toggles?: Array<HTMLElement | null>;

    // On a phone a popover is a sheet from the bottom of the screen, over a scrim; false keeps it beside its anchor.
    sheet?: boolean;

    // What Escape does first: true when it was handled (the picker going back from a view to its grid), so the
    // popover stays open. Without it, or when it returns false, Escape closes the popover.
    onEscape?: () => boolean;
};

const MARGIN = 8;

// Where the last press happened: a popover whose anchor isn't shown (a message's toolbar on a touch screen, opened by
// a long press) opens there instead.
let lastPress: {x: number; y: number} | undefined;
if (typeof document !== 'undefined') {
    document.addEventListener('pointerdown', (e) => {
        lastPress = {x: e.clientX, y: e.clientY};
    }, true);
}

const clamp = (v: number, size: number, max: number) => Math.max(MARGIN, Math.min(v, max - size - MARGIN));

function place(pop: HTMLElement, placement: Placement, anchor?: HTMLElement | null, point?: {x: number; y: number}, pressed?: {x: number; y: number}) {
    const w = pop.offsetWidth;
    const h = pop.offsetHeight;

    // The window it's in: the call's window has its own
    const view = pop.ownerDocument.defaultView || window;
    const vw = view.innerWidth;
    const vh = view.innerHeight;
    let left = 0;
    let top = 0;
    const hidden = anchor ? !anchor.getClientRects().length : false;
    const at = hidden && !point ? pressed : point;
    if (placement === 'point' || !anchor || hidden) {
        left = at?.x ?? vw / 2;
        top = at?.y ?? vh / 3;
    } else {
        const r = anchor.getBoundingClientRect();
        if (placement === 'below') {
            left = r.right - w;
            top = r.bottom + 4;
            if (top + h > vh - MARGIN) {
                top = r.top - h - 4;
            }
        } else if (placement === 'below-start') {
            left = r.left;
            top = r.bottom + 4;
            if (top + h > vh - MARGIN) {
                top = r.top - h - 4;
            }
        } else if (placement === 'above' || placement === 'above-center') {
            left = placement === 'above' ? r.left : r.left + ((r.width - w) / 2);
            top = r.top - h - 8;
            if (top < MARGIN) {
                top = r.bottom + 8;
            }
        } else if (placement === 'picker' || placement === 'above-end') {
            left = r.right - w;
            top = r.top - h - 8;
            if (top < MARGIN) {
                top = Math.min(r.bottom + 8, vh - h - MARGIN);
            }
        } else if (placement === 'left') {
            const bar = (anchor.closest('[role="toolbar"]') as HTMLElement | null)?.getBoundingClientRect() || r;
            left = r.left - w - 4;
            if (left < MARGIN) {
                left = r.right + 4;
            }
            top = bar.top;
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

// The open popovers and dialogs of each window, newest last. Escape closes only the newest: a picker opened from a
// dialog, or a confirmation over the settings, closes without what's under it.
const escapeStacks = new WeakMap<Document, Array<{current: () => void}>>();

// useEscape calls onEscape when Escape is pressed while this layer is the newest of its window.
function useEscape(onEscape: () => void, ref: React.RefObject<HTMLElement>) {
    const handler = useRef(onEscape);
    handler.current = onEscape;
    useEffect(() => {
        const doc = ref.current?.ownerDocument || document;
        const stack = escapeStacks.get(doc) || [];
        escapeStacks.set(doc, stack);
        stack.push(handler);
        const onKeyDown = (e: KeyboardEvent) => {
            if (e.key === 'Escape' && stack[stack.length - 1] === handler) {
                e.stopPropagation();
                handler.current();
            }
        };
        doc.addEventListener('keydown', onKeyDown, true);
        return () => {
            stack.splice(stack.indexOf(handler), 1);
            doc.removeEventListener('keydown', onKeyDown, true);
        };

        // The layer keeps its place in the stack for as long as it's open.
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, []);
}

// useReturnFocus gives the focus back, when the layer closes, to what had it when the layer opened (the button that
// opened it), unless the focus has gone somewhere else since.
function useReturnFocus(ref: React.RefObject<HTMLElement>) {
    useEffect(() => {
        const layer = ref.current;
        const doc = layer?.ownerDocument || document;
        const opener = doc.activeElement as HTMLElement | null;
        return () => {
            const now = doc.activeElement;
            if (opener?.isConnected && (!now || now === doc.body || layer?.contains(now))) {
                opener.focus({preventScroll: true});
            }
        };

        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, []);
}

// Popover is a floating .am-pop: placed next to its anchor, closed by Escape (when it's the newest layer) or a click
// elsewhere; closing gives the focus back to the button that opened it. On a phone it's a sheet (.am-sheet) from the
// bottom of the screen instead, the whole width, as menus are in phone apps.
export function Popover({anchor, point, placement = 'below', className, role = 'dialog', label, describedBy, onClose, children, style, toggles, sheet: canSheet = true, onEscape}: PopoverProps) {
    const ref = useRef<HTMLDivElement>(null);
    const [, setPlaced] = useState(false);
    const togglesRef = useRef(toggles);
    togglesRef.current = toggles;
    const onEscapeRef = useRef(onEscape);
    onEscapeRef.current = onEscape;
    const sheet = useIsPhone() && canSheet;
    useEscape(() => {
        // onEscape may take Escape for itself (a view inside the popover goes back) and keep the popover open.
        if (!onEscapeRef.current?.()) {
            onClose();
        }
    }, ref);
    useReturnFocus(ref);

    // The press that opened it, for an anchor that isn't shown: taken once, as the popover follows its anchor.
    const pressed = useRef(lastPress);

    useLayoutEffect(() => {
        if (ref.current && !sheet) {
            place(ref.current, placement, anchor, point, pressed.current);
            setPlaced(true);
        }
    });

    // On a touch screen a sheet doesn't bring the keyboard up by itself (over half the screen, for a search the
    // picker doesn't need): its field is a tap away. The fields focused themselves as they mounted, before this.
    useEffect(() => {
        const focused = ref.current?.ownerDocument.activeElement;
        if (sheet && isTouch() && focused instanceof HTMLElement && ref.current?.contains(focused) && focused.matches('input, textarea')) {
            focused.blur();
        }
    }, [sheet]);

    useEffect(() => {
        const onPointerDown = (e: PointerEvent) => {
            const target = e.target as Node;
            if (ref.current?.contains(target) || anchor?.contains(target) || togglesRef.current?.some((t) => t?.contains(target))) {
                return;
            }

            // A popover opened from this one (a submenu, a picker) comes after it in the layer: clicking in it, or on
            // the scrim of its sheet, isn't clicking away.
            const above = (target as Element).closest?.('.am-pop, .am-sheet-scrim');
            if (above && ref.current && (ref.current.compareDocumentPosition(above) & Node.DOCUMENT_POSITION_FOLLOWING)) {
                return;
            }
            onClose();
        };
        const doc = ref.current?.ownerDocument || document;
        doc.addEventListener('pointerdown', onPointerDown, true);
        return () => doc.removeEventListener('pointerdown', onPointerDown, true);
    }, [anchor, onClose]);

    return (
        <Layer>
            {sheet && (
                <div
                    className={am('sheet-scrim')}
                    aria-hidden='true'
                />
            )}
            <div
                ref={ref}
                className={am('pop', className, {sheet})}
                role={role}
                aria-label={label}
                aria-describedby={describedBy}
                style={sheet ? style : {left: -9999, top: -9999, ...style}}
            >
                {children}
            </div>
        </Layer>
    );
}

// SheetBack is the back button of a panel shown as a full-screen sheet on a phone (a thread, an app, the member list):
// it takes the place of the panel's close button, at the start of its header.
export function SheetBack({onBack}: {onBack: () => void}) {
    const {formatMessage} = useIntl();
    return (
        <button
            type='button'
            className={am('icon-btn', 'sheet-back')}
            title={formatMessage({id: 'fusion.sheet.back', defaultMessage: 'Back'})}
            aria-label={formatMessage({id: 'fusion.sheet.back', defaultMessage: 'Back'})}
            onClick={onBack}
        >
            <Icon name='chev'/>
        </button>
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

const FOCUSABLE = 'input:not([disabled]), textarea:not([disabled]), select:not([disabled]), button:not([disabled]), a[href], [tabindex]:not([tabindex="-1"])';

// Dialog is a modal on a scrim (.am-scrim > .am-modal), closed by Escape or a click on the scrim. It takes the focus
// (unless a field of it has it already) and gives it back when it closes.
export function Dialog({label, onClose, small = true, top = false, children, className, onSubmit}: DialogProps) {
    const ref = useRef<HTMLElement>(null);
    useEscape(onClose, ref);
    useReturnFocus(ref);

    useEffect(() => {
        const modal = ref.current;
        if (modal && !modal.contains(document.activeElement)) {
            (modal.querySelector<HTMLElement>(FOCUSABLE) || modal).focus({preventScroll: true});
        }
    }, []);

    const content = onSubmit ? (
        <form
            ref={ref as React.RefObject<HTMLFormElement>}
            className={am('modal', {small}) + (className ? ' ' + className : '')}
            role='dialog'
            aria-modal='true'
            aria-label={label}
            tabIndex={-1}
            onSubmit={(e) => {
                e.preventDefault();
                onSubmit(e);
            }}
        >
            {children}
        </form>
    ) : (
        <div
            ref={ref as React.RefObject<HTMLDivElement>}
            className={am('modal', {small}) + (className ? ' ' + className : '')}
            role='dialog'
            aria-modal='true'
            aria-label={label}
            tabIndex={-1}
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
