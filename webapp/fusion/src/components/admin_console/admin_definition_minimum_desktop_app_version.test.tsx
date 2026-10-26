// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import AdminDefinition from './admin_definition';
import type {AdminDefinitionSetting} from './types';

function findSetting(key: string): AdminDefinitionSetting | undefined {
    for (const section of Object.values(AdminDefinition)) {
        for (const subsection of Object.values(section.subsections ?? {})) {
            const schema = (subsection as {schema?: unknown})?.schema as {settings?: AdminDefinitionSetting[]; sections?: Array<{settings?: AdminDefinitionSetting[]}>} | undefined;
            const settings = [
                ...(schema?.settings ?? []),
                ...(schema?.sections ?? []).flatMap((s) => s.settings ?? []),
            ];
            const found = settings.find((item) => item.key === key);
            if (found) {
                return found;
            }
        }
    }
    return undefined;
}

describe('AdminDefinition - Minimum desktop app version setting', () => {
    test('explains that the server compares the "Mattermost/" user-agent version, which Antimatter Desktop reports as 6.4.0', () => {
        const setting = findSetting('ServiceSettings.MinimumDesktopAppVersion');

        expect(setting).toBeDefined();
        expect(setting?.help_text).toMatchObject({
            id: 'admin.customization.minimumDesktopAppVersionDesc',
            defaultMessage: expect.stringContaining('"Mattermost/"'),
        });
        expect(setting?.help_text).toMatchObject({
            defaultMessage: expect.stringContaining('(6.4.0)'),
        });
        expect(setting?.help_text).toMatchObject({
            defaultMessage: expect.not.stringContaining('minimum version of the Antimatter desktop app'),
        });
    });
});
