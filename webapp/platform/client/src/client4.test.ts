// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import nock from 'nock';

import Client4, {ClientError, HEADER_X_VERSION_ID} from './client4';
import {buildQueryString} from './helpers';

describe('Client4', () => {
    beforeAll(() => {
        if (!nock.isActive()) {
            nock.activate();
        }
    });

    afterAll(() => {
        nock.restore();
    });

    describe('channel types', () => {
        let client: Client4;

        beforeEach(() => {
            client = new Client4();
            client.setUrl('http://mattermost.example.com');
        });

        test('updateChannelType should PUT the body to the channel\'s type route', async () => {
            const channel = {id: 'ch1', props: {am_type: 'forum', am_mode: 'qa', am_root_posts: 'members', am_preset: 'forum_qa', am_forum_view: 'topics'}};
            nock(client.getBaseRoute()).
                put('/channels/ch1/type', {preset: 'forum_qa'}).
                reply(200, channel);

            const result = await client.updateChannelType('ch1', {preset: 'forum_qa'});

            expect(result).toEqual(channel);
        });

        test('createChannel should pass a preset through the props', async () => {
            const body = {name: 'beam-questions', display_name: 'Beam questions', type: 'O' as const, team_id: 'team1', props: {am_preset: 'forum_qa'}};
            nock(client.getBaseRoute()).
                post('/channels', body).
                reply(201, {...body, id: 'ch1'});

            const result = await client.createChannel(body);

            expect(result.id).toBe('ch1');
        });
    });

    describe('assistant', () => {
        let client: Client4;

        beforeEach(() => {
            client = new Client4();
            client.setUrl('http://mattermost.example.com');
        });

        test('getAssistantFeed should GET the caller\'s feed of the channel', async () => {
            const feed = {channel_id: 'ch1', enabled: true, on: true, engine: 'rules', cards: []};
            nock(client.getBaseRoute()).
                get('/assistant/channels/ch1/feed').
                reply(200, feed);

            expect(await client.getAssistantFeed('ch1')).toEqual(feed);
        });

        test('setAssistantPresence should POST whether the panel is open', async () => {
            const presence = {following: true, ttl_seconds: 90, heartbeat_seconds: 30};
            nock(client.getBaseRoute()).
                post('/assistant/channels/ch1/presence', {open: true}).
                reply(200, presence);

            expect(await client.setAssistantPresence('ch1', true)).toEqual(presence);
        });

        test('setAssistantChannelOn should PUT the switch', async () => {
            nock(client.getBaseRoute()).
                put('/assistant/channels/ch1/enabled', {on: false}).
                reply(200, {on: false});

            expect(await client.setAssistantChannelOn('ch1', false)).toEqual({on: false});
        });

        test('card actions should POST to the card\'s routes', async () => {
            const card = {id: 'c1', channel_id: 'ch1', kind: 'action'};
            nock(client.getBaseRoute()).
                post('/assistant/channels/ch1/cards/c1/dismiss').reply(200, card).
                post('/assistant/channels/ch1/cards/c1/pin', {pinned: true}).reply(200, card).
                post('/assistant/channels/ch1/cards/c1/mute').reply(200, card).
                post('/assistant/channels/ch1/cards/c1/claim').reply(200, card).
                post('/assistant/channels/ch1/cards/c1/run').reply(200, {card, executed: false}).
                post('/assistant/channels/ch1/cards/c1/post').reply(201, {id: 'p1'});

            expect(await client.dismissAssistantCard('ch1', 'c1')).toEqual(card);
            expect(await client.pinAssistantCard('ch1', 'c1', true)).toEqual(card);
            expect(await client.muteAssistantCard('ch1', 'c1')).toEqual(card);
            expect(await client.claimAssistantCard('ch1', 'c1')).toEqual(card);
            expect(await client.runAssistantCard('ch1', 'c1')).toEqual({card, executed: false});
            expect((await client.postAssistantCard('ch1', 'c1')).id).toBe('p1');
        });

        test('a second run should reject with the conflict', async () => {
            nock(client.getBaseRoute()).
                post('/assistant/channels/ch1/cards/c1/run').
                reply(409, {id: 'app.assistant.offer_done.app_error', message: 'done', status_code: 409});

            await expect(client.runAssistantCard('ch1', 'c1')).rejects.toThrow(ClientError);
        });

        test('muted tools and a community\'s settings', async () => {
            const settings = {enabled: false, engine: 'rules' as const, connection_id: '', model: ''};
            nock(client.getBaseRoute()).
                get('/assistant/muted_tools').reply(200, {tools: []}).
                put('/assistant/muted_tools', {tools: ['mcp.grafana.query']}).reply(200, {tools: ['mcp.grafana.query']}).
                get('/assistant/communities/t1/settings').reply(200, {team_id: 't1', server_enabled: true, may_override: true, settings}).
                put('/assistant/communities/t1/settings', settings).reply(200, {team_id: 't1', server_enabled: true, may_override: true, settings});

            expect(await client.getAssistantMutedTools()).toEqual({tools: []});
            expect(await client.setAssistantMutedTools(['mcp.grafana.query'])).toEqual({tools: ['mcp.grafana.query']});
            expect((await client.getAssistantCommunitySettings('t1')).may_override).toBe(true);
            expect((await client.setAssistantCommunitySettings('t1', settings)).settings).toEqual(settings);
        });
    });

    describe('forum topics', () => {
        let client: Client4;

        beforeEach(() => {
            client = new Client4();
            client.setUrl('http://mattermost.example.com');
        });

        test('getForumTopics should GET the channel\'s topics with the query', async () => {
            const list = {topics: [], next_cursor: ''};
            nock(client.getBaseRoute()).
                get('/channels/ch1/topics').
                query({sort: 'votes', answered: 'false', q: 'beam align', before: 'abc', per_page: '10', replies: '3'}).
                reply(200, list);

            const result = await client.getForumTopics('ch1', {sort: 'votes', answered: false, q: 'beam align', before: 'abc', per_page: 10, replies: 3});

            expect(result).toEqual(list);
        });

        test('getForumTopics should leave the query out when there is none', async () => {
            nock(client.getBaseRoute()).
                get('/channels/ch1/topics').
                reply(200, {topics: [], next_cursor: ''});

            const result = await client.getForumTopics('ch1');

            expect(result.next_cursor).toBe('');
        });

        test('patchForumTopic should PATCH the topic route', async () => {
            const post = {id: 'p1', props: {am_accepted_answer: 'r1'}};
            nock(client.getBaseRoute()).
                patch('/posts/p1/topic', {accepted_answer_id: 'r1'}).
                reply(200, post);

            const result = await client.patchForumTopic('p1', {accepted_answer_id: 'r1'});

            expect(result).toEqual(post);
        });

        test('moveThreadToChannel should send the title a forum needs', async () => {
            nock(client.getBaseRoute()).
                post('/posts/p1/move_to_channel', {channel_id: 'ch1', flatten: false, title: 'Beam alignment'}).
                reply(200, {id: 'p2'});

            const result = await client.moveThreadToChannel('p1', 'ch1', false, 'Beam alignment');

            expect(result.id).toBe('p2');
        });
    });

    describe('property field routes', () => {
        let client: Client4;

        beforeEach(() => {
            client = new Client4();
            client.setUrl('http://mattermost.example.com');
        });

        test('getPropertyFieldsRoute should build correct URL', () => {
            expect(client.getPropertyFieldsRoute('my_group', 'user')).toBe(
                'http://mattermost.example.com/api/v4/properties/groups/my_group/user/fields',
            );
        });

        test('getPropertyFieldRoute should build correct URL', () => {
            expect(client.getPropertyFieldRoute('my_group', 'user', 'field123')).toBe(
                'http://mattermost.example.com/api/v4/properties/groups/my_group/user/fields/field123',
            );
        });

        test('getPropertyFields should send GET with correct query params', async () => {
            const fields = [{id: 'f1', name: 'test'}];
            nock(client.getBaseRoute()).
                get('/properties/groups/grp/user/fields').
                query({target_type: 'system', per_page: '10', cursor_id: 'abc', cursor_create_at: '999'}).
                reply(200, fields);

            const result = await client.getPropertyFields('grp', 'user', 'system', undefined, {
                perPage: 10,
                cursorId: 'abc',
                cursorCreateAt: 999,
            });

            expect(result).toEqual(fields);
        });

        test('getPropertyFields should include target_id when provided', async () => {
            nock(client.getBaseRoute()).
                get('/properties/groups/grp/user/fields').
                query({target_type: 'channel', target_id: 'ch1'}).
                reply(200, []);

            const result = await client.getPropertyFields('grp', 'user', 'channel', 'ch1');
            expect(result).toEqual([]);
        });

        test('getPropertyFields should send minimal params when no options', async () => {
            nock(client.getBaseRoute()).
                get('/properties/groups/grp/user/fields').
                query({target_type: 'system'}).
                reply(200, []);

            const result = await client.getPropertyFields('grp', 'user', 'system');
            expect(result).toEqual([]);
        });

        test('getPropertyFieldOptionsRoute should build correct URL', () => {
            expect(client.getPropertyFieldOptionsRoute('access_control', 'user', 'field123')).toBe(
                'http://mattermost.example.com/api/v4/properties/groups/access_control/user/fields/field123/options',
            );
        });

        test('getPropertyFieldOptions should send per_page and both cursor halves', async () => {
            const options = [{id: 'opt-10', name: 'Program', parents: [], create_at: 1010}];
            const page = {options, has_more: false};
            nock(client.getBaseRoute()).
                get('/properties/groups/access_control/user/fields/field123/options').
                query({per_page: '200', cursor_id: 'opt-9', cursor_create_at: '1009'}).
                reply(200, page);

            const result = await client.getPropertyFieldOptions('access_control', 'user', 'field123', {
                perPage: 200,
                cursorId: 'opt-9',
                cursorCreateAt: 1009,
            });

            expect(result).toEqual(page);
        });

        test('getPropertyFieldOptions should default per_page to 200 when called with no options argument', async () => {
            const page = {options: [], has_more: false};
            nock(client.getBaseRoute()).
                get('/properties/groups/access_control/user/fields/field123/options').
                query({per_page: '200'}).
                reply(200, page);

            const result = await client.getPropertyFieldOptions('access_control', 'user', 'field123');
            expect(result).toEqual(page);
        });

        test('getPropertyFieldOptions should send per_page and omit both cursor keys on the first request', async () => {
            const page = {options: [], has_more: false};
            nock(client.getBaseRoute()).
                get('/properties/groups/access_control/user/fields/field123/options').
                query({per_page: '200'}).
                reply(200, page);

            const result = await client.getPropertyFieldOptions('access_control', 'user', 'field123', {perPage: 200});
            expect(result).toEqual(page);
        });

        test('getPropertyFieldOptions should honour an explicit perPage', async () => {
            const page = {options: [], has_more: false};
            nock(client.getBaseRoute()).
                get('/properties/groups/access_control/user/fields/field123/options').
                query({per_page: '25'}).
                reply(200, page);

            const result = await client.getPropertyFieldOptions('access_control', 'user', 'field123', {perPage: 25});
            expect(result).toEqual(page);
        });

        test('getPropertyFieldOptions should omit cursor_create_at when it is 0', async () => {
            const page = {options: [], has_more: false};
            nock(client.getBaseRoute()).
                get('/properties/groups/access_control/user/fields/field123/options').
                query({per_page: '200', cursor_id: 'opt-9'}).
                reply(200, page);

            const result = await client.getPropertyFieldOptions('access_control', 'user', 'field123', {
                cursorId: 'opt-9',
                cursorCreateAt: 0,
            });

            expect(result).toEqual(page);
        });

        test('getPropertyFieldOptions should omit cursor_id when it is empty', async () => {
            const page = {options: [], has_more: false};
            nock(client.getBaseRoute()).
                get('/properties/groups/access_control/user/fields/field123/options').
                query({per_page: '200', cursor_create_at: '1009'}).
                reply(200, page);

            const result = await client.getPropertyFieldOptions('access_control', 'user', 'field123', {
                cursorId: '',
                cursorCreateAt: 1009,
            });

            expect(result).toEqual(page);
        });

        test('getPropertyFieldOptions should return options carrying read_only and create_at', async () => {
            const options = [{id: 'o1', name: 'F-18 Program', parents: [], read_only: true, create_at: 1700000000000}];
            const page = {options, has_more: false};
            nock(client.getBaseRoute()).
                get('/properties/groups/access_control/user/fields/field123/options').
                query({per_page: '200'}).
                reply(200, page);

            const result = await client.getPropertyFieldOptions('access_control', 'user', 'field123');
            expect(result).toEqual(page);
        });

        test('getPropertyFieldOptions should return has_more and both cursor halves', async () => {
            const page = {
                options: [{id: 'opt-10', name: 'Program', parents: [], create_at: 1010}],
                has_more: true,
                next_cursor_create_at: 5000,
                next_cursor_id: 'examined-last',
            };
            nock(client.getBaseRoute()).
                get('/properties/groups/access_control/user/fields/field123/options').
                query({per_page: '200'}).
                reply(200, page);

            const result = await client.getPropertyFieldOptions('access_control', 'user', 'field123');
            expect(result).toEqual(page);
            expect(result.has_more).toBe(true);
            expect(result.next_cursor_create_at).toBe(5000);
            expect(result.next_cursor_id).toBe('examined-last');
        });

        test('getPropertyFieldOptions should throw ClientError on 403', async () => {
            nock(client.getBaseRoute()).
                get('/properties/groups/access_control/user/fields/field123/options').
                query({per_page: '200'}).
                reply(403, {
                    id: 'api.property_field.options.no_permission.app_error',
                    message: 'forbidden',
                    status_code: 403,
                }, {'Content-Type': 'application/json'});

            await expect(client.getPropertyFieldOptions('access_control', 'user', 'field123')).rejects.toThrow(ClientError);
        });

        test('getPropertyFieldOptions should throw ClientError on 404', async () => {
            nock(client.getBaseRoute()).
                get('/properties/groups/access_control/user/fields/field123/options').
                query({per_page: '200'}).
                reply(404, {
                    message: 'not found',
                    status_code: 404,
                }, {'Content-Type': 'application/json'});

            await expect(client.getPropertyFieldOptions('access_control', 'user', 'field123')).rejects.toThrow(ClientError);
        });

        test('createPropertyField should send POST with field body', async () => {
            const field = {name: 'classification', type: 'select' as const, target_type: 'system'};
            const created = {id: 'new1', ...field};

            nock(client.getBaseRoute()).
                post('/properties/groups/grp/user/fields', (body) => {
                    return body.name === 'classification' && body.type === 'select';
                }).
                reply(201, created);

            const result = await client.createPropertyField('grp', 'user', field);
            expect(result).toEqual(created);
        });

        test('patchPropertyField should send PATCH to field URL', async () => {
            const patch = {attrs: {options: []}};
            const patched = {id: 'f1', name: 'classification', attrs: {options: []}};

            nock(client.getBaseRoute()).
                patch('/properties/groups/grp/user/fields/f1', (body) => {
                    return body.attrs !== undefined;
                }).
                reply(200, patched);

            const result = await client.patchPropertyField('grp', 'user', 'f1', patch);
            expect(result).toEqual(patched);
        });

        test('deletePropertyField should send DELETE to field URL', async () => {
            nock(client.getBaseRoute()).
                delete('/properties/groups/grp/user/fields/f1').
                reply(200, {status: 'OK'});

            const result = await client.deletePropertyField('grp', 'user', 'f1');
            expect(result).toEqual({status: 'OK'});
        });
    });

    describe('access control field autocomplete', () => {
        let client: Client4;

        beforeEach(() => {
            client = new Client4();
            client.setUrl('http://mattermost.example.com');
        });

        test('getAccessControlFields sends include_resource_fields when requested', async () => {
            const fields = [{id: 'f1', name: 'classification'}];
            nock(client.getBaseRoute()).
                get('/access_control_policies/cel/autocomplete/fields').
                query({after: '', limit: '100', include_resource_fields: 'true'}).
                reply(200, fields);

            const result = await client.getAccessControlFields('', 100, undefined, undefined, true);
            expect(result).toEqual(fields);
        });

        test('getAccessControlFields omits include_resource_fields by default', async () => {
            nock(client.getBaseRoute()).
                get('/access_control_policies/cel/autocomplete/fields').
                query((q) => q.include_resource_fields === undefined && q.after === '' && q.limit === '100').
                reply(200, []);

            const result = await client.getAccessControlFields('', 100);
            expect(result).toEqual([]);
        });
    });

    describe('content flagging routes', () => {
        let client: Client4;

        beforeEach(() => {
            client = new Client4();
            client.setUrl('http://mattermost.example.com');
        });

        test('flagPost should send comment as a plain string', async () => {
            let receivedBody: any;
            nock(client.getBaseRoute()).
                post('/content_flagging/post/post123/flag', (body) => {
                    receivedBody = body;
                    return true;
                }).
                reply(200, {status: 'OK'});

            await client.flagPost('post123', 'Spam', 'looks suspicious');

            expect(receivedBody).toEqual({reason: 'Spam', comment: 'looks suspicious'});
        });

        test('flagPost should preserve an empty comment as an empty string', async () => {
            let receivedBody: any;
            nock(client.getBaseRoute()).
                post('/content_flagging/post/post123/flag', (body) => {
                    receivedBody = body;
                    return true;
                }).
                reply(200, {status: 'OK'});

            await client.flagPost('post123', 'Spam', '');

            expect(receivedBody).toEqual({reason: 'Spam', comment: ''});
        });

        test('removeFlaggedPost should send comment as a plain string', async () => {
            let receivedBody: any;
            nock(client.getBaseRoute()).
                put('/content_flagging/post/post123/remove', (body) => {
                    receivedBody = body;
                    return true;
                }).
                reply(200, {status: 'OK'});

            await client.removeFlaggedPost('post123', 'violates policy');

            expect(receivedBody).toEqual({comment: 'violates policy'});
        });

        test('removeFlaggedPost should preserve an empty comment as an empty string', async () => {
            let receivedBody: any;
            nock(client.getBaseRoute()).
                put('/content_flagging/post/post123/remove', (body) => {
                    receivedBody = body;
                    return true;
                }).
                reply(200, {status: 'OK'});

            await client.removeFlaggedPost('post123', '');

            expect(receivedBody).toEqual({comment: ''});
        });

        test('keepFlaggedPost should send comment as a plain string', async () => {
            let receivedBody: any;
            nock(client.getBaseRoute()).
                put('/content_flagging/post/post123/keep', (body) => {
                    receivedBody = body;
                    return true;
                }).
                reply(200, {status: 'OK'});

            await client.keepFlaggedPost('post123', 'looks fine');

            expect(receivedBody).toEqual({comment: 'looks fine'});
        });

        test('keepFlaggedPost should preserve an empty comment as an empty string', async () => {
            let receivedBody: any;
            nock(client.getBaseRoute()).
                put('/content_flagging/post/post123/keep', (body) => {
                    receivedBody = body;
                    return true;
                }).
                reply(200, {status: 'OK'});

            await client.keepFlaggedPost('post123', '');

            expect(receivedBody).toEqual({comment: ''});
        });

        test('getPostExposureReportUrl should build correct URL', () => {
            expect(client.getPostExposureReportUrl('post123')).toBe(
                'http://mattermost.example.com/api/v4/content_flagging/post/post123/exposure_report',
            );
        });

        test('generatePostExposureReport should return the CSV blob and the filename from Content-Disposition', async () => {
            nock(client.getBaseRoute()).
                post('/content_flagging/post/post123/exposure_report').
                reply(200, '#Post ID,post123\nUser ID,Username\nuid1,alice\n', {
                    'Content-Type': 'text/csv; charset=utf-8',
                    'Content-Disposition': 'attachment; filename="post-exposure-post123-1700000000000.csv"',
                });

            const result = await client.generatePostExposureReport('post123');

            expect(result.filename).toEqual('post-exposure-post123-1700000000000.csv');
            expect(typeof result.blob.text).toBe('function');
            expect(await result.blob.text()).toContain('uid1,alice');
        });

        test('generatePostExposureReport should fall back to a generated filename when the header is absent', async () => {
            nock(client.getBaseRoute()).
                post('/content_flagging/post/post123/exposure_report').
                reply(200, 'User ID,Username\n', {'Content-Type': 'text/csv; charset=utf-8'});

            const result = await client.generatePostExposureReport('post123');

            expect(result.filename).toMatch(/^post-exposure-post123-\d+\.csv$/);
        });

        test('generatePostExposureReport should surface server errors as ClientError', async () => {
            nock(client.getBaseRoute()).
                post('/content_flagging/post/post123/exposure_report').
                reply(400, {
                    id: 'api.data_spillage.error.post_not_in_progress',
                    message: 'The review of this post is already closed.',
                    status_code: 400,
                }, {'Content-Type': 'application/json'});

            await expect(client.generatePostExposureReport('post123')).rejects.toMatchObject({
                server_error_id: 'api.data_spillage.error.post_not_in_progress',
                status_code: 400,
            });
        });
    });

    describe('team access control routes', () => {
        let client: Client4;

        beforeEach(() => {
            client = new Client4();
            client.setUrl('http://mattermost.example.com');
        });

        test('assignTeamsToAccessControlPolicy POSTs team_ids to the assign route', async () => {
            let receivedBody: any;
            nock(client.getBaseRoute()).
                post('/access_control_policies/pol1/assign', (body) => {
                    receivedBody = body;
                    return true;
                }).
                reply(200, {status: 'OK'});

            const result = await client.assignTeamsToAccessControlPolicy('pol1', ['team1', 'team2']);

            expect(receivedBody).toEqual({team_ids: ['team1', 'team2']});
            expect(result).toEqual({status: 'OK'});
        });

        test('unassignTeamsFromAccessControlPolicy DELETEs team_ids to the unassign route', async () => {
            let receivedBody: any;
            nock(client.getBaseRoute()).
                delete('/access_control_policies/pol1/unassign', (body) => {
                    receivedBody = body;
                    return true;
                }).
                reply(200, {status: 'OK'});

            const result = await client.unassignTeamsFromAccessControlPolicy('pol1', ['team1']);

            expect(receivedBody).toEqual({team_ids: ['team1']});
            expect(result).toEqual({status: 'OK'});
        });

        test('getTeamAccessControlPolicy GETs the per-team policy route', async () => {
            const response = {policy: null, enforced: false};
            nock(client.getBaseRoute()).
                get('/teams/team1/access_control/policy').
                reply(200, response);

            const result = await client.getTeamAccessControlPolicy('team1');
            expect(result).toEqual(response);
        });

        test('getProfilesMatchingTeamPolicy GETs users with not_in_team + abac_match_only', async () => {
            const profiles = [{id: 'u1'}];
            nock(client.getBaseRoute()).
                get('/users').
                query({not_in_team: 'team1', per_page: '60', abac_match_only: 'true'}).
                reply(200, profiles);

            const result = await client.getProfilesMatchingTeamPolicy('team1');
            expect(result).toEqual(profiles);
        });

        test('getProfilesMatchingTeamPolicy includes cursor_id when provided', async () => {
            const profiles: any[] = [];
            nock(client.getBaseRoute()).
                get('/users').
                query({not_in_team: 'team1', per_page: '60', abac_match_only: 'true', cursor_id: 'u9'}).
                reply(200, profiles);

            const result = await client.getProfilesMatchingTeamPolicy('team1', 60, 'u9');
            expect(result).toEqual(profiles);
        });

        test('getTeams sends for_directory=true so admins are filtered on the directory listing', async () => {
            const teams = [{id: 't1'}];
            nock(client.getBaseRoute()).
                get('/teams').
                query({page: '0', per_page: '60', include_total_count: 'true', exclude_policy_constrained: 'false', for_directory: 'true'}).
                reply(200, teams);

            const result = await client.getTeams(0, 60, true, false, true);
            expect(result).toEqual(teams);
        });

        test('getTeams defaults for_directory to false so the management listing stays complete', async () => {
            const teams = [{id: 't1'}];
            nock(client.getBaseRoute()).
                get('/teams').
                query({page: '0', per_page: '60', include_total_count: 'false', exclude_policy_constrained: 'false', for_directory: 'false'}).
                reply(200, teams);

            const result = await client.getTeams(0, 60);
            expect(result).toEqual(teams);
        });
    });

    // Browsers upper-case only DELETE, GET, HEAD, OPTIONS, POST and PUT (the Fetch standard): a 'patch' goes out as
    // written, matches none of the server's PATCH routes and gets a 404. node-fetch, which the tests use, upper-cases
    // every method, so nock alone can't see it: these check what Client4 hands to fetch.
    describe('PATCH routes', () => {
        let client: Client4;
        let fetchSpy: jest.SpyInstance;

        beforeEach(() => {
            client = new Client4();
            client.setUrl('http://mattermost.example.com');
            fetchSpy = jest.spyOn(globalThis, 'fetch');
        });

        afterEach(() => {
            fetchSpy.mockRestore();
            nock.cleanAll();
        });

        test.each([
            ['patchCommunityProfile', '/teams/t1/members/u1/profile', (c: Client4) => c.patchCommunityProfile('t1', 'u1', {nickname: 'Mayo'})],
            ['patchCommunityAttribute', '/teams/t1/user_attributes/fields/f1', (c: Client4) => c.patchCommunityAttribute('t1', 'f1', {name: 'Role'})],
            ['patchCommunityAttributeValues', '/teams/t1/members/u1/user_attributes', (c: Client4) => c.patchCommunityAttributeValues('t1', 'u1', {f1: 'x'})],
            ['patchAIConnection', '/ai/connections/c1', (c: Client4) => c.patchAIConnection('c1', {name: 'Local'})],
            ['patchAIAgent', '/ai/agents/a1', (c: Client4) => c.patchAIAgent('a1', {})],
            ['patchAIMCPServer', '/ai/mcp_servers/s1', (c: Client4) => c.patchAIMCPServer('s1', {})],
        ])('%s sends PATCH to %s', async (_name, path, call) => {
            const scope = nock(client.getBaseRoute()).patch(path).reply(200, {}, {'Content-Type': 'application/json'});

            await call(client);

            expect(scope.isDone()).toBe(true);
            expect(fetchSpy).toHaveBeenCalledTimes(1);
            expect(fetchSpy.mock.calls[0][0]).toBe(client.getBaseRoute() + path);
            expect(fetchSpy.mock.calls[0][1].method).toBe('PATCH');
        });
    });

    describe('doFetchWithResponse', () => {
        test('serverVersion should be set from response header', async () => {
            const client = new Client4();
            client.setUrl('http://mattermost.example.com');

            expect(client.serverVersion).toEqual('');

            nock(client.getBaseRoute()).
                get('/users/me').
                reply(200, '{}', {[HEADER_X_VERSION_ID]: '5.0.0.5.0.0.abc123'});

            await client.getMe();

            expect(client.serverVersion).toEqual('5.0.0.5.0.0.abc123');

            nock(client.getBaseRoute()).
                get('/users/me').
                reply(200, '{}', {[HEADER_X_VERSION_ID]: '5.3.0.5.3.0.abc123'});

            await client.getMe();

            expect(client.serverVersion).toEqual('5.3.0.5.3.0.abc123');
        });

        test('should parse NDJSON responses correctly', async () => {
            const client = new Client4();
            client.setUrl('http://mattermost.example.com');

            const userId = 'dummy-user-id';
            const page = -1; // Special value to trigger NDJSON response

            // Sample NDJSON data with multiple channel memberships on separate lines
            const ndjsonData = '{"user_id":"dummy-user-id","channel_id":"channel1","roles":"channel_user"}\n' +
                '{"user_id":"dummy-user-id","channel_id":"channel2","roles":"channel_user channel_admin"}\n' +
                '{"user_id":"dummy-user-id","channel_id":"channel3","roles":"channel_user"}';

            // Create a mock endpoint for getAllChannelsMembers that returns NDJSON data
            nock(client.getBaseRoute()).
                get(`/users/${userId}/channel_members${buildQueryString({page, per_page: 60})}`).
                reply(200, ndjsonData, {'Content-Type': 'application/x-ndjson'});

            // Call the getAllChannelsMembers method which will use our implementation for NDJSON
            const result = await client.getAllChannelsMembers(userId, page);

            // Verify the response was parsed as an array of objects
            expect(Array.isArray(result)).toBe(true);
            expect(result).toHaveLength(3);
            expect(result[0]).toEqual({user_id: 'dummy-user-id', channel_id: 'channel1', roles: 'channel_user'});
            expect(result[1]).toEqual({user_id: 'dummy-user-id', channel_id: 'channel2', roles: 'channel_user channel_admin'});
            expect(result[2]).toEqual({user_id: 'dummy-user-id', channel_id: 'channel3', roles: 'channel_user'});
        });

        test('should parse ZIP responses as blobs', async () => {
            const client = new Client4();
            client.setUrl('http://mattermost.example.com');

            const postId = 'dummy-post-id';
            const zipData = Buffer.from('zip contents');

            nock(client.getBaseRoute()).
                post(`/content_flagging/post/${postId}/report`, {comment: 'investigation note'}).
                reply(200, zipData, {'Content-Type': 'application/zip'});

            const result = await client.generateFlaggedPostReport(postId, 'investigation note');

            expect(typeof result.text).toBe('function');
            expect(result.size).toEqual(zipData.length);
            expect(result.type).toEqual('application/zip');
            expect(await result.text()).toEqual('zip contents');
        });
    });
});

describe('ClientError', () => {
    test('standard fields should be enumerable', () => {
        const error = new ClientError('https://example.com', {
            message: 'This is a message',
            server_error_id: 'test.app_error',
            status_code: 418,
            url: 'https://example.com/api/v4/error',
        });

        const copy = {...error};

        expect(copy.message).toEqual(error.message);
        expect(copy.server_error_id).toEqual(error.server_error_id);
        expect(copy.status_code).toEqual(error.status_code);
        expect(copy.url).toEqual(error.url);
    });

    test('cause should be preserved when provided', () => {
        const cause = new Error('the original error');
        const error = new ClientError('https://example.com', {
            message: 'This is a message',
            server_error_id: 'test.app_error',
            status_code: 418,
            url: 'https://example.com/api/v4/error',
        }, cause);

        const copy = {...error};

        expect(copy.message).toEqual(error.message);
        expect(copy.server_error_id).toEqual(error.server_error_id);
        expect(copy.status_code).toEqual(error.status_code);
        expect(copy.url).toEqual(error.url);
        expect(error.cause).toEqual(cause);
    });
});

