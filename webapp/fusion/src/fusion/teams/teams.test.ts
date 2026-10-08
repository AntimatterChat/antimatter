// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {Group} from '@mattermost/types/groups';
import type {UserProfile} from '@mattermost/types/users';

import {badgeTeam, mainTeam, TEAM_BADGE_PROP} from './teams';

const team = (id: string, tag = '') => ({id, tag, display_name: id}) as Group;
const user = (id: string, badge?: string) => ({id, props: badge === undefined ? {} : {[TEAM_BADGE_PROP]: badge}}) as unknown as UserProfile;

describe('teams', () => {
    const det = team('det', 'DET');
    const ops = team('ops', 'OPS');
    const untagged = team('beam');
    const teamsByUser = {ada: [det, ops], ivo: [untagged], sam: [det]};

    test('the badge is the team someone chose, if they belong to it and it has a tag', () => {
        expect(badgeTeam(user('ada', 'ops'), teamsByUser)).toBe(ops);
        expect(badgeTeam(user('sam', 'ops'), teamsByUser)).toBeUndefined();
        expect(badgeTeam(user('ivo', 'beam'), teamsByUser)).toBeUndefined();
    });

    test('no choice, no badge', () => {
        expect(badgeTeam(user('ada'), teamsByUser)).toBeUndefined();
        expect(badgeTeam(user('ada', ''), teamsByUser)).toBeUndefined();
    });

    test('people are listed under their badge team, else their first team', () => {
        expect(mainTeam(user('ada', 'ops'), teamsByUser)).toBe(ops);
        expect(mainTeam(user('ada'), teamsByUser)).toBe(det);
        expect(mainTeam(user('ivo'), teamsByUser)).toBe(untagged);
        expect(mainTeam(user('lena'), teamsByUser)).toBeUndefined();
    });
});
