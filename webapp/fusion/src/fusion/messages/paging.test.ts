// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {pageEdges} from './paging';

describe('pageEdges', () => {
    it('pages from real posts at both ends', () => {
        expect(pageEdges(['p3', 'p2', 'p1'])).toEqual({oldest: 'p1', newest: 'p3'});
    });

    it('pages older from the oldest post of a folded run of join and leave messages, never from the folded row', () => {
        // The oldest rows of a channel whose history starts with people joining (the demo's and many servers' General).
        const items = ['p9', 'date-1760000000000', 'user-activity-p4_p3_p2', 'date-1750000000000'];
        const edges = pageEdges(items);

        expect(edges.oldest).toBe('p2');
        expect(edges.oldest.startsWith('user-activity-')).toBe(false);
        expect(edges.newest).toBe('p9');
    });

    it('pages newer from the newest post of a folded run at the newest end', () => {
        expect(pageEdges(['user-activity-p8_p7', 'p6']).newest).toBe('p8');
    });

    it('skips the date and new-messages lines', () => {
        expect(pageEdges(['start-of-new-messages-1760000000000', 'p2', 'p1', 'date-1760000000000'])).toEqual({oldest: 'p1', newest: 'p2'});
    });

    it('has nothing to page from in an empty list', () => {
        expect(pageEdges([])).toEqual({oldest: '', newest: ''});
    });
});
