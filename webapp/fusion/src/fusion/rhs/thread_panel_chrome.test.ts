// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import fs from 'fs';
import path from 'path';

// The thread view stays plain: making a thread a topic or a channel is in its ⋯ menu, not in a prompt at its top, and
// the panel's width is dragged, not toggled by an "Expand the panel" button.
const read = (file: string) => fs.readFileSync(path.join(__dirname, file), 'utf8');

describe('the thread view', () => {
    const panel = read('thread_panel.tsx');

    it('has no "Worth its own space?" prompt', () => {
        expect(panel).not.toMatch(/ThreadSuggestion/);
        expect(read('thread_to.tsx')).not.toMatch(/Worth its own space|ThreadSuggestion/);
    });

    it('has no expand button, and puts back the width of a panel left expanded', () => {
        expect(panel).not.toMatch(/fusion\.thread\.expand|fusion\.thread\.collapse/);
        expect(panel).toMatch(/if \(expanded\) \{\n\s+dispatch\(toggleRhsExpanded\(\)\);/);
    });

    it('still offers making the thread a topic or a channel from its menu', () => {
        expect(panel).toMatch(/<ThreadMenu[\s\S]*?onConvert=\{setConverting\}/);
        expect(panel).toMatch(/ThreadToChannelDialog/);
    });

    it('nor does the search results panel have an expand button', () => {
        const results = read('results_panel.tsx');

        expect(results).not.toMatch(/fusion\.thread\.expand|fusion\.thread\.collapse/);
        expect(results).toMatch(/if \(expanded\) \{\n\s+dispatch\(toggleRhsExpanded\(\)\);/);
    });

    it('leaves no style for the prompt', () => {
        expect(fs.readFileSync(path.join(__dirname, '..', 'styles', '_07_rhs.scss'), 'utf8')).not.toMatch(/\.am-thread-to\b/);
    });
});
