// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {fakeMatchMedia} from 'fusion/hooks/media_test_utils';
import {renderWithContext, screen, userEvent} from 'tests/react_testing_utils';

import PresetPicker from './preset_picker';

describe('PresetPicker', () => {
    test('a locked type shows a line naming it, no type switch', () => {
        renderWithContext(
            <PresetPicker
                lockedType='forum'
                value='forum_agora'
                onChange={jest.fn()}
            />,
        );

        expect(screen.getByText('Forum. The type is set when the channel is created.')).toBeInTheDocument();
        expect(screen.queryByRole('radiogroup', {name: 'Type'})).not.toBeInTheDocument();
        expect(screen.getByRole('radio', {name: /^Discussion/})).toHaveAttribute('aria-checked', 'true');
    });

    test('keyboard: arrows move the type switch and the preset grid, each within itself', async () => {
        const onTypeChange = jest.fn();
        const onChange = jest.fn();
        renderWithContext(
            <PresetPicker
                type='text'
                value='text_chat'
                onTypeChange={onTypeChange}
                onChange={onChange}
            />,
            {plugins: {plugins: {'com.antimatterchat.voice-channels': {id: 'com.antimatterchat.voice-channels', version: '1.0.0'}}}} as never,
        );

        const text = screen.getByRole('radio', {name: 'Text'});
        text.focus();
        await userEvent.keyboard('{ArrowRight}');
        expect(onTypeChange).toHaveBeenCalledWith('voice');

        const chat = screen.getByRole('radio', {name: /^Chat/});
        chat.focus();
        await userEvent.keyboard('{ArrowRight}');
        expect(onChange).toHaveBeenCalledWith('text_announcement');
    });

    test('a disabled Voice segment (the app is not active) is skipped by the keyboard, not switched to', async () => {
        const onTypeChange = jest.fn();
        renderWithContext(
            <PresetPicker
                type='text'
                value='text_chat'
                onTypeChange={onTypeChange}
                onChange={jest.fn()}
            />,
        );

        expect(screen.getByRole('radio', {name: 'Voice'})).toBeDisabled();
        const text = screen.getByRole('radio', {name: 'Text'});
        text.focus();
        await userEvent.keyboard('{ArrowRight}');
        expect(onTypeChange).not.toHaveBeenCalled();
    });

    test('phone width adds the phone layout class', () => {
        fakeMatchMedia((query) => query.includes('max-width: 640px'));
        const {container} = renderWithContext(
            <PresetPicker
                type='text'
                value='text_chat'
                onTypeChange={jest.fn()}
                onChange={jest.fn()}
            />,
        );

        expect(container.querySelector('.am-preset-picker')).toHaveClass('am-phone');
    });

    test('not on a phone: no phone layout class', () => {
        fakeMatchMedia(() => false);
        const {container} = renderWithContext(
            <PresetPicker
                type='text'
                value='text_chat'
                onTypeChange={jest.fn()}
                onChange={jest.fn()}
            />,
        );

        expect(container.querySelector('.am-preset-picker')).not.toHaveClass('am-phone');
    });
});
