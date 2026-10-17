// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {Client4} from 'mattermost-redux/client';

import {renderWithContext, screen, userEvent, waitFor, within} from 'tests/react_testing_utils';

import {attributesState, field} from './attributes_test_utils';
import CommunityAttributesSettings from './community_attributes_dialog';

const mockToast = jest.fn();
jest.mock('fusion/shell/toast_context', () => ({useToast: () => mockToast}));

// Each test has its own community: the attributes are cached by community.
let team = 0;

describe('fusion/attributes/CommunityAttributesSettings', () => {
    beforeEach(() => {
        team++;
        if (!document.getElementById('am-layer')) {
            const layer = document.createElement('div');
            layer.id = 'am-layer';
            document.body.appendChild(layer);
        }
    });

    const render = (fields: Array<ReturnType<typeof field>>) => {
        jest.spyOn(Client4, 'getCommunityAttributes').mockResolvedValue(fields);
        jest.spyOn(Client4, 'createCommunityAttribute').mockResolvedValue(field('new'));
        jest.spyOn(Client4, 'patchCommunityAttribute').mockResolvedValue(field('x'));
        jest.spyOn(Client4, 'deleteCommunityAttribute').mockResolvedValue({status: 'OK'} as never);
        mockToast.mockClear();
        const onClose = jest.fn();
        renderWithContext(
            <CommunityAttributesSettings
                teamId={`team${team}`}
                onClose={onClose}
            />,
            attributesState(`team${team}`) as never,
        );
        return mockToast;
    };

    test('closes the community settings with Close', async () => {
        jest.spyOn(Client4, 'getCommunityAttributes').mockResolvedValue([]);
        const onClose = jest.fn();
        renderWithContext(
            <CommunityAttributesSettings
                teamId={`team${team}`}
                onClose={onClose}
            />,
            attributesState(`team${team}`) as never,
        );
        await userEvent.click(await screen.findByRole('button', {name: 'Close'}));
        expect(onClose).toHaveBeenCalled();
    });

    test('lists the attributes with who sets and who sees them', async () => {
        render([field('Company', {managed: 'admin', visibility: 'admin', member_list: true}), field('Site', {visibility: 'hidden'})]);
        expect(await screen.findByText('Text · set by admins · admins only · in the member list')).toBeInTheDocument();
        expect(screen.getByText('Text · set by members · hidden')).toBeInTheDocument();
    });

    test('says when there are none', async () => {
        render([]);
        expect(await screen.findByText(/No attributes yet/)).toBeInTheDocument();
    });

    test('adds a choice attribute only admins set and see', async () => {
        render([]);
        await userEvent.click(await screen.findByRole('button', {name: 'Add attribute'}));
        const add = screen.getAllByRole('button', {name: 'Add attribute'}).at(-1)!;
        expect(add).toBeDisabled();

        await userEvent.type(screen.getByPlaceholderText('e.g. Company'), 'Company');
        await userEvent.selectOptions(screen.getByRole('combobox'), 'select');

        // A choice needs at least one choice.
        expect(add).toBeDisabled();
        await userEvent.type(screen.getByRole('textbox', {name: 'Choice 1'}), 'Lumen');
        await userEvent.click(screen.getByRole('button', {name: 'Add a choice'}));
        await userEvent.type(screen.getByRole('textbox', {name: 'Choice 2'}), 'Acme');

        await userEvent.click(within(screen.getByRole('group', {name: 'Who sets it'})).getByRole('button', {name: 'Admins'}));
        await userEvent.click(within(screen.getByRole('group', {name: 'Who sees it'})).getByRole('button', {name: 'Admins'}));
        await userEvent.click(screen.getByRole('switch', {name: 'Show in the member list'}));
        await userEvent.click(add);

        await waitFor(() => expect(Client4.createCommunityAttribute).toHaveBeenCalledWith(`team${team}`, {
            name: 'Company',
            type: 'select',
            attrs: {
                managed: 'admin',
                visibility: 'admin',
                member_list: true,
                options: [{id: '', name: 'Lumen', color: ''}, {id: '', name: 'Acme', color: ''}],
            },
        }));
    });

    test('an existing attribute keeps its kind', async () => {
        render([field('Company')]);
        await userEvent.click(await screen.findByRole('button', {name: 'Edit the attribute'}));
        expect(screen.getByRole('combobox')).toBeDisabled();
        await userEvent.type(screen.getByPlaceholderText('e.g. Company'), ' name');
        await userEvent.click(screen.getByRole('button', {name: 'Save'}));
        await waitFor(() => expect(Client4.patchCommunityAttribute).toHaveBeenCalledWith(`team${team}`, 'Company', expect.objectContaining({name: 'Company name'})));
    });

    test('deleting asks first', async () => {
        const toast = render([field('Company')]);
        await userEvent.click(await screen.findByRole('button', {name: 'Delete the attribute'}));
        expect(screen.getByText('Delete it and everyone\'s values?')).toBeInTheDocument();
        expect(Client4.deleteCommunityAttribute).not.toHaveBeenCalled();

        await userEvent.click(screen.getByRole('button', {name: 'Delete'}));
        await waitFor(() => expect(Client4.deleteCommunityAttribute).toHaveBeenCalledWith(`team${team}`, 'Company'));
        expect(toast).toHaveBeenCalledWith('Deleted the attribute Company');
    });

    test('tells why an attribute couldn\'t be saved', async () => {
        render([]);
        jest.mocked(Client4.createCommunityAttribute).mockRejectedValueOnce({message: 'Name taken'});
        await userEvent.click(await screen.findByRole('button', {name: 'Add attribute'}));
        await userEvent.type(screen.getByPlaceholderText('e.g. Company'), 'Company');
        await userEvent.click(screen.getAllByRole('button', {name: 'Add attribute'}).at(-1)!);
        expect(await screen.findByRole('alert')).toHaveTextContent('Name taken');
    });
});
