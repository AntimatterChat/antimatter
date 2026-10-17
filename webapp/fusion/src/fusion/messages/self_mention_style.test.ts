// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import fs from 'fs';
import path from 'path';

// A mention of the person reading is rendered as <span class="mention--highlight"><a class="mention-link">: both
// classes get the mention's background, so the link inside must not draw a second one.
const css = fs.readFileSync(path.join(__dirname, '..', 'styles', '_90_integration.scss'), 'utf8');

describe('a mention of you', () => {
    it('draws one highlight: the link inside it has no background or padding of its own', () => {
        const rule = css.match(/\.mention--highlight \.mention-link \{([^}]*)\}/);

        expect(rule).not.toBeNull();
        expect(rule![1]).toMatch(/background: none/);
        expect(rule![1]).toMatch(/padding: 0/);
    });

    it('comes after the rule that gives both classes the background, so it wins', () => {
        const both = css.indexOf('.mention-link, .mention--highlight {');
        const inner = css.indexOf('.mention--highlight .mention-link {');

        expect(both).toBeGreaterThan(-1);
        expect(inner).toBeGreaterThan(both);
    });
});
