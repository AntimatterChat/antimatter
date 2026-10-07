// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useRef, useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import {CollapsedThreads} from '@mattermost/types/config';
import type {Session} from '@mattermost/types/sessions';
import type {UserNotifyProps, UserProfile} from '@mattermost/types/users';

import {savePreferences, saveTheme} from 'mattermost-redux/actions/preferences';
import {getMe, getSessions, revokeSession, setDefaultProfileImage, updateMe, updateUserPassword, uploadProfileImage} from 'mattermost-redux/actions/users';
import {Permissions, Preferences} from 'mattermost-redux/constants';
import {getConfig, getPasswordConfig} from 'mattermost-redux/selectors/entities/general';
import {get as getPreference, getCollapsedThreadsPreference, getTheme, isCollapsedThreadsAllowed, shouldShowUnreadsCategory, DEFAULT_VISIBLE_DM_GM_LIMIT} from 'mattermost-redux/selectors/entities/preferences';
import type {Theme} from 'mattermost-redux/selectors/entities/preferences';
import {haveISystemPermission} from 'mattermost-redux/selectors/entities/roles';
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
import Constants, {AcceptedProfileImageTypes, LOCK_PROFILE_FIELDS, normalizeLockProfileFieldsSetting} from 'utils/constants';
import {isValidPassword} from 'utils/password';
import {isValidUsername} from 'utils/utils';
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

type ProfileField = 'name' | 'username' | 'nickname' | 'position' | 'picture';

// useProfileFieldLock tells why the person can't change a field of their profile, as the classic settings do: their
// login provider (AD/LDAP, SAML, OAuth) provides it, or the admin locked it for email accounts. '' when they can.
function useProfileFieldLock(): (field: ProfileField) => 'provider' | 'admin' | '' {
    const me = useSelector(getCurrentUser);
    const config = useSelector(getConfig);
    const canEditOtherUsers = useSelector((state: GlobalState) => haveISystemPermission(state, {permission: Permissions.EDIT_OTHER_USERS}));
    const ldap = me.auth_service === Constants.LDAP_SERVICE;
    const saml = me.auth_service === Constants.SAML_SERVICE;
    const set = (name: string) => config[name as keyof typeof config] === 'true';

    return (field) => {
        const fromProvider = {
            name: (ldap && (set('LdapFirstNameAttributeSet') || set('LdapLastNameAttributeSet'))) || (saml && (set('SamlFirstNameAttributeSet') || set('SamlLastNameAttributeSet'))) || Constants.OAUTH_SERVICES.includes(me.auth_service),
            username: me.auth_service !== '',
            nickname: (ldap && set('LdapNicknameAttributeSet')) || (saml && set('SamlNicknameAttributeSet')),
            position: (ldap && set('LdapPositionAttributeSet')) || (saml && set('SamlPositionAttributeSet')),
            picture: ldap && set('LdapPictureAttributeSet'),
        }[field];
        if (fromProvider) {
            return 'provider';
        }
        if (me.auth_service === '' && !canEditOtherUsers) {
            const lock = normalizeLockProfileFieldsSetting(config.LockProfileFieldsForEmailUsers);
            if (lock === LOCK_PROFILE_FIELDS.ALL || (lock === LOCK_PROFILE_FIELDS.NAME_AND_USERNAME && (field === 'name' || field === 'username'))) {
                return 'admin';
            }
        }
        return '';
    };
}

function ProfilePicture({me, locked}: {me: UserProfile; locked: boolean}) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const maxFileSize = parseInt(useSelector(getConfig).MaxFileSize || '0', 10);
    const fileInput = useRef<HTMLInputElement>(null);
    const [error, setError] = useState('');
    const [busy, setBusy] = useState(false);

    const upload = async (file: File) => {
        if (!AcceptedProfileImageTypes.includes(file.type)) {
            setError(formatMessage({id: 'fusion.settings.pictureType', defaultMessage: 'Pick a JPEG, PNG or BMP image.'}));
            return;
        }
        if (maxFileSize && file.size > maxFileSize) {
            setError(formatMessage({id: 'fusion.settings.pictureSize', defaultMessage: 'This image is too large.'}));
            return;
        }
        setError('');
        setBusy(true);
        const {error: err} = await dispatch(uploadProfileImage(me.id, file)) as {error?: {message: string}};
        setBusy(false);
        if (err) {
            setError(err.message);
        }
    };

    return (
        <div className={am('pic-row')}>
            <Avatar
                userId={me.id}
                size='xl'
            />
            <div className={am('pic-who')}>
                <div className={am('pic-name')}>{[me.first_name, me.last_name].filter(Boolean).join(' ') || me.username}</div>
                <div className={am('pic-sub')}>{`@${me.username} · ${me.email}`}</div>
                {!locked && (
                    <div className={am('pic-actions')}>
                        <input
                            ref={fileInput}
                            type='file'
                            accept={AcceptedProfileImageTypes.join(',')}
                            style={{display: 'none'}}
                            onChange={(e) => {
                                const file = e.target.files?.[0];
                                e.target.value = '';
                                if (file) {
                                    upload(file);
                                }
                            }}
                        />
                        <button
                            type='button'
                            className={am('btn')}
                            disabled={busy}
                            onClick={() => fileInput.current?.click()}
                        >
                            {formatMessage({id: 'fusion.settings.changePicture', defaultMessage: 'Change picture'})}
                        </button>
                        {me.last_picture_update > 0 && (
                            <button
                                type='button'
                                className={am('btn')}
                                disabled={busy}
                                onClick={() => dispatch(setDefaultProfileImage(me.id))}
                            >
                                {formatMessage({id: 'fusion.settings.removePicture', defaultMessage: 'Remove'})}
                            </button>
                        )}
                    </div>
                )}
                {error && <div className={am('field-error')}>{error}</div>}
            </div>
        </div>
    );
}

function AccountPane() {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const me = useSelector(getCurrentUser);
    const lockOf = useProfileFieldLock();
    const initial = {first_name: me.first_name, last_name: me.last_name, nickname: me.nickname, position: me.position, username: me.username};
    const [fields, setFields] = useState(initial);
    const [saved, setSaved] = useState(false);
    const [error, setError] = useState('');
    const dirty = Object.entries(fields).some(([k, v]) => v !== me[k as keyof typeof fields]);
    const usernameError = fields.username === me.username ? undefined : isValidUsername(fields.username);

    const lockNote = (field: ProfileField) => {
        const lock = lockOf(field);
        if (lock === 'provider') {
            return formatMessage({id: 'fusion.settings.lockedByProvider', defaultMessage: 'Set by your login provider.'});
        }
        if (lock === 'admin') {
            return formatMessage({id: 'fusion.settings.lockedByAdmin', defaultMessage: 'Managed by your System Admin.'});
        }
        return undefined;
    };
    const field = (key: keyof typeof fields, profileField: ProfileField, label: string, help?: string) => {
        const note = lockNote(profileField);
        return (
            <label className={am('field')}>
                <span>{label}</span>
                <input
                    type='text'
                    value={fields[key]}
                    disabled={Boolean(note)}
                    maxLength={key === 'username' ? Constants.MAX_USERNAME_LENGTH : 64}
                    onChange={(e) => {
                        setSaved(false);
                        setError('');
                        setFields({...fields, [key]: key === 'username' ? e.target.value.toLowerCase() : e.target.value});
                    }}
                />
                {(note || help) && <small>{note || help}</small>}
            </label>
        );
    };

    return (
        <>
            <h2>{formatMessage({id: 'fusion.settings.account', defaultMessage: 'My account'})}</h2>
            <p className={am('lead')}>{formatMessage({id: 'fusion.settings.accountLead', defaultMessage: 'How you appear to others on this server.'})}</p>
            <ProfilePicture
                me={me}
                locked={Boolean(lockOf('picture'))}
            />
            <div className={am('field-row')}>
                {field('first_name', 'name', formatMessage({id: 'fusion.settings.firstName', defaultMessage: 'First name'}))}
                {field('last_name', 'name', formatMessage({id: 'fusion.settings.lastName', defaultMessage: 'Last name'}))}
            </div>
            {field('username', 'username', formatMessage({id: 'fusion.settings.username', defaultMessage: 'Username'}), usernameError ? formatMessage({id: 'fusion.settings.usernameRules', defaultMessage: 'Use {min} to {max} lowercase letters, numbers, periods, dashes and underscores, starting with a letter.'}, {min: Constants.MIN_USERNAME_LENGTH, max: Constants.MAX_USERNAME_LENGTH}) : undefined)}
            {field('nickname', 'nickname', formatMessage({id: 'fusion.settings.nickname', defaultMessage: 'Nickname'}))}
            {field('position', 'position', formatMessage({id: 'fusion.settings.position', defaultMessage: 'Position'}))}
            {error && <p className={am('field-error')}>{error}</p>}
            <div className={am('modal-actions')}>
                <button
                    type='button'
                    className={am('btn')}
                    onClick={() => dispatch(openClassicUserSettings('profile'))}
                >
                    {formatMessage({id: 'fusion.settings.changeEmail', defaultMessage: 'Change email…'})}
                </button>
                <button
                    type='button'
                    className={am('btn', 'primary')}
                    disabled={!dirty || Boolean(usernameError)}
                    onClick={async () => {
                        const result = await dispatch(updateMe(fields)) as {error?: {message: string}};
                        if (result.error) {
                            setError(result.error.message);
                        } else {
                            setSaved(true);
                        }
                    }}
                >
                    {saved ? formatMessage({id: 'fusion.settings.saved', defaultMessage: 'Saved'}) : formatMessage({id: 'fusion.settings.save', defaultMessage: 'Save'})}
                </button>
            </div>
        </>
    );
}

function PasswordForm() {
    const intl = useIntl();
    const {formatMessage} = intl;
    const dispatch = useDispatch();
    const me = useSelector(getCurrentUser);
    const passwordConfig = useSelector(getPasswordConfig);
    const [open, setOpen] = useState(false);
    const [current, setCurrent] = useState('');
    const [next, setNext] = useState('');
    const [confirm, setConfirm] = useState('');
    const [error, setError] = useState('');
    const [done, setDone] = useState(false);

    if (!open) {
        return (
            <Action
                title={formatMessage({id: 'fusion.settings.password', defaultMessage: 'Password'})}
                desc={done ? formatMessage({id: 'fusion.settings.passwordChanged', defaultMessage: 'Your password was changed.'}) : formatMessage({id: 'fusion.settings.passwordDesc', defaultMessage: 'Change the password you sign in with.'})}
                label={formatMessage({id: 'fusion.settings.change', defaultMessage: 'Change…'})}
                onClick={() => {
                    setDone(false);
                    setOpen(true);
                }}
            />
        );
    }

    const {valid, error: rules} = isValidPassword(next, passwordConfig, intl);
    const mismatch = confirm !== '' && confirm !== next;
    const input = (label: string, value: string, set: (v: string) => void, help?: string) => (
        <label className={am('field')}>
            <span>{label}</span>
            <input
                type='password'
                value={value}
                autoComplete={set === setCurrent ? 'current-password' : 'new-password'}
                onChange={(e) => {
                    setError('');
                    set(e.target.value);
                }}
            />
            {help && <small>{help}</small>}
        </label>
    );

    return (
        <div className={am('sub-form')}>
            <b>{formatMessage({id: 'fusion.settings.password', defaultMessage: 'Password'})}</b>
            {input(formatMessage({id: 'fusion.settings.currentPassword', defaultMessage: 'Current password'}), current, setCurrent)}
            {input(formatMessage({id: 'fusion.settings.newPassword', defaultMessage: 'New password'}), next, setNext, next && !valid ? String(rules) : undefined)}
            {input(formatMessage({id: 'fusion.settings.confirmPassword', defaultMessage: 'Confirm the new password'}), confirm, setConfirm, mismatch ? formatMessage({id: 'fusion.settings.passwordMismatch', defaultMessage: 'The passwords don\'t match.'}) : undefined)}
            {error && <p className={am('field-error')}>{error}</p>}
            <div className={am('modal-actions')}>
                <button
                    type='button'
                    className={am('btn')}
                    onClick={() => setOpen(false)}
                >
                    {formatMessage({id: 'fusion.settings.cancel', defaultMessage: 'Cancel'})}
                </button>
                <button
                    type='button'
                    className={am('btn', 'primary')}
                    disabled={!current || !valid || next !== confirm}
                    onClick={async () => {
                        const result = await dispatch(updateUserPassword(me.id, current, next)) as {error?: {message: string}};
                        if (result.error) {
                            setError(result.error.message);
                            return;
                        }
                        dispatch(getMe());
                        setCurrent('');
                        setNext('');
                        setConfirm('');
                        setOpen(false);
                        setDone(true);
                    }}
                >
                    {formatMessage({id: 'fusion.settings.changePassword', defaultMessage: 'Change password'})}
                </button>
            </div>
        </div>
    );
}

// Sessions lists where the person is signed in, as the classic activity log does, to sign out of any of them.
function Sessions() {
    const {formatMessage, formatDate} = useIntl();
    const dispatch = useDispatch();
    const me = useSelector(getCurrentUser);
    const sessions = useSelector((state: GlobalState) => state.entities.users.mySessions as Session[]);

    useEffect(() => {
        dispatch(getSessions(me.id));
    }, [dispatch, me.id]);

    const signedIn = (sessions || []).filter((session) => session.props?.type !== 'UserAccessToken');
    if (!signedIn.length) {
        return null;
    }
    const describe = (session: Session) => {
        if (session.device_id && (session.device_id.includes('apple') || session.device_id.includes('android'))) {
            return formatMessage({id: 'fusion.settings.sessionMobile', defaultMessage: 'Mobile app'});
        }
        return [session.props?.browser, session.props?.os].filter(Boolean).join(' · ') || formatMessage({id: 'fusion.settings.sessionUnknown', defaultMessage: 'Unknown device'});
    };

    return (
        <>
            <h3>{formatMessage({id: 'fusion.settings.sessions', defaultMessage: 'Where you\'re signed in'})}</h3>
            {signedIn.sort((a, b) => b.last_activity_at - a.last_activity_at).map((session) => (
                <Action
                    key={session.id}
                    title={describe(session)}
                    desc={formatMessage({id: 'fusion.settings.sessionActive', defaultMessage: 'Last active {when}'}, {when: formatDate(session.last_activity_at, {dateStyle: 'medium', timeStyle: 'short'})})}
                    label={formatMessage({id: 'fusion.settings.signOut', defaultMessage: 'Sign out'})}
                    onClick={async () => {
                        await dispatch(revokeSession(me.id, session.id));
                        dispatch(getSessions(me.id));
                    }}
                />
            ))}
        </>
    );
}

function SecurityPane() {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const config = useSelector(getConfig);
    const me = useSelector(getCurrentUser);
    return (
        <>
            <h2>{formatMessage({id: 'fusion.settings.security', defaultMessage: 'Security'})}</h2>
            <p className={am('lead')}>{formatMessage({id: 'fusion.settings.securityLead', defaultMessage: 'Your password, two-step sign-in and the devices you\'re signed in on.'})}</p>
            {me.auth_service === '' ? <PasswordForm/> : (
                <div className={am('note-box')}>{formatMessage({id: 'fusion.settings.signInProvider', defaultMessage: 'You sign in through {provider}: your password is managed there.'}, {provider: me.auth_service.toUpperCase() === 'LDAP' ? 'AD/LDAP' : me.auth_service.charAt(0).toUpperCase() + me.auth_service.slice(1)})}</div>
            )}
            {config.EnableMultifactorAuthentication === 'true' && (
                <Action
                    title={formatMessage({id: 'fusion.settings.mfa', defaultMessage: 'Multi-factor authentication'})}
                    desc={formatMessage({id: 'fusion.settings.mfaDesc', defaultMessage: 'Ask for a code from an authenticator app when you sign in.'})}
                    label={formatMessage({id: 'fusion.settings.manage', defaultMessage: 'Manage…'})}
                    onClick={() => dispatch(openClassicUserSettings('security'))}
                />
            )}
            {config.EnableUserDeactivation === 'true' && (
                <Action
                    title={formatMessage({id: 'fusion.settings.deactivate', defaultMessage: 'Deactivate account'})}
                    desc={formatMessage({id: 'fusion.settings.deactivateDesc', defaultMessage: 'Leave this server. An admin can reactivate your account later.'})}
                    label={formatMessage({id: 'fusion.settings.deactivateButton', defaultMessage: 'Deactivate…'})}
                    danger={true}
                    onClick={() => dispatch(openClassicUserSettings('advanced'))}
                />
            )}
            <Sessions/>
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
    const groupUnreadsByDefault = useSelector(shouldShowUnreadsCategory);
    const [groupUnreads, setGroupUnreads] = usePreference(Preferences.CATEGORY_SIDEBAR_SETTINGS, Preferences.SHOW_UNREAD_SECTION, String(groupUnreadsByDefault));
    const [dmLimit, setDmLimit] = usePreference(Preferences.CATEGORY_SIDEBAR_SETTINGS, Preferences.LIMIT_VISIBLE_DMS_GMS, String(DEFAULT_VISIBLE_DM_GM_LIMIT));
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
            <Toggle
                title={formatMessage({id: 'fusion.settings.groupUnreads', defaultMessage: 'Group unread channels'})}
                desc={formatMessage({id: 'fusion.settings.groupUnreadsDesc', defaultMessage: 'Gather the channels with unread messages in an Unreads section, above the categories.'})}
                on={groupUnreads === 'true'}
                onChange={(on) => setGroupUnreads(String(on))}
            />
            <Choice
                title={formatMessage({id: 'fusion.settings.dmLimit', defaultMessage: 'Open conversations'})}
                desc={formatMessage({id: 'fusion.settings.dmLimitDesc', defaultMessage: 'How many direct messages the sidebar lists. Unread ones always show.'})}
                value={dmLimit}
                options={['5', '10', '15', '20', '40'].map((n): [string, string] => [n, n])}
                onChange={setDmLimit}
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
