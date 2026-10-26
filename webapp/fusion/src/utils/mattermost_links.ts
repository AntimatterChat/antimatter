// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

// Antimatter doesn't send its users to mattermost.com (its docs, forum, marketplace, sales...): the links the classic
// web app has there are shown as text, and opening them is refused. That's for Antimatter's own UI only: links people
// write in messages, and links and windows apps open from their own UI, work as anywhere else (an app's help page,
// a message pointing at upstream docs).

export function isMattermostURL(url?: string | URL | null): boolean {
    if (!url) {
        return false;
    }
    try {
        const {hostname} = new URL(String(url), window.location.href);
        return hostname === 'mattermost.com' || hostname.endsWith('.mattermost.com');
    } catch {
        return false;
    }
}

// What people write: messages (with their link previews and quoted messages), wherever they're shown.
const USER_CONTENT = '.am-msg, .post-message__text, [data-am-user-content]';

// Long enough for any component tree; a guard against a cycle.
const MAX_DEPTH = 2000;

type Fiber = {type?: unknown; return?: Fiber | null};

// drawnByApp says whether React drew an element inside an app's component: they're all rendered inside a
// PluggableErrorBoundary (amAppContent), also when they portal elsewhere (a dialog), since portals keep their place
// in the component tree.
function drawnByApp(el: Element): boolean {
    const key = Object.keys(el).find((k) => k.startsWith('__reactFiber$'));
    let fiber = key ? (el as unknown as Record<string, Fiber>)[key] : null;
    for (let depth = 0; fiber && depth < MAX_DEPTH; depth++) {
        const type = fiber.type as {amAppContent?: boolean} | undefined;
        if (type && (typeof type === 'function' || typeof type === 'object') && type.amAppContent) {
            return true;
        }
        fiber = fiber.return;
    }
    return false;
}

// isUserOrAppContent says whether an element belongs to what people wrote or to an app's UI, not to Antimatter's.
export function isUserOrAppContent(el: Element | null | undefined): boolean {
    if (!el) {
        return false;
    }
    if (el.closest(USER_CONTENT)) {
        return true;
    }

    // Text nodes and the like have no fiber of their own: their element has.
    return drawnByApp(el);
}

function eventElement(event: Event | undefined): Element | null {
    const target = event?.target as Node | null | undefined;
    if (!target) {
        return null;
    }
    return target.nodeType === Node.ELEMENT_NODE ? target as Element : target.parentElement;
}

// blockMattermostLinks refuses to follow the links and open the windows that lead to mattermost.com from Antimatter's
// own UI, for the links not drawn by ExternalLink. A window opened outside any event (after an app's request
// returns, say) can't be told apart: it's opened. It returns what undoes it.
export function blockMattermostLinks(): () => void {
    const onClick = (e: MouseEvent) => {
        const link = (e.target as Element | null)?.closest?.('a[href]');
        if (link && isMattermostURL(link.getAttribute('href')) && !isUserOrAppContent(link)) {
            e.preventDefault();
            e.stopPropagation();
        }
    };
    const open = window.open;
    window.open = function openUnlessMattermost(this: Window, url?: string | URL, ...rest: unknown[]) {
        if (isMattermostURL(url)) {
            // window.event is the event being handled, if any: the click that asked for the window.
            const current = (window as unknown as {event?: Event}).event;
            const from = eventElement(current);
            if (from && !isUserOrAppContent(from)) {
                return null;
            }
        }
        return (open as (...args: unknown[]) => Window | null).call(this, url, ...rest);
    } as typeof window.open;
    document.addEventListener('click', onClick, true);
    return () => {
        document.removeEventListener('click', onClick, true);
        window.open = open;
    };
}
