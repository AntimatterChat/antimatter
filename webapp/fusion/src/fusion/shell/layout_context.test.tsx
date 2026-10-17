// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {renderWithContext, screen} from 'tests/react_testing_utils';

import {LayoutProvider, useLayout} from './layout_context';

function Members() {
    const layout = useLayout();
    return <span>{layout.showMembers ? 'members shown' : 'members hidden'}</span>;
}

describe('fusion/shell/LayoutProvider', () => {
    const width = window.innerWidth;
    const render = (simplified: boolean) => renderWithContext(
        <LayoutProvider><Members/></LayoutProvider>,
        {
            entities: {
                users: {currentUserId: 'me'},
                preferences: {myPreferences: simplified ? {'fusion_display--simplified_interface': {user_id: 'me', category: 'fusion_display', name: 'simplified_interface', value: 'true'}} : {}},
            },
        } as never,
    );

    beforeEach(() => {
        Object.defineProperty(window, 'innerWidth', {configurable: true, value: 1400});
    });
    afterEach(() => {
        Object.defineProperty(window, 'innerWidth', {configurable: true, value: width});
    });

    test('shows the member list beside the conversation on a wide screen', () => {
        render(false);

        expect(screen.getByText('members shown')).toBeInTheDocument();
    });

    test('keeps the member list closed in the simplified interface, until it is opened', () => {
        render(true);

        expect(screen.getByText('members hidden')).toBeInTheDocument();
    });
});
