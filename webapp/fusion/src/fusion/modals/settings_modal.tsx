// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import {CollapsedThreads} from '@mattermost/types/config';
import type {UserNotifyProps} from '@mattermost/types/users';

import {savePreferences, saveTheme} from 'mattermost-redux/actions/preferences';
import {updateMe} from 'mattermost-redux/actions/users';
import {Preferences} from 'mattermost-redux/constants';
import {getConfig} from 'mattermost-redux/selectors/entities/general';
import {get as getPreference, getCollapsedThreadsPreference, getTheme, isCollapsedThreadsAllowed} from 'mattermost-redux/selectors/entities/preferences';
import type {Theme} from 'mattermost-redux/selectors/entities/preferences';
import {getCurrentTimezoneLabel} from 'mattermost-redux/selectors/entities/timezone';
import {getCurrentUser} from 'mattermost-redux/selectors/entities/users';
import {isSystemAdmin} from 'mattermost-redux/utils/user_utils';

import {switchWebUI} from 'actions/web_ui';

import Avatar from 'fusion/components/avatar';
import Icon from 'fusion/components/icon';
import {Dialog} from 'fusion/components/layer';
import {am} from 'fusion/utils/class_names';
import {openClassicUserSettings} from 'fusion/utils/modals';
import {getLanguages} from 'i18n/i18n';
import {getHistory} from 'utils/browser_history';
import Constants from 'utils/constants';
import {CURRENT_WEB_UI, WebUIs} from 'utils/web_ui';
import type {WebUI} from 'utils/web_ui';

import type {GlobalState} from 'types/store';

import CustomThemeEditor, {useStartCustomTheme} from './custom_theme_editor';

export type SettingsTab = 'account' | 'security' | 'notifications' | 'appearance' | 'messages' | 'language';

const TABS: Array<[SettingsTab, {id: string; defaultMessage: string}]> = [
    ['account', {id: 'fusion.settings.account', defaultMessage: 'My account'}],
    ['security', {id: 'fusion.settings.security', defaultMessage: 'Security'}],
    ['notifications', {id: 'fusion.settings.notifications', defaultMessage: 'Notifications'}],
    ['appearance', {id: 'fusion.settings.appearance', defaultMessage: 'Appearance'}],
    ['messages', {id: 'fusion.settings.messages', defaultMessage: 'Messages'}],
    ['language', {id: 'fusion.settings.language', defaultMessage: 'Language & time'}],
];

function Toggle({title, desc, on, onChange}: {title: string; desc: string; on: boolean; onChange: (on: boolean) => void}) {
    return (
        <div className={am('toggle-row')}>
            <div className={am('txt')}>
                <b>{title}</b>
                <span>{desc}</span>
            </div>
            <button
                type='button'
                className={am('switch')}
                role='switch'
                aria-checked={on}
                aria-label={title}
                onClick={() => onChange(!on)}
            />
        </div>
    );
}

// Choice is a setting row with a few mutually exclusive options, as a segmented control.
function Choice<T extends string>({title, desc, value, options, onChange}: {title: string; desc?: string; value: T; options: Array<[T, string]>; onChange: (value: T) => void}) {
    return (
        <div className={am('toggle-row', 'choice-row')}>
            <div className={am('txt')}>
                <b>{title}</b>
                {desc && <span>{desc}</span>}
            </div>
            <div
                className={am('seg')}
                role='group'
                aria-label={title}
            >
                {options.map(([key, label]) => (
                    <button
                        key={key}
                        type='button'
                        className={am({on: key === value})}
                        aria-pressed={key === value}
                        onClick={() => key !== value && onChange(key)}
                    >
                        {label}
                    </button>
                ))}
            </div>
        </div>
    );
}

// Action is a setting row whose button opens where the setting is changed.
function Action({title, desc, label, danger = false, onClick}: {title: string; desc: string; label: string; danger?: boolean; onClick: () => void}) {
    return (
        <div className={am('toggle-row')}>
            <div className={am('txt')}>
                <b>{title}</b>
                <span>{desc}</span>
            </div>
            <button
                type='button'
                className={am('btn', {danger})}
                onClick={onClick}
            >
                {label}
            </button>
        </div>
    );
}

// usePreference reads one of the current user's preferences, and saves it.
function usePreference(category: string, name: string, fallback: string): [string, (value: string) => void] {
    const dispatch = useDispatch();
    const me = useSelector(getCurrentUser);
    const value = useSelector((state: GlobalState) => getPreference(state, category, name, fallback));
    const save = (next: string) => dispatch(savePreferences(me.id, [{user_id: me.id, category, name, value: next}]));
    return [value, save];
}

function AccountPane() {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const me = useSelector(getCurrentUser);
    const [fields, setFields] = useState({first_name: me.first_name, last_name: me.last_name, nickname: me.nickname, position: me.position});
    const [saved, setSaved] = useState(false);
    const dirty = Object.entries(fields).some(([k, v]) => v !== me[k as keyof typeof fields]);
    const field = (key: keyof typeof fields, label: string) => (
        <label className={am('field')}>
            <span>{label}</span>
            <input
                type='text'
                value={fields[key]}
                onChange={(e) => {
                    setSaved(false);
                    setFields({...fields, [key]: e.target.value});
                }}
            />
        </label>
    );
    return (
        <>
            <h2>{formatMessage({id: 'fusion.settings.account', defaultMessage: 'My account'})}</h2>
            <p className={am('lead')}>{formatMessage({id: 'fusion.settings.accountLead', defaultMessage: 'How you appear to others on this server.'})}</p>
            <div style={{display: 'flex', gap: 16, alignItems: 'center', marginBottom: 18}}>
                <Avatar
                    userId={me.id}
                    size='xl'
                    status={true}
                />
                <div>
                    <div style={{font: '600 18px var(--am-font-display)'}}>{[me.first_name, me.last_name].filter(Boolean).join(' ') || me.username}</div>
                    <div style={{color: 'var(--am-muted)'}}>{`@${me.username} · ${me.email}`}</div>
                </div>
            </div>
            <div className={am('field-row')}>
                {field('first_name', formatMessage({id: 'fusion.settings.firstName', defaultMessage: 'First name'}))}
                {field('last_name', formatMessage({id: 'fusion.settings.lastName', defaultMessage: 'Last name'}))}
            </div>
            {field('nickname', formatMessage({id: 'fusion.settings.nickname', defaultMessage: 'Nickname'}))}
            {field('position', formatMessage({id: 'fusion.settings.position', defaultMessage: 'Position'}))}
            <div className={am('modal-actions')}>
                <button
                    type='button'
                    className={am('btn')}
                    onClick={() => dispatch(openClassicUserSettings('profile'))}
                >
                    {formatMessage({id: 'fusion.settings.moreAccount', defaultMessage: 'Picture, username and email…'})}
                </button>
                <button
                    type='button'
                    className={am('btn', 'primary')}
                    disabled={!dirty}
                    onClick={async () => {
                        await dispatch(updateMe(fields));
                        setSaved(true);
                    }}
                >
                    {saved ? formatMessage({id: 'fusion.settings.saved', defaultMessage: 'Saved'}) : formatMessage({id: 'fusion.settings.save', defaultMessage: 'Save'})}
                </button>
            </div>
        </>
    );
}

function SecurityPane() {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const config = useSelector(getConfig);
    const openSecurity = () => dispatch(openClassicUserSettings('security'));
    return (
        <>
            <h2>{formatMessage({id: 'fusion.settings.security', defaultMessage: 'Security'})}</h2>
            <p className={am('lead')}>{formatMessage({id: 'fusion.settings.securityLead', defaultMessage: 'Your password, two-step sign-in and the devices you\'re signed in on.'})}</p>
            <Action
                title={formatMessage({id: 'fusion.settings.password', defaultMessage: 'Password'})}
                desc={formatMessage({id: 'fusion.settings.passwordDesc', defaultMessage: 'Change the password you sign in with.'})}
                label={formatMessage({id: 'fusion.settings.change', defaultMessage: 'Change…'})}
                onClick={openSecurity}
            />
            {config.EnableMultifactorAuthentication === 'true' && (
                <Action
                    title={formatMessage({id: 'fusion.settings.mfa', defaultMessage: 'Multi-factor authentication'})}
                    desc={formatMessage({id: 'fusion.settings.mfaDesc', defaultMessage: 'Ask for a code from an authenticator app when you sign in.'})}
                    label={formatMessage({id: 'fusion.settings.manage', defaultMessage: 'Manage…'})}
                    onClick={openSecurity}
                />
            )}
            <Action
                title={formatMessage({id: 'fusion.settings.sessions', defaultMessage: 'Where you\'re signed in'})}
                desc={formatMessage({id: 'fusion.settings.sessionsDesc', defaultMessage: 'See the browsers and devices signed in to your account, and sign them out.'})}
                label={formatMessage({id: 'fusion.settings.view', defaultMessage: 'View…'})}
                onClick={openSecurity}
            />
            {config.EnableUserDeactivation === 'true' && (
                <Action
                    title={formatMessage({id: 'fusion.settings.deactivate', defaultMessage: 'Deactivate account'})}
                    desc={formatMessage({id: 'fusion.settings.deactivateDesc', defaultMessage: 'Leave this server. An admin can reactivate your account later.'})}
                    label={formatMessage({id: 'fusion.settings.deactivateButton', defaultMessage: 'Deactivate…'})}
                    danger={true}
                    onClick={() => dispatch(openClassicUserSettings('advanced'))}
                />
            )}
        </>
    );
}

function NotificationsPane() {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const me = useSelector(getCurrentUser);
    const threads = useSelector((state: GlobalState) => isCollapsedThreadsAllowed(state) && getCollapsedThreadsPreference(state) === Preferences.COLLAPSED_REPLY_THREADS_ON);
    const notifyProps = me.notify_props;
    const set = (patch: Partial<UserNotifyProps>) => dispatch(updateMe({notify_props: {...notifyProps, ...patch}}));
    return (
        <>
            <h2>{formatMessage({id: 'fusion.settings.notifications', defaultMessage: 'Notifications'})}</h2>
            <p className={am('lead')}>{formatMessage({id: 'fusion.settings.notificationsLead', defaultMessage: 'Choose what reaches you on desktop, mobile and by email.'})}</p>
            <Toggle
                title={formatMessage({id: 'fusion.settings.desktopAll', defaultMessage: 'Desktop notifications for all activity'})}
                desc={formatMessage({id: 'fusion.settings.desktopAllDesc', defaultMessage: 'Otherwise only mentions, direct messages and followed threads.'})}
                on={notifyProps.desktop === 'all'}
                onChange={(on) => set({desktop: on ? 'all' : 'mention'})}
            />
            <Toggle
                title={formatMessage({id: 'fusion.settings.desktopSound', defaultMessage: 'Notification sound'})}
                desc={formatMessage({id: 'fusion.settings.desktopSoundDesc', defaultMessage: 'Play a sound with desktop notifications.'})}
                on={notifyProps.desktop_sound !== 'false'}
                onChange={(on) => set({desktop_sound: on ? 'true' : 'false'})}
            />
            {threads && (
                <Toggle
                    title={formatMessage({id: 'fusion.settings.threadReplies', defaultMessage: 'Replies to threads you follow'})}
                    desc={formatMessage({id: 'fusion.settings.threadRepliesDesc', defaultMessage: 'Notify me of every reply in the threads I follow, not only mentions.'})}
                    on={notifyProps.desktop_threads === 'all'}
                    onChange={(on) => set({desktop_threads: on ? 'all' : 'mention'})}
                />
            )}
            <Toggle
                title={formatMessage({id: 'fusion.settings.push', defaultMessage: 'Mobile push notifications'})}
                desc={formatMessage({id: 'fusion.settings.pushDesc', defaultMessage: 'For mentions and direct messages.'})}
                on={notifyProps.push !== 'none'}
                onChange={(on) => set({push: on ? 'mention' : 'none'})}
            />
            <Toggle
                title={formatMessage({id: 'fusion.settings.email', defaultMessage: 'Email notifications'})}
                desc={formatMessage({id: 'fusion.settings.emailDesc', defaultMessage: 'When you are away or offline.'})}
                on={notifyProps.email === 'true'}
                onChange={(on) => set({email: on ? 'true' : 'false'})}
            />
            <Toggle
                title={formatMessage({id: 'fusion.settings.channelWide', defaultMessage: 'Channel-wide mentions'})}
                desc={formatMessage({id: 'fusion.settings.channelWideDesc', defaultMessage: 'Notify me for @channel, @all and @here.'})}
                on={notifyProps.channel === 'true'}
                onChange={(on) => set({channel: on ? 'true' : 'false'})}
            />
            <div className={am('modal-actions')}>
                <button
                    type='button'
                    className={am('btn')}
                    onClick={() => dispatch(openClassicUserSettings('notifications'))}
                >
                    {formatMessage({id: 'fusion.settings.moreNotifications', defaultMessage: 'Keywords, automatic replies and more…'})}
                </button>
            </div>
        </>
    );
}

const THEME_CARDS: Array<{key: keyof typeof Preferences.THEMES; name: {id: string; defaultMessage: string}; sw?: [string, string, string]; acc: string}> = [
    {key: 'fusionSystem', name: {id: 'fusion.settings.themeSystem', defaultMessage: 'System'}, acc: '#8B5CF6'},
    {key: 'fusionDark', name: {id: 'fusion.settings.themeDark', defaultMessage: 'Dark'}, sw: ['#1E1F22', '#2B2D31', '#313338'], acc: '#8B5CF6'},
    {key: 'fusionOled', name: {id: 'fusion.settings.themeOled', defaultMessage: 'OLED'}, sw: ['#000000', '#0E0E10', '#000000'], acc: '#9D7BFA'},
    {key: 'fusionLight', name: {id: 'fusion.settings.themeLight', defaultMessage: 'Light'}, sw: ['#E9E6F3', '#F2F0F9', '#FFFFFF'], acc: '#6D28D9'},
];

function AppearancePane() {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const config = useSelector(getConfig);
    const theme = useSelector(getTheme);
    const [militaryTime, setMilitaryTime] = usePreference(Preferences.CATEGORY_DISPLAY_SETTINGS, Preferences.USE_MILITARY_TIME, 'false');
    const [nameFormat, setNameFormat] = usePreference(Preferences.CATEGORY_DISPLAY_SETTINGS, Preferences.NAME_NAME_FORMAT, config.TeammateNameDisplay || Preferences.DISPLAY_PREFER_USERNAME);
    const custom = !THEME_CARDS.some((c) => Preferences.THEMES[c.key].type === theme.type);
    const startCustom = useStartCustomTheme();

    return (
        <>
            <h2>{formatMessage({id: 'fusion.settings.appearance', defaultMessage: 'Appearance'})}</h2>
            <p className={am('lead')}>{formatMessage({id: 'fusion.settings.appearanceLead', defaultMessage: 'Pick a theme or build your own. It follows you on every device.'})}</p>
            <div
                className={am('theme-grid')}
                role='group'
                aria-label={formatMessage({id: 'fusion.settings.theme', defaultMessage: 'Theme'})}
            >
                {THEME_CARDS.map((card) => {
                    const on = Preferences.THEMES[card.key].type === theme.type;
                    return (
                        <button
                            key={card.key}
                            type='button'
                            className={am('theme-card', {on})}
                            aria-pressed={on}
                            onClick={() => dispatch(saveTheme('', Preferences.THEMES[card.key] as Theme))}
                        >
                            {card.sw ? (
                                <span
                                    className={am('sw')}
                                    style={{'--am-acc': card.acc} as React.CSSProperties}
                                >
                                    <i style={{background: card.sw[0]}}/>
                                    <i style={{background: card.sw[1]}}/>
                                    <i style={{background: card.sw[2]}}/>
                                </span>
                            ) : (
                                <span
                                    className={am('sw')}
                                    style={{'--am-acc': card.acc, background: 'linear-gradient(135deg,#313338 50%,#F2F0F9 50%)'} as React.CSSProperties}
                                >
                                    <i/><i/><i/>
                                </span>
                            )}
                            {formatMessage(card.name)}
                        </button>
                    );
                })}
                <button
                    type='button'
                    className={am('theme-card', {on: custom})}
                    aria-pressed={custom}
                    onClick={() => !custom && startCustom()}
                >
                    <span
                        className={am('sw')}
                        style={{'--am-acc': theme.buttonBg} as React.CSSProperties}
                    >
                        <i style={{background: theme.sidebarTeamBarBg}}/>
                        <i style={{background: theme.sidebarBg}}/>
                        <i style={{background: theme.centerChannelBg}}/>
                    </span>
                    {formatMessage({id: 'fusion.settings.themeCustom', defaultMessage: 'Custom'})}
                </button>
            </div>
            {custom && <CustomThemeEditor/>}
            {config.AllowUserWebUISelection === 'true' && (
                <Choice<WebUI>
                    title={formatMessage({id: 'fusion.settings.interface', defaultMessage: 'Web interface'})}
                    desc={formatMessage({id: 'fusion.settings.interfaceDesc', defaultMessage: 'Antimatter has two web interfaces. Your choice applies to every browser you sign in from.'})}
                    value={CURRENT_WEB_UI}
                    options={[
                        [WebUIs.FUSION, formatMessage({id: 'fusion.settings.fusion', defaultMessage: 'Fusion'})],
                        [WebUIs.CLASSIC, formatMessage({id: 'fusion.settings.classic', defaultMessage: 'Classic'})],
                    ]}
                    onChange={(ui) => dispatch(switchWebUI(ui))}
                />
            )}
            {config.LockTeammateNameDisplay !== 'true' && (
                <Choice
                    title={formatMessage({id: 'fusion.settings.names', defaultMessage: 'Show people as'})}
                    value={nameFormat}
                    options={[
                        [Preferences.DISPLAY_PREFER_USERNAME, formatMessage({id: 'fusion.settings.namesUsername', defaultMessage: 'Username'})],
                        [Preferences.DISPLAY_PREFER_NICKNAME, formatMessage({id: 'fusion.settings.namesNickname', defaultMessage: 'Nickname'})],
                        [Preferences.DISPLAY_PREFER_FULL_NAME, formatMessage({id: 'fusion.settings.namesFull', defaultMessage: 'Full name'})],
                    ]}
                    onChange={setNameFormat}
                />
            )}
            <Toggle
                title={formatMessage({id: 'fusion.settings.clock', defaultMessage: '24-hour clock'})}
                desc={formatMessage({id: 'fusion.settings.clockDesc', defaultMessage: 'Show times as 16:00 instead of 4:00 PM.'})}
                on={militaryTime === 'true'}
                onChange={(on) => setMilitaryTime(String(on))}
            />
        </>
    );
}

function MessagesPane() {
    const {formatMessage} = useIntl();
    const config = useSelector(getConfig);
    const threadsChoice = useSelector((state: GlobalState) => isCollapsedThreadsAllowed(state) && getConfig(state).CollapsedThreads !== CollapsedThreads.ALWAYS_ON);
    const [threads, setThreads] = usePreference(Preferences.CATEGORY_DISPLAY_SETTINGS, Preferences.COLLAPSED_REPLY_THREADS, useSelector(getCollapsedThreadsPreference));
    const [linkPreviews, setLinkPreviews] = usePreference(Preferences.CATEGORY_DISPLAY_SETTINGS, Constants.Preferences.LINK_PREVIEW_DISPLAY, Constants.Preferences.LINK_PREVIEW_DISPLAY_DEFAULT);
    const [emoticons, setEmoticons] = usePreference(Preferences.CATEGORY_DISPLAY_SETTINGS, Preferences.RENDER_EMOTICONS_AS_EMOJI, 'true');
    const [joinLeave, setJoinLeave] = usePreference(Preferences.CATEGORY_ADVANCED_SETTINGS, Preferences.ADVANCED_FILTER_JOIN_LEAVE, config.EnableJoinLeaveMessageByDefault || 'true');
    const [ctrlSend, setCtrlSend] = usePreference(Preferences.CATEGORY_ADVANCED_SETTINGS, Preferences.ADVANCED_SEND_ON_CTRL_ENTER, 'false');
    const [syncDrafts, setSyncDrafts] = usePreference(Preferences.CATEGORY_ADVANCED_SETTINGS, Preferences.ADVANCED_SYNC_DRAFTS, 'true');

    return (
        <>
            <h2>{formatMessage({id: 'fusion.settings.messages', defaultMessage: 'Messages'})}</h2>
            <p className={am('lead')}>{formatMessage({id: 'fusion.settings.messagesLead', defaultMessage: 'How conversations are shown, and how you write in them.'})}</p>
            {threadsChoice && (
                <Toggle
                    title={formatMessage({id: 'fusion.settings.threads', defaultMessage: 'Keep replies in threads'})}
                    desc={formatMessage({id: 'fusion.settings.threadsDesc', defaultMessage: 'Thread replies stay in their thread instead of the channel, and the threads you follow are listed in Threads.'})}
                    on={threads === Preferences.COLLAPSED_REPLY_THREADS_ON}
                    onChange={(on) => setThreads(on ? Preferences.COLLAPSED_REPLY_THREADS_ON : Preferences.COLLAPSED_REPLY_THREADS_OFF)}
                />
            )}
            {config.EnableLinkPreviews === 'true' && (
                <Toggle
                    title={formatMessage({id: 'fusion.settings.linkPreviews', defaultMessage: 'Link previews'})}
                    desc={formatMessage({id: 'fusion.settings.linkPreviewsDesc', defaultMessage: 'Show a preview of the websites linked in messages.'})}
                    on={linkPreviews === 'true'}
                    onChange={(on) => setLinkPreviews(String(on))}
                />
            )}
            <Toggle
                title={formatMessage({id: 'fusion.settings.emoticons', defaultMessage: 'Turn emoticons into emoji'})}
                desc={formatMessage({id: 'fusion.settings.emoticonsDesc', defaultMessage: 'Show :) and <3 as emoji.'})}
                on={emoticons === 'true'}
                onChange={(on) => setEmoticons(String(on))}
            />
            <Toggle
                title={formatMessage({id: 'fusion.settings.joinLeave', defaultMessage: 'Join and leave messages'})}
                desc={formatMessage({id: 'fusion.settings.joinLeaveDesc', defaultMessage: 'Show when people join or leave channels.'})}
                on={joinLeave === 'true'}
                onChange={(on) => setJoinLeave(String(on))}
            />
            <Toggle
                title={formatMessage({id: 'fusion.settings.ctrlSend', defaultMessage: 'Send with Ctrl+Enter'})}
                desc={formatMessage({id: 'fusion.settings.ctrlSendDesc', defaultMessage: 'Enter starts a new line, and Ctrl+Enter (⌘+Enter on a Mac) sends.'})}
                on={ctrlSend === 'true'}
                onChange={(on) => setCtrlSend(String(on))}
            />
            {config.AllowSyncedDrafts === 'true' && (
                <Toggle
                    title={formatMessage({id: 'fusion.settings.syncDrafts', defaultMessage: 'Sync drafts'})}
                    desc={formatMessage({id: 'fusion.settings.syncDraftsDesc', defaultMessage: 'Keep the messages you haven\'t sent yet on the server, to finish them on another device.'})}
                    on={syncDrafts === 'true'}
                    onChange={(on) => setSyncDrafts(String(on))}
                />
            )}
        </>
    );
}

function LanguagePane() {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const me = useSelector(getCurrentUser);
    const languages = useSelector(getLanguages);
    const timezone = useSelector(getCurrentTimezoneLabel);
    const sorted = Object.values(languages).sort((a, b) => a.order - b.order);

    return (
        <>
            <h2>{formatMessage({id: 'fusion.settings.language', defaultMessage: 'Language & time'})}</h2>
            <p className={am('lead')}>{formatMessage({id: 'fusion.settings.languageLead', defaultMessage: 'The language of the interface, and the time zone your times are shown in.'})}</p>
            <label className={am('field')}>
                <span>{formatMessage({id: 'fusion.settings.languageField', defaultMessage: 'Language'})}</span>
                <select
                    value={me.locale}
                    onChange={(e) => dispatch(updateMe({locale: e.target.value}))}
                >
                    {sorted.map((language) => (
                        <option
                            key={language.value}
                            value={language.value}
                        >
                            {language.name}
                        </option>
                    ))}
                </select>
            </label>
            <Action
                title={formatMessage({id: 'fusion.settings.timezone', defaultMessage: 'Time zone'})}
                desc={timezone}
                label={formatMessage({id: 'fusion.settings.change', defaultMessage: 'Change…'})}
                onClick={() => dispatch(openClassicUserSettings('display'))}
            />
        </>
    );
}

type Props = {
    tab: SettingsTab;
    onTab: (tab: SettingsTab) => void;
    onClose: () => void;
};

// SettingsModal is the mockup's settings dialog: user settings, and a link to the System Console for admins.
export default function SettingsModal({tab, onTab, onClose}: Props) {
    const {formatMessage} = useIntl();
    const admin = useSelector((state: GlobalState) => isSystemAdmin(getCurrentUser(state)?.roles || ''));

    const pane = {
        account: <AccountPane/>,
        security: <SecurityPane/>,
        notifications: <NotificationsPane/>,
        appearance: <AppearancePane/>,
        messages: <MessagesPane/>,
        language: <LanguagePane/>,
    }[tab];

    return (
        <Dialog
            label={formatMessage({id: 'fusion.settings.label', defaultMessage: 'Settings'})}
            small={false}
            onClose={onClose}
        >
            <nav>
                <h5>{formatMessage({id: 'fusion.settings.user', defaultMessage: 'User settings'})}</h5>
                {TABS.map(([key, label]) => (
                    <button
                        key={key}
                        type='button'
                        className={am({on: tab === key})}
                        onClick={() => onTab(key)}
                    >
                        {formatMessage(label)}
                    </button>
                ))}
                {admin && (
                    <>
                        <h5>{formatMessage({id: 'fusion.settings.console', defaultMessage: 'System Console'})}</h5>
                        <button
                            type='button'
                            onClick={() => {
                                onClose();
                                getHistory().push('/admin_console');
                            }}
                        >
                            {formatMessage({id: 'fusion.settings.openConsole', defaultMessage: 'Open the System Console'})}
                        </button>
                    </>
                )}
            </nav>
            <div className={am('pane')}>
                {pane}
                <button
                    type='button'
                    className={am('icon-btn', 'close')}
                    aria-label={formatMessage({id: 'fusion.settings.close', defaultMessage: 'Close settings'})}
                    onClick={onClose}
                >
                    <Icon name='x'/>
                </button>
            </div>
        </Dialog>
    );
}
