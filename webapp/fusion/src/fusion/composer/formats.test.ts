// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {Editor} from '@tiptap/core';
import Link from '@tiptap/extension-link';
import {Markdown} from '@tiptap/markdown';
import StarterKit from '@tiptap/starter-kit';

import {formatRich} from './formats';

const createEditor = (markdown: string) => new Editor({
    extensions: [StarterKit.configure({link: false}), Link.configure({openOnClick: false}), Markdown],
    content: markdown,
    contentType: 'markdown',
});

describe('formatRich', () => {
    let editor: Editor;

    beforeAll(() => {
        document.elementFromPoint = () => null;
    });

    afterEach(() => {
        editor?.destroy();
    });

    test('bold with the caret inside a word takes the whole word and keeps the caret', () => {
        editor = createEditor('hello world');
        editor.commands.setTextSelection(3);

        formatRich(editor, 'bold');

        expect(editor.getHTML()).toBe('<p><strong>hello</strong> world</p>');
        expect(editor.state.selection.empty).toBe(true);
        expect(editor.state.selection.from).toBe(3);
    });

    test.each([
        ['italic', '<p><em>hello</em> world</p>'],
        ['strike', '<p><s>hello</s> world</p>'],
    ])('%s with the caret inside a word takes the whole word', (id, html) => {
        editor = createEditor('hello world');
        editor.commands.setTextSelection(3);

        formatRich(editor, id);

        expect(editor.getHTML()).toBe(html);
    });

    test('inline code stays Fusion\'s: it wraps the selection', () => {
        editor = createEditor('hello world');
        editor.commands.setTextSelection({from: 7, to: 12});

        formatRich(editor, 'code');

        expect(editor.getHTML()).toBe('<p>hello <code>world</code></p>');
    });

    test('code block makes the line a code block', () => {
        editor = createEditor('const a = 1;');
        editor.commands.setTextSelection(3);

        formatRich(editor, 'codeblock');

        expect(editor.getJSON().content?.[0]?.type).toBe('codeBlock');
    });

    test.each([
        ['heading', 'heading'],
        ['quote', 'blockquote'],
        ['ul', 'bulletList'],
        ['ol', 'orderedList'],
    ])('%s applies to the line', (id, type) => {
        editor = createEditor('line');
        editor.commands.setTextSelection(2);

        formatRich(editor, id);

        expect(editor.getJSON().content?.[0]?.type).toBe(type);
    });

    test('link makes the selected text a link to fill in', () => {
        editor = createEditor('see the docs');
        editor.commands.setTextSelection({from: 9, to: 13});

        formatRich(editor, 'link');

        expect(editor.getHTML()).toMatch(/<a [^>]*href="https:\/\/"[^>]*>docs<\/a>/);
    });
});
