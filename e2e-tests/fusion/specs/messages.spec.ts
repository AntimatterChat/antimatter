// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {expect, test, uniq} from '../lib/fixtures';

test('sends, edits and deletes a message', async ({fusion, page}) => {
    await fusion.openChannel();
    const text = uniq('hello');
    const message = await fusion.send(text);

    await (await fusion.moreActions(message)).getByRole('menuitem', {name: 'Edit'}).click();
    const editor = fusion.main.getByRole('textbox', {name: 'Edit message'});
    await expect(editor).toBeVisible();
    await editor.fill(`${text} edited`);
    await editor.press('Enter');
    await expect(editor).toBeHidden();
    await expect(fusion.message(`${text} edited`)).toBeVisible();
    await expect(fusion.message(`${text} edited`)).toContainText('(edited)');

    await (await fusion.moreActions(fusion.message(text))).getByRole('menuitem', {name: 'Delete'}).click();
    const confirm = page.getByRole('dialog', {name: 'Delete message?'});
    await expect(confirm).toBeVisible();
    await confirm.getByRole('button', {name: 'Delete'}).click();
    await expect(fusion.message(text)).toHaveCount(0);
});

test('cancels deleting a message', async ({fusion, page}) => {
    await fusion.openChannel();
    const text = uniq('keep');
    const message = await fusion.send(text);

    await (await fusion.moreActions(message)).getByRole('menuitem', {name: 'Delete'}).click();
    await page.getByRole('dialog', {name: 'Delete message?'}).getByRole('button', {name: 'Cancel'}).click();
    await expect(page.getByRole('dialog', {name: 'Delete message?'})).toBeHidden();
    await expect(message).toBeVisible();
});

test('replies in a thread', async ({fusion, page}) => {
    await fusion.openChannel();
    const root = uniq('root');
    const message = await fusion.send(root);

    await (await fusion.actions(message)).getByRole('button', {name: 'Reply in thread'}).click();
    const thread = page.getByRole('complementary').filter({has: page.getByRole('heading', {name: 'Thread'})});
    await expect(thread).toBeVisible();
    await expect(thread).toContainText(root);

    const reply = uniq('reply');
    const box = thread.getByRole('textbox', {name: 'Reply to thread'});
    await box.click();
    await box.fill(reply);
    await box.press('Enter');
    await expect(thread.locator('.am-msg').filter({hasText: reply})).toBeVisible();
    await expect(thread).toContainText('1 reply');

    await thread.getByRole('button', {name: 'Close thread'}).click();
    await expect(page.getByRole('heading', {name: 'Thread'})).toBeHidden();
});

test('reacts to a message and takes the reaction back', async ({fusion}) => {
    await fusion.openChannel();
    const message = await fusion.send(uniq('react'));

    await (await fusion.actions(message)).getByRole('button', {name: 'React with :+1:'}).click();
    const reaction = message.getByRole('button', {name: /^:\+1: 1/});
    await expect(reaction).toBeVisible();
    await expect(reaction).toHaveAttribute('aria-pressed', 'true');

    await reaction.click();
    await expect(message.getByRole('button', {name: /^:\+1: /})).toHaveCount(0);
});

test('reacts with an emoji found in the picker', async ({fusion, page}) => {
    await fusion.openChannel();
    const message = await fusion.send(uniq('pick'));

    await (await fusion.actions(message)).getByRole('button', {name: 'Add reaction'}).click();
    const picker = page.getByRole('dialog', {name: 'Emoji'});
    await expect(picker).toBeVisible();
    await picker.getByRole('textbox', {name: 'Search emoji'}).fill('tada');
    await picker.getByRole('button', {name: 'tada', exact: true}).first().click();

    await expect(picker).toBeHidden();
    await expect(message.getByRole('button', {name: /^:tada: 1/})).toBeVisible();
});
