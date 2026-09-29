// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {type JSX} from 'react';
import {IntlProvider} from 'react-intl';
import {Provider} from 'react-redux';
import {BrowserRouter as Router} from 'react-router-dom';

import type {AllowedIPRange, FetchIPResponse} from '@mattermost/types/config';

import {Client4} from 'mattermost-redux/client';

import configureStore from 'store';

import ModalController from 'components/modal_controller';

import {render, screen, userEvent, waitFor} from 'tests/react_testing_utils';

import IPFiltering from './index';

jest.mock('mattermost-redux/client');

describe('IPFiltering', () => {
    const ipFilters = [
        {
            cidr_block: '10.0.0.0/8',
            description: 'Test IP Filter',
            enabled: true,
        },
    ] as AllowedIPRange[];

    const intlProviderProps = {
        defaultLocale: 'en',
        locale: 'en',
    };
    const currentIP = '10.0.0.1';
    const applyIPFiltersMock = jest.fn(() => Promise.resolve(ipFilters));
    const getIPFiltersMock = jest.fn(() => Promise.resolve(ipFilters));
    const getCurrentIPMock = jest.fn(() => Promise.resolve({ip: currentIP} as FetchIPResponse));

    beforeEach(() => {
        Client4.applyIPFilters = applyIPFiltersMock;
        Client4.getIPFilters = getIPFiltersMock;
        Client4.getCurrentIP = getCurrentIPMock;
    });

    const mockedStore = configureStore({
        entities: {
            users: {
                currentUserId: 'current_user_id',
            },
            general: {
                config: {},
                license: {},
            },
        },
        views: {
            admin: {
                navigationBlock: {
                    blocked: false,
                },
            },
        },
    });

    const wrapWithIntlProviderAndStore = (component: JSX.Element) => (
        <Router>
            <IntlProvider {...intlProviderProps}>
                <Provider store={mockedStore} >
                    <ModalController/>
                    {component}
                </Provider>
            </IntlProvider>
        </Router>
    );

    test('renders the IP Filtering page', async () => {
        const {getByText} = render(wrapWithIntlProviderAndStore(<IPFiltering/>));

        expect(getByText('IP Filtering')).toBeInTheDocument();
        expect(getByText('Enable IP Filtering')).toBeInTheDocument();

        await waitFor(() => {
            expect(getByText('Add Filter')).toBeInTheDocument();
            expect(getByText('Test IP Filter')).toBeInTheDocument();
            expect(getByText('10.0.0.0/8')).toBeInTheDocument();
        });

        expect(getByText('Save')).toBeInTheDocument();
    });

    test('disables IP Filtering when the toggle is turned off', async () => {
        render(wrapWithIntlProviderAndStore(<IPFiltering/>));

        await waitFor(() => {
            expect(screen.getByTestId('filterToggle-button')).toBeInTheDocument();
            expect(screen.getByRole('button', {pressed: true})).toBeInTheDocument();
        });

        await userEvent.click(screen.getByTestId('filterToggle-button'));

        await waitFor(() => {
            expect(screen.getByRole('button', {pressed: false})).toBeInTheDocument();
        });
    });

    test('adds a new IP filter when the "Add IP Filter" button is clicked', async () => {
        const {getByLabelText, getByText} = render(wrapWithIntlProviderAndStore(<IPFiltering/>));

        await waitFor(() => {
            expect(getByText('Add Filter')).toBeInTheDocument();
        });

        await userEvent.click(getByText('Add Filter'));

        const descriptionInput = getByLabelText('Enter a name for this rule');
        const cidrInput = getByLabelText('Enter an IP address or range');
        const saveButton = screen.getByTestId('save-add-edit-button');

        await userEvent.clear(cidrInput);
        await userEvent.type(cidrInput, '192.168.0.0/16');
        await userEvent.clear(descriptionInput);
        await userEvent.type(descriptionInput, 'Test IP Filter 2');
        await userEvent.click(saveButton);

        await waitFor(() => {
            expect(getByText('Test IP Filter 2')).toBeInTheDocument();
            expect(getByText('192.168.0.0/16')).toBeInTheDocument();
        });
    });

    test('edits an existing IP filter when the "Edit" button is clicked', async () => {
        const {getByLabelText, getByText, queryByText} = render(wrapWithIntlProviderAndStore(<IPFiltering/>));

        await waitFor(() => {
            expect(getByText('Test IP Filter')).toBeInTheDocument();
        });

        await userEvent.hover(screen.getByText('Test IP Filter'));
        await userEvent.click(screen.getByRole('button', {
            name: /Edit/i,
        }));

        const descriptionInput = getByLabelText('Enter a name for this rule');
        const cidrInput = getByLabelText('Enter an IP address or range');
        const saveButton = screen.getByTestId('save-add-edit-button');

        await userEvent.clear(cidrInput);
        await userEvent.type(cidrInput, '192.168.0.0/16');
        await userEvent.clear(descriptionInput);
        await userEvent.type(descriptionInput, 'zzzzzfilter');
        await userEvent.click(saveButton);

        await waitFor(() => {
            expect(getByText('zzzzzfilter')).toBeInTheDocument();
            expect(getByText('192.168.0.0/16')).toBeInTheDocument();

            // ensure that the old description is gone, because we've now changed it
            expect(queryByText('Test IP Filter')).toBeNull();
        });
    });

    test('deletes an existing IP filter when the "Delete" button is clicked', async () => {
        const {getByText, queryByText} = render(wrapWithIntlProviderAndStore(<IPFiltering/>));

        await waitFor(() => {
            expect(getByText('Test IP Filter')).toBeInTheDocument();
        });

        await userEvent.hover(screen.getByText('Test IP Filter'));
        await userEvent.click(screen.getByRole('button', {
            name: /Delete/i,
        }));

        const confirmButton = getByText('Delete filter');

        await userEvent.click(confirmButton);

        await waitFor(() => {
            expect(queryByText('Test IP Filter')).not.toBeInTheDocument();
        });
    });

    test('saves changes when the "Save" button is clicked', async () => {
        const {getByText, queryByText} = render(wrapWithIntlProviderAndStore(<IPFiltering/>));

        await waitFor(() => {
            expect(screen.getByTestId('filterToggle-button')).toBeInTheDocument();
            expect(screen.getByRole('button', {pressed: true})).toBeInTheDocument();
        });

        await userEvent.click(screen.getByTestId('filterToggle-button'));

        await waitFor(() => {
            expect(screen.getByRole('button', {pressed: false})).toBeInTheDocument();
        });

        await waitFor(() => {
            expect(queryByText('Test IP Filter')).not.toBeInTheDocument();
        });

        await userEvent.click(getByText('Save'));
        await userEvent.click(screen.getByTestId('save-confirmation-button'));

        await waitFor(() => {
            expect(applyIPFiltersMock).toHaveBeenCalledTimes(1);
        });
    });

    test('Save button is disabled when users IP is not within the allowed ranges', async () => {
        const {getByLabelText, getByText, queryByText, getByTestId} = render(wrapWithIntlProviderAndStore(<IPFiltering/>));

        await waitFor(() => {
            expect(getByText('Test IP Filter')).toBeInTheDocument();
        });

        await userEvent.hover(screen.getByText('Test IP Filter'));
        await userEvent.click(screen.getByRole('button', {
            name: /Edit/i,
        }));

        const descriptionInput = getByLabelText('Enter a name for this rule');
        const cidrInput = getByLabelText('Enter an IP address or range');
        const saveButton = screen.getByTestId('save-add-edit-button');

        await userEvent.clear(cidrInput);
        await userEvent.type(cidrInput, '192.168.0.0/16');
        await userEvent.clear(descriptionInput);
        await userEvent.type(descriptionInput, 'zzzzzfilter');
        await userEvent.click(saveButton);

        await waitFor(() => {
            expect(getByText('zzzzzfilter')).toBeInTheDocument();
            expect(getByText('192.168.0.0/16')).toBeInTheDocument();

            // ensure that the old description is gone, because we've now changed it
            expect(queryByText('Test IP Filter')).toBeNull();
            expect(getByTestId('saveSetting')).toBeDisabled();
        });
    });

    test('shows the server error when the rules are refused', async () => {
        Client4.applyIPFilters = jest.fn(() => Promise.reject(new Error('These rules would block your own address')));

        render(wrapWithIntlProviderAndStore(<IPFiltering/>));

        await waitFor(() => {
            expect(screen.getByText('Test IP Filter')).toBeInTheDocument();
        });

        await userEvent.hover(screen.getByText('Test IP Filter'));
        await userEvent.click(screen.getByRole('button', {name: /Delete/i}));
        await userEvent.click(screen.getByText('Delete filter'));
        await userEvent.click(screen.getByText('Save'));
        await userEvent.click(screen.getByText('Yes, apply changes'));

        await waitFor(() => {
            expect(screen.getByText('These rules would block your own address', {exact: false})).toBeInTheDocument();
        });
        expect(screen.getByText('Save')).toBeInTheDocument();
    });
});
