// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {parseTemplate, PRESETS} from './templates';

describe('parseTemplate', () => {
    test('reads a template', () => {
        const json = JSON.stringify(PRESETS.hybrid);
        expect(parseTemplate(json)).toEqual(PRESETS.hybrid);
    });

    test('refuses what is not a template', () => {
        expect(parseTemplate('nope')).toBeUndefined();
        expect(parseTemplate('{}')).toBeUndefined();
        expect(parseTemplate('{"version":2,"server_categories":[],"channels":[],"roles":[]}')).toBeUndefined();
        expect(parseTemplate('{"version":1,"server_categories":{},"channels":[],"roles":[]}')).toBeUndefined();
    });
});

describe('PRESETS', () => {
    test('hybrid has a locked category and an open one, and a hoisted Moderators role', () => {
        const {server_categories: categories, roles} = PRESETS.hybrid;
        expect(categories.map((c) => c.locked)).toEqual([true, false]);
        expect(roles).toHaveLength(1);
        expect(roles[0]).toMatchObject({display_name: 'Moderators', hoist: true});
        expect(roles[0].permissions).toContain('delete_others_posts');
    });

    test('community keeps every category locked', () => {
        expect(PRESETS.community.server_categories.every((c) => c.locked)).toBe(true);
    });

    test('workplace has no categories for everyone', () => {
        expect(PRESETS.workplace.server_categories).toEqual([]);
    });

    test('no category is listed twice and no channel is filed twice', () => {
        for (const preset of Object.values(PRESETS)) {
            const names = preset.server_categories.map((c) => c.name.toLowerCase());
            expect(new Set(names).size).toBe(names.length);
            const channels = preset.server_categories.flatMap((c) => c.channels);
            expect(new Set(channels).size).toBe(channels.length);
        }
    });
});
