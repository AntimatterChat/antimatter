// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

import React from 'react';

import {renderWithContext, screen, userEvent, waitFor} from 'tests/react_testing_utils';

import OnboardingRulesSettings from './onboarding_rules_settings';

// Each part tells when it has changes of its own.
jest.mock('./community_rules_settings', () => ({setUnsaved}: {setUnsaved: (unsaved: boolean) => void}) => (
    <button
        type='button'
        onClick={() => setUnsaved(true)}
    >
        {'Edit the rules'}
    </button>
));
jest.mock('./onboarding_editor', () => ({setUnsaved}: {setUnsaved: (unsaved: boolean) => void}) => (
    <>
        <button
            type='button'
            onClick={() => setUnsaved(true)}
        >
            {'Edit the onboarding'}
        </button>
        <button
            type='button'
            onClick={() => setUnsaved(false)}
        >
            {'Save the onboarding'}
        </button>
    </>
));

describe('fusion/teams/OnboardingRulesSettings', () => {
    test('the rules, then the onboarding, in one panel', () => {
        renderWithContext(
            <OnboardingRulesSettings
                setUnsaved={jest.fn()}
                onboarding={true}
            />,
        );
        expect(screen.getAllByRole('heading').map((h) => h.textContent)).toEqual(['Rules', 'Onboarding']);
    });

    test('without roles on the server, the rules alone', () => {
        renderWithContext(
            <OnboardingRulesSettings
                setUnsaved={jest.fn()}
                onboarding={false}
            />,
        );
        expect(screen.getAllByRole('heading').map((h) => h.textContent)).toEqual(['Rules']);
        expect(screen.queryByRole('button', {name: 'Edit the onboarding'})).not.toBeInTheDocument();
    });

    test('either part unsaved keeps the panel unsaved', async () => {
        const setUnsaved = jest.fn();
        renderWithContext(
            <OnboardingRulesSettings
                setUnsaved={setUnsaved}
                onboarding={true}
            />,
        );
        await userEvent.click(screen.getByRole('button', {name: 'Edit the rules'}));
        await userEvent.click(screen.getByRole('button', {name: 'Edit the onboarding'}));
        await waitFor(() => expect(setUnsaved).toHaveBeenLastCalledWith(true));

        // The onboarding saved, the rules still aren't.
        await userEvent.click(screen.getByRole('button', {name: 'Save the onboarding'}));
        expect(setUnsaved).toHaveBeenLastCalledWith(true);
    });
});
