// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {env} from '../lib/env';
import {expect, test} from '../lib/fixtures';

// Logging in through the form, without the saved session.
test.use({storageState: {cookies: [], origins: []}});

test('logs in with the form and out again', async ({page, fusion}) => {
    await page.goto('/login');
    await page.getByRole('textbox', {name: 'Email or Username'}).fill(env.user);
    await page.getByRole('textbox', {name: 'Password'}).fill(env.password);
    await page.getByRole('button', {name: 'Log in'}).click();

    await expect(page).toHaveURL(/\/channels\//, {timeout: 30_000});
    await expect(page.getByRole('navigation', {name: 'Communities'})).toBeVisible();
    await expect(fusion.composer).toBeVisible();

    const menu = await fusion.openStatusMenu();
    await menu.getByRole('button', {name: 'Log out'}).click();
    await expect(page).toHaveURL(/\/login/);
    await expect(page.getByRole('button', {name: 'Log in'})).toBeVisible();
});

test('refuses a wrong password', async ({page}) => {
    await page.goto('/login');
    await page.getByRole('textbox', {name: 'Email or Username'}).fill(env.user);
    await page.getByRole('textbox', {name: 'Password'}).fill('not-the-password-123');
    await page.getByRole('button', {name: 'Log in'}).click();

    await expect(page.getByText('The email/username or password is invalid.')).toBeVisible();
    await expect(page).toHaveURL(/\/login/);
});
