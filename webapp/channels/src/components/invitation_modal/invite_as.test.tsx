// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {screen} from '@testing-library/react';
import React from 'react';

import {renderWithContext} from 'tests/react_testing_utils';

import InviteAs, {InviteType} from './invite_as';

describe('components/invitation_modal/invite_as', () => {
    const props = {
        setInviteAs: jest.fn(),
        inviteType: InviteType.MEMBER,
        titleClass: 'title',
        canInviteGuests: true,
    };

    test('should match snapshot', () => {
        const {container} = renderWithContext(
            <InviteAs {...props}/>,
        );
        expect(container).toMatchSnapshot();
    });

    test('shows the radio buttons', () => {
        renderWithContext(
            <InviteAs {...props}/>,
        );
        expect(screen.getAllByRole('radio')).toHaveLength(2);
    });

    test('guest radio-button is disabled when canInviteGuests prop is false', () => {
        renderWithContext(
            <InviteAs
                {...props}
                canInviteGuests={false}
            />,
        );

        expect(screen.getByDisplayValue('GUEST')).toBeDisabled();
    });

    test('guest radio-button is enabled when canInviteGuests prop is true', () => {
        renderWithContext(
            <InviteAs {...props}/>,
        );

        expect(screen.getByDisplayValue('GUEST')).not.toBeDisabled();
        expect(screen.queryByText('Upgrade')).not.toBeInTheDocument();
    });
});
