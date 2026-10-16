// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {request} from 'playwright/test';

import {NO_COOKIES} from '../lib/api';
import {env, MEMBER_STATE} from '../lib/env';
import {expect, test, uniq} from '../lib/fixtures';

test.describe('a member without admin rights', () => {
    test.use({storageState: MEMBER_STATE});

    test('has no System Console in their menu', async ({fusion}) => {
        await fusion.openChannel();
        const menu = await fusion.openStatusMenu();
        await expect(menu.getByRole('button', {name: 'Profile & settings'})).toBeVisible();
        await expect(menu.getByRole('button', {name: 'System Console'})).toHaveCount(0);
    });

    test('gets no moderation in an admin\'s menu', async ({fusion, page, admin, run}) => {
        const text = uniq('mod');
        await admin.createPost(run.channel.id, text);
        await fusion.openChannel();
        const author = fusion.message(text).locator('.am-author');
        const name = (await author.innerText()).trim();

        await author.click({button: 'right'});
        const menu = page.getByRole('menu', {name});
        await expect(menu.getByRole('menuitem', {name: 'Message'})).toBeVisible();
        // Only the channel's admins, the community's admins and moderators remove others from a channel.
        for (const item of ['Mod view', 'Time out…', 'Remove from channel', 'Remove from community…']) {
            await expect(menu.getByRole('menuitem', {name: item})).toHaveCount(0);
        }

        // System admins can be ignored, not blocked.
        await expect(menu.getByRole('menuitem', {name: 'Block…'})).toHaveCount(0);
        await page.keyboard.press('Escape');
        await expect(menu).toBeHidden();
    });

    test('is turned away from the System Console', async ({page}) => {
        await page.goto('/admin_console');
        await expect(page).not.toHaveURL(/\/admin_console/, {timeout: 30_000});
        await expect(page.getByRole('textbox', {name: 'Find settings'})).toHaveCount(0);
    });

    test('is refused the server configuration by the API', async ({run}) => {
        const api = await request.newContext({baseURL: env.baseURL, storageState: NO_COOKIES, extraHTTPHeaders: {Authorization: `Bearer ${run.member.token}`}});
        expect((await api.get('/api/v4/config')).status()).toBe(403);
        const me = await api.get('/api/v4/users/me');
        expect(me.status()).toBe(200);
        expect((await me.json()).id).toBe(run.member.id);
        await api.dispose();
    });
});

test('an admin opens the System Console', async ({page}) => {
    await page.goto('/admin_console');
    await expect(page.getByRole('textbox', {name: 'Find settings'})).toBeVisible({timeout: 30_000});
    await expect(page).toHaveURL(/\/admin_console\//);
});
