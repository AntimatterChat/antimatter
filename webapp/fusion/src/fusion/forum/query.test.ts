// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {buildTopicsQuery, defaultSortForMode, filtersForMode, sortsForMode} from './query';

describe('fusion/forum/query', () => {
    test('sortsForMode: discussion has activity (default) and newest', () => {
        expect(sortsForMode('discussion').map((s) => s.id)).toEqual(['activity', 'newest']);
        expect(defaultSortForMode('discussion')).toBe('activity');
    });

    test('sortsForMode: qa has unanswered (default), newest, votes', () => {
        expect(sortsForMode('qa').map((s) => s.id)).toEqual(['unanswered', 'newest', 'votes']);
        expect(defaultSortForMode('qa')).toBe('unanswered');
    });

    test('sortsForMode: ideas has votes (default), newest, status', () => {
        expect(sortsForMode('ideas').map((s) => s.id)).toEqual(['votes', 'newest', 'status']);
        expect(defaultSortForMode('ideas')).toBe('votes');
    });

    test('filtersForMode: the base four, plus Unanswered for qa', () => {
        expect(filtersForMode('qa').map((f) => f.id)).toEqual(['all', 'unread', 'mine', 'following', 'unanswered']);
    });

    test('filtersForMode: the base four, plus a chip per status for ideas', () => {
        expect(filtersForMode('ideas').map((f) => f.id)).toEqual(['all', 'unread', 'mine', 'following', 'status:open', 'status:planned', 'status:in_progress', 'status:done', 'status:declined']);
    });

    test('filtersForMode: just the base four for discussion', () => {
        expect(filtersForMode('discussion').map((f) => f.id)).toEqual(['all', 'unread', 'mine', 'following']);
    });

    test('buildTopicsQuery: all/unread filters add nothing but sort', () => {
        expect(buildTopicsQuery('activity', 'all', '')).toEqual({sort: 'activity'});
        expect(buildTopicsQuery('activity', 'unread', '')).toEqual({sort: 'activity'});
    });

    test('buildTopicsQuery: mine, following, unanswered, and a status chip', () => {
        expect(buildTopicsQuery('newest', 'mine', '')).toEqual({sort: 'newest', mine: true});
        expect(buildTopicsQuery('newest', 'following', '')).toEqual({sort: 'newest', following: true});
        expect(buildTopicsQuery('unanswered', 'unanswered', '')).toEqual({sort: 'unanswered', answered: false});
        expect(buildTopicsQuery('votes', 'status:planned', '')).toEqual({sort: 'votes', status: 'planned'});
    });

    test('buildTopicsQuery: a trimmed search term becomes q', () => {
        expect(buildTopicsQuery('activity', 'all', '  beam align  ')).toEqual({sort: 'activity', q: 'beam align'});
        expect(buildTopicsQuery('activity', 'all', '   ')).toEqual({sort: 'activity'});
    });

    test('buildTopicsQuery: before carries the previous page\'s cursor', () => {
        expect(buildTopicsQuery('activity', 'all', '', 'abc')).toEqual({sort: 'activity', before: 'abc'});
    });
});
