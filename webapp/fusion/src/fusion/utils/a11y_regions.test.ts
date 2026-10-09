// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {A11yAttributeNames, A11yClassNames} from 'utils/constants';

import {a11yRegion} from './a11y_regions';

describe('a11yRegion', () => {
    test('keeps the pane\'s class and adds the region class', () => {
        expect(a11yRegion('am-sidebar', 'sidebar').className).toBe(`am-sidebar ${A11yClassNames.REGION}`);
    });

    test('F6 goes rail, sidebar, header, messages, composer, then the right-hand side', () => {
        const order = (['rhs', 'composer', 'messages', 'header', 'sidebar', 'rail'] as const).
            map((region) => ({region, place: a11yRegion('x', region)[A11yAttributeNames.SORT_ORDER] as number})).
            sort((a, b) => a.place - b.place).
            map((r) => r.region);
        expect(order).toEqual(['rail', 'sidebar', 'header', 'messages', 'composer', 'rhs']);
    });
});
