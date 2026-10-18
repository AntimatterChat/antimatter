// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import fs from 'fs';
import path from 'path';

// The picker's add views (a custom emoji, a GIF, a sticker, a meme template) fit the picker without scrolling, on a
// desktop and on a phone. The test browser lays nothing out, so this adds up the heights the styles give the parts of
// the tallest views, as the browser would, and checks the sum against the room the picker leaves them.

const read = (name: string) => fs.readFileSync(path.join(__dirname, '..', 'styles', name), 'utf8');
const picker = read('_12_picker.scss');
const phone = read('_99_phone.scss');

// The declarations of the one-line rule for a selector (as these files write them): the top-level one, else the one in
// a media query (the phone styles are all in one).
function rule(css: string, selector: string): string {
    const prefix = ':is(.am-app, .am-layer) ';
    const lines = css.split('\n');
    const line = lines.find((l) => l.startsWith(prefix + selector + ' {')) || lines.find((l) => l.trim().startsWith(prefix + selector + ' {'));
    if (!line) {
        throw new Error(`no rule for ${selector}`);
    }
    return line.slice(line.indexOf('{') + 1, line.lastIndexOf('}'));
}

// A length in pixels of a declaration ('height', 'padding-top' from a padding shorthand, 'line-height' from a font
// shorthand); 0 when there's none.
function px(css: string, selector: string, prop: string): number {
    const decls = rule(css, selector);
    const value = (name: string) => new RegExp(`(?:^|;)\\s*${name}:\\s*([^;]+)`).exec(decls)?.[1].trim();
    const num = (v: string | undefined) => (v ? parseFloat(v) || 0 : 0);
    if (prop === 'padding-top' || prop === 'padding-bottom') {
        const parts = (value('padding') || '').split(/\s+/);
        return num(prop === 'padding-top' ? parts[0] : parts[2] ?? parts[0]);
    }
    if (prop === 'line-height' && !value('line-height')) {
        return num((/\/(\d+(?:\.\d+)?)px/).exec(value('font') || '')?.[1]);
    }
    return num(value(prop));
}

const BORDER = 1;

type Sizes = {pickerHeight: number; input: number; head: number; actions: number; dropButton: number};

const desktop: Sizes = {
    pickerHeight: px(picker, '.am-pop.am-picker', 'height'),
    input: px(picker, '.am-pv .am-pv-field > :is(input[type="text"], select)', 'height'),
    head: px(picker, '.am-pv-head', 'height'),
    actions: px(picker, '.am-pv-actions .am-btn', 'height'),
    dropButton: px(picker, '.am-pv-drop', 'min-height'),
};

// A phone's sheet (as high as a small phone allows) with the bigger targets of the phone styles.
const onPhone: Sizes = {
    pickerHeight: Math.min(px(phone, '.am-pop.am-sheet.am-picker', 'height') || 560, 560),
    input: px(phone, '.am-pv .am-pv-field > :is(input[type="text"], select), :is(.am-app, .am-layer) .am-pv .am-ce-name', 'height'),
    head: px(phone, '.am-pv-head', 'height'),
    actions: px(phone, '.am-pv-actions .am-btn', 'min-height'),
    dropButton: Math.max(px(picker, '.am-pv-drop', 'min-height'), px(phone, '.am-pv-drop-btn', 'min-height') + 3),
};

const line = px(picker, '.am-pv-message', 'line-height');
const label = px(picker, '.am-pv-field-head > label', 'line-height');
const gap = px(picker, '.am-pv-field', 'gap');
const fieldMargin = px(picker, '.am-pv-field', 'margin-bottom');

// A field: its label, its input, and its hint when it has one (a field's empty message takes no room).
const field = (s: Sizes, hintLines = 0) => label + gap + s.input + (hintLines ? gap + (hintLines * line) : 0) + fieldMargin;

// The drop zone: empty with the types and limits under it, or with the chosen file's preview beside the button.
const previewHeight = px(picker, '.am-pv-drop-preview', 'max-height') + 8;
const drop = (s: Sizes, hintLines: number) => Math.max(s.dropButton, hintLines ? 0 : previewHeight) + px(picker, '.am-pv-drop', 'margin-bottom') + (hintLines * line) + px(picker, '.am-pv-drop-wrap', 'margin-bottom');

// What the picker leaves the body of a view: its header and actions (the tabs make room for a view).
const room = (s: Sizes) => s.pickerHeight - (s.head + BORDER) - (px(picker, '.am-pv-actions', 'padding-top') + px(picker, '.am-pv-actions', 'padding-bottom') + s.actions + BORDER) -
    (px(picker, '.am-pv-body', 'padding-top') + px(picker, '.am-pv-body', 'padding-bottom'));

describe('the picker\'s add views fit it without scrolling', () => {
    test('the picker hides its tabs while a view shows, for the view to have the room', () => {
        expect(picker).toMatch(/\.am-pop\.am-picker:has\(\.am-pv\) > \.am-picker-tabs \{ display: none; \}/);
    });

    test('a field\'s empty message takes no room', () => {
        expect(rule(picker, '.am-pv-message:empty')).toMatch(/position: absolute/);
        expect(rule(picker, '.am-pv-message')).not.toMatch(/min-height/);
    });

    test('the drop zone is low: no tall empty box', () => {
        expect(px(picker, '.am-pv-drop', 'min-height')).toBeLessThanOrEqual(64);
        expect(px(picker, '.am-pv-drop-preview', 'max-height')).toBeLessThanOrEqual(80);
    });

    // [what, drop zone hint lines (0: a file is chosen), hint lines of each field]
    const views: Array<[string, number, number[]]> = [
        ['a custom emoji, before an image is chosen', 2, [2]],
        ['a GIF, before a file is chosen', 2, [0, 1]],
        ['a GIF, with its file', 0, [0, 1]],
        ['a sticker, before a file is chosen', 2, [0, 1, 1]],
        ['a sticker, with its file and an error under its title', 0, [1, 1, 1]],
        ['a meme template, before an image is chosen', 2, [1, 1, 0]],
        ['a meme template, with its image', 0, [1, 1, 0]],
    ];

    test.each(views)('on a desktop: %s', (what, dropLines, fields) => {
        const height = drop(desktop, dropLines) + fields.reduce((sum, lines) => sum + field(desktop, lines), 0);
        expect(height).toBeLessThanOrEqual(room(desktop));
    });

    // A phone is narrower: the hints take a line more.
    test.each(views)('on a phone: %s', (what, dropLines, fields) => {
        const height = drop(onPhone, dropLines ? dropLines + 1 : 0) + fields.reduce((sum, lines) => sum + field(onPhone, lines ? lines + 1 : 0), 0);
        expect(height).toBeLessThanOrEqual(room(onPhone));
    });

    test('the sizes were read from the styles', () => {
        expect(desktop).toEqual({pickerHeight: 450, input: 32, head: 44, actions: 32, dropButton: 56});
        expect(onPhone.input).toBe(40);
        expect(onPhone.head).toBe(52);
        expect(onPhone.actions).toBe(44);
        expect(line).toBe(16);
        expect(label).toBe(14);
    });
});
