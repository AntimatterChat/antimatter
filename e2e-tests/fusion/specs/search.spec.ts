// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {expect, test, uniq} from '../lib/fixtures';

test('finds a message by a word in it', async ({fusion, page, admin, run}) => {
    const word = uniq('kiwi');
    await admin.createPost(run.channel.id, `Searching for ${word} here`);
    await fusion.openChannel();
    await expect(fusion.message(word)).toBeVisible();

    const search = fusion.main.getByRole('textbox', {name: 'Search messages'});
    await search.click();
    await search.fill(word);
    await search.press('Enter');

    const results = page.getByRole('complementary').filter({has: page.getByRole('heading', {name: 'Search results'})});
    await expect(results).toBeVisible();
    await expect(results.getByRole('tab', {name: /^Messages 1/})).toBeVisible();
    await expect(results.getByRole('button', {name: new RegExp(word)})).toBeVisible();

    await results.getByRole('button', {name: 'Close'}).click();
    await expect(results).toBeHidden();
});
