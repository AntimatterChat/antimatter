// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {Client4} from 'mattermost-redux/client';

import {AutolinkPreviewError, PRESETS, newAutolink, parseAutolinks, previewAutolinks, serializeAutolinks} from './autolinks';

describe('autolinks', () => {
    test('parses what the server keeps, with keys for the editor', () => {
        const parsed = parseAutolinks('{"links":[{"name":"Jira","pattern":"(LUM-\\\\d+)","template":"[$1](https://jira/$1)","word_match":true}],"on_edit":true}');
        expect(parsed.on_edit).toBe(true);
        expect(parsed.links).toHaveLength(1);
        expect(parsed.links[0]).toMatchObject({name: 'Jira', pattern: '(LUM-\\d+)', word_match: true});
        expect(parsed.links[0].key).toBeTruthy();
    });

    test('parses nothing from nothing or from garbage', () => {
        expect(parseAutolinks('')).toEqual({links: []});
        expect(parseAutolinks(undefined)).toEqual({links: []});
        expect(parseAutolinks('{not json')).toEqual({links: []});
        expect(parseAutolinks('{"links":[{"name":"no pattern"}]}').links).toHaveLength(0);
    });

    test('serializes without editor keys, false options nor blank rules', () => {
        const link = {...newAutolink(PRESETS[0], 'Issue keys'), channels: [], process_bot_posts: false};
        const blank = newAutolink();
        const raw = serializeAutolinks({links: [link, blank]});
        expect(JSON.parse(raw)).toEqual({links: [{name: 'Issue keys', pattern: '(PROJ-\\d+)', template: '[$1](https://issues.example.com/browse/$1)'}]});
        expect(serializeAutolinks({links: [blank], on_edit: true})).toBe('');
    });

    test('round-trips', () => {
        const links = {links: [{...newAutolink(PRESETS[3]), channels: ['c1'], disabled: true}], on_edit: true};
        const raw = serializeAutolinks(links);
        expect(serializeAutolinks(parseAutolinks(raw))).toBe(raw);
    });

    test('a live preview leaves out the rules being written', async () => {
        const spy = jest.spyOn(Client4, 'previewTeamAutolinks').mockResolvedValue({text: 'done'});
        const half = {...newAutolink(), pattern: 'LUM-\\d+'};
        const autolinks = {links: [newAutolink(PRESETS[1]), half]};

        await expect(previewAutolinks('team', autolinks, '#1', true)).resolves.toBe('done');
        expect(spy).toHaveBeenLastCalledWith('team', {links: [expect.objectContaining({pattern: '#(\\d+)'})]}, '#1');

        await previewAutolinks('team', autolinks, '#1');
        expect((spy.mock.calls[1][1] as {links: unknown[]}).links).toHaveLength(2);
        spy.mockRestore();
    });

    test('tells what the server refused', async () => {
        const spy = jest.spyOn(Client4, 'previewTeamAutolinks').mockRejectedValue({message: 'Invalid autolinks.', detailed_error: 'autolink 1: missing closing )'});
        await expect(previewAutolinks('team', {links: [newAutolink(PRESETS[0])]}, '')).rejects.toEqual(new AutolinkPreviewError('autolink 1: missing closing )'));
        spy.mockRestore();
    });
});
