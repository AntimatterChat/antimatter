// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {render, screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import React, {useRef, useState} from 'react';

import {MenuItem} from './menu';
import MenuKeys, {rovingKeyDown} from './menu_keys';

function Menu() {
    return (
        <div role='menu'>
            <MenuKeys>
                <MenuItem label='One'/>
                <MenuItem
                    label='Two'
                    disabled={true}
                />
                <MenuItem
                    label='Three'
                    role='menuitemcheckbox'
                    checked={true}
                />
                <MenuItem label='Four'/>
            </MenuKeys>
        </div>
    );
}

// The menu's button: opening it focuses the menu's first item; Escape closes it, and the focus goes back.
function Opener() {
    const [open, setOpen] = useState(false);
    return (
        <div
            onKeyDown={(e) => {
                if (e.key === 'Escape') {
                    setOpen(false);
                }
            }}
        >
            <button
                type='button'
                onClick={() => setOpen(!open)}
            >
                {'open'}
            </button>
            {open && <Menu/>}
        </div>
    );
}

describe('fusion/components/menu/MenuKeys', () => {
    test('the first item takes the focus; the arrows skip disabled items and go round; Home and End go to the ends', async () => {
        render(<Menu/>);
        expect(screen.getByRole('menuitem', {name: 'One'})).toHaveFocus();
        await userEvent.keyboard('{ArrowDown}');
        expect(screen.getByRole('menuitemcheckbox', {name: 'Three'})).toHaveFocus();
        await userEvent.keyboard('{ArrowDown}{ArrowDown}');
        expect(screen.getByRole('menuitem', {name: 'One'})).toHaveFocus();
        await userEvent.keyboard('{ArrowUp}');
        expect(screen.getByRole('menuitem', {name: 'Four'})).toHaveFocus();
        await userEvent.keyboard('{Home}');
        expect(screen.getByRole('menuitem', {name: 'One'})).toHaveFocus();
        await userEvent.keyboard('{End}');
        expect(screen.getByRole('menuitem', {name: 'Four'})).toHaveFocus();
    });

    test('closing the menu gives the focus back to its button', async () => {
        render(<Opener/>);
        const opener = screen.getByRole('button', {name: 'open'});
        await userEvent.click(opener);
        expect(screen.getByRole('menuitem', {name: 'One'})).toHaveFocus();

        // Closing it with the focus in it would leave the focus nowhere: it goes back.
        await userEvent.keyboard('{Escape}');
        expect(screen.queryByRole('menu')).not.toBeInTheDocument();
        expect(opener).toHaveFocus();
    });

    test('an item\'s id is its data-action, and an expandable item says whether it\'s open', () => {
        render(
            <div role='menu'>
                <MenuItem
                    label='Mute'
                    action='mute'
                    expanded={false}
                />
            </div>,
        );
        expect(screen.getByRole('menuitem', {name: 'Mute'})).toHaveAttribute('data-action', 'mute');
        expect(screen.getByRole('menuitem', {name: 'Mute'})).toHaveAttribute('aria-expanded', 'false');
    });
});

describe('fusion/components/menu/rovingKeyDown', () => {
    function Toolbar() {
        const ref = useRef<HTMLDivElement>(null);
        return (
            <div
                ref={ref}
                role='toolbar'
                aria-orientation='vertical'
                onKeyDown={(e) => rovingKeyDown(e, ref.current, 'button')}
            >
                <button type='button'>{'A'}</button>
                <button type='button'>{'B'}</button>
                <input aria-label='field'/>
            </div>
        );
    }

    test('moves along a toolbar\'s buttons, and leaves a field\'s own keys alone', async () => {
        render(<Toolbar/>);
        screen.getByRole('button', {name: 'A'}).focus();
        await userEvent.keyboard('{ArrowDown}');
        expect(screen.getByRole('button', {name: 'B'})).toHaveFocus();
        await userEvent.keyboard('{ArrowDown}');
        expect(screen.getByRole('button', {name: 'A'})).toHaveFocus();

        screen.getByRole('textbox', {name: 'field'}).focus();
        await userEvent.keyboard('{Home}');
        expect(screen.getByRole('textbox', {name: 'field'})).toHaveFocus();
    });
});
