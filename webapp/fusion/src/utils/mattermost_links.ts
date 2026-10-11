// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

// Antimatter doesn't send its users to mattermost.com (its docs, forum, marketplace, sales...): the links the classic
// web app has there are shown as text, and opening them is refused.

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

// blockMattermostLinks refuses to follow the links and open the windows that lead to mattermost.com, for the links
// not drawn by ExternalLink. It returns what undoes it.
export function blockMattermostLinks(): () => void {
    const onClick = (e: MouseEvent) => {
        const link = (e.target as Element | null)?.closest?.('a[href]');
        if (link && isMattermostURL(link.getAttribute('href'))) {
            e.preventDefault();
            e.stopPropagation();
        }
    };
    const open = window.open;
    window.open = function openUnlessMattermost(this: Window, url?: string | URL, ...rest: unknown[]) {
        if (isMattermostURL(url)) {
            return null;
        }
        return (open as (...args: unknown[]) => Window | null).call(this, url, ...rest);
    } as typeof window.open;
    document.addEventListener('click', onClick, true);
    return () => {
        document.removeEventListener('click', onClick, true);
        window.open = open;
    };
}
