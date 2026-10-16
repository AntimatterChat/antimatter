// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {UserProfile} from '@mattermost/types/users';

import {Client4} from 'mattermost-redux/client';

import webSocketClient from 'client/web_websocket_client';
import {act, renderWithContext, screen, userEvent, waitFor} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import {attributesState, field} from './attributes_test_utils';
import {saveMemberAttributeValues, useMemberAttributeValues} from './community_attributes';
import {useMemberListAttribute} from './member_list_attribute';

// The server's "attributes changed" events reach the cache through the one listener it adds.
let mockListener: ((msg: unknown) => void) | undefined;
jest.spyOn(webSocketClient, 'addMessageListener').mockImplementation((listener) => {
    mockListener = listener as never;
});

let team = 0;
const people = ['ana', 'ben', 'cleo'].map((id) => TestHelper.getUserMock({id, username: id}));

function Members({profiles}: {profiles: UserProfile[]}) {
    const attribute = useMemberListAttribute(`team${team}`, profiles);
    return (
        <>
            {attribute.filter}
            {profiles.filter((p) => attribute.shown(p.id)).map((p) => (
                <div
                    key={p.id}
                    data-testid='member'
                >
                    {`${p.username}: ${attribute.textOf(p.id)}`}
                </div>
            ))}
        </>
    );
}

function Values({userId}: {userId: string}) {
    const values = useMemberAttributeValues(`team${team}`, userId);
    return <div data-testid='values'>{JSON.stringify(values || null)}</div>;
}

describe('fusion/attributes/community_attributes', () => {
    beforeEach(() => {
        team++;
        if (!document.getElementById('am-layer')) {
            const layer = document.createElement('div');
            layer.id = 'am-layer';
            document.body.appendChild(layer);
        }
    });

    const company = field('company', {member_list: true, options: [{id: 'l', name: 'Lumen', color: ''}, {id: 'a', name: 'Acme', color: ''}]}, 'select');

    test('the member list shows the listed attribute and filters by its choices', async () => {
        jest.spyOn(Client4, 'getCommunityAttributes').mockResolvedValue([field('site'), company]);
        jest.spyOn(Client4, 'getCommunityAttributeValues').mockResolvedValue({ana: {company: 'l'}, ben: {company: 'a'}});
        renderWithContext(<Members profiles={people}/>, attributesState(`team${team}`) as never);

        expect(await screen.findByText('ana: Lumen')).toBeInTheDocument();
        expect(screen.getByText('ben: Acme')).toBeInTheDocument();
        expect(screen.getByText('cleo:')).toBeInTheDocument();

        await userEvent.click(screen.getByRole('button', {name: 'Filter by company'}));
        await userEvent.click(screen.getByRole('menuitemradio', {name: 'Acme'}));
        expect(screen.getAllByTestId('member').map((el) => el.textContent)).toEqual(['ben: Acme']);

        await userEvent.click(screen.getByRole('button', {name: 'Filter by company'}));
        await userEvent.click(screen.getByRole('menuitemradio', {name: 'Everyone'}));
        expect(screen.getAllByTestId('member')).toHaveLength(3);
    });

    test('nothing is listed or filterable when no attribute is marked for the member list', async () => {
        jest.spyOn(Client4, 'getCommunityAttributes').mockResolvedValue([field('site')]);
        jest.spyOn(Client4, 'getCommunityAttributeValues').mockResolvedValue({});
        renderWithContext(<Members profiles={people}/>, attributesState(`team${team}`) as never);
        await waitFor(() => expect(Client4.getCommunityAttributes).toHaveBeenCalled());
        expect(screen.queryByRole('button', {name: /Filter by/})).not.toBeInTheDocument();
        expect(Client4.getCommunityAttributeValues).not.toHaveBeenCalled();
    });

    test('values are asked for again when the server says someone\'s changed', async () => {
        jest.spyOn(Client4, 'getCommunityAttributes').mockResolvedValue([company]);
        const values = jest.spyOn(Client4, 'getCommunityAttributeValues').mockResolvedValue({ana: {company: 'l'}});
        renderWithContext(<Values userId='ana'/>, attributesState(`team${team}`) as never);
        await waitFor(() => expect(screen.getByTestId('values')).toHaveTextContent('{"company":"l"}'));

        values.mockResolvedValue({ana: {company: 'a'}});
        act(() => mockListener?.({event: 'community_attributes_updated', data: {team_id: `team${team}`, user_id: 'ana'}}));
        await waitFor(() => expect(screen.getByTestId('values')).toHaveTextContent('{"company":"a"}'));

        // Another community's events are not this one's.
        values.mockClear();
        act(() => mockListener?.({event: 'community_attributes_updated', data: {team_id: 'elsewhere', user_id: 'ana'}}));
        expect(values).not.toHaveBeenCalled();
    });

    test('saved values show at once', async () => {
        jest.spyOn(Client4, 'getCommunityAttributes').mockResolvedValue([company]);
        jest.spyOn(Client4, 'getCommunityAttributeValues').mockResolvedValue({ana: {company: 'l'}});
        jest.spyOn(Client4, 'patchCommunityAttributeValues').mockResolvedValue({company: 'a'});
        renderWithContext(<Values userId='ana'/>, attributesState(`team${team}`) as never);
        await waitFor(() => expect(screen.getByTestId('values')).toHaveTextContent('{"company":"l"}'));

        await act(() => saveMemberAttributeValues(`team${team}`, 'ana', {company: 'a'}));
        expect(screen.getByTestId('values')).toHaveTextContent('{"company":"a"}');
    });
});
