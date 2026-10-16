// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import fs from 'fs';

import {Api} from './lib/api';
import {AUTH_DIR, RUN_FILE} from './lib/env';
import type {Run} from './lib/run';

// The global teardown deletes the test channel's messages, then the channel (or archives it where the server keeps
// channels), removes the member it created, and logs both sessions out.
export default async function globalTeardown() {
    if (!fs.existsSync(RUN_FILE)) {
        return;
    }
    const run = JSON.parse(fs.readFileSync(RUN_FILE, 'utf8')) as Run;
    const admin = await Api.withToken(run.admin.token);
    try {
        await admin.clearChannel(run.channel.id);
        await admin.removeChannel(run.channel.id);
        if (run.member.created) {
            await admin.removeUser(run.member.id);
        } else {
            await (await Api.withToken(run.member.token)).logout();
        }
    } finally {
        await admin.logout();

        // The saved sessions are logged out: drop them.
        fs.rmSync(AUTH_DIR, {recursive: true, force: true});
    }
}
