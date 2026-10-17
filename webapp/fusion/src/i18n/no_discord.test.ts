// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

// The web UI doesn't compare itself with Discord: no English string names it, in en.json nor as a defaultMessage in the
// source (comments and translators' descriptions may). The translations follow when they're redone.

import fs from 'fs';
import path from 'path';

const SRC = path.resolve(__dirname, '..');
const DISCORD = /discord/i;

const en = JSON.parse(fs.readFileSync(path.join(__dirname, 'en.json'), 'utf8')) as Record<string, {defaultMessage: string}>;

function sourceFiles(dir: string, out: string[] = []): string[] {
    for (const entry of fs.readdirSync(dir, {withFileTypes: true})) {
        const p = path.join(dir, entry.name);
        if (entry.isDirectory()) {
            if (entry.name !== 'node_modules' && entry.name !== '__snapshots__' && entry.name !== 'tests') {
                sourceFiles(p, out);
            }
        } else if ((/\.(tsx?|jsx?)$/).test(entry.name) && !(/\.test\.(tsx?|jsx?)$/).test(entry.name)) {
            out.push(p);
        }
    }
    return out;
}

// A defaultMessage written as a string literal (`defaultMessage: '...'`, `defaultMessage='...'`, or in braces).
const DEFAULT_MESSAGE = /defaultMessage\s*[:=]\s*\{?\s*(['"`])((?:\\.|(?!\1)[^\\])*)\1/g;

describe('the web UI does not name Discord', () => {
    test('no en.json string mentions Discord', () => {
        const offending = Object.entries(en).
            filter(([, v]) => DISCORD.test(v.defaultMessage)).
            map(([id, v]) => `${id}: ${v.defaultMessage}`);
        expect(offending).toEqual([]);
    });

    test('no defaultMessage in the source mentions Discord', () => {
        const offending: string[] = [];
        for (const file of sourceFiles(SRC)) {
            const text = fs.readFileSync(file, 'utf8');
            if (!DISCORD.test(text)) {
                continue;
            }
            for (const match of text.matchAll(DEFAULT_MESSAGE)) {
                if (DISCORD.test(match[2])) {
                    offending.push(`${path.relative(SRC, file)}: ${match[2].slice(0, 120)}`);
                }
            }
        }
        expect(offending).toEqual([]);
    });

    test('the Channels page of the community settings explains itself without Discord', () => {
        for (const id of ['fusion.teamChannels.kindDesc', 'fusion.teamChannels.allDesc', 'fusion.layout.communityDesc', 'admin.team.allChannelsMembershipDescription']) {
            expect(en[id].defaultMessage).not.toMatch(DISCORD);
        }
        expect(en['fusion.teamChannels.allDesc'].defaultMessage).toMatch(/^Everyone in the community is in all its public channels/);
    });
});
