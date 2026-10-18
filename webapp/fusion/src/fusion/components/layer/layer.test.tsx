// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {fireEvent, render, screen} from '@testing-library/react';
import React from 'react';
import {IntlProvider} from 'react-intl';

import {Dialog, LAYER_ID, Popover} from './layer';

// matchMedia answering `phone` for the phone breakpoint and nothing else.
function mockMedia(phone: boolean) {
    window.matchMedia = jest.fn((query: string) => ({
        matches: phone && query.includes('max-width: 640px'),
        media: query,
        addEventListener: jest.fn(),
        removeEventListener: jest.fn(),
    })) as unknown as typeof window.matchMedia;
}

const wrap = (node: React.ReactNode) => (
    <IntlProvider locale='en'>{node}</IntlProvider>
);

describe('fusion/components/layer', () => {
    const originalMatchMedia = window.matchMedia;

    beforeEach(() => {
        document.body.innerHTML = '';
        const layer = document.createElement('div');
        layer.id = LAYER_ID;
        document.body.appendChild(layer);
        mockMedia(false);
    });

    afterAll(() => {
        window.matchMedia = originalMatchMedia;
    });

    describe('Popover', () => {
        const setup = (props: Partial<React.ComponentProps<typeof Popover>> = {}) => {
            const anchor = document.createElement('button');
            anchor.textContent = 'anchor';
            document.body.appendChild(anchor);
            const toggle = document.createElement('button');
            toggle.textContent = 'toggle';
            document.body.appendChild(toggle);
            const outside = document.createElement('button');
            outside.textContent = 'outside';
            document.body.appendChild(outside);
            const onClose = jest.fn();
            render(wrap(
                <Popover
                    anchor={anchor}
                    className='test-pop'
                    label='Test popover'
                    onClose={onClose}
                    toggles={[toggle]}
                    {...props}
                >
                    <button>{'inside'}</button>
                </Popover>,
            ));
            return {anchor, toggle, outside, onClose};
        };

        test('renders into the layer with its label and role', () => {
            setup();
            const pop = screen.getByRole('dialog', {name: 'Test popover'});
            expect(document.getElementById(LAYER_ID)).toContainElement(pop);
            expect(pop).toHaveClass('am-pop');
        });

        test('closes on a press elsewhere and on Escape', () => {
            const {outside, onClose} = setup();
            fireEvent.pointerDown(outside);
            expect(onClose).toHaveBeenCalledTimes(1);
            fireEvent.keyDown(document, {key: 'Escape'});
            expect(onClose).toHaveBeenCalledTimes(2);
        });

        test('pressing inside it, its anchor or one of its toggles does not close it', () => {
            const {anchor, toggle, onClose} = setup();
            fireEvent.pointerDown(screen.getByText('inside'));
            fireEvent.pointerDown(anchor);
            fireEvent.pointerDown(toggle);
            expect(onClose).not.toHaveBeenCalled();
        });

        test('pressing a popover opened from it (later in the layer) is not clicking away', () => {
            const {onClose} = setup();
            const sub = document.createElement('div');
            sub.className = 'am-pop';
            const item = document.createElement('button');
            sub.appendChild(item);
            document.getElementById(LAYER_ID)!.appendChild(sub);
            fireEvent.pointerDown(item);
            expect(onClose).not.toHaveBeenCalled();
        });

        test('pressing a popover that came before it in the layer closes it', () => {
            const before = document.createElement('div');
            before.className = 'am-pop';
            const item = document.createElement('button');
            before.appendChild(item);
            document.getElementById(LAYER_ID)!.appendChild(before);
            const {onClose} = setup();
            fireEvent.pointerDown(item);
            expect(onClose).toHaveBeenCalledTimes(1);
        });

        test('is a sheet over a scrim on a phone', () => {
            mockMedia(true);
            setup();
            const pop = screen.getByRole('dialog', {name: 'Test popover'});
            expect(pop).toHaveClass('am-sheet');
            expect(document.querySelector('.am-sheet-scrim')).toBeInTheDocument();
        });

        test('stays beside its anchor on a phone when it is not to be a sheet', () => {
            mockMedia(true);
            setup({sheet: false});
            expect(screen.getByRole('dialog', {name: 'Test popover'})).not.toHaveClass('am-sheet');
            expect(document.querySelector('.am-sheet-scrim')).not.toBeInTheDocument();
        });

        test.each([
            ['above-end', '750px'],
            ['above', '960px'],
        ] as const)('%s places it above its anchor, its left side at %s', (placement, left) => {
            const width = jest.spyOn(HTMLElement.prototype, 'offsetWidth', 'get').mockReturnValue(240);
            const height = jest.spyOn(HTMLElement.prototype, 'offsetHeight', 'get').mockReturnValue(200);
            Object.defineProperty(window, 'innerWidth', {configurable: true, value: 1280});
            Object.defineProperty(window, 'innerHeight', {configurable: true, value: 800});
            const anchor = document.createElement('button');
            document.body.appendChild(anchor);
            anchor.getBoundingClientRect = () => ({left: 960, right: 990, top: 700, bottom: 730, width: 30, height: 30, x: 960, y: 700, toJSON: () => ({})});
            anchor.getClientRects = () => [anchor.getBoundingClientRect()] as unknown as DOMRectList;

            render(wrap(
                <Popover
                    anchor={anchor}
                    placement={placement}
                    className='test-pop'
                    label='Placed popover'
                    onClose={jest.fn()}
                >
                    <button>{'inside'}</button>
                </Popover>,
            ));
            const pop = screen.getByRole('dialog', {name: 'Placed popover'});

            // above-end ends where the anchor ends (990 - 240), as the "Send later" menu ends with the send button;
            // above starts where it starts.
            expect(pop.style.left).toBe(left);
            expect(pop.style.top).toBe('492px');
            width.mockRestore();
            height.mockRestore();
        });

        test('is not a sheet on a desktop', () => {
            setup();
            expect(screen.getByRole('dialog', {name: 'Test popover'})).not.toHaveClass('am-sheet');
        });
    });

    describe('Dialog', () => {
        const setup = (onSubmit?: () => void) => {
            const onClose = jest.fn();
            render(wrap(
                <Dialog
                    label='Test dialog'
                    onClose={onClose}
                    onSubmit={onSubmit}
                >
                    <button type='submit'>{'Save'}</button>
                </Dialog>,
            ));
            return onClose;
        };

        test('is a modal dialog that closes on Escape', () => {
            const onClose = setup();
            const dialog = screen.getByRole('dialog', {name: 'Test dialog'});
            expect(dialog).toHaveAttribute('aria-modal', 'true');
            fireEvent.keyDown(document, {key: 'Escape'});
            expect(onClose).toHaveBeenCalledTimes(1);
        });

        test('closes on a press on the scrim, not inside the dialog', () => {
            const onClose = setup();
            fireEvent.mouseDown(screen.getByText('Save'));
            expect(onClose).not.toHaveBeenCalled();
            fireEvent.mouseDown(document.querySelector('.am-scrim')!);
            expect(onClose).toHaveBeenCalledTimes(1);
        });

        test('submits as a form when it has onSubmit', () => {
            const onSubmit = jest.fn();
            const onClose = setup(onSubmit);
            fireEvent.click(screen.getByText('Save'));
            expect(onSubmit).toHaveBeenCalledTimes(1);
            expect(onClose).not.toHaveBeenCalled();
        });
    });
});
