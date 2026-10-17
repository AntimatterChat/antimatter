// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import Permissions from 'mattermost-redux/constants/permissions';

import AdminDefinition from 'components/admin_console/admin_definition';

import type {GlobalState} from 'types/store';

import {getAdminDefinition, getConsoleAccess} from './admin_console';

type TestReducerState = {
    something?: string;
    otherThing?: string;
};

describe('Selectors.AdminConsole', () => {
    describe('get admin definitions', () => {
        it('should return the default admin definition if there is not plugins', () => {
            const state = {plugins: {adminConsoleReducers: {}}} as GlobalState;
            expect(getAdminDefinition(state)).toEqual(AdminDefinition);
        });

        it('should allow to remove everything with a plugin', () => {
            const result = getAdminDefinition({
                plugins: {
                    adminConsoleReducers: {clean: () => ({})},
                },
            } as unknown as GlobalState);
            expect(result).toEqual({});
        });

        it('should allow to add a value to the existing definition', () => {
            const result = getAdminDefinition({
                plugins: {
                    adminConsoleReducers: {
                        'add-something': (data: TestReducerState) => {
                            return {
                                ...data,
                                something: 'test',
                            };
                        },
                    },
                },
            } as unknown as GlobalState);
            expect(result.something).toEqual('test');
        });

        it('should allow to use multiple plugins', () => {
            type TestReducerState = {
                something?: string;
                otherThing?: string;
            };
            const result = getAdminDefinition({
                plugins: {
                    adminConsoleReducers: {
                        'add-something': (data: TestReducerState) => {
                            return {
                                ...data,
                                something: 'test',
                            };
                        },
                        'add-other-thing': (data: TestReducerState) => {
                            return {
                                ...data,
                                otherThing: 'other-thing',
                            };
                        },
                    },
                },
            } as unknown as GlobalState);
            expect(result.something).toEqual('test');
            expect(result.otherThing).toEqual('other-thing');
        });
    });

    describe('getConsoleAccess', () => {
        const stateWith = (permissions: string[], adminConsoleReducers: Record<string, unknown> = {}) => ({
            entities: {
                users: {currentUserId: 'me', profiles: {me: {id: 'me', roles: 'system_user console_role'}}},
                roles: {roles: {console_role: {name: 'console_role', permissions}}},
            },
            plugins: {adminConsoleReducers},
        } as unknown as GlobalState);

        // The AI section has no System Console permissions: the console used to fail to open for everyone.
        it('gives no access to the sections without System Console permissions, without failing', () => {
            const access = getConsoleAccess(stateWith([Permissions.SYSCONSOLE_READ_PLUGINS]));
            expect(AdminDefinition).toHaveProperty('ai');
            expect(access.read.ai).toBe(false);
            expect(access.write.ai).toBe(false);
            expect(access.read.plugins).toBe(true);
            expect(access.write.plugins).toBe(false);
        });

        it('works out the access of a section a plugin adds', () => {
            const access = getConsoleAccess(stateWith([], {'add-section': (data: object) => ({...data, my_plugin: {}})}));
            expect(access.read.my_plugin).toBe(false);
            expect(access.read.plugins).toBe(false);
        });
    });
});
