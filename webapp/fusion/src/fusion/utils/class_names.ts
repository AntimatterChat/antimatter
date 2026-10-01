// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

// The Fusion UI's styles prefix every class with am- so they never meet the classic web app's styles (see styles/).

type ClassArg = string | false | null | undefined | Record<string, unknown>;

/**
 * am builds a className from Fusion UI class names, adding the am- prefix:
 * am('ch', {active: isActive}, isUnread && 'unread') -> 'am-ch am-active am-unread'.
 */
export function am(...args: ClassArg[]): string {
    const out: string[] = [];
    for (const arg of args) {
        if (!arg) {
            continue;
        }
        if (typeof arg === 'string') {
            for (const name of arg.split(' ')) {
                if (name) {
                    out.push('am-' + name);
                }
            }
        } else {
            for (const [name, on] of Object.entries(arg)) {
                if (on) {
                    out.push('am-' + name);
                }
            }
        }
    }
    return out.join(' ');
}
