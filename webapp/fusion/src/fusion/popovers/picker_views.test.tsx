// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useState} from 'react';

import {fireEvent, renderWithContext, screen, userEvent, within} from 'tests/react_testing_utils';

import {PickerDropZone, PickerViewFrame} from './picker_views';

// The drop zone of every add view (custom emoji here, and the GIFs, stickers and memes apps' through the picker kit).
function Zone({error, preview}: {error?: string; preview?: boolean}) {
    const [file, setFile] = useState<File | null>(null);
    return (
        <PickerViewFrame
            title='Add a sticker'
            onBack={jest.fn()}
        >
            <PickerDropZone
                accept='image/png'
                file={file}
                onFile={setFile}
                label='Choose a file, drop it here or paste it'
                changeLabel='Choose another file'
                hint='PNG, up to 16 MB.'
                error={error}
                preview={preview && file ? <span>{'Preview'}</span> : undefined}
            />
        </PickerViewFrame>
    );
}

const zone = () => document.querySelector('.am-pv-drop') as HTMLElement;
const fileInput = () => document.querySelector('input[type="file"]') as HTMLInputElement;

describe('fusion/popovers/PickerDropZone', () => {
    beforeAll(() => {
        URL.createObjectURL = jest.fn(() => 'blob:preview');
        URL.revokeObjectURL = jest.fn();
    });

    test('has one upload control: a button the size of the zone, with the types and limits under it', () => {
        renderWithContext(<Zone/>);

        const buttons = within(zone()).getAllByRole('button');
        expect(buttons).toHaveLength(1);
        expect(buttons[0]).toHaveAccessibleName('Choose a file, drop it here or paste it');
        expect(buttons[0]).toHaveAccessibleDescription('PNG, up to 16 MB.');

        // Nothing else in the view uploads: no second button, nor a visible file field.
        expect(screen.getAllByRole('button', {name: /choose|upload/i})).toHaveLength(1);
        expect(fileInput()).not.toBeVisible();
    });

    test('the button opens the file chooser', async () => {
        renderWithContext(<Zone/>);
        const click = jest.spyOn(fileInput(), 'click');
        await userEvent.click(within(zone()).getByRole('button'));
        expect(click).toHaveBeenCalledTimes(1);
    });

    test('once a file is chosen, it is still one button, saying the file, and the hint makes room', async () => {
        renderWithContext(<Zone/>);
        await userEvent.upload(fileInput(), new File(['x'], 'parrot.png', {type: 'image/png'}));

        const buttons = within(zone()).getAllByRole('button');
        expect(buttons).toHaveLength(1);
        expect(buttons[0]).toHaveAccessibleName(/^Choose another file\s*parrot\.png$/);
        expect(buttons[0]).not.toHaveAttribute('aria-describedby');
        expect(screen.queryByText('PNG, up to 16 MB.')).not.toBeInTheDocument();

        // The preview stands beside the button, not inside it.
        expect(zone().querySelector('.am-pv-drop-preview img')).toBeInTheDocument();
        expect(buttons[0].querySelector('img')).toBeNull();
    });

    test('an app\'s preview takes the place of the image', async () => {
        renderWithContext(<Zone preview={true}/>);
        fireEvent.drop(zone(), {dataTransfer: {files: [new File(['x'], 'dropped.png', {type: 'image/png'})]}});
        expect(within(zone()).getByText('Preview')).toBeInTheDocument();
        expect(within(zone()).getAllByRole('button')).toHaveLength(1);
    });

    test('what is wrong with the file shows under the zone, as an alert', () => {
        renderWithContext(<Zone error='This file is too large.'/>);
        expect(screen.getByRole('alert')).toHaveTextContent('This file is too large.');
        expect(within(zone()).getByRole('button')).toHaveAccessibleDescription('This file is too large.');
        expect(screen.queryByText('PNG, up to 16 MB.')).not.toBeInTheDocument();
    });
});
