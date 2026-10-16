// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useState} from 'react';
import {useIntl} from 'react-intl';
import type {IntlShape} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import type {Audit} from '@mattermost/types/audits';
import type {OAuthApp} from '@mattermost/types/integrations';
import type {UserPropertyField} from '@mattermost/types/properties_user';
import type {UserAccessToken} from '@mattermost/types/users';

import {getCustomProfileAttributeValues, getMe, saveCustomProfileAttribute, updateMe} from 'mattermost-redux/actions/users';
import {Client4} from 'mattermost-redux/client';
import {getConfig, getCustomProfileAttributes} from 'mattermost-redux/selectors/entities/general';
import {getCurrentUser} from 'mattermost-redux/selectors/entities/users';
import {isEmail} from 'mattermost-redux/utils/helpers';
import {hasUserAccessTokenRole, isSystemAdmin} from 'mattermost-redux/utils/user_utils';

import {emitUserLoggedOutEvent} from 'actions/global_actions';

import {Dialog} from 'fusion/components/layer';
import {useToast} from 'fusion/shell/toast_context';
import {am} from 'fusion/utils/class_names';
import {getHistory} from 'utils/browser_history';
import Constants from 'utils/constants';
import {getUserPropertyFieldLabel} from 'utils/properties';
import {copyToClipboard} from 'utils/utils';

import type {GlobalState} from 'types/store';

import {Action, Section} from './settings_controls';

const errorText = (e: unknown) => (e && typeof e === 'object' && 'message' in e ? String((e as {message: unknown}).message) : String(e));

// MfaSection sets up multi-factor authentication with an authenticator app (a QR code, then a code from the app),
// or removes it unless the server requires it.
export function MfaSection() {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const toast = useToast();
    const me = useSelector(getCurrentUser);
    const config = useSelector(getConfig);
    const [setup, setSetup] = useState<{secret: string; qr: string} | null>(null);
    const [code, setCode] = useState('');
    const [error, setError] = useState('');

    if (config.EnableMultifactorAuthentication !== 'true' || me.auth_service !== '') {
        return null;
    }
    const enforced = config.EnforceMultifactorAuthentication === 'true';

    const start = async () => {
        setError('');
        try {
            const secret = await Client4.generateMfaSecret(me.id);
            setSetup({secret: secret.secret, qr: secret.qr_code});
        } catch (e) {
            setError(errorText(e));
        }
    };
    const activate = async () => {
        try {
            await Client4.updateUserMfa(me.id, true, code.trim());
            await dispatch(getMe());
            setSetup(null);
            setCode('');
            toast(formatMessage({id: 'fusion.settings.mfaOn', defaultMessage: 'Multi-factor authentication is on'}));
        } catch (e) {
            setError(errorText(e));
        }
    };
    const remove = async () => {
        try {
            await Client4.updateUserMfa(me.id, false, '');
            await dispatch(getMe());
            toast(formatMessage({id: 'fusion.settings.mfaOff', defaultMessage: 'Multi-factor authentication is off'}));
        } catch (e) {
            setError(errorText(e));
        }
    };

    if (setup) {
        return (
            <div className={am('sub-form')}>
                <b>{formatMessage({id: 'fusion.settings.mfaSetup', defaultMessage: 'Set up multi-factor authentication'})}</b>
                <p className={am('lead')}>{formatMessage({id: 'fusion.settings.mfaSetupLead', defaultMessage: 'Scan this code with an authenticator app (such as Aegis, Google Authenticator or 1Password), then enter the code it shows.'})}</p>
                <div className={am('mfa-qr')}>
                    <img
                        src={`data:image/png;base64,${setup.qr}`}
                        alt={formatMessage({id: 'fusion.settings.mfaQr', defaultMessage: 'QR code for the authenticator app'})}
                    />
                    <span>
                        {formatMessage({id: 'fusion.settings.mfaSecret', defaultMessage: 'Or type this key:'})}
                        <code>{setup.secret}</code>
                    </span>
                </div>
                <label className={am('field')}>
                    <span>{formatMessage({id: 'fusion.settings.mfaCode', defaultMessage: 'Code from the app'})}</span>
                    <input
                        type='text'
                        inputMode='numeric'
                        autoComplete='one-time-code'
                        value={code}
                        onChange={(e) => setCode(e.target.value)}
                    />
                </label>
                {error && <p className={am('field-error')}>{error}</p>}
                <div className={am('modal-actions')}>
                    <button
                        type='button'
                        className={am('btn')}
                        onClick={() => setSetup(null)}
                    >
                        {formatMessage({id: 'fusion.settings.cancel', defaultMessage: 'Cancel'})}
                    </button>
                    <button
                        type='button'
                        className={am('btn', 'primary')}
                        disabled={code.trim().length < 6}
                        onClick={activate}
                    >
                        {formatMessage({id: 'fusion.settings.mfaActivate', defaultMessage: 'Turn on'})}
                    </button>
                </div>
            </div>
        );
    }
    return (
        <>
            <Action
                title={formatMessage({id: 'fusion.settings.mfa', defaultMessage: 'Multi-factor authentication'})}
                desc={me.mfa_active ? formatMessage({id: 'fusion.settings.mfaActive', defaultMessage: 'On: signing in asks for a code from your authenticator app.'}) : formatMessage({id: 'fusion.settings.mfaDesc', defaultMessage: 'Ask for a code from an authenticator app when you sign in.'})}
                label={me.mfa_active ? formatMessage({id: 'fusion.settings.mfaReset', defaultMessage: 'Set up again…'}) : formatMessage({id: 'fusion.settings.mfaStart', defaultMessage: 'Set up…'})}
                onClick={start}
            />
            {me.mfa_active && !enforced && (
                <Action
                    title={formatMessage({id: 'fusion.settings.mfaRemove', defaultMessage: 'Remove multi-factor authentication'})}
                    desc={formatMessage({id: 'fusion.settings.mfaRemoveDesc', defaultMessage: 'Sign in with your password only.'})}
                    label={formatMessage({id: 'fusion.settings.remove', defaultMessage: 'Remove'})}
                    danger={true}
                    onClick={remove}
                />
            )}
            {error && <p className={am('field-error')}>{error}</p>}
        </>
    );
}

// TokensSection lists the personal access tokens, for scripts and integrations acting as you: create one (shown
// once), turn it off and on, or revoke it.
export function TokensSection() {
    const {formatMessage} = useIntl();
    const toast = useToast();
    const me = useSelector(getCurrentUser);
    const enabled = useSelector((state: GlobalState) => getConfig(state).EnableUserAccessTokens === 'true') && (hasUserAccessTokenRole(me.roles) || isSystemAdmin(me.roles));
    const [tokens, setTokens] = useState<UserAccessToken[] | null>(null);
    const [description, setDescription] = useState('');
    const [created, setCreated] = useState<UserAccessToken | null>(null);

    const load = async () => setTokens(await Client4.getUserAccessTokensForUser(me.id, 0, 200).catch(() => []));
    useEffect(() => {
        if (enabled) {
            load();
        }
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [enabled, me.id]);

    if (!enabled) {
        return null;
    }
    const create = async () => {
        try {
            const token = await Client4.createUserAccessToken(me.id, description.trim());
            setCreated(token);
            setDescription('');
            load();
        } catch (e) {
            toast(errorText(e));
        }
    };

    return (
        <>
            <Section title={formatMessage({id: 'fusion.settings.tokens', defaultMessage: 'Personal access tokens'})}/>
            {created && (
                <div className={am('note-box', 'token-created')}>
                    <span>
                        {formatMessage({id: 'fusion.settings.tokenCreated', defaultMessage: 'Copy this token now: it won\'t be shown again.'})}
                        <code>{created.token}</code>
                    </span>
                    <button
                        type='button'
                        className={am('btn')}
                        onClick={() => {
                            copyToClipboard(created.token || '');
                            toast(formatMessage({id: 'fusion.settings.tokenCopied', defaultMessage: 'Token copied'}));
                        }}
                    >
                        {formatMessage({id: 'fusion.settings.copy', defaultMessage: 'Copy'})}
                    </button>
                </div>
            )}
            {(tokens || []).map((token) => (
                <Action
                    key={token.id}
                    title={token.description || token.id}
                    desc={token.is_active ? formatMessage({id: 'fusion.settings.tokenActive', defaultMessage: 'Active · ID {id}'}, {id: token.id}) : formatMessage({id: 'fusion.settings.tokenOff', defaultMessage: 'Turned off · ID {id}'}, {id: token.id})}
                    label={formatMessage({id: 'fusion.settings.revoke', defaultMessage: 'Revoke'})}
                    danger={true}
                    onClick={async () => {
                        await Client4.revokeUserAccessToken(token.id).catch((e) => toast(errorText(e)));
                        load();
                    }}
                />
            ))}
            <div className={am('theme-code', 'token-new')}>
                <input
                    value={description}
                    placeholder={formatMessage({id: 'fusion.settings.tokenDescription', defaultMessage: 'What the new token is for'})}
                    aria-label={formatMessage({id: 'fusion.settings.tokenDescription', defaultMessage: 'What the new token is for'})}
                    onChange={(e) => setDescription(e.target.value)}
                />
                <button
                    type='button'
                    className={am('btn', 'primary')}
                    disabled={!description.trim()}
                    onClick={create}
                >
                    {formatMessage({id: 'fusion.settings.tokenCreate', defaultMessage: 'Create token'})}
                </button>
            </div>
        </>
    );
}

// OAuthAppsSection lists the OAuth 2.0 apps you allowed to act for you, to take that permission back.
export function OAuthAppsSection() {
    const {formatMessage} = useIntl();
    const toast = useToast();
    const me = useSelector(getCurrentUser);
    const enabled = useSelector((state: GlobalState) => getConfig(state).EnableOAuthServiceProvider === 'true');
    const [apps, setApps] = useState<OAuthApp[]>([]);

    const load = async () => setApps(await Client4.getAuthorizedOAuthApps(me.id).catch(() => []));
    useEffect(() => {
        if (enabled) {
            load();
        }
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [enabled, me.id]);

    if (!enabled || !apps.length) {
        return null;
    }
    return (
        <>
            <Section title={formatMessage({id: 'fusion.settings.oauthApps', defaultMessage: 'Apps you authorized'})}/>
            {apps.map((app) => (
                <Action
                    key={app.id}
                    title={app.name}
                    desc={app.description || app.homepage}
                    label={formatMessage({id: 'fusion.settings.deauthorize', defaultMessage: 'Remove access'})}
                    danger={true}
                    onClick={async () => {
                        await Client4.deauthorizeOAuthApp(app.id).catch((e) => toast(errorText(e)));
                        load();
                    }}
                />
            ))}
        </>
    );
}

// AccessHistory shows the account's recent sign-ins and security events, as the classic access history.
export function AccessHistory() {
    const {formatMessage, formatDate} = useIntl();
    const me = useSelector(getCurrentUser);
    const [audits, setAudits] = useState<Audit[] | null>(null);

    if (!audits) {
        return (
            <Action
                title={formatMessage({id: 'fusion.settings.accessHistory', defaultMessage: 'Access history'})}
                desc={formatMessage({id: 'fusion.settings.accessHistoryDesc', defaultMessage: 'Recent sign-ins and changes to your account, with where they came from.'})}
                label={formatMessage({id: 'fusion.settings.show', defaultMessage: 'Show'})}
                onClick={async () => setAudits(await Client4.getUserAudits(me.id, 0, 50).catch(() => []))}
            />
        );
    }
    return (
        <>
            <Section title={formatMessage({id: 'fusion.settings.accessHistory', defaultMessage: 'Access history'})}/>
            <div className={am('audit-list')}>
                {audits.map((audit) => (
                    <div
                        key={audit.id}
                        className={am('audit-row')}
                    >
                        <span>{formatDate(audit.create_at, {dateStyle: 'medium', timeStyle: 'short'})}</span>
                        <b>{audit.action.replace(/^\/api\/v4\//, '').replace(/\b[a-z0-9]{26}\b/g, '…')}</b>
                        <span>{audit.ip_address}</span>
                    </div>
                ))}
                {!audits.length && <p className={am('lead')}>{formatMessage({id: 'fusion.settings.noAccessHistory', defaultMessage: 'Nothing recorded yet.'})}</p>}
            </div>
        </>
    );
}

// DeactivateDialog confirms deactivating one's own account, then signs out.
export function DeactivateDialog({onClose}: {onClose: () => void}) {
    const {formatMessage} = useIntl();
    const me = useSelector(getCurrentUser);
    const [error, setError] = useState('');
    const title = formatMessage({id: 'fusion.settings.deactivateTitle', defaultMessage: 'Deactivate your account?'});
    return (
        <Dialog
            label={title}
            onClose={onClose}
            onSubmit={async () => {
                try {
                    await Client4.updateUserActive(me.id, false);
                    emitUserLoggedOutEvent();
                } catch (e) {
                    setError(errorText(e));
                }
            }}
        >
            <div className={am('pane')}>
                <h2>{title}</h2>
                <p className={am('lead')}>{formatMessage({id: 'fusion.settings.deactivateLead', defaultMessage: 'You\'ll be signed out and won\'t be able to sign in again. Your messages stay. An admin can reactivate your account.'})}</p>
                {error && <p className={am('field-error')}>{error}</p>}
                <div className={am('modal-actions')}>
                    <button
                        type='button'
                        className={am('btn')}
                        onClick={onClose}
                    >
                        {formatMessage({id: 'fusion.settings.cancel', defaultMessage: 'Cancel'})}
                    </button>
                    <button className={am('btn', 'danger')}>
                        {formatMessage({id: 'fusion.settings.deactivate', defaultMessage: 'Deactivate account'})}
                    </button>
                </div>
            </div>
        </Dialog>
    );
}

// EmailChange changes the account's email address, confirmed with the current password, as the classic settings do.
export function EmailChange({onClose}: {onClose: () => void}) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const toast = useToast();
    const me = useSelector(getCurrentUser);
    const verify = useSelector((state: GlobalState) => getConfig(state).RequireEmailVerification === 'true');
    const [email, setEmail] = useState('');
    const [password, setPassword] = useState('');
    const [error, setError] = useState('');
    const valid = isEmail(email.trim()) && email.trim().toLowerCase() !== me.email && password !== '';

    const save = async () => {
        const result = await dispatch(updateMe({email: email.trim().toLowerCase(), password})) as {error?: {message: string}};
        if (result.error) {
            setError(result.error.message);
            return;
        }
        toast(verify ? formatMessage({id: 'fusion.settings.emailVerify', defaultMessage: 'Check {email} to confirm the change'}, {email: email.trim()}) : formatMessage({id: 'fusion.settings.emailChanged', defaultMessage: 'Email changed'}));
        onClose();
    };

    return (
        <div className={am('sub-form')}>
            <b>{formatMessage({id: 'fusion.settings.changeEmailTitle', defaultMessage: 'Change your email'})}</b>
            {me.email && <p className={am('lead')}>{formatMessage({id: 'fusion.settings.currentEmail', defaultMessage: 'Now: {email}'}, {email: me.email})}</p>}
            <div className={am('field-row')}>
                <label className={am('field')}>
                    <span>{formatMessage({id: 'fusion.settings.newEmail', defaultMessage: 'New email'})}</span>
                    <input
                        type='email'
                        autoComplete='email'
                        value={email}
                        onChange={(e) => {
                            setError('');
                            setEmail(e.target.value);
                        }}
                    />
                </label>
                <label className={am('field')}>
                    <span>{formatMessage({id: 'fusion.settings.currentPassword', defaultMessage: 'Current password'})}</span>
                    <input
                        type='password'
                        autoComplete='current-password'
                        value={password}
                        onChange={(e) => {
                            setError('');
                            setPassword(e.target.value);
                        }}
                    />
                </label>
            </div>
            {verify && <small className={am('sub-hint')}>{formatMessage({id: 'fusion.settings.emailVerifyNote', defaultMessage: 'You\'ll get a message at the new address to confirm it.'})}</small>}
            {error && <p className={am('field-error')}>{error}</p>}
            <div className={am('modal-actions')}>
                <button
                    type='button'
                    className={am('btn')}
                    onClick={onClose}
                >
                    {formatMessage({id: 'fusion.settings.cancel', defaultMessage: 'Cancel'})}
                </button>
                <button
                    type='button'
                    className={am('btn', 'primary')}
                    disabled={!valid}
                    onClick={save}
                >
                    {formatMessage({id: 'fusion.settings.changeEmailButton', defaultMessage: 'Change email'})}
                </button>
            </div>
        </div>
    );
}

// customFieldLock tells why the person can't edit a custom profile field (the classic settings' rules), or '' when
// they can.
function customFieldLock(field: UserPropertyField, authService: string, formatMessage: IntlShape['formatMessage']): string {
    const attrs = field.attrs || {};
    if ((authService === Constants.LDAP_SERVICE && attrs.ldap) || (authService === Constants.SAML_SERVICE && attrs.saml)) {
        return formatMessage({id: 'fusion.settings.lockedByProvider', defaultMessage: 'Set by your login provider.'});
    }
    if (attrs.owners?.length || attrs.protected) {
        return formatMessage({id: 'fusion.settings.lockedByIntegration', defaultMessage: 'Managed by an integration.'});
    }
    if (attrs.managed === 'admin') {
        return formatMessage({id: 'fusion.settings.lockedByAdmin', defaultMessage: 'Managed by your System Admin.'});
    }
    if (attrs.options_omitted || (field.type !== 'text' && field.type !== 'select')) {
        return formatMessage({id: 'fusion.settings.lockedClassic', defaultMessage: 'Edit this one in the classic settings.'});
    }
    return '';
}

const INPUT_TYPES: Record<string, string> = {email: 'email', url: 'url', phone: 'tel'};

// CustomProfileFields are the extra profile fields the admins defined (department, location…): text fields and
// single choices are edited here, the others show their value.
export function CustomProfileFields() {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const me = useSelector(getCurrentUser);
    const fields = useSelector(getCustomProfileAttributes);
    const [values, setValues] = useState<Record<string, string | string[]>>({});
    const [error, setError] = useState('');
    const stored = me.custom_profile_attributes || {};

    useEffect(() => {
        if (!me.custom_profile_attributes) {
            dispatch(getCustomProfileAttributeValues(me.id));
        }
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [me.id]);

    const shown = fields.filter((field) => field.attrs?.access_mode !== 'source_only');
    if (!shown.length) {
        return null;
    }
    const dirty = Object.keys(values).some((id) => values[id] !== (stored[id] ?? ''));
    const save = async () => {
        setError('');
        const changed = Object.keys(values).filter((id) => values[id] !== (stored[id] ?? ''));
        const results = await Promise.all(changed.map((id) => dispatch(saveCustomProfileAttribute(me.id, id, values[id] as string)) as Promise<{error?: {message: string}}>));
        const failed = results.find((result) => result.error);
        if (failed) {
            setError(failed.error!.message);
            return;
        }
        setValues({});
    };

    return (
        <>
            <Section title={formatMessage({id: 'fusion.settings.profileFields', defaultMessage: 'Profile details'})}/>
            {shown.map((field) => {
                const label = getUserPropertyFieldLabel(field);
                const lock = customFieldLock(field, me.auth_service, formatMessage);
                const value = values[field.id] ?? stored[field.id] ?? '';
                const optionName = (id: string) => field.attrs?.options?.find((option) => option.id === id)?.name ?? id;
                if (lock) {
                    const text = Array.isArray(value) ? value.map(optionName).join(', ') : optionName(value);
                    return (
                        <label
                            key={field.id}
                            className={am('field')}
                        >
                            <span>{label}</span>
                            <input
                                type='text'
                                value={text}
                                disabled={true}
                            />
                            <small>{lock}</small>
                        </label>
                    );
                }
                if (field.type === 'select') {
                    return (
                        <label
                            key={field.id}
                            className={am('field')}
                        >
                            <span>{label}</span>
                            <select
                                className={am('select')}
                                value={value as string}
                                onChange={(e) => setValues({...values, [field.id]: e.target.value})}
                            >
                                <option value=''>{formatMessage({id: 'fusion.settings.fieldNone', defaultMessage: '—'})}</option>
                                {(field.attrs?.options || []).map((option) => (
                                    <option
                                        key={option.id}
                                        value={option.id}
                                    >
                                        {option.name}
                                    </option>
                                ))}
                            </select>
                        </label>
                    );
                }
                return (
                    <label
                        key={field.id}
                        className={am('field')}
                    >
                        <span>{label}</span>
                        <input
                            type={INPUT_TYPES[field.attrs?.value_type || ''] || 'text'}
                            value={value as string}
                            maxLength={64}
                            onChange={(e) => setValues({...values, [field.id]: e.target.value})}
                        />
                    </label>
                );
            })}
            {error && <p className={am('field-error')}>{error}</p>}
            {dirty && (
                <div className={am('modal-actions')}>
                    <button
                        type='button'
                        className={am('btn')}
                        onClick={() => setValues({})}
                    >
                        {formatMessage({id: 'fusion.settings.cancel', defaultMessage: 'Cancel'})}
                    </button>
                    <button
                        type='button'
                        className={am('btn', 'primary')}
                        onClick={save}
                    >
                        {formatMessage({id: 'fusion.settings.save', defaultMessage: 'Save'})}
                    </button>
                </div>
            )}
        </>
    );
}

// SignInMethod offers the classic switches between signing in with email and a password, and through a login
// provider (when the server allows moving accounts between them). Each one goes to the classic claim page, which
// asks for the password and hands over to the provider.
export function SignInMethod() {
    const {formatMessage} = useIntl();
    const me = useSelector(getCurrentUser);
    const config = useSelector(getConfig);
    const email = encodeURIComponent(me.email);
    const providers: Array<[boolean, string, string]> = [
        [config.EnableSignUpWithGitLab === 'true', Constants.GITLAB_SERVICE, 'GitLab'],
        [config.EnableSignUpWithGoogle === 'true', Constants.GOOGLE_SERVICE, 'Google'],
        [config.EnableSignUpWithOffice365 === 'true', Constants.OFFICE365_SERVICE, 'Entra ID'],
        [config.EnableSignUpWithOpenId === 'true', Constants.OPENID_SERVICE, 'OpenID'],
        [config.EnableSaml === 'true', Constants.SAML_SERVICE, 'SAML'],
        [config.EnableLdap === 'true', Constants.LDAP_SERVICE, 'AD/LDAP'],
    ];
    const enabled = providers.filter(([on]) => on);
    const toEmail = config.EnableSignUpWithEmail === 'true' && (config.EnableSignInWithEmail === 'true' || config.EnableSignInWithUsername === 'true');
    if (config.ExperimentalEnableAuthenticationTransfer !== 'true' || !enabled.length || (me.auth_service !== '' && (!toEmail || me.auth_service === Constants.MAGIC_LINK_SERVICE))) {
        return null;
    }

    const go = (path: string) => getHistory().push(path);
    if (me.auth_service !== '') {
        const path = me.auth_service === Constants.LDAP_SERVICE ? `/claim/ldap_to_email?email=${email}` : `/claim/oauth_to_email?email=${email}&old_type=${me.auth_service}`;
        return (
            <Action
                title={formatMessage({id: 'fusion.settings.signInMethod', defaultMessage: 'Sign-in method'})}
                desc={formatMessage({id: 'fusion.settings.signInSwitchEmail', defaultMessage: 'Sign in with your email and a password instead of your login provider.'})}
                label={formatMessage({id: 'fusion.settings.signInUseEmail', defaultMessage: 'Use email…'})}
                onClick={() => go(path)}
            />
        );
    }
    return (
        <div className={am('toggle-row')}>
            <div className={am('txt')}>
                <b>{formatMessage({id: 'fusion.settings.signInMethod', defaultMessage: 'Sign-in method'})}</b>
                <span>{formatMessage({id: 'fusion.settings.signInSwitchProvider', defaultMessage: 'Sign in through a login provider instead of your password.'})}</span>
            </div>
            <div className={am('btn-group')}>
                {enabled.map(([, service, name]) => (
                    <button
                        key={service}
                        type='button'
                        className={am('btn')}
                        onClick={() => go(service === Constants.LDAP_SERVICE ? `/claim/email_to_ldap?email=${email}` : `/claim/email_to_oauth?email=${email}&old_type=&new_type=${service}`)}
                    >
                        {name}
                    </button>
                ))}
            </div>
        </div>
    );
}
