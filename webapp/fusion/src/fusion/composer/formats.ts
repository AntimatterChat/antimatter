// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {IconName} from 'fusion/components/icon';

// The mockup's formatting buttons: markdown inserted around the selection, or at the start of the selected lines.
export type Format = {id: IconName; label: {id: string; defaultMessage: string}; before: string; after: string};

// Groups (separated by null) fold into "More" when the composer is narrow.
export const FORMATS: Array<Format | null> = [
    {id: 'bold', label: {id: 'fusion.format.bold', defaultMessage: 'Bold'}, before: '**', after: '**'},
    {id: 'italic', label: {id: 'fusion.format.italic', defaultMessage: 'Italic'}, before: '*', after: '*'},
    {id: 'strike', label: {id: 'fusion.format.strike', defaultMessage: 'Strikethrough'}, before: '~~', after: '~~'},
    null,
    {id: 'heading', label: {id: 'fusion.format.heading', defaultMessage: 'Heading'}, before: '### ', after: ''},
    {id: 'link', label: {id: 'fusion.format.link', defaultMessage: 'Link'}, before: '[', after: '](https://)'},
    null,
    {id: 'code', label: {id: 'fusion.format.code', defaultMessage: 'Inline code'}, before: '`', after: '`'},
    {id: 'codeblock', label: {id: 'fusion.format.codeblock', defaultMessage: 'Code block'}, before: '```\n', after: '\n```'},
    null,
    {id: 'quote', label: {id: 'fusion.format.quote', defaultMessage: 'Quote'}, before: '> ', after: ''},
    {id: 'ul', label: {id: 'fusion.format.ul', defaultMessage: 'Bulleted list'}, before: '- ', after: ''},
    {id: 'ol', label: {id: 'fusion.format.ol', defaultMessage: 'Numbered list'}, before: '1. ', after: ''},
];

// applyFormat returns the new text and selection after applying a format to the textarea's selection.
export function applyFormat(value: string, start: number, end: number, format: Format): {value: string; start: number; end: number} {
    const lineStyle = format.after === '' && format.before.endsWith(' ');
    if (lineStyle) {
        const lineStart = value.lastIndexOf('\n', start - 1) + 1;
        const selected = value.slice(lineStart, end);
        const prefixed = selected.split('\n').map((line, i) => (format.id === 'ol' ? `${i + 1}. ` : format.before) + line).join('\n');
        const next = value.slice(0, lineStart) + prefixed + value.slice(end);
        const caret = lineStart + prefixed.length;
        return {value: next, start: caret, end: caret};
    }
    const selected = value.slice(start, end) || (format.id === 'link' ? 'text' : '');
    const next = value.slice(0, start) + format.before + selected + format.after + value.slice(end);
    const selStart = start + format.before.length;
    return {value: next, start: selStart, end: selStart + selected.length};
}
