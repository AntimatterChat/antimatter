// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {fireEvent, renderHook, screen} from '@testing-library/react';

import {LAYER_ID} from 'fusion/components/layer';

import {useFaceTips} from './face_tips';

describe('fusion/shell/useFaceTips', () => {
    beforeEach(() => {
        document.body.innerHTML = `<div id="${LAYER_ID}"></div><span id="mark" data-am-tip="Private"></span><span id="plain"></span>`;
    });

    test('shows the Fusion tooltip of a data-am-tip element under the pointer, and hides it after', () => {
        const {unmount} = renderHook(() => useFaceTips());

        fireEvent.mouseOver(document.getElementById('mark')!);
        expect(screen.getByRole('tooltip')).toHaveTextContent('Private');
        expect(document.getElementById(LAYER_ID)).toContainElement(screen.getByRole('tooltip'));

        fireEvent.mouseOver(document.getElementById('plain')!);
        expect(screen.queryByRole('tooltip')).not.toBeInTheDocument();
        unmount();
    });

    test('names a data-am-tip element the keyboard moves to, until the focus leaves it', () => {
        const {unmount} = renderHook(() => useFaceTips());
        const mark = document.getElementById('mark')!;
        jest.spyOn(mark, 'matches').mockImplementation((selector: string) => selector === ':focus-visible');

        fireEvent.focusIn(mark);
        expect(screen.getByRole('tooltip')).toHaveTextContent('Private');

        fireEvent.focusOut(mark);
        expect(screen.queryByRole('tooltip')).not.toBeInTheDocument();
        unmount();
    });

    test('keeps the tooltip of the pointer when the focus moves elsewhere', () => {
        const {unmount} = renderHook(() => useFaceTips());

        fireEvent.mouseOver(document.getElementById('mark')!);
        fireEvent.focusOut(document.getElementById('plain')!);
        expect(screen.getByRole('tooltip')).toHaveTextContent('Private');
        unmount();
    });
});
