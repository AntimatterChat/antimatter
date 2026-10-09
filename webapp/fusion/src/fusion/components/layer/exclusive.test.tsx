// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {renderHook} from '@testing-library/react';

import {useExclusiveCard} from './exclusive';

describe('useExclusiveCard', () => {
    test('opening a card closes the one open', () => {
        const closeFirst = jest.fn();
        const closeSecond = jest.fn();
        renderHook(() => useExclusiveCard(closeFirst));
        expect(closeFirst).not.toHaveBeenCalled();
        renderHook(() => useExclusiveCard(closeSecond));
        expect(closeFirst).toHaveBeenCalledTimes(1);
        expect(closeSecond).not.toHaveBeenCalled();
    });

    test('a closed card stops listening', () => {
        const close = jest.fn();
        const {unmount} = renderHook(() => useExclusiveCard(close));
        unmount();
        renderHook(() => useExclusiveCard(jest.fn()));
        expect(close).not.toHaveBeenCalled();
    });

    test('rerendering a card does not close it', () => {
        const close = jest.fn();
        const {rerender} = renderHook(({onClose}) => useExclusiveCard(onClose), {initialProps: {onClose: close}});
        rerender({onClose: close});
        expect(close).not.toHaveBeenCalled();
    });
});
