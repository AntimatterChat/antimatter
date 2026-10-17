// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {expect, test, uniq} from '../lib/fixtures';

test('the composer\'s emoji picker searches and inserts an emoji', async ({fusion, page}) => {
    await fusion.openChannel();
    await fusion.main.getByRole('button', {name: 'Emoji', exact: true}).click();
    const picker = page.getByRole('dialog', {name: 'Emoji'});
    await expect(picker).toBeVisible();

    // The picker has tabs when plugins add some (GIFs, stickers): it opens on Emoji. A server without them has none.
    if (await picker.getByRole('tablist').count() > 0) {
        await expect(picker.getByRole('tab', {name: 'Emoji'})).toHaveAttribute('aria-selected', 'true');
    }

    await picker.getByRole('textbox', {name: 'Search emoji'}).fill('tada');
    await expect(picker.getByRole('heading', {name: 'Search results'})).toBeVisible();
    await picker.getByRole('button', {name: 'tada', exact: true}).first().click();

    // The composer's picker stays open for more.
    await expect(fusion.composer).toContainText(':tada:');
    await page.keyboard.press('Escape');
    await expect(picker).toBeHidden();
    await fusion.composer.fill('');
});

// A 2×2 PNG, for the custom emoji the test adds.
const PNG = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAIAAAACCAYAAABytg0kAAAAEUlEQVR4nGP438DwH4QZYAwAWsoJ+e+uaqEAAAAASUVORK5CYII=', 'base64');

test('adds and deletes a custom emoji inside the picker, which stays open', async ({fusion, page}) => {
    const name = uniq('e2e_').toLowerCase();
    await fusion.openChannel();
    await fusion.main.getByRole('button', {name: 'Emoji', exact: true}).click();
    const picker = page.getByRole('dialog', {name: 'Emoji'});
    await picker.getByRole('button', {name: 'Add custom emoji'}).click();

    // The add view takes the grid's place: the picker stays open.
    const add = picker.getByRole('form', {name: 'Add a custom emoji'});
    await expect(add).toBeVisible();
    await add.locator('input[type="file"]').setInputFiles({name: `${name}.png`, mimeType: 'image/png', buffer: PNG});

    // The name comes from the file, and the server says it's free.
    await expect(add.getByRole('textbox', {name: 'Name'})).toHaveValue(name);
    await expect(add.getByText(`:${name}: is free.`)).toBeVisible();
    await add.getByRole('button', {name: 'Save'}).click();

    // Back on the grid, the new emoji first and ready to use.
    await expect(add).toBeHidden();
    await expect(picker.getByRole('heading', {name: 'Just added'})).toBeVisible();
    await expect(picker.getByRole('button', {name, exact: true}).first()).toBeFocused();

    // Manage: search it, delete it after confirming in its row.
    await picker.getByRole('button', {name: 'Custom emoji', exact: true}).click();
    const manage = picker.getByRole('region', {name: 'Custom emoji'});
    await manage.getByRole('searchbox', {name: 'Search custom emoji'}).fill(name);
    await manage.getByRole('button', {name: `Delete :${name}:`}).click();
    await manage.getByRole('group').getByRole('button', {name: 'Delete', exact: true}).click();
    await expect(manage.getByText(`:${name}: was deleted.`)).toBeVisible();

    // Escape goes back to the grid, then closes the picker.
    await page.keyboard.press('Escape');
    await expect(manage).toBeHidden();
    await expect(picker.getByRole('textbox', {name: 'Search emoji'})).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(picker).toBeHidden();
});
