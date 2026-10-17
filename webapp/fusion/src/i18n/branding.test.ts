// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

// Branding: the web UI says Antimatter, never Mattermost (UPSTREAM_MERGE_RULES.md, rules 10, 14 and 16). This test
// fails when a string in en.json, a defaultMessage in the source, or a translation of a rebranded string mentions
// Mattermost without being on the allowlist below. Add to the allowlist only with a reason; when a string is fixed,
// remove its entry (the test also fails on stale entries, so the list only shrinks).

import fs from 'fs';
import path from 'path';

const SRC = path.resolve(__dirname, '..');
const I18N = __dirname;
const MATTERMOST = /mattermost/i;

// Strings allowed to name Mattermost, and why.
const ALLOWED: Record<string, string> = {

    // Legal notices stay intact (rule 16).
    'about.copyright': 'copyright credit to Mattermost, Inc.',
    'fusion.about.basedOn': 'copyright credit to Mattermost, Inc.',

    // Identifiers that keep their name (rule 17).
    'admin.accesscontrol.trustProxyDeviceIdentityHeader.desc': 'the X-Mattermost-Session-Attribute-Device-Id HTTP header',

    // Strings that tell admins something is Mattermost's, not Antimatter's.
    'admin.email.mattermostPushServer': 'warns that a push server is run by Mattermost, Inc. and only serves its apps',
    'admin.plugin.addOn.notSupported': 'a third-party plugin needs an add-on Mattermost sells',
    'admin.plugin.addOn.notSupportedWarning': 'a third-party plugin needs an add-on Mattermost sells',
    'fusion.layout.workplaceDesc': 'compares the Workplace layout with Mattermost\'s',
};

// Known branding debt: strings inherited from Mattermost that should be reworded (and translated into the 37
// languages, rule 25). Listed so this test catches new ones; fix them and remove them from here. It is empty: keep it
// that way.
const KNOWN_DEBT: Record<string, string> = {};

// Translations of strings that en.json rebranded but the translation still says Mattermost, per language: stale
// upstream translations (rule 26). Unlisted languages have none. The counts may only go down; lower them when you fix
// translations. All are fixed: keep it that way.
const STALE_TRANSLATIONS: Record<string, number> = {};

// Mattermost in the Latin script, or transliterated as some translations did (Korean, Japanese).
const MATTERMOST_IN_TRANSLATION = /mattermost|매터모스트|マターモスト/i;

type Catalog = Record<string, string | {defaultMessage: string}>;

const readCatalog = (file: string): Record<string, string> => {
    const raw = JSON.parse(fs.readFileSync(path.join(I18N, file), 'utf8')) as Catalog;
    return Object.fromEntries(Object.entries(raw).map(([id, v]) => [id, typeof v === 'string' ? v : v.defaultMessage]));
};

const en = readCatalog('en.json');
const allowed = (id: string) => id in ALLOWED || id in KNOWN_DEBT;

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
const ID = /\bid\s*[:=]\s*\{?\s*(['"`])([\w.-]+)\1/g;

// The message id that goes with a defaultMessage: the nearest id before it, or else the first one after it.
function idNear(text: string, at: number): string | undefined {
    const before = text.slice(Math.max(0, at - 600), at);
    const preceding = [...before.matchAll(ID)].pop();
    if (preceding) {
        return preceding[2];
    }
    const following = ID.exec(text.slice(at, at + 600));
    ID.lastIndex = 0;
    return following?.[2];
}

describe('branding: the web UI says Antimatter, not Mattermost', () => {
    test('en.json only mentions Mattermost in allowlisted strings', () => {
        const offending = Object.entries(en).
            filter(([id, text]) => MATTERMOST.test(text) && !allowed(id)).
            map(([id, text]) => `${id}: ${text}`);

        expect(offending).toEqual([]);
    });

    test('the allowlists have no stale entries', () => {
        const stale = [...Object.keys(ALLOWED), ...Object.keys(KNOWN_DEBT)].
            filter((id) => !(id in en) || !MATTERMOST.test(en[id]));

        expect(stale).toEqual([]);
    });

    test('defaultMessages in the source only mention Mattermost in allowlisted strings', () => {
        const offending: string[] = [];
        for (const file of sourceFiles(SRC)) {
            const text = fs.readFileSync(file, 'utf8');
            if (!MATTERMOST.test(text)) {
                continue;
            }
            for (const match of text.matchAll(DEFAULT_MESSAGE)) {
                if (!MATTERMOST.test(match[2])) {
                    continue;
                }
                const id = idNear(text, match.index ?? 0);
                if (!id || !allowed(id)) {
                    const line = text.slice(0, match.index).split('\n').length;
                    offending.push(`${path.relative(SRC, file)}:${line} (${id ?? 'no id'}): ${match[2].slice(0, 120)}`);
                }
            }
        }

        expect(offending).toEqual([]);
    });

    test('translations of rebranded strings do not bring Mattermost back', () => {
        const rebranded = new Set(Object.keys(en).filter((id) => !MATTERMOST.test(en[id])));
        const counts: Record<string, number> = {};
        const locales = fs.readdirSync(I18N).filter((f) => f.endsWith('.json') && f !== 'en.json');
        for (const file of locales) {
            const catalog = readCatalog(file);
            counts[file.replace(/\.json$/, '')] = Object.entries(catalog).
                filter(([id, text]) => rebranded.has(id) && MATTERMOST_IN_TRANSLATION.test(text)).length;
        }

        for (const [locale, count] of Object.entries(counts)) {
            // A new language has none; a known one may only improve.
            expect({locale, count: Math.min(count, STALE_TRANSLATIONS[locale] ?? 0)}).toEqual({locale, count});
        }
    });
});
