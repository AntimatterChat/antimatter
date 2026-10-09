// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {Team} from '@mattermost/types/teams';

import {LAYER_ID} from 'fusion/components/layer';
import {renderWithContext, screen, userEvent, waitFor} from 'tests/react_testing_utils';

import {finishOnboarding} from './onboarding';
import type {Onboarding} from './onboarding';
import OnboardingDialog from './onboarding_dialog';

jest.mock('./onboarding', () => ({
    ...jest.requireActual('./onboarding'),
    finishOnboarding: jest.fn(() => ({type: 'FINISHED'})),
}));

const finish = finishOnboarding as jest.Mock;

describe('OnboardingDialog', () => {
    const team = {id: 'team', display_name: 'Antimatter'} as Team;
    const onboarding: Onboarding = {
        welcome: 'Hello there',
        questions: [
            {id: 'q1', title: 'What brings you here?', multiple: true, options: [{id: 'games', label: 'Games', role_ids: [], channel_ids: []}, {id: 'dev', label: 'Development', role_ids: [], channel_ids: []}]},
            {id: 'q2', title: 'Where are you?', multiple: false, options: [{id: 'eu', label: 'Europe', role_ids: [], channel_ids: []}, {id: 'us', label: 'Americas', role_ids: [], channel_ids: []}]},
        ],
    };

    const render = () => {
        const onClose = jest.fn();
        renderWithContext(
            <OnboardingDialog
                team={team}
                onboarding={onboarding}
                onClose={onClose}
            />,
        );
        return onClose;
    };

    beforeEach(() => {
        finish.mockClear();
        const layer = document.createElement('div');
        layer.id = LAYER_ID;
        document.body.appendChild(layer);
    });

    afterEach(() => {
        document.getElementById(LAYER_ID)?.remove();
    });

    test('welcomes with the team\'s name, its welcome and its questions', () => {
        render();
        expect(screen.getByRole('heading', {name: 'Welcome to Antimatter'})).toBeInTheDocument();
        expect(screen.getByText('Hello there')).toBeInTheDocument();
        expect(screen.getByRole('group', {name: 'What brings you here?'})).toBeInTheDocument();
        expect(screen.getByRole('radiogroup', {name: 'Where are you?'})).toBeInTheDocument();
    });

    test('several answers to one question, one to the other', async () => {
        const onClose = render();
        await userEvent.click(screen.getByRole('checkbox', {name: 'Games'}));
        await userEvent.click(screen.getByRole('checkbox', {name: 'Development'}));
        await userEvent.click(screen.getByRole('radio', {name: 'Europe'}));
        await userEvent.click(screen.getByRole('radio', {name: 'Americas'}));
        expect(screen.getByRole('radio', {name: 'Europe'})).toHaveAttribute('aria-checked', 'false');
        await userEvent.click(screen.getByRole('checkbox', {name: 'Games'}));
        await userEvent.click(screen.getByRole('button', {name: 'Done'}));
        await waitFor(() => expect(onClose).toHaveBeenCalled());
        expect(finish).toHaveBeenCalledWith('team', onboarding, ['dev', 'us']);
    });

    test('skipping gives nothing but still counts as done', async () => {
        const onClose = render();
        await userEvent.click(screen.getByRole('checkbox', {name: 'Games'}));
        await userEvent.click(screen.getByRole('button', {name: 'Skip'}));
        await waitFor(() => expect(onClose).toHaveBeenCalled());
        expect(finish).toHaveBeenCalledWith('team', onboarding, []);
    });
});
