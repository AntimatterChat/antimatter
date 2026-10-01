// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {Preferences} from 'mattermost-redux/constants';
import type {Theme} from 'mattermost-redux/selectors/entities/preferences';

import {ANTIMATTER_TOKENS, parseThemeCode, themeCode, themeFromTokens} from './custom_theme';
import {deriveTokens} from './fusion_theme';

describe('fusion/theme/custom_theme', () => {
    test('theme codes round-trip', () => {
        const code = themeCode(ANTIMATTER_TOKENS);
        expect(code).toBe('am1:1e1f22-2b2d31-313338-383a40-26272b-dbdee1-949ba4-8b5cf6-22d3ee');
        expect(parseThemeCode(code)).toEqual(ANTIMATTER_TOKENS);
        expect(parseThemeCode(' ' + code.toUpperCase().replace('AM1', 'am1') + ' ')).toEqual(ANTIMATTER_TOKENS);
    });

    test('rejects what is not a theme code', () => {
        expect(parseThemeCode('')).toBeNull();
        expect(parseThemeCode('am1:1e1f22-2b2d31')).toBeNull();
        expect(parseThemeCode('am2:1e1f22-2b2d31-313338-383a40-26272b-dbdee1-949ba4-8b5cf6-22d3ee')).toBeNull();
    });

    test('a custom theme gives the Fusion UI its nine colours', () => {
        const tokens = {...ANTIMATTER_TOKENS, side: '#101010', line: '#202020'};
        const theme = themeFromTokens(tokens, Preferences.THEMES.fusionDark as Theme);
        expect(theme.type).toBe('custom');
        expect(theme.sidebarBg).toBe('#101010');
        const derived = deriveTokens(theme);
        expect(derived.dark).toBe(true);
        expect(derived.values.side).toBe('#101010');
        expect(derived.values.line).toBe('#202020');
        expect(derived.values.main).toBe(tokens.main);
        expect(derived.values.matter).toBe(tokens.matter);
    });
});
