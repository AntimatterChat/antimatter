// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {expect, test, uniq} from '../lib/fixtures';

// A phone held upright: one pane at a time, the navigation in a drawer.
test.use({viewport: {width: 390, height: 844}, isMobile: true, hasTouch: true});

test('shows one pane at a time on a phone', async ({fusion, page, run}) => {
    await fusion.openChannel();

    // The conversation fills the screen, with nothing wider than it.
    await expect(fusion.composer).toBeInViewport();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
    await expect(fusion.sidebar.locator(`a[href$="/channels/${run.channel.name}"]`)).not.toBeInViewport();

    // The navigation slides in, and going to a channel brings the conversation back.
    await fusion.main.getByRole('button', {name: 'Open navigation'}).click();
    const link = fusion.sidebar.locator('a[href$="/channels/town-square"]');
    await expect(link).toBeInViewport();
    await link.click();
    await expect(page).toHaveURL(/\/channels\/town-square$/);
    await expect(fusion.composer).toBeInViewport();
    await expect(link).not.toBeInViewport();
});

test('sends a message on a phone', async ({fusion}) => {
    await fusion.openChannel();
    const message = await fusion.send(uniq('phone'));
    await expect(message).toBeInViewport();
});
