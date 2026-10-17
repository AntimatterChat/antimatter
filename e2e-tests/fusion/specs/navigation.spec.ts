// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {expect, test} from '../lib/fixtures';

const escape = (s: string) => s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');

test('shows the communities and their channels', async ({fusion, run}) => {
    await fusion.openChannel();

    const rail = fusion.page.getByRole('navigation', {name: 'Communities'});
    await expect(rail).toBeVisible();

    // The community, on its own or in a folder.
    await expect(rail.getByRole('button', {name: new RegExp(escape(run.team.displayName))}).first()).toBeVisible();

    await expect(fusion.sidebar.getByRole('heading', {level: 2, name: run.team.displayName})).toBeVisible();
    await expect(fusion.sidebar.locator(`a[href$="/channels/${run.channel.name}"]`)).toBeVisible();
    await expect(fusion.sidebar.locator('a[href$="/channels/general"]')).toBeVisible();
    await expect(fusion.main.getByRole('heading', {level: 1})).toContainText(run.channel.displayName);
});

test('switches channel from the sidebar', async ({fusion, page, run}) => {
    await fusion.openChannel();

    await fusion.sidebar.locator('a[href$="/channels/general"]').click();
    await expect(page).toHaveURL(/\/channels\/general$/);
    await expect(fusion.main.getByRole('heading', {level: 1})).not.toContainText(run.channel.displayName);

    // Back: the test channel may sit in a collapsed category, which only shows the open channel.
    await page.goBack();
    await expect(page).toHaveURL(new RegExp(`/channels/${run.channel.name}$`));
    await expect(fusion.main.getByRole('heading', {level: 1})).toContainText(run.channel.displayName);
    await expect(fusion.composer).toBeVisible();
});

test('an old link to town-square, the default channel\'s former name, lands in general', async ({fusion, page, run}) => {
    await fusion.openChannel();

    await page.goto(`/${run.team.name}/channels/town-square`);
    await expect(page).toHaveURL(/\/channels\/general$/);
    await expect(fusion.main.getByRole('heading', {level: 1})).toContainText('General');
});
