// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {UserPropertyField} from '@mattermost/types/properties_user';

import {attributesState} from 'fusion/attributes/attributes_test_utils';
import {renderWithContext, screen} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import ProfileFacts, {attributeText} from './profile_facts';

const field = (type: string, options?: Array<{id: string; name: string}>) => ({id: 'f', name: 'Field', type, attrs: {options}} as unknown as UserPropertyField);

describe('attributeText', () => {
    test('shows text values as they are', () => {
        expect(attributeText(field('text'), 'Paris')).toBe('Paris');
    });

    test('shows the names of the chosen options', () => {
        const options = [{id: 'a', name: 'Red'}, {id: 'b', name: 'Blue'}];
        expect(attributeText(field('select', options), 'b')).toBe('Blue');
        expect(attributeText(field('multiselect', options), ['a', 'b'])).toBe('Red, Blue');
    });

    test('drops options that no longer exist, and empty values', () => {
        expect(attributeText(field('select', [{id: 'a', name: 'Red'}]), 'gone')).toBe('');
        expect(attributeText(field('text'), '')).toBe('');
        expect(attributeText(field('text'), undefined)).toBe('');
    });
});

describe('ProfileFacts', () => {
    const serverFields = {
        team: {id: 'team', name: 'Team', type: 'text', attrs: {visibility: 'when_set', sort_order: 0}},
        clr: {id: 'clr', name: 'Clearance', type: 'text', attrs: {visibility: 'admin', sort_order: 1}},
        hidden: {id: 'hidden', name: 'Badge', type: 'text', attrs: {visibility: 'hidden', sort_order: 2}},
    };

    test('a bot\'s card shows its server attributes, but not its email', () => {
        const bot = TestHelper.getUserMock({id: 'bot', username: 'bot', is_bot: true, email: 'bot@example.com', custom_profile_attributes: {team: 'Platform', clr: 'Top', hidden: 'B-7'}});
        renderWithContext(<ProfileFacts user={bot}/>, attributesState('t1', serverFields, {bot}) as never);

        expect(screen.getByText('Platform')).toBeInTheDocument();
        expect(screen.getByText('Top')).toBeInTheDocument();
        expect(screen.getByTitle('Clearance · Only admins and those who manage this bot see this')).toBeInTheDocument();
        expect(screen.queryByText('B-7')).not.toBeInTheDocument();
        expect(screen.queryByText('bot@example.com')).not.toBeInTheDocument();
    });

    test('a person\'s admin-only line still says only admins and they see it', () => {
        const person = TestHelper.getUserMock({id: 'sofia', username: 'sofia', custom_profile_attributes: {clr: 'Top'}});
        renderWithContext(<ProfileFacts user={person}/>, attributesState('t1', serverFields, {sofia: person}) as never);
        expect(screen.getByTitle('Clearance · Only admins and this person see this')).toBeInTheDocument();
    });
});
