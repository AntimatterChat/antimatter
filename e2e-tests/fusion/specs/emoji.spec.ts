// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {expect, test} from '../lib/fixtures';

test('the composer\'s emoji picker searches and inserts an emoji', async ({fusion, page}) => {
    await fusion.openChannel();
    await fusion.main.getByRole('button', {name: 'Emoji', exact: true}).click();
    const picker = page.getByRole('dialog', {name: 'Emoji'});
    await expect(picker).toBeVisible();
    await expect(picker.getByRole('tab', {name: 'Emoji'})).toHaveAttribute('aria-selected', 'true');

    await picker.getByRole('textbox', {name: 'Search emoji'}).fill('tada');
    await expect(picker.getByRole('heading', {name: 'Search results'})).toBeVisible();
    await picker.getByRole('button', {name: 'tada', exact: true}).first().click();

    // The composer's picker stays open for more.
    await expect(fusion.composer).toContainText(':tada:');
    await page.keyboard.press('Escape');
    await expect(picker).toBeHidden();
    await fusion.composer.fill('');
});

test('opens the custom emoji dialog from the picker', async ({fusion, page}) => {
    await fusion.openChannel();
    await fusion.main.getByRole('button', {name: 'Emoji', exact: true}).click();
    await page.getByRole('dialog', {name: 'Emoji'}).getByRole('button', {name: 'Add custom emoji'}).click();

    const dialog = page.getByRole('dialog', {name: 'Custom emoji'});
    await expect(dialog).toBeVisible();
    await expect(dialog.getByRole('tab', {name: 'Add'})).toBeVisible();
    await expect(dialog.getByRole('tab', {name: 'All custom emoji'})).toBeVisible();

    // Nothing to add until there's an image and a name.
    await expect(dialog.getByRole('button', {name: 'Add emoji'})).toBeDisabled();

    await dialog.getByRole('tab', {name: 'All custom emoji'}).click();
    await expect(dialog.getByRole('tab', {name: 'All custom emoji'})).toHaveAttribute('aria-selected', 'true');

    await page.keyboard.press('Escape');
    await expect(dialog).toBeHidden();
});
