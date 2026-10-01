// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {Preferences} from 'mattermost-redux/constants';
import type {Theme} from 'mattermost-redux/selectors/entities/preferences';

// The Fusion UI's design tokens (styles/_00_tokens.scss) come in dark, light and OLED sets, picked with the
// data-am-theme attribute on <html>. The Fusion themes select a set; other themes (the classic presets and custom
// themes) get tokens derived from their colours.

const THEME_TOKEN_SETS: Record<string, string> = {
    'Fusion System': 'system',
    'Fusion Dark': 'dark',
    'Fusion Light': 'light',
    'Fusion OLED': 'oled',
};

const DERIVED_TOKENS = [
    'ground', 'rail', 'side', 'main', 'raise', 'raise-2', 'line', 'text', 'text-2', 'muted', 'faint',
    'matter', 'matter-soft', 'anti', 'anti-soft', 'spark', 'danger', 'ok', 'away', 'dnd', 'hover', 'select',
];

const systemPrefersLight = () => typeof window !== 'undefined' && window.matchMedia?.('(prefers-color-scheme: light)').matches;

// resolveSystemTheme gives Fusion System the colours of Fusion Light or Fusion Dark, following the operating system.
export function resolveSystemTheme(theme: Theme): Theme {
    if (theme.type !== 'Fusion System') {
        return theme;
    }
    return {...(systemPrefersLight() ? Preferences.THEMES.fusionLight : Preferences.THEMES.fusionDark), type: 'Fusion System'};
}

export function applyFusionTheme(theme: Theme) {
    const root = document.documentElement;
    for (const token of DERIVED_TOKENS) {
        root.style.removeProperty('--am-' + token);
    }

    const set = THEME_TOKEN_SETS[theme.type || ''];
    if (set) {
        root.dataset.amTheme = set;
        return;
    }

    const tokens = deriveTokens(theme);
    root.dataset.amTheme = tokens.dark ? 'dark' : 'light';
    for (const [token, value] of Object.entries(tokens.values)) {
        root.style.setProperty('--am-' + token, value);
    }
}

type RGB = [number, number, number];

export function parseColor(color: string): RGB {
    const hex = color.trim().replace('#', '');
    if ((/^[0-9a-f]{3}$/i).test(hex)) {
        return [0, 1, 2].map((i) => parseInt(hex[i] + hex[i], 16)) as RGB;
    }
    if ((/^[0-9a-f]{6}/i).test(hex)) {
        return [0, 2, 4].map((i) => parseInt(hex.slice(i, i + 2), 16)) as RGB;
    }
    const rgb = (/rgba?\(([^)]+)\)/).exec(color);
    if (rgb) {
        return rgb[1].split(',').slice(0, 3).map((v) => parseFloat(v)) as RGB;
    }
    return [128, 128, 128];
}

export const toHex = ([r, g, b]: RGB) => '#' + [r, g, b].map((v) => Math.round(v).toString(16).padStart(2, '0')).join('');
const mix = (a: string, b: string, amount: number) => {
    const x = parseColor(a);
    const y = parseColor(b);
    return toHex([0, 1, 2].map((i) => x[i] + ((y[i] - x[i]) * amount)) as RGB);
};
const alpha = (color: string, a: number) => `rgba(${parseColor(color).join(', ')}, ${a})`;
const luminance = (color: string) => {
    const [r, g, b] = parseColor(color).map((v) => v / 255);
    return (0.2126 * r) + (0.7152 * g) + (0.0722 * b);
};

// Custom themes made in the Fusion settings carry the surfaces the mockup's theme editor sets, beside the classic
// theme colours (see custom_theme.ts).
function customTokens(theme: Theme): {dark: boolean; values: Record<string, string>} | null {
    const {fusionRail: rail, fusionSide: side, fusionRaise: raise, fusionLine: line, fusionMuted: muted} = theme;
    if (!rail || !side || !raise || !line || !muted) {
        return null;
    }
    const main = theme.centerChannelBg;
    const text = theme.centerChannelColor;
    const dark = luminance(main) < 0.5;

    // As the mockup derives the tokens its editor doesn't show.
    return {
        dark,
        values: {
            ground: rail,
            rail,
            side,
            main,
            raise,
            'raise-2': raise,
            line,
            text,
            'text-2': text,
            muted,
            faint: muted,
            matter: theme.buttonBg,
            'matter-soft': alpha(theme.buttonBg, 0.16),
            anti: theme.linkColor,
            'anti-soft': alpha(theme.linkColor, 0.14),
            spark: theme.awayIndicator,
            danger: theme.errorTextColor || theme.dndIndicator,
            ok: theme.onlineIndicator,
            away: theme.awayIndicator,
            dnd: theme.dndIndicator,
            hover: alpha(text, dark ? 0.07 : 0.05),
            select: alpha(theme.buttonBg, 0.22),
        },
    };
}

// deriveTokens builds a single-surface palette from a theme's center channel colours and its accents, since
// the Fusion UI draws its sidebars in the same colour family as the conversation.
export function deriveTokens(theme: Theme): {dark: boolean; values: Record<string, string>} {
    const custom = customTokens(theme);
    if (custom) {
        return custom;
    }
    const main = theme.centerChannelBg;
    const text = theme.centerChannelColor;
    const dark = luminance(main) < 0.5;
    const side = dark ? mix(main, '#000000', 0.13) : mix(main, text, 0.05);
    const rail = dark ? mix(main, '#000000', 0.38) : mix(main, text, 0.085);
    return {
        dark,
        values: {
            ground: rail,
            rail,
            side,
            main,
            raise: mix(main, text, dark ? 0.07 : 0.05),
            'raise-2': mix(main, text, dark ? 0.12 : 0.1),
            line: dark ? mix(main, '#000000', 0.25) : mix(main, text, 0.12),
            text,
            'text-2': mix(text, main, 0.18),
            muted: mix(text, main, 0.38),
            faint: mix(text, main, 0.55),
            matter: theme.buttonBg,
            'matter-soft': alpha(theme.buttonBg, dark ? 0.18 : 0.1),
            anti: theme.linkColor,
            'anti-soft': alpha(theme.linkColor, 0.14),
            spark: theme.awayIndicator,
            danger: theme.errorTextColor || theme.dndIndicator,
            ok: theme.onlineIndicator,
            away: theme.awayIndicator,
            dnd: theme.dndIndicator,
            hover: alpha(text, dark ? 0.07 : 0.05),
            select: alpha(text, dark ? 0.13 : 0.09),
        },
    };
}
