// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {expect, test} from '../lib/fixtures';

test('opens the settings and moves between their pages', async ({fusion, page, run}) => {
    await fusion.openChannel();
    const menu = await fusion.openStatusMenu();
    await menu.getByRole('button', {name: 'Profile & settings'}).click();

    const settings = page.getByRole('dialog', {name: 'Settings'});
    await expect(settings).toBeVisible();
    await expect(settings.getByRole('heading', {level: 2, name: 'My account'})).toBeVisible();
    await expect(settings.getByRole('textbox', {name: 'Username'})).toHaveValue(run.admin.username);

    await settings.getByRole('button', {name: 'Appearance'}).click();
    await expect(settings.getByRole('heading', {level: 2, name: 'Appearance'})).toBeVisible();

    await page.keyboard.press('Escape');
    await expect(settings).toBeHidden();
});
