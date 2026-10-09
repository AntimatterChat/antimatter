// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {chosenGrants, parseOnboarding} from './onboarding';

describe('parseOnboarding', () => {
    test('nothing, or nothing to show, is no onboarding', () => {
        expect(parseOnboarding(undefined)).toBeUndefined();
        expect(parseOnboarding('')).toBeUndefined();
        expect(parseOnboarding('not json')).toBeUndefined();
        expect(parseOnboarding('{"welcome":"","questions":[]}')).toBeUndefined();
    });

    test('a welcome alone is an onboarding', () => {
        expect(parseOnboarding('{"welcome":"Hi"}')).toEqual({welcome: 'Hi', questions: []});
    });

    test('drops questions without a title or answers, and answers without a label', () => {
        const parsed = parseOnboarding(JSON.stringify({
            welcome: 'Hi',
            questions: [
                {id: 'q1', title: 'Why?', options: [{id: 'a', label: 'Fun', role_ids: ['r1', 3], channel_ids: 'c1'}, {id: 'b', label: ''}]},
                {id: 'q2', title: '', options: [{id: 'c', label: 'x'}]},
                {id: 'q3', title: 'Empty', options: []},
            ],
        }));
        expect(parsed).toEqual({
            welcome: 'Hi',
            questions: [{id: 'q1', title: 'Why?', multiple: true, options: [{id: 'a', label: 'Fun', role_ids: ['r1'], channel_ids: []}]}],
        });
    });

    test('keeps single-answer questions', () => {
        const parsed = parseOnboarding('{"questions":[{"id":"q","title":"T","multiple":false,"options":[{"id":"a","label":"A"}]}]}');
        expect(parsed?.questions[0].multiple).toBe(false);
    });
});

describe('chosenGrants', () => {
    const onboarding = {
        welcome: '',
        questions: [
            {id: 'q1', title: 'A', multiple: true, options: [{id: 'a', label: 'a', role_ids: ['r1'], channel_ids: ['c1']}, {id: 'b', label: 'b', role_ids: ['r1', 'r2'], channel_ids: []}]},
            {id: 'q2', title: 'B', multiple: false, options: [{id: 'c', label: 'c', role_ids: [], channel_ids: ['c1', 'c2']}]},
        ],
    };

    test('gathers the roles and channels of the chosen answers, once each', () => {
        expect(chosenGrants(onboarding, ['a', 'b', 'c'])).toEqual({roleIds: ['r1', 'r2'], channelIds: ['c1', 'c2']});
    });

    test('nothing chosen grants nothing', () => {
        expect(chosenGrants(onboarding, [])).toEqual({roleIds: [], channelIds: []});
    });
});
