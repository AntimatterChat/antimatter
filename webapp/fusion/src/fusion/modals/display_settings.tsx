// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import {savePreferences, saveTheme} from 'mattermost-redux/actions/preferences';
import {updateMe} from 'mattermost-redux/actions/users';
import {Preferences as ReduxPreferences} from 'mattermost-redux/constants';
import {getConfig} from 'mattermost-redux/selectors/entities/general';
import {getTheme} from 'mattermost-redux/selectors/entities/preferences';
import type {Theme} from 'mattermost-redux/selectors/entities/preferences';
import {getCurrentUser} from 'mattermost-redux/selectors/entities/users';

import {
    CATEGORY_FUSION_DISPLAY,
    COLORIZE_USERNAMES_DEFAULT,
    DEVELOPER_MODE,
    getDensity,
    SIMPLIFIED_INTERFACE,
    SPACIOUS_DISPLAY,
    STREAMER_MODE,
} from 'fusion/hooks/display';
import type {Density} from 'fusion/hooks/display';
import {useToast} from 'fusion/shell/toast_context';
import {am} from 'fusion/utils/class_names';
import {Preferences} from 'utils/constants';
import {copyToClipboard} from 'utils/utils';

import type {GlobalState} from 'types/store';

import {Choice, Section, Toggle, usePreference} from './settings_controls';

// Theme share codes: "am1:" and the theme's colours, to pass a custom theme on as text.
const THEME_CODE_PREFIX = 'am1:';

export function themeToCode(theme: Theme): string {
    return THEME_CODE_PREFIX + btoa(unescape(encodeURIComponent(JSON.stringify(theme))));
}

export function codeToTheme(code: string): Theme | null {
    const text = code.trim();
    if (!text.startsWith(THEME_CODE_PREFIX)) {
        return null;
    }
    try {
        const theme = JSON.parse(decodeURIComponent(escape(atob(text.slice(THEME_CODE_PREFIX.length)))));
        if (!theme || typeof theme !== 'object' || typeof theme.centerChannelBg !== 'string' || typeof theme.buttonBg !== 'string') {
            return null;
        }
        return {...theme, type: 'custom'} as Theme;
    } catch {
        return null;
    }
}

// DisplayRows are the Appearance pane's display settings: message density, availability, channel width, and the
// theme share codes.
export function DisplayRows() {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const toast = useToast();
    const theme = useSelector(getTheme);
    const density = useSelector(getDensity);
    const [width, setWidth] = usePreference(Preferences.CATEGORY_DISPLAY_SETTINGS, Preferences.CHANNEL_DISPLAY_MODE, Preferences.CHANNEL_DISPLAY_MODE_DEFAULT);
    const [statusOnPosts, setStatusOnPosts] = usePreference(Preferences.CATEGORY_DISPLAY_SETTINGS, Preferences.AVAILABILITY_STATUS_ON_POSTS, Preferences.AVAILABILITY_STATUS_ON_POSTS_DEFAULT);
    const me = useSelector(getCurrentUser);

    // Compact is Mattermost's message display; spacious is Antimatter's, beside it, with Mattermost's left standard.
    const setDensity = (next: Density) => dispatch(savePreferences(me.id, [
        {user_id: me.id, category: Preferences.CATEGORY_DISPLAY_SETTINGS, name: Preferences.MESSAGE_DISPLAY, value: next === 'compact' ? Preferences.MESSAGE_DISPLAY_COMPACT : Preferences.MESSAGE_DISPLAY_CLEAN},
        {user_id: me.id, category: CATEGORY_FUSION_DISPLAY, name: SPACIOUS_DISPLAY, value: String(next === 'spacious')},
    ]));
    const lastActiveEnabled = useSelector((state: GlobalState) => getConfig(state).EnableLastActiveTime === 'true');
    const [code, setCode] = useState('');

    const applyCode = () => {
        const imported = codeToTheme(code);
        if (!imported) {
            toast(formatMessage({id: 'fusion.settings.themeCodeBad', defaultMessage: 'That isn\'t a theme code'}));
            return;
        }
        dispatch(saveTheme('', imported));
        setCode('');
        toast(formatMessage({id: 'fusion.settings.themeCodeApplied', defaultMessage: 'Theme applied'}));
    };

    return (
        <>
            <Section title={formatMessage({id: 'fusion.settings.display', defaultMessage: 'Display'})}/>
            <Choice<Density>
                title={formatMessage({id: 'fusion.settings.messageDisplay', defaultMessage: 'Message display'})}
                desc={formatMessage({id: 'fusion.settings.messageDisplayDesc', defaultMessage: 'Compact fits more on screen: no pictures, tighter rows. Spacious leaves more room around messages and in the lists.'})}
                value={density}
                options={[
                    ['compact', formatMessage({id: 'fusion.settings.displayCompact', defaultMessage: 'Compact'})],
                    ['standard', formatMessage({id: 'fusion.settings.displayStandard', defaultMessage: 'Standard'})],
                    ['spacious', formatMessage({id: 'fusion.settings.displaySpacious', defaultMessage: 'Spacious'})],
                ]}
                onChange={setDensity}
            />
            <Toggle
                title={formatMessage({id: 'fusion.settings.statusOnPosts', defaultMessage: 'Availability on pictures'})}
                desc={formatMessage({id: 'fusion.settings.statusOnPostsDesc', defaultMessage: 'Show whether people are online on their pictures in messages.'})}
                on={statusOnPosts === 'true'}
                onChange={(on) => setStatusOnPosts(String(on))}
            />
            {lastActiveEnabled && (
                <Toggle
                    title={formatMessage({id: 'fusion.settings.shareLastActive', defaultMessage: 'Share when you were last online'})}
                    desc={formatMessage({id: 'fusion.settings.shareLastActiveDesc', defaultMessage: 'Others see "Last online 5 minutes ago" while you\'re away or offline.'})}
                    on={me.props?.show_last_active !== 'false'}
                    onChange={(on) => dispatch(updateMe({props: {...me.props, show_last_active: String(on)}}))}
                />
            )}
            <Choice
                title={formatMessage({id: 'fusion.settings.channelWidth', defaultMessage: 'Message width'})}
                value={width}
                options={[
                    [Preferences.CHANNEL_DISPLAY_MODE_FULL_SCREEN, formatMessage({id: 'fusion.settings.widthFull', defaultMessage: 'Full width'})],
                    [Preferences.CHANNEL_DISPLAY_MODE_CENTERED, formatMessage({id: 'fusion.settings.widthCentered', defaultMessage: 'Centred'})],
                ]}
                onChange={setWidth}
            />
            <Section title={formatMessage({id: 'fusion.settings.themeCode', defaultMessage: 'Share a theme'})}/>
            <div className={am('theme-code')}>
                <button
                    type='button'
                    className={am('btn')}
                    onClick={() => {
                        copyToClipboard(themeToCode(theme));
                        toast(formatMessage({id: 'fusion.settings.themeCodeCopied', defaultMessage: 'Theme code copied'}));
                    }}
                >
                    {formatMessage({id: 'fusion.settings.copyThemeCode', defaultMessage: 'Copy my theme\'s code'})}
                </button>
                <input
                    value={code}
                    placeholder={formatMessage({id: 'fusion.settings.themeCodePlaceholder', defaultMessage: 'Paste a theme code (am1:…)'})}
                    aria-label={formatMessage({id: 'fusion.settings.themeCodeLabel', defaultMessage: 'Theme code'})}
                    onChange={(e) => setCode(e.target.value)}
                    onKeyDown={(e) => e.key === 'Enter' && applyCode()}
                />
                <button
                    type='button'
                    className={am('btn', 'primary')}
                    disabled={!code.trim()}
                    onClick={applyCode}
                >
                    {formatMessage({id: 'fusion.settings.applyThemeCode', defaultMessage: 'Apply'})}
                </button>
            </div>
        </>
    );
}

// AccessibilityPane gathers what makes Antimatter easier to read and to use: names in colours of their own, which
// start off (roles already colour names, and colours alone tell little), and the simplified interface.
export function AccessibilityPane() {
    const {formatMessage} = useIntl();

    // The preference stays Mattermost's: whoever turned it on or off keeps their choice.
    const [colorize, setColorize] = usePreference(Preferences.CATEGORY_DISPLAY_SETTINGS, Preferences.COLORIZE_USERNAMES, COLORIZE_USERNAMES_DEFAULT);
    const [simplified, setSimplified] = usePreference(CATEGORY_FUSION_DISPLAY, SIMPLIFIED_INTERFACE, 'false');

    return (
        <>
            <h2>{formatMessage({id: 'fusion.settings.accessibility', defaultMessage: 'Accessibility'})}</h2>
            <p className={am('lead')}>{formatMessage({id: 'fusion.settings.accessibilityLead', defaultMessage: 'Make Antimatter easier to read and to use. Your system\'s reduced motion setting is followed too.'})}</p>
            <Toggle
                title={formatMessage({id: 'fusion.settings.simplified', defaultMessage: 'Simplified interface'})}
                desc={formatMessage({id: 'fusion.settings.simplifiedDesc', defaultMessage: 'Less on screen at once: bigger buttons and rows, no animations, the member list closed until you open it, counts only for mentions, and fewer buttons over messages. Everything stays within reach.'})}
                on={simplified === 'true'}
                onChange={(on) => setSimplified(String(on))}
            />
            <Toggle
                title={formatMessage({id: 'fusion.settings.colorize', defaultMessage: 'Colour names'})}
                desc={formatMessage({id: 'fusion.settings.colorizeDesc', defaultMessage: 'Give each person\'s name in messages a colour of their own, to tell people apart at a glance. Off by default: roles already colour the names of the people who hold them.'})}
                on={colorize === 'true'}
                onChange={(on) => setColorize(String(on))}
            />
        </>
    );
}

// ReadingRows are the Messages pane's reading settings: formatting, image previews, one-click reactions, and where
// unread channels open.
export function ReadingRows() {
    const {formatMessage} = useIntl();
    const [formatting, setFormatting] = usePreference(Preferences.CATEGORY_ADVANCED_SETTINGS, 'formatting', 'true');
    const [collapse, setCollapse] = usePreference(Preferences.CATEGORY_DISPLAY_SETTINGS, Preferences.COLLAPSE_DISPLAY, Preferences.COLLAPSE_DISPLAY_DEFAULT);
    const [oneClick, setOneClick] = usePreference(Preferences.CATEGORY_DISPLAY_SETTINGS, Preferences.ONE_CLICK_REACTIONS_ENABLED, Preferences.ONE_CLICK_REACTIONS_ENABLED_DEFAULT);
    const [scroll, setScroll] = usePreference(Preferences.CATEGORY_ADVANCED_SETTINGS, Preferences.UNREAD_SCROLL_POSITION, Preferences.UNREAD_SCROLL_POSITION_START_FROM_LEFT);
    const previewsAllowed = useSelector((state: GlobalState) => getConfig(state).EnableLinkPreviews === 'true');
    const [linkPreviews, setLinkPreviews] = usePreference(Preferences.CATEGORY_DISPLAY_SETTINGS, Preferences.LINK_PREVIEW_DISPLAY, Preferences.LINK_PREVIEW_DISPLAY_DEFAULT);
    const joinLeaveDefault = useSelector((state: GlobalState) => getConfig(state).EnableJoinLeaveMessageByDefault === 'true');
    const [joinLeave, setJoinLeave] = usePreference(Preferences.CATEGORY_ADVANCED_SETTINGS, ReduxPreferences.ADVANCED_FILTER_JOIN_LEAVE, String(joinLeaveDefault));

    return (
        <>
            <Section title={formatMessage({id: 'fusion.settings.reading', defaultMessage: 'Reading'})}/>
            <Choice
                title={formatMessage({id: 'fusion.settings.scrollPosition', defaultMessage: 'Open unread conversations at'})}
                value={scroll}
                options={[
                    [Preferences.UNREAD_SCROLL_POSITION_START_FROM_LEFT, formatMessage({id: 'fusion.settings.scrollLeftOff', defaultMessage: 'Where I left off'})],
                    [Preferences.UNREAD_SCROLL_POSITION_START_FROM_NEWEST, formatMessage({id: 'fusion.settings.scrollNewest', defaultMessage: 'The newest message'})],
                ]}
                onChange={setScroll}
            />
            <Toggle
                title={formatMessage({id: 'fusion.settings.expandImages', defaultMessage: 'Show images expanded'})}
                desc={formatMessage({id: 'fusion.settings.expandImagesDesc', defaultMessage: 'Otherwise images start folded, to open one by one. /expand and /collapse switch this too.'})}
                on={collapse !== 'true'}
                onChange={(on) => setCollapse(String(!on))}
            />
            {previewsAllowed && (
                <Toggle
                    title={formatMessage({id: 'fusion.settings.linkPreviews', defaultMessage: 'Link previews'})}
                    desc={formatMessage({id: 'fusion.settings.linkPreviewsDesc', defaultMessage: 'Show a preview of the websites linked in messages.'})}
                    on={linkPreviews !== 'false'}
                    onChange={(on) => setLinkPreviews(String(on))}
                />
            )}
            <Toggle
                title={formatMessage({id: 'fusion.settings.oneClick', defaultMessage: 'One-click reactions'})}
                desc={formatMessage({id: 'fusion.settings.oneClickDesc', defaultMessage: 'Your recent emoji in the toolbar over messages.'})}
                on={oneClick === 'true'}
                onChange={(on) => setOneClick(String(on))}
            />
            <Toggle
                title={formatMessage({id: 'fusion.settings.joinLeave', defaultMessage: 'Join and leave messages'})}
                desc={formatMessage({id: 'fusion.settings.joinLeaveDesc', defaultMessage: 'Show when people join or leave channels.'})}
                on={joinLeave !== 'false'}
                onChange={(on) => setJoinLeave(String(on))}
            />
            <Toggle
                title={formatMessage({id: 'fusion.settings.formatting', defaultMessage: 'Message formatting'})}
                desc={formatMessage({id: 'fusion.settings.formattingDesc', defaultMessage: 'Show links, emoji, bold and the rest of Markdown. Otherwise messages show as plain text.'})}
                on={formatting !== 'false'}
                onChange={(on) => setFormatting(String(on))}
            />
        </>
    );
}

// AdvancedPane has the settings few people need: streamer mode, developer mode, and the classic performance debugging
// switches when the server allows them.
export function AdvancedPane() {
    const {formatMessage} = useIntl();
    const config = useSelector(getConfig);
    const [streamer, setStreamer] = usePreference(CATEGORY_FUSION_DISPLAY, STREAMER_MODE, 'false');
    const [developer, setDeveloper] = usePreference(CATEGORY_FUSION_DISPLAY, DEVELOPER_MODE, 'false');
    const [syncDrafts, setSyncDrafts] = usePreference(Preferences.CATEGORY_ADVANCED_SETTINGS, ReduxPreferences.ADVANCED_SYNC_DRAFTS, 'true');
    const [noPlugins, setNoPlugins] = usePreference(ReduxPreferences.CATEGORY_PERFORMANCE_DEBUGGING, ReduxPreferences.NAME_DISABLE_CLIENT_PLUGINS, 'false');
    const [noTyping, setNoTyping] = usePreference(ReduxPreferences.CATEGORY_PERFORMANCE_DEBUGGING, ReduxPreferences.NAME_DISABLE_TYPING_MESSAGES, 'false');

    return (
        <>
            <h2>{formatMessage({id: 'fusion.settings.advanced', defaultMessage: 'Advanced'})}</h2>
            <p className={am('lead')}>{formatMessage({id: 'fusion.settings.advancedLead', defaultMessage: 'Settings for streaming, development and troubleshooting.'})}</p>
            <Toggle
                title={formatMessage({id: 'fusion.settings.streamer', defaultMessage: 'Streamer mode'})}
                desc={formatMessage({id: 'fusion.settings.streamerDesc', defaultMessage: 'Hide your email address and conversation previews, and silence desktop notifications, e.g. while sharing your screen.'})}
                on={streamer === 'true'}
                onChange={(on) => setStreamer(String(on))}
            />
            <Toggle
                title={formatMessage({id: 'fusion.settings.developer', defaultMessage: 'Developer mode'})}
                desc={formatMessage({id: 'fusion.settings.developerDesc', defaultMessage: 'Add Copy ID to the menus of messages, channels, communities and people, for the API and bots.'})}
                on={developer === 'true'}
                onChange={(on) => setDeveloper(String(on))}
            />
            {config.AllowSyncedDrafts === 'true' && (
                <Toggle
                    title={formatMessage({id: 'fusion.settings.syncDrafts', defaultMessage: 'Sync drafts'})}
                    desc={formatMessage({id: 'fusion.settings.syncDraftsDesc', defaultMessage: 'Keep the messages you haven\'t sent yet on the server, to finish them on another device.'})}
                    on={syncDrafts !== 'false'}
                    onChange={(on) => setSyncDrafts(String(on))}
                />
            )}
            {config.EnableClientPerformanceDebugging === 'true' && (
                <>
                    <Section title={formatMessage({id: 'fusion.settings.performance', defaultMessage: 'Performance debugging'})}/>
                    <Toggle
                        title={formatMessage({id: 'fusion.settings.noPlugins', defaultMessage: 'Turn off plugins in this browser'})}
                        desc={formatMessage({id: 'fusion.settings.noPluginsDesc', defaultMessage: 'Takes effect after reloading the page.'})}
                        on={noPlugins === 'true'}
                        onChange={(on) => setNoPlugins(String(on))}
                    />
                    <Toggle
                        title={formatMessage({id: 'fusion.settings.noTyping', defaultMessage: 'Don\'t send typing notices'})}
                        desc={formatMessage({id: 'fusion.settings.noTypingDesc', defaultMessage: 'Others don\'t see when you type.'})}
                        on={noTyping === 'true'}
                        onChange={(on) => setNoTyping(String(on))}
                    />
                </>
            )}
        </>
    );
}
