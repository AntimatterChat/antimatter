// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useRef, useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import {saveTheme} from 'mattermost-redux/actions/preferences';
import {getTheme} from 'mattermost-redux/selectors/entities/preferences';

import {useToast} from 'fusion/shell/toast_context';
import {ANTIMATTER_TOKENS, CUSTOM_TOKENS, currentTokens, parseThemeCode, themeCode, themeFromTokens} from 'fusion/theme/custom_theme';
import type {CustomToken, CustomTokens} from 'fusion/theme/custom_theme';
import {am} from 'fusion/utils/class_names';
import {applyTheme, copyToClipboard} from 'utils/utils';

const LABELS: Record<CustomToken, {id: string; defaultMessage: string}> = {
    rail: {id: 'fusion.theme.rail', defaultMessage: 'Team rail'},
    side: {id: 'fusion.theme.side', defaultMessage: 'Sidebars'},
    main: {id: 'fusion.theme.main', defaultMessage: 'Chat area'},
    raise: {id: 'fusion.theme.raise', defaultMessage: 'Inputs & cards'},
    line: {id: 'fusion.theme.line', defaultMessage: 'Borders'},
    text: {id: 'fusion.theme.text', defaultMessage: 'Text'},
    muted: {id: 'fusion.theme.muted', defaultMessage: 'Secondary text'},
    matter: {id: 'fusion.theme.matter', defaultMessage: 'Accent'},
    anti: {id: 'fusion.theme.anti', defaultMessage: 'Links'},
};

// How long the editor waits after the last change before saving the theme.
const SAVE_DELAY = 600;

// useStartCustomTheme switches to a custom theme made from the colours shown now: the mockup's "Custom" card.
export function useStartCustomTheme() {
    const dispatch = useDispatch();
    const theme = useSelector(getTheme);
    return () => dispatch(saveTheme('', themeFromTokens(currentTokens(), theme)));
}

// CustomThemeEditor edits a custom theme: the mockup's nine colours and its shareable theme code.
export default function CustomThemeEditor() {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const toast = useToast();
    const theme = useSelector(getTheme);
    const [tokens, setTokens] = useState<CustomTokens>(currentTokens);
    const [code, setCode] = useState(() => themeCode(tokens));
    const timer = useRef<ReturnType<typeof setTimeout>>(undefined);
    const themeRef = useRef(theme);
    themeRef.current = theme;

    useEffect(() => () => clearTimeout(timer.current), []);

    // Colours apply at once and are saved (to every device) once the picking stops.
    const change = (next: CustomTokens, saveNow = false) => {
        setTokens(next);
        setCode(themeCode(next));
        const nextTheme = themeFromTokens(next, themeRef.current);
        applyTheme(nextTheme);
        clearTimeout(timer.current);
        timer.current = setTimeout(() => dispatch(saveTheme('', nextTheme)), saveNow ? 0 : SAVE_DELAY);
    };

    const importCode = () => {
        const parsed = parseThemeCode(code);
        if (!parsed) {
            toast(formatMessage({id: 'fusion.toast.badThemeCode', defaultMessage: 'That is not a theme code. It starts with am1: followed by nine colours.'}));
            return;
        }
        change(parsed, true);
        toast(formatMessage({id: 'fusion.toast.themeImported', defaultMessage: 'Theme imported'}));
    };

    return (
        <>
            <div className={am('field')}>
                <span>{formatMessage({id: 'fusion.theme.custom', defaultMessage: 'Custom colours'})}</span>
            </div>
            <div className={am('custom-grid')}>
                {CUSTOM_TOKENS.map((t) => (
                    <label key={t}>
                        <input
                            type='color'
                            value={tokens[t]}
                            onChange={(e) => change({...tokens, [t]: e.target.value})}
                        />
                        {formatMessage(LABELS[t])}
                    </label>
                ))}
            </div>
            <div className={am('field')}>
                <span>{formatMessage({id: 'fusion.theme.code', defaultMessage: 'Theme code'})}</span>
                <div className={am('theme-code')}>
                    <input
                        value={code}
                        aria-label={formatMessage({id: 'fusion.theme.code', defaultMessage: 'Theme code'})}
                        spellCheck={false}
                        onChange={(e) => setCode(e.target.value)}
                        onKeyDown={(e) => {
                            if (e.key === 'Enter') {
                                e.preventDefault();
                                importCode();
                            }
                        }}
                    />
                    <button
                        type='button'
                        className={am('btn')}
                        onClick={() => {
                            copyToClipboard(themeCode(tokens));
                            toast(formatMessage({id: 'fusion.toast.themeCopied', defaultMessage: 'Theme code copied'}));
                        }}
                    >
                        {formatMessage({id: 'fusion.theme.copy', defaultMessage: 'Copy'})}
                    </button>
                    <button
                        type='button'
                        className={am('btn')}
                        onClick={importCode}
                    >
                        {formatMessage({id: 'fusion.theme.import', defaultMessage: 'Import'})}
                    </button>
                    <button
                        type='button'
                        className={am('btn')}
                        onClick={() => change(ANTIMATTER_TOKENS, true)}
                    >
                        {formatMessage({id: 'fusion.theme.reset', defaultMessage: 'Start from Antimatter'})}
                    </button>
                </div>
                <small>{formatMessage({id: 'fusion.theme.codeHint', defaultMessage: 'Share this code, or paste one from someone else and choose Import.'})}</small>
            </div>
        </>
    );
}
