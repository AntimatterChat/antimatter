// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {Client4} from 'mattermost-redux/client';

// A team's autolinks (server/public/model/team_autolink.go): patterns of its messages that the server turns into
// links as they're posted.

export type Autolink = {

    // A key for the editor only, not saved.
    key: string;
    name: string;
    disabled?: boolean;
    pattern: string;
    template: string;
    channels?: string[];
    word_match?: boolean;
    disable_non_word_prefix?: boolean;
    disable_non_word_suffix?: boolean;
    process_bot_posts?: boolean;
};

export type Autolinks = {
    links: Autolink[];
    on_edit?: boolean;
};

let counter = 0;
const newKey = () => `autolink-${++counter}`;

export function parseAutolinks(raw?: string): Autolinks {
    if (!raw) {
        return {links: []};
    }
    try {
        const parsed = JSON.parse(raw) as Partial<Autolinks>;
        const links = Array.isArray(parsed.links) ? parsed.links.filter((l) => l && typeof l.pattern === 'string') : [];
        return {links: links.map((l) => ({...l, name: l.name || '', template: l.template || '', key: newKey()})), on_edit: Boolean(parsed.on_edit)};
    } catch {
        return {links: []};
    }
}

// serializeAutolinks gives what the server keeps: the rules without their editor keys nor empty options; nothing
// without rules.
export function serializeAutolinks(autolinks: Autolinks): string {
    const links = autolinks.links.filter((l) => l.pattern.trim() || l.template.trim()).map(({key: _key, ...rest}) => { // eslint-disable-line @typescript-eslint/no-unused-vars
        const out: Record<string, unknown> = {name: rest.name.trim(), pattern: rest.pattern, template: rest.template};
        for (const option of ['disabled', 'word_match', 'disable_non_word_prefix', 'disable_non_word_suffix', 'process_bot_posts'] as const) {
            if (rest[option]) {
                out[option] = true;
            }
        }
        if (rest.channels?.length) {
            out.channels = rest.channels;
        }
        return out;
    });
    if (!links.length) {
        return '';
    }
    return JSON.stringify(autolinks.on_edit ? {links, on_edit: true} : {links});
}

export type Preset = {
    id: string;
    name: string;
    pattern: string;
    template: string;
    word_match?: boolean;
    example: string;
};

// Presets to start from: the address and the project key are the team's to fill in.
export const PRESETS: Preset[] = [
    {id: 'issues', name: 'Issue keys', pattern: '(PROJ-\\d+)', template: '[$1](https://issues.example.com/browse/$1)', example: 'Fixed in PROJ-123.'},
    {id: 'pulls', name: 'Pull requests', pattern: '#(\\d+)', template: '[#$1](https://github.com/org/repo/pull/$1)', example: 'Merged #42 this morning.'},
    {id: 'merge', name: 'Merge requests', pattern: '!(\\d+)', template: '[!$1](https://gitlab.example.com/group/project/-/merge_requests/$1)', example: 'Review !615 please'},
    {id: 'cve', name: 'CVE identifiers', pattern: 'CVE-\\d{4}-\\d{4,}', template: '[$0](https://www.cve.org/CVERecord?id=$0)', word_match: true, example: 'Patched CVE-2026-12345.'},
];

export function newAutolink(preset?: Preset): Autolink {
    return {
        key: newKey(),
        name: preset?.name || '',
        pattern: preset?.pattern || '',
        template: preset?.template || '',
        word_match: preset?.word_match,
    };
}

export class AutolinkPreviewError extends Error {}

// previewAutolinks asks the server what rules (as edited, not saved) make of a text; for a live preview, the rules
// still being written (without both a pattern and a template) are left out.
export async function previewAutolinks(teamId: string, autolinks: Autolinks, text: string, live = false): Promise<string> {
    const links = live ? {...autolinks, links: autolinks.links.filter((l) => l.pattern && l.template)} : autolinks;
    try {
        const response = await Client4.previewTeamAutolinks(teamId, JSON.parse(serializeAutolinks(links) || '{"links":[]}'), text);
        return response.text;
    } catch (err) {
        const detail = (err as {server_error_id?: string; message?: string; detailed_error?: string});
        throw new AutolinkPreviewError(detail.detailed_error || detail.message || String(err));
    }
}
