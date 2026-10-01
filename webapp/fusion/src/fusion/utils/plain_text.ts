// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

// plainText gives a one-line plain version of a message's markdown, as the mockup's plain(): for quotes, snippets
// and result rows that show a message without rendering it.
export function plainText(message: string, max = 300): string {
    return message.
        replace(/```[\s\S]*?```/g, ' [code] ').
        replace(/!\[([^\]]*)\]\([^)]*\)/g, '$1').
        replace(/\[([^\]]*)\]\([^)]*\)/g, '$1').
        replace(/^\s{0,3}(?:#{1,6}|>|[-*+]|\d+\.)\s+/gm, '').
        replace(/(\*\*|__|~~|`)/g, '').
        replace(/(^|\W)[*_]([^*_\n]+)[*_](?=\W|$)/g, '$1$2').
        replace(/\s+/g, ' ').
        trim().
        slice(0, max);
}
