// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {expect, test} from '@mattermost/playwright-lib';

import {setupDemoPlugin} from '../../helpers';

test('should parse user and channel mentions from /show_mentions command text', async ({pw}) => {
    test.setTimeout(120000);
    // 1. Setup
    const {adminClient, user, team} = await pw.initSetup();
    await setupDemoPlugin(adminClient, pw);

    // 2. Login
    const {channelsPage} = await pw.testBrowser.login(user);
    await channelsPage.goto();
    await channelsPage.toBeVisible();

    // 3. Navigate to General
    await channelsPage.goto(team.name, 'general');
    await channelsPage.toBeVisible();

    // Re-apply setupDemoPlugin: concurrent initSetup() resets PluginSettings.Plugins = {}
    await setupDemoPlugin(adminClient, pw);

    // 4. Send /show_mentions (retry once if plugin not yet ready)
    const responsePost = channelsPage.centerView.container
        .getByRole('listitem')
        .filter({hasText: 'contains the following different mentions'})
        .last();
    for (let attempt = 0; attempt < 2; attempt++) {
        await channelsPage.centerView.postCreate.input.fill('/show_mentions @sysadmin ~general');
        await channelsPage.centerView.postCreate.sendMessage();
        try {
            await expect(responsePost).toBeVisible({timeout: 15000});
            break;
        } catch (err) {
            if (attempt === 1) {
                throw err;
            }
            await setupDemoPlugin(adminClient, pw);
        }
    }

    // 5. Bot response is now visible

    // 6. Assert user mentions section
    await expect(responsePost.getByRole('heading', {name: 'Mentions to users in the team'})).toBeVisible();
    await expect(responsePost.getByRole('columnheader', {name: 'User name'})).toBeVisible();
    await expect(responsePost.getByRole('cell', {name: '@sysadmin'})).toBeVisible();

    // 7. Assert channel mentions section
    await expect(responsePost.getByRole('heading', {name: 'Mentions to public channels'})).toBeVisible();
    await expect(responsePost.getByRole('columnheader', {name: 'Channel name'})).toBeVisible();
    await expect(responsePost.getByRole('cell', {name: '~General'})).toBeVisible();

    // 8. Assert ~General is a link pointing to the general channel
    await expect(responsePost.getByRole('link', {name: '~General'})).toHaveAttribute(
        'href',
        /\/channels\/general$/,
    );
});
