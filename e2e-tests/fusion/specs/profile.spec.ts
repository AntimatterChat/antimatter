// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {expect, test, uniq, type Fusion} from '../lib/fixtures';
import type {Api} from '../lib/api';
import type {Run} from '../lib/run';

// A message of the member's, and its author's name as the conversation shows it.
async function memberMessage(fusion: Fusion, member: Api, run: Run) {
    const text = uniq('hi');
    await member.createPost(run.channel.id, text);
    await fusion.openChannel();
    const message = fusion.message(text);
    await expect(message).toBeVisible();
    const author = message.locator('.am-author');
    return {author, name: (await author.innerText()).trim()};
}

test('a click on someone\'s name opens their profile card, a second click closes it', async ({fusion, page, member, run}) => {
    const {author, name} = await memberMessage(fusion, member, run);

    await author.click();
    const card = page.getByRole('dialog', {name});
    await expect(card).toBeVisible();
    await expect(card).toContainText(`@${run.member.username}`);

    await author.click();
    await expect(card).toBeHidden();

    await author.click();
    await expect(card).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(card).toBeHidden();
});

test('a right-click opens someone\'s menu, a second right-click closes it', async ({fusion, page, member, run}) => {
    const {author, name} = await memberMessage(fusion, member, run);

    await author.click({button: 'right'});
    const menu = page.getByRole('menu', {name});
    await expect(menu).toBeVisible();
    for (const item of ['Profile', 'Message', 'Mention', 'Copy username']) {
        await expect(menu.getByRole('menuitem', {name: item})).toBeVisible();
    }

    // The admin may moderate them.
    for (const item of ['Mod view', 'Time out…', 'Remove from community…']) {
        await expect(menu.getByRole('menuitem', {name: item})).toBeVisible();
    }

    await author.click({button: 'right'});
    await expect(menu).toBeHidden();

    // From the menu to the profile card.
    await author.click({button: 'right'});
    await menu.getByRole('menuitem', {name: 'Profile'}).click();
    await expect(menu).toBeHidden();
    await expect(page.getByRole('dialog', {name})).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(page.getByRole('dialog', {name})).toBeHidden();
});

test('mentions someone from their menu', async ({fusion, page, member, run}) => {
    const {author} = await memberMessage(fusion, member, run);

    await author.click({button: 'right'});
    await page.getByRole('menuitem', {name: 'Mention'}).click();
    await expect(fusion.composer).toContainText(`@${run.member.username}`);
    await fusion.composer.fill('');
});
