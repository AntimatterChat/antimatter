// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {Theme} from 'mattermost-redux/selectors/entities/preferences';

import {parseColor, toHex} from './fusion_theme';

// The colours of the mockup's custom theme editor, in the order of its theme codes.
export const CUSTOM_TOKENS = ['rail', 'side', 'main', 'raise', 'line', 'text', 'muted', 'matter', 'anti'] as const;
export type CustomToken = typeof CUSTOM_TOKENS[number];
export type CustomTokens = Record<CustomToken, string>;

const hex = (color: string) => toHex(parseColor(color));

// Antimatter's own colours (Fusion Dark, styles/_00_tokens.scss): where "Start from Antimatter" starts.
export const ANTIMATTER_TOKENS: CustomTokens = {
    rail: '#1e1f22',
    side: '#2b2d31',
    main: '#313338',
    raise: '#383a40',
    line: '#26272b',
    text: '#dbdee1',
    muted: '#949ba4',
    matter: '#8b5cf6',
    anti: '#22d3ee',
};

// currentTokens gives the editor's colours as the Fusion UI shows them now, whatever the theme, as the mockup does.
export function currentTokens(): CustomTokens {
    const style = getComputedStyle(document.documentElement);
    return Object.fromEntries(CUSTOM_TOKENS.map((t) => {
        const value = style.getPropertyValue('--am-' + t).trim();
        return [t, value ? hex(value) : ANTIMATTER_TOKENS[t]];
    })) as CustomTokens;
}

// themeFromTokens makes a custom Mattermost theme from the editor's colours: the classic theme colours they map to,
// so the classic web app follows, and the surfaces only the Fusion UI uses.
export function themeFromTokens(tokens: CustomTokens, base: Theme): Theme {
    return {
        ...base,
        type: 'custom',
        sidebarTeamBarBg: tokens.rail,
        sidebarBg: tokens.side,
        sidebarHeaderBg: tokens.side,
        sidebarHeaderTextColor: tokens.text,
        sidebarText: tokens.muted,
        sidebarUnreadText: tokens.text,
        sidebarTextHoverBg: tokens.raise,
        sidebarTextActiveBorder: tokens.matter,
        sidebarTextActiveColor: tokens.text,
        centerChannelBg: tokens.main,
        centerChannelColor: tokens.text,
        buttonBg: tokens.matter,
        buttonColor: '#ffffff',
        linkColor: tokens.anti,
        fusionRail: tokens.rail,
        fusionSide: tokens.side,
        fusionRaise: tokens.raise,
        fusionLine: tokens.line,
        fusionMuted: tokens.muted,
    };
}

// A theme code is "am1:" and the nine colours, as the mockup shares custom themes.
export function themeCode(tokens: CustomTokens): string {
    return 'am1:' + CUSTOM_TOKENS.map((t) => tokens[t].replace('#', '')).join('-');
}

export function parseThemeCode(code: string): CustomTokens | null {
    const m = (/^am1:((?:[0-9a-f]{6}-){8}[0-9a-f]{6})$/i).exec(code.trim());
    if (!m) {
        return null;
    }
    const parts = m[1].split('-');
    return Object.fromEntries(CUSTOM_TOKENS.map((t, i) => [t, '#' + parts[i].toLowerCase()])) as CustomTokens;
}
