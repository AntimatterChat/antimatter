// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {createIntl} from 'react-intl';

import {attributeLabel, attributeValueText, isEmptyValue} from './attribute_inputs';
import type {AttributeField} from './attribute_inputs';

const intl = createIntl({locale: 'en', messages: {}});
const options = [{id: 'a', name: 'Lumen', color: ''}, {id: 'b', name: 'Acme', color: ''}];
const field = (type: string, attrs: AttributeField['attrs'] = {}): AttributeField => ({id: 'f', name: 'company', type, attrs});

describe('attribute values as text', () => {
    test('choices show their names, unknown ones dropped', () => {
        expect(attributeValueText(intl, field('select', {options}), 'b')).toBe('Acme');
        expect(attributeValueText(intl, field('multiselect', {options}), ['a', 'gone', 'b'])).toBe('Lumen, Acme');
    });

    test('dates are written in the reader\'s language, texts as they are', () => {
        expect(attributeValueText(intl, field('date'), '2026-10-13')).toBe('Oct 13, 2026');
        expect(attributeValueText(intl, field('date'), 'soon')).toBe('soon');
        expect(attributeValueText(intl, field('text'), 'Paris')).toBe('Paris');
    });

    test('empty values', () => {
        expect(isEmptyValue('')).toBe(true);
        expect(isEmptyValue([])).toBe(true);
        expect(isEmptyValue(undefined)).toBe(true);
        expect(isEmptyValue(['a'])).toBe(false);
        expect(attributeValueText(intl, field('text'), undefined)).toBe('');
    });

    test('an attribute is labelled by its display name, else its name', () => {
        expect(attributeLabel(field('text', {display_name: ' Company '}))).toBe('Company');
        expect(attributeLabel(field('text'))).toBe('company');
    });
});
