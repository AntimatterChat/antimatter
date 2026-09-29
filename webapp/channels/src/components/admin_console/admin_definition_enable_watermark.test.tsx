// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import AdminDefinition from './admin_definition';
import type {AdminDefinitionSetting} from './types';

describe('AdminDefinition - Enable Watermark setting', () => {
    const getFeatureSettings = () => {
        const featureSection = AdminDefinition.experimental.subsections.experimental_features;
        const settings = 'settings' in featureSection.schema! ? featureSection.schema.settings : undefined;

        return settings || [];
    };

    const getEnableWatermarkSetting = () => {
        return getFeatureSettings().find((setting: AdminDefinitionSetting) => setting.key === 'ExperimentalSettings.EnableWatermark');
    };

    test('uses ExperimentalSettings.EnableWatermark in the experimental features section', () => {
        const enableWatermarkSetting = getEnableWatermarkSetting();

        expect(enableWatermarkSetting).toBeDefined();
        expect(enableWatermarkSetting?.type).toBe('bool');
        expect(enableWatermarkSetting?.label).toBeDefined();
        expect(enableWatermarkSetting?.help_text).toBeDefined();
    });

    test('is always visible', () => {
        expect(getEnableWatermarkSetting()?.isHidden).toBeUndefined();
    });
});
