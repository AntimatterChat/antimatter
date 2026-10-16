// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {defineConfig, devices} from 'playwright/test';

import {ADMIN_STATE, env} from './lib/env';

// Smoke tests of the Fusion web UI against a running server: AM_E2E_BASE_URL, AM_E2E_USER and AM_E2E_PASSWORD (an
// admin) say which. See README.md.
export default defineConfig({
    testDir: './specs',
    globalSetup: './global_setup.ts',
    globalTeardown: './global_teardown.ts',

    // One worker: the tests share a server, a channel and an admin.
    workers: 1,
    fullyParallel: false,
    forbidOnly: Boolean(process.env.CI),
    retries: process.env.CI ? 1 : 0,
    timeout: 60_000,
    expect: {timeout: 10_000},
    reporter: process.env.CI ? [['list'], ['html', {open: 'never'}], ['github']] : [['list'], ['html', {open: 'never'}]],
    use: {
        baseURL: env.baseURL,
        storageState: ADMIN_STATE,
        trace: 'retain-on-failure',
        screenshot: 'only-on-failure',
        video: 'off',
        launchOptions: env.chromium ? {executablePath: env.chromium} : {},
    },
    projects: [
        {
            name: 'chromium',
            use: {...devices['Desktop Chrome'], viewport: {width: 1400, height: 900}},
        },
    ],
});
