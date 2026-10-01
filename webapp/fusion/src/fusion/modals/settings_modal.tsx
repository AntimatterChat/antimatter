// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import type {UserNotifyProps} from '@mattermost/types/users';

import {savePreferences, saveTheme} from 'mattermost-redux/actions/preferences';
import {updateMe} from 'mattermost-redux/actions/users';
import {Preferences} from 'mattermost-redux/constants';
import {getConfig} from 'mattermost-redux/selectors/entities/general';
import {getBool, getTheme} from 'mattermost-redux/selectors/entities/preferences';
import type {Theme} from 'mattermost-redux/selectors/entities/preferences';
import {getCurrentUser} from 'mattermost-redux/selectors/entities/users';
import {isSystemAdmin} from 'mattermost-redux/utils/user_utils';

import {switchWebUI} from 'actions/web_ui';

import Avatar from 'fusion/components/avatar';
import Icon from 'fusion/components/icon';
import {Dialog} from 'fusion/components/layer';
import {am} from 'fusion/utils/class_names';
import {openClassicUserSettings} from 'fusion/utils/modals';
import {getHistory} from 'utils/browser_history';
import {CURRENT_WEB_UI, WebUIs} from 'utils/web_ui';

import type {GlobalState} from 'types/store';

import CustomThemeEditor, {useStartCustomTheme} from './custom_theme_editor';

export type SettingsTab = 'account' | 'notifications' | 'appearance' | 'interface';

const TABS: Array<[SettingsTab, {id: string; defaultMessage: string}]> = [
    ['account', {id: 'fusion.settings.account', defaultMessage: 'My account'}],
    ['notifications', {id: 'fusion.settings.notifications', defaultMessage: 'Notifications'}],
    ['appearance', {id: 'fusion.settings.appearance', defaultMessage: 'Appearance'}],
    ['interface', {id: 'fusion.settings.interface', defaultMessage: 'Web interface'}],
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
                    {formatMessage({id: 'fusion.settings.moreAccount', defaultMessage: 'Picture, email, password and security…'})}
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

function NotificationsPane() {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const me = useSelector(getCurrentUser);
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
                    {formatMessage({id: 'fusion.settings.moreNotifications', defaultMessage: 'Keywords, sounds and more…'})}
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
    const me = useSelector(getCurrentUser);
    const theme = useSelector(getTheme);
    const militaryTime = useSelector((state: GlobalState) => getBool(state, Preferences.CATEGORY_DISPLAY_SETTINGS, Preferences.USE_MILITARY_TIME, false));
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
            <Toggle
                title={formatMessage({id: 'fusion.settings.clock', defaultMessage: '24-hour clock'})}
                desc={formatMessage({id: 'fusion.settings.clockDesc', defaultMessage: 'Show times as 16:00 instead of 4:00 PM.'})}
                on={militaryTime}
                onChange={(on) => dispatch(savePreferences(me.id, [{user_id: me.id, category: Preferences.CATEGORY_DISPLAY_SETTINGS, name: Preferences.USE_MILITARY_TIME, value: String(on)}]))}
            />
            <div className={am('modal-actions')}>
                <button
                    type='button'
                    className={am('btn')}
                    onClick={() => dispatch(openClassicUserSettings('display'))}
                >
                    {formatMessage({id: 'fusion.settings.moreDisplay', defaultMessage: 'Language, time zone and more display settings…'})}
                </button>
            </div>
        </>
    );
}

function InterfacePane() {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const allowed = useSelector(getConfig).AllowUserWebUISelection === 'true';
    return (
        <>
            <h2>{formatMessage({id: 'fusion.settings.interface', defaultMessage: 'Web interface'})}</h2>
            <p className={am('lead')}>{formatMessage({id: 'fusion.settings.interfaceLead', defaultMessage: 'Antimatter has two web interfaces. Your choice applies to every browser you sign in from.'})}</p>
            {allowed ? (
                <div
                    className={am('seg')}
                    role='group'
                >
                    {[WebUIs.CLASSIC, WebUIs.FUSION].map((ui) => (
                        <button
                            key={ui}
                            type='button'
                            className={am({on: ui === CURRENT_WEB_UI})}
                            onClick={() => ui !== CURRENT_WEB_UI && dispatch(switchWebUI(ui))}
                        >
                            {ui === WebUIs.CLASSIC ? formatMessage({id: 'fusion.settings.classic', defaultMessage: 'Classic'}) : formatMessage({id: 'fusion.settings.fusion', defaultMessage: 'Fusion'})}
                        </button>
                    ))}
                </div>
            ) : (
                <div className={am('note-box')}>{formatMessage({id: 'fusion.settings.interfaceLocked', defaultMessage: 'Your system admin chose the web interface for everyone.'})}</div>
            )}
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
        notifications: <NotificationsPane/>,
        appearance: <AppearancePane/>,
        interface: <InterfacePane/>,
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
