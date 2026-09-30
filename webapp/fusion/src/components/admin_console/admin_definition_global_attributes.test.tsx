// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {AdminConfig} from '@mattermost/types/config';

import AdminDefinition from './admin_definition';
import type {Check, ConsoleAccess} from './types';

const config = {} as Partial<AdminConfig>;

const consoleAccess = {read: {}, write: {}} as ConsoleAccess;

function isDisabled(isSystemAdmin: boolean) {
    const subsection = AdminDefinition.system_attributes.subsections.global_attributes;
    const check = subsection.isDisabled as Extract<Check, (...args: any[]) => boolean>;
    return check(config, {}, consoleAccess, isSystemAdmin);
}

describe('AdminDefinition - Global Attributes access gate', () => {
    test('is always visible', () => {
        expect(AdminDefinition.system_attributes.subsections.global_attributes.isHidden).toBeUndefined();
    });

    test('disables the page for non-sysadmins', () => {
        expect(isDisabled(true)).toBe(false);
        expect(isDisabled(false)).toBe(true);
    });
});
