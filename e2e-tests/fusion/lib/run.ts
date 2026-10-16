// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import fs from 'fs';

import {RUN_FILE} from './env';

// Run is what the global setup created for the tests.
export type Run = {
    team: {id: string; name: string; displayName: string};
    channel: {id: string; name: string; displayName: string};
    admin: {id: string; username: string; token: string};

    // created: the setup made this member (and the teardown removes them).
    member: {id: string; username: string; token: string; created: boolean};
};

export function readRun(): Run {
    return JSON.parse(fs.readFileSync(RUN_FILE, 'utf8')) as Run;
}
