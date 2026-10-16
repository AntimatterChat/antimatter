// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import fs from 'fs';

import {Api} from './lib/api';
import {ADMIN_STATE, AUTH_DIR, MEMBER_STATE, RUN_FILE, env} from './lib/env';
import type {Run} from './lib/run';

// The onboarding checklist opens by itself for someone new (a fresh server's admin, a member the setup creates), and
// its overlay takes every click until it's closed: the tests start with it closed (its button stays).
const ONBOARDING_CLOSED = {category: 'onboarding_task_list', name: 'onboarding_task_list_open', value: 'false'};

// The global setup logs the admin in, picks the community, creates the channel the tests work in and a member without
// admin rights, and saves their browser sessions. The global teardown removes what it created.
export default async function globalSetup() {
    fs.mkdirSync(AUTH_DIR, {recursive: true});
    const stamp = Date.now().toString(36);

    const admin = await Api.login(env.user, env.password);
    await admin.context.storageState({path: ADMIN_STATE});

    const teams = await admin.myTeams();
    const team = env.team ? await admin.teamByName(env.team) : teams.sort((a, b) => a.name.localeCompare(b.name))[0];
    if (!team) {
        throw new Error(`${env.user} isn't in any community: add them to one, or set AM_E2E_TEAM.`);
    }

    // Fusion offers a community's admins a layout the first time they open it: not in the middle of a test.
    await admin.savePreferences([{category: 'fusion_layout_offered', name: team.id, value: 'true'}, ONBOARDING_CLOSED]);

    const channel = await admin.createChannel(team.id, `e2e-${stamp}`, `E2E ${stamp}`);

    let created = false;
    let memberLogin = env.memberUser;
    let memberPassword = env.memberPassword;
    if (!memberLogin) {
        memberLogin = `e2e-member-${stamp}`;
        memberPassword = `E2e-${stamp}-${Math.random().toString(36).slice(2, 10)}`;
        const user = await admin.createUser(memberLogin, memberPassword);
        await admin.addToTeam(team.id, user.id);
        created = true;
    }
    const member = await Api.login(memberLogin, memberPassword);
    await member.savePreferences([ONBOARDING_CLOSED]);
    await member.context.storageState({path: MEMBER_STATE});
    await admin.addToChannel(channel.id, member.me.id);

    const run: Run = {
        team: {id: team.id, name: team.name, displayName: team.display_name},
        channel: {id: channel.id, name: channel.name, displayName: channel.display_name},
        admin: {id: admin.me.id, username: admin.me.username, token: admin.token},
        member: {id: member.me.id, username: member.me.username, token: member.token, created},
    };
    fs.writeFileSync(RUN_FILE, JSON.stringify(run, null, 2));

    // The sessions stay open for the browsers; the teardown logs them out.
    await admin.context.dispose();
    await member.context.dispose();
}
