// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import path from 'path';

// Where the suite runs, read from the environment (AM_ variables; see README.md).
export const env = {

    // The server, and an admin account on it.
    baseURL: (process.env.AM_E2E_BASE_URL || 'http://localhost:8067').replace(/\/$/, ''),
    user: process.env.AM_E2E_USER || 'maya',
    password: process.env.AM_E2E_PASSWORD || 'Antimatter-Local-1',

    // The community (team name) to work in; by default the admin's first one by name.
    team: process.env.AM_E2E_TEAM || '',

    // A member without admin rights for the permission tests. When unset, the suite creates one and deactivates it
    // afterwards.
    memberUser: process.env.AM_E2E_MEMBER_USER || '',
    memberPassword: process.env.AM_E2E_MEMBER_PASSWORD || '',

    // A Chromium to use instead of Playwright's own download.
    chromium: process.env.AM_E2E_CHROMIUM || '',
};

// Files the global setup writes for the tests: the browser sessions and what it created.
export const AUTH_DIR = path.join(__dirname, '..', '.auth');
export const ADMIN_STATE = path.join(AUTH_DIR, 'admin.json');
export const MEMBER_STATE = path.join(AUTH_DIR, 'member.json');
export const RUN_FILE = path.join(AUTH_DIR, 'run.json');
