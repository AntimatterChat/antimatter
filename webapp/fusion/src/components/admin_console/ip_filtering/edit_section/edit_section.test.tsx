// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {AllowedIPRange} from '@mattermost/types/config';

import {renderWithContext, screen, userEvent} from 'tests/react_testing_utils';

import EditSection from './';

describe('EditSection', () => {
    const ipFilters = [
        {
            cidr_block: '192.168.0.0/24',
            description: 'Test Filter',
        },
    ] as AllowedIPRange[];
    const currentUsersIP = '192.168.0.1';
    const setShowAddModal = jest.fn();
    const setEditFilter = jest.fn();
    const handleConfirmDeleteFilter = jest.fn();
    const currentIPIsInRange = true;

    const baseProps = {
        ipFilters,
        currentUsersIP,
        setShowAddModal,
        setEditFilter,
        handleConfirmDeleteFilter,
        currentIPIsInRange,
    };

    test('renders the component', () => {
        renderWithContext(
            <EditSection
                {...baseProps}
            />,
        );

        expect(screen.getByText('IP Filter Rules')).toBeInTheDocument();
        expect(screen.getByText('Deny rules always block the addresses they match. If any allow rule exists, only addresses matching an allow rule can reach the server.')).toBeInTheDocument();
        expect(screen.getByText('If no rules are enabled, all IP addresses are allowed.')).toBeInTheDocument();
        expect(screen.getByText('Add Filter')).toBeInTheDocument();
        expect(screen.getByText('Filter Name')).toBeInTheDocument();
        expect(screen.getByText('IP Address Range')).toBeInTheDocument();
        expect(screen.getByText('Test Filter')).toBeInTheDocument();
        expect(screen.getByText('192.168.0.0/24')).toBeInTheDocument();
    });

    test('clicking the Add Filter button calls setShowAddModal', async () => {
        renderWithContext(
            <EditSection
                {...baseProps}
            />,
        );

        await userEvent.click(screen.getByText('Add Filter'));

        expect(setShowAddModal).toHaveBeenCalledTimes(1);
        expect(setShowAddModal).toHaveBeenCalledWith(true);
    });

    test('clicking the Edit button calls setEditFilter', async () => {
        renderWithContext(
            <EditSection
                {...baseProps}
            />,
        );

        await userEvent.hover(screen.getByText('Test Filter'));
        await userEvent.click(screen.getByRole('button', {
            name: /Edit/i,
        }));

        expect(setEditFilter).toHaveBeenCalledTimes(1);
        expect(setEditFilter).toHaveBeenCalledWith(ipFilters[0]);
    });

    test('clicking the Delete button calls handleConfirmDeleteFilter', async () => {
        renderWithContext(
            <EditSection
                {...baseProps}
            />,
        );

        await userEvent.hover(screen.getByText('Test Filter'));
        await userEvent.click(screen.getByRole('button', {
            name: /Delete/i,
        }));

        expect(handleConfirmDeleteFilter).toHaveBeenCalledTimes(1);
        expect(handleConfirmDeleteFilter).toHaveBeenCalledWith(ipFilters[0]);
    });

    test('displays an error panel if current IP is not in range', () => {
        renderWithContext(
            <EditSection
                {...baseProps}
                currentUsersIP='192.168.1.1'
                currentIPIsInRange={false}
            />,
        );

        expect(screen.getByText('These rules would block your own IP address 192.168.1.1.')).toBeInTheDocument();
        expect(screen.getByText('Allow your IP address, or remove the deny rule matching it, to continue.')).toBeInTheDocument();
        expect(screen.getByText('Add your IP address')).toBeInTheDocument();
    });

    test('displays a message if no filters are added', () => {
        renderWithContext(
            <EditSection
                {...baseProps}
                ipFilters={[]}
            />,
        );

        expect(screen.getByText('No IP filtering rules added')).toBeInTheDocument();
        expect(screen.getByText('Add a filter')).toBeInTheDocument();
    });
});
