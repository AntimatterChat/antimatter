// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {test as base, expect, type Locator, type Page} from 'playwright/test';

import {Api} from './api';
import {readRun, type Run} from './run';

type Fixtures = {

    // The Fusion UI in the test channel, ready to type in.
    fusion: Fusion;

    // Deletes what the test posted in the test channel, whether it passed or not.
    cleanChannel: void;
};

type WorkerFixtures = {
    run: Run;
    admin: Api;

    // The member without admin rights, through the API.
    member: Api;
};

export const test = base.extend<Fixtures, WorkerFixtures>({
    run: [async ({}, use) => { // eslint-disable-line no-empty-pattern
        await use(readRun());
    }, {scope: 'worker'}],

    admin: [async ({run}, use) => {
        const api = await Api.withToken(run.admin.token);
        await use(api);
        await api.context.dispose();
    }, {scope: 'worker'}],

    member: [async ({run}, use) => {
        const api = await Api.withToken(run.member.token);
        await use(api);
        await api.context.dispose();
    }, {scope: 'worker'}],

    cleanChannel: [async ({admin, run}, use) => {
        await use();
        await admin.clearChannel(run.channel.id);
    }, {auto: true}],

    fusion: async ({page, run}, use) => {
        await use(new Fusion(page, run));
    },
});

export {expect};

// uniq gives a word no other message has, for finding a test's own message (and searching for it).
export function uniq(prefix: string): string {
    return `${prefix}${Date.now().toString(36)}${Math.random().toString(36).slice(2, 6)}`;
}

// Fusion drives the Fusion UI the way people do: through roles and labels, as screen readers see it.
export class Fusion {
    constructor(readonly page: Page, readonly run: Run) {}

    get channelPath() {
        return `/${this.run.team.name}/channels/${this.run.channel.name}`;
    }

    async openChannel() {
        await this.page.goto(this.channelPath);
        await expect(this.composer).toBeVisible({timeout: 30_000});
    }

    get main() {
        return this.page.getByRole('main');
    }

    get sidebar() {
        return this.page.getByRole('complementary', {name: 'Channels and direct messages'});
    }

    // The conversation's message box.
    get composer() {
        return this.main.getByRole('textbox', {name: /^Message /});
    }

    async send(text: string): Promise<Locator> {
        await this.composer.click();
        await this.composer.fill(text);
        await this.composer.press('Enter');
        const message = this.message(text);
        await expect(message).toBeVisible();

        // Shown at once, but saved once the server gives it its id (a pending message has a temporary one).
        await expect(message).toHaveAttribute('id', /^post_[a-z0-9]{26}$/);
        return message;
    }

    // A message of the conversation, by its text.
    message(text: string) {
        return this.main.locator('.am-msg').filter({hasText: text});
    }

    // The toolbar a message shows on hover.
    async actions(message: Locator) {
        await message.hover();
        const toolbar = message.getByRole('toolbar', {name: 'Message actions'});
        await expect(toolbar).toBeVisible();
        return toolbar;
    }

    // A message's ⋯ menu ("Message actions").
    async moreActions(message: Locator) {
        await (await this.actions(message)).getByRole('button', {name: 'Message actions'}).click();
        const menu = this.page.getByRole('menu', {name: 'Message actions'});
        await expect(menu).toBeVisible();
        return menu;
    }

    // The menu behind the button at the bottom of the left rail: status, settings, System Console, log out.
    async openStatusMenu() {
        await this.page.getByRole('button', {name: 'Your status, profile and settings'}).click();
        const menu = this.page.getByRole('dialog', {name: 'Set your status'});
        await expect(menu).toBeVisible();
        return menu;
    }
}
