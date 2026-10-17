// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {LAYER_ID} from 'fusion/components/layer';
import {renderWithContext, screen} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import {SchedulePopover} from './schedule';

describe('fusion/composer/SchedulePopover', () => {
    beforeEach(() => {
        const layer = document.createElement('div');
        layer.id = LAYER_ID;
        document.body.appendChild(layer);
    });
    afterEach(() => {
        document.getElementById(LAYER_ID)?.remove();
        jest.restoreAllMocks();
    });

    test('opens above the send button, ending where the button ends', () => {
        jest.spyOn(HTMLElement.prototype, 'offsetWidth', 'get').mockReturnValue(280);
        jest.spyOn(HTMLElement.prototype, 'offsetHeight', 'get').mockReturnValue(240);
        Object.defineProperty(window, 'innerWidth', {configurable: true, value: 1440});
        Object.defineProperty(window, 'innerHeight', {configurable: true, value: 900});

        // The send button's chevron, at the composer's right end.
        const chevron = document.createElement('button');
        document.body.appendChild(chevron);
        chevron.getBoundingClientRect = () => ({left: 1000, right: 1020, top: 820, bottom: 852, width: 20, height: 32, x: 1000, y: 820, toJSON: () => ({})});
        chevron.getClientRects = () => [chevron.getBoundingClientRect()] as unknown as DOMRectList;

        renderWithContext(
            <SchedulePopover
                anchor={chevron}
                onSchedule={jest.fn()}
                onClose={jest.fn()}
            />,
            {
                entities: {
                    users: {currentUserId: 'me', profiles: {me: TestHelper.getUserMock({id: 'me', timezone: {useAutomaticTimezone: 'true', automaticTimezone: 'UTC', manualTimezone: ''}})}},
                    preferences: {myPreferences: {}},
                },
            } as never,
        );

        const menu = screen.getByRole('menu', {name: 'Schedule message'});
        expect(menu.style.left).toBe(`${1020 - 280}px`);
        expect(menu.style.top).toBe(`${820 - 240 - 8}px`);
    });
});
