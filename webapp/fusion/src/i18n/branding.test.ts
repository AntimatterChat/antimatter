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
    'fusion.layout.workplaceDesc': 'compares a community layout with Mattermost\'s, as the next option does with Discord',
};

// Known branding debt: strings inherited from Mattermost that should be reworded (and translated into the 37
// languages, rule 25). Listed so this test catches new ones; fix them and remove them from here.
const KNOWN_DEBT: Record<string, string> = {

    // Links and contacts at Mattermost, Inc. (rules 6 and 10).
    'channel_loader.socketError': 'links to docs.mattermost.com',
    'announcement_bar.warn.contact_support_text': 'licence renewal through support@mattermost.com',
    'user.settings.languages.promote2': 'the Mattermost Translation Server',
    'admin.general.localization.availableLocalesDescription': 'the Mattermost Translation Server',
    'admin.support.enableAskCommunityDesc': 'the Mattermost Community',
    'admin.support.reportAProblemTypeDescriptionLicensed': 'the Mattermost technical support team',
    'admin.support.termsDesc': 'the Mattermost Acceptable Use Policy',

    // Mattermost's apps, plugin and command names.
    'admin.customization.appDownloadLinkDesc': 'the Mattermost apps',
    'admin.customization.appDownloadLinkTitle': 'the Mattermost apps',
    'admin.customization.minimumDesktopAppVersionDesc': 'the Mattermost Desktop App',
    'channel_notifications.desktopNotificationsDesc': 'the Mattermost Desktop App',
    'admin.intune.clientIdDescription': 'the Mattermost Mobile app',
    'admin.intune.enableDescription': 'Mattermost Mobile',
    'app.agents.bridge.not_available.plugin_not_active': 'the Mattermost Agents plugin',
    'app.agents.bridge.not_available.plugin_not_registered': 'the Mattermost Agents plugin',
    'app.agents.bridge.not_available.plugin_version_too_old': 'the Mattermost Agents plugin',
    'app.agents.bridge.unavailable_reason': 'the Mattermost Agents plugin',
    'admin.ldap.idAttrDesc': 'the "mattermost ldap idmigrate" command (the binary is antimatter)',

    // Example values with mattermost in them (example.com would do).
    'admin.complianceExport.globalRelayEmailAddress.example': 'globalrelay@mattermost.com',
    'admin.complianceExport.globalRelaySenderAddress.example': 'compliance-export@mattermost.com',
    'admin.environment.notifications.feedbackEmail.placeholder': 'mattermost@yourcompany.com',
    'admin.environment.notifications.replyToAddress.placeholder': 'mattermost@yourcompany.com',
    'admin.image.amazonS3BucketExample': 'mattermost-media',
    'admin.image.azureContainerExample': 'mattermost-media',
    'admin.image.azureStorageAccountExample': 'mattermoststorage',
    'admin.image.azureEndpointDescription': 'acmemattermost.blob.core...',
    'admin.team.restrictExample': 'corp.mattermost.com, mattermost.com',
    'admin.team.restrictGuestDescription': 'mattermost.com domains',
    'general_tab.AllowedDomainsExample': 'corp.mattermost.com, mattermost.com',
    'general_tab.AllowedDomainsInfo': 'mattermost.org',
    'admin.service.siteURLDescription': 'http://example.com:8065/company/mattermost',
    'admin.saml.assertionConsumerServiceURLEx': 'your-mattermost-url',
    'admin.saml.serviceProviderIdentifierEx': 'your-mattermost-url',
    'admin.gitlab.EnableMarkdownDesc': 'your-mattermost-url',
    'admin.google.EnableMarkdownDesc': 'your-mattermost-url',
    'admin.office365.EnableMarkdownDesc': 'your-mattermost-url',
    'admin.openid.EnableMarkdownDesc': 'your-mattermost-url',
};

// Translations of strings that en.json rebranded but the translation still says Mattermost, per language: stale
// upstream translations (rule 26). Unlisted languages have none. The counts may only go down; lower them when you fix
// translations.
const STALE_TRANSLATIONS: Record<string, number> = {
    ar: 1,
    bg: 1,
    bn: 1,
    de: 54,
    fa: 1,
    fr: 1,
    hu: 39,
    it: 2,
    ja: 162,
    ko: 81,
    nl: 12,
    pl: 2,
    'pt-BR': 1,
    ro: 1,
    sv: 37,
    ta: 1,
    th: 1,
    uk: 1,
    ur: 1,
    vi: 2,
    'zh-CN': 1,
};

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
                filter(([id, text]) => rebranded.has(id) && MATTERMOST.test(text)).length;
        }

        for (const [locale, count] of Object.entries(counts)) {
            // A new language has none; a known one may only improve.
            expect({locale, count: Math.min(count, STALE_TRANSLATIONS[locale] ?? 0)}).toEqual({locale, count});
        }
    });
});
