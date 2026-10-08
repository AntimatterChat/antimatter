// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {UserPropertyField} from '@mattermost/types/properties_user';

import {attributeText} from './profile_facts';

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
