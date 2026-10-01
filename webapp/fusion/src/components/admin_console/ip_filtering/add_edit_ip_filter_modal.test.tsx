// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {AllowedIPRange} from '@mattermost/types/config';

import {fireEvent, renderWithContext, userEvent, waitFor} from 'tests/react_testing_utils';

import IPFilteringAddOrEditModal from './add_edit_ip_filter_modal';

jest.mock('components/external_link', () => {
    return jest.fn().mockImplementation(({children, ...props}) => {
        return <a {...props}>{children}</a>;
    });
});

describe('IPFilteringAddOrEditModal', () => {
    const onExited = jest.fn();
    const onSave = jest.fn();
    const existingRange: AllowedIPRange = {
        cidr_block: '192.168.0.0/16',
        description: 'Test IP Filter',
        enabled: true,
        owner_id: '',
    };
    const currentIP = '192.168.0.1';

    const baseProps = {
        onExited,
        onSave,
        existingRange,
        currentIP,
    };

    test('renders the modal with the correct title when an existingRange is provided', () => {
        const {getByText} = renderWithContext(
            <IPFilteringAddOrEditModal
                {...baseProps}
            />,
        );

        expect(getByText('Edit IP Filter')).toBeInTheDocument();
    });

    test('renders the modal with the correct title when an existingRange is omitted (ie, Add Modal)', () => {
        const {getByText} = renderWithContext(
            <IPFilteringAddOrEditModal
                {...baseProps}
                existingRange={undefined}
            />,
        );

        expect(getByText('Add IP Filter')).toBeInTheDocument();
    });

    test('renders the modal with the correct inputs and values', () => {
        const {getByLabelText} = renderWithContext(
            <IPFilteringAddOrEditModal
                {...baseProps}
            />,
        );

        expect(getByLabelText('Enter a name for this rule')).toHaveValue('Test IP Filter');
        expect(getByLabelText('Enter an IP address or range')).toHaveValue('192.168.0.0/16');
    });

    test('calls the onSave function with the correct values when the Save button is clicked', async () => {
        const {getByLabelText, getByTestId} = renderWithContext(
            <IPFilteringAddOrEditModal
                {...baseProps}
            />,
        );

        await userEvent.clear(getByLabelText('Enter a name for this rule'));
        await userEvent.type(getByLabelText('Enter a name for this rule'), 'Test IP Filter 2');
        await userEvent.clear(getByLabelText('Enter an IP address or range'));
        await userEvent.type(getByLabelText('Enter an IP address or range'), '10.0.0.0/8');
        await userEvent.click(getByTestId('save-add-edit-button'));

        await waitFor(() => {
            expect(onSave).toHaveBeenCalledWith({
                cidr_block: '10.0.0.0/8',
                description: 'Test IP Filter 2',
                enabled: true,
                owner_id: '',
                action: 'allow',
            }, existingRange);
            expect(onExited).toHaveBeenCalled();
        });
    });

    test('calls the onSave function with the correct values when the Save button is clicked for a new IP filter', async () => {
        const {getByLabelText, getByTestId} = renderWithContext(
            <IPFilteringAddOrEditModal
                {...baseProps}
                existingRange={undefined}
            />,
        );

        await userEvent.clear(getByLabelText('Enter a name for this rule'));
        await userEvent.type(getByLabelText('Enter a name for this rule'), 'Test IP Filter 2');
        await userEvent.clear(getByLabelText('Enter an IP address or range'));
        await userEvent.type(getByLabelText('Enter an IP address or range'), '10.0.0.0/8');
        await userEvent.click(getByTestId('save-add-edit-button'));

        await waitFor(() => {
            expect(onSave).toHaveBeenCalledWith({
                cidr_block: '10.0.0.0/8',
                description: 'Test IP Filter 2',
                enabled: true,
                owner_id: '',
                action: 'allow',
            });
            expect(onExited).toHaveBeenCalled();
        });
    });

    test('displays an error message when an invalid CIDR is entered', async () => {
        const {getByLabelText, getByTestId, getByText} = renderWithContext(
            <IPFilteringAddOrEditModal
                {...baseProps}
            />,
        );

        await userEvent.clear(getByLabelText('Enter an IP address or range'));
        await userEvent.type(getByLabelText('Enter an IP address or range'), 'invalid-cidr');

        // Trigger validation on blur - fireEvent used because userEvent doesn't have direct focus/blur methods
        fireEvent.blur(getByLabelText('Enter an IP address or range'));
        await userEvent.click(getByTestId('save-add-edit-button'));

        await waitFor(() => {
            expect(getByText('Enter an IP address or a range in CIDR format')).toBeInTheDocument();
            expect(onSave).not.toHaveBeenCalled();
            expect(onExited).not.toHaveBeenCalled();
        });
    });

    test('disables the Save button when an invalid CIDR is entered', async () => {
        const {getByLabelText, getByTestId} = renderWithContext(
            <IPFilteringAddOrEditModal
                {...baseProps}
            />,
        );

        await userEvent.clear(getByLabelText('Enter an IP address or range'));
        await userEvent.type(getByLabelText('Enter an IP address or range'), 'invalid-cidr');

        fireEvent.blur(getByLabelText('Enter an IP address or range'));

        expect(getByTestId('save-add-edit-button')).toBeDisabled();
    });

    test('saves a deny rule when Deny is chosen', async () => {
        const {getByLabelText, getByTestId} = renderWithContext(
            <IPFilteringAddOrEditModal
                {...baseProps}
                existingRange={undefined}
            />,
        );

        await userEvent.type(getByLabelText('Enter a name for this rule'), 'Blocked range');
        await userEvent.type(getByLabelText('Enter an IP address or range'), '192.0.2.7');
        await userEvent.click(getByTestId('ip-filter-action-deny'));
        await userEvent.click(getByTestId('save-add-edit-button'));

        await waitFor(() => {
            expect(onSave).toHaveBeenCalledWith({
                cidr_block: '192.0.2.7',
                description: 'Blocked range',
                enabled: true,
                owner_id: '',
                action: 'deny',
            });
        });
    });
});
