// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import type {UserNotifyProps} from '@mattermost/types/users';

import {updateMe} from 'mattermost-redux/actions/users';
import {Client4} from 'mattermost-redux/client';
import {Preferences} from 'mattermost-redux/constants';
import {getConfig} from 'mattermost-redux/selectors/entities/general';
import {getCollapsedThreadsPreference, isCollapsedThreadsAllowed} from 'mattermost-redux/selectors/entities/preferences';
import {getCurrentUser} from 'mattermost-redux/selectors/entities/users';

import Icon from 'fusion/components/icon';
import {useToast} from 'fusion/shell/toast_context';
import {am} from 'fusion/utils/class_names';
import Constants from 'utils/constants';
import {callNotificationSoundKeys, notificationSoundKeys, stopTryNotificationRing, tryNotificationRing, tryNotificationSound} from 'utils/notification_sounds';

import type {GlobalState} from 'types/store';

import {Choice, Section, Select, Toggle, usePreference} from './settings_controls';

type Level = 'all' | 'mention' | 'none';

// KeywordsField edits a comma-separated list of words as chips: Enter or a comma adds the word being typed.
export function KeywordsField({label, desc, value, onChange}: {label: string; desc: string; value: string; onChange: (value: string) => void}) {
    const {formatMessage} = useIntl();
    const [draft, setDraft] = useState('');
    const words = value.split(',').map((w) => w.trim()).filter(Boolean);
    const add = () => {
        const word = draft.trim().replace(/,/g, '');
        setDraft('');
        if (word && !words.includes(word)) {
            onChange([...words, word].join(','));
        }
    };
    return (
        <div className={am('field', 'keywords')}>
            <span>{label}</span>
            <div className={am('chips-input')}>
                {words.map((word) => (
                    <span
                        key={word}
                        className={am('chip')}
                    >
                        {word}
                        <button
                            type='button'
                            aria-label={formatMessage({id: 'fusion.settings.removeKeyword', defaultMessage: 'Remove {word}'}, {word})}
                            onClick={() => onChange(words.filter((w) => w !== word).join(','))}
                        >
                            <Icon
                                name='x'
                                size='xs'
                            />
                        </button>
                    </span>
                ))}
                <input
                    value={draft}
                    aria-label={label}
                    placeholder={words.length ? '' : formatMessage({id: 'fusion.settings.keywordsPlaceholder', defaultMessage: 'Type a word, then Enter'})}
                    onChange={(e) => setDraft(e.target.value)}
                    onKeyDown={(e) => {
                        if (e.key === 'Enter' || e.key === ',') {
                            e.preventDefault();
                            add();
                        } else if (e.key === 'Backspace' && !draft && words.length) {
                            onChange(words.slice(0, -1).join(','));
                        }
                    }}
                    onBlur={add}
                />
            </div>
            <small>{desc}</small>
        </div>
    );
}

// NotificationsPane is every notification setting of the classic settings: desktop, mobile and email notifications,
// sounds, what counts as a mention (keywords included), thread replies, the automatic reply, and a test notification.
export default function NotificationsPane() {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const toast = useToast();
    const me = useSelector(getCurrentUser);
    const config = useSelector(getConfig);
    const threads = useSelector((state: GlobalState) => isCollapsedThreadsAllowed(state) && getCollapsedThreadsPreference(state) === Preferences.COLLAPSED_REPLY_THREADS_ON);
    const [emailInterval, setEmailInterval] = usePreference(Preferences.CATEGORY_NOTIFICATIONS, Preferences.EMAIL_INTERVAL, String(Preferences.INTERVAL_IMMEDIATE));
    const [autoReply, setAutoReply] = useState(me.notify_props.auto_responder_message || '');
    const [testing, setTesting] = useState(false);

    const notify = me.notify_props;
    const set = (patch: Partial<UserNotifyProps>) => dispatch(updateMe({notify_props: {...notify, ...patch}}));
    const level = (value: string | undefined, fallback: Level): Level => (value === 'all' || value === 'mention' || value === 'none' ? value : fallback);
    const levels: Array<[Level, string]> = [
        ['all', formatMessage({id: 'fusion.settings.levelAll', defaultMessage: 'All'})],
        ['mention', formatMessage({id: 'fusion.settings.levelMentions', defaultMessage: 'Mentions'})],
        ['none', formatMessage({id: 'fusion.settings.levelNone', defaultMessage: 'Nothing'})],
    ];
    const pushEnabled = config.SendPushNotifications === 'true';
    const emailEnabled = config.SendEmailNotifications === 'true';
    const firstNameTitle = me.first_name ? formatMessage({id: 'fusion.settings.firstNameIs', defaultMessage: 'My first name, “{name}”'}, {name: me.first_name}) : formatMessage({id: 'fusion.settings.firstName', defaultMessage: 'First name'});

    const sendTest = async () => {
        setTesting(true);
        try {
            await Client4.sendTestNotificaiton();
            toast(formatMessage({id: 'fusion.settings.testSent', defaultMessage: 'Test notification sent: check your desktop and devices'}));
        } catch {
            toast(formatMessage({id: 'fusion.settings.testFailed', defaultMessage: 'The test notification couldn\'t be sent'}));
        }
        setTesting(false);
    };

    return (
        <>
            <h2>{formatMessage({id: 'fusion.settings.notifications', defaultMessage: 'Notifications'})}</h2>
            <p className={am('lead')}>{formatMessage({id: 'fusion.settings.notificationsLead', defaultMessage: 'Choose what reaches you on desktop, mobile and by email.'})}</p>

            <Section title={formatMessage({id: 'fusion.settings.desktop', defaultMessage: 'Desktop and browser'})}/>
            <Choice
                title={formatMessage({id: 'fusion.settings.desktopLevel', defaultMessage: 'Notify me about'})}
                desc={formatMessage({id: 'fusion.settings.desktopLevelDesc', defaultMessage: 'Direct and group messages count as mentions.'})}
                value={level(notify.desktop, 'mention')}
                options={levels}
                onChange={(v) => set({desktop: v})}
            />
            {threads && (
                <Toggle
                    title={formatMessage({id: 'fusion.settings.threadReplies', defaultMessage: 'Replies to threads you follow'})}
                    desc={formatMessage({id: 'fusion.settings.threadRepliesDesc', defaultMessage: 'Notify me of every reply in the threads I follow, not only mentions.'})}
                    on={notify.desktop_threads === 'all'}
                    onChange={(on) => set({desktop_threads: on ? 'all' : 'mention'})}
                />
            )}
            <Toggle
                title={formatMessage({id: 'fusion.settings.desktopSound', defaultMessage: 'Notification sound'})}
                desc={formatMessage({id: 'fusion.settings.desktopSoundDesc', defaultMessage: 'Play a sound with desktop notifications.'})}
                on={notify.desktop_sound !== 'false'}
                onChange={(on) => set({desktop_sound: on ? 'true' : 'false'})}
            />
            {notify.desktop_sound !== 'false' && (
                <Select
                    title={formatMessage({id: 'fusion.settings.messageSound', defaultMessage: 'Message sound'})}
                    value={notify.desktop_notification_sound && notify.desktop_notification_sound !== 'default' ? notify.desktop_notification_sound : 'Bing'}
                    options={notificationSoundKeys.map((k): [string, string] => [k, k])}
                    onChange={(v) => {
                        tryNotificationSound(v);
                        set({desktop_notification_sound: v as UserNotifyProps['desktop_notification_sound']});
                    }}
                    after={(
                        <button
                            type='button'
                            className={am('icon-btn')}
                            title={formatMessage({id: 'fusion.settings.playSound', defaultMessage: 'Play'})}
                            aria-label={formatMessage({id: 'fusion.settings.playSound', defaultMessage: 'Play'})}
                            onClick={() => tryNotificationSound(notify.desktop_notification_sound || 'Bing')}
                        >
                            <Icon
                                name='bell'
                                size='sm'
                            />
                        </button>
                    )}
                />
            )}
            <Toggle
                title={formatMessage({id: 'fusion.settings.callSound', defaultMessage: 'Incoming call ring'})}
                desc={formatMessage({id: 'fusion.settings.callSoundDesc', defaultMessage: 'Ring when someone calls you in a direct message.'})}
                on={notify.calls_desktop_sound !== 'false'}
                onChange={(on) => set({calls_desktop_sound: on ? 'true' : 'false'})}
            />
            {notify.calls_desktop_sound !== 'false' && (
                <Select
                    title={formatMessage({id: 'fusion.settings.ringSound', defaultMessage: 'Ring'})}
                    value={notify.calls_notification_sound || 'Calm'}
                    options={callNotificationSoundKeys.map((k): [string, string] => [k, k])}
                    onChange={(v) => {
                        stopTryNotificationRing();
                        tryNotificationRing(v);
                        set({calls_notification_sound: v as UserNotifyProps['calls_notification_sound']});
                    }}
                />
            )}

            {pushEnabled && (
                <>
                    <Section title={formatMessage({id: 'fusion.settings.mobile', defaultMessage: 'Mobile'})}/>
                    <Choice
                        title={formatMessage({id: 'fusion.settings.pushLevel', defaultMessage: 'Push notifications about'})}
                        value={level(notify.push, 'mention')}
                        options={levels}
                        onChange={(v) => set({push: v})}
                    />
                    {notify.push !== 'none' && (
                        <Choice
                            title={formatMessage({id: 'fusion.settings.pushStatus', defaultMessage: 'Send them when I\'m'})}
                            desc={formatMessage({id: 'fusion.settings.pushStatusDesc', defaultMessage: 'Your status on the desktop or web app.'})}
                            value={notify.push_status === 'online' || notify.push_status === 'away' ? notify.push_status : 'offline'}
                            options={[
                                ['online', formatMessage({id: 'fusion.settings.pushOnline', defaultMessage: 'Online, away or offline'})],
                                ['away', formatMessage({id: 'fusion.settings.pushAway', defaultMessage: 'Away or offline'})],
                                ['offline', formatMessage({id: 'fusion.settings.pushOffline', defaultMessage: 'Offline'})],
                            ]}
                            onChange={(v) => set({push_status: v as UserNotifyProps['push_status']})}
                        />
                    )}
                    {threads && notify.push !== 'none' && (
                        <Toggle
                            title={formatMessage({id: 'fusion.settings.pushThreads', defaultMessage: 'Replies to threads you follow'})}
                            desc={formatMessage({id: 'fusion.settings.pushThreadsDesc', defaultMessage: 'On mobile too, for every reply.'})}
                            on={notify.push_threads === 'all'}
                            onChange={(on) => set({push_threads: on ? 'all' : 'mention'})}
                        />
                    )}
                </>
            )}

            {emailEnabled && (
                <>
                    <Section title={formatMessage({id: 'fusion.settings.emailSection', defaultMessage: 'Email'})}/>
                    <Toggle
                        title={formatMessage({id: 'fusion.settings.email', defaultMessage: 'Email notifications'})}
                        desc={formatMessage({id: 'fusion.settings.emailDesc', defaultMessage: 'For mentions and direct messages, when you are away or offline.'})}
                        on={notify.email === 'true'}
                        onChange={(on) => set({email: on ? 'true' : 'false'})}
                    />
                    {notify.email === 'true' && config.EnableEmailBatching === 'true' && (
                        <Choice
                            title={formatMessage({id: 'fusion.settings.emailInterval', defaultMessage: 'Send them'})}
                            value={emailInterval}
                            options={[
                                [String(Preferences.INTERVAL_IMMEDIATE), formatMessage({id: 'fusion.settings.emailNow', defaultMessage: 'Right away'})],
                                [String(Preferences.INTERVAL_FIFTEEN_MINUTES), formatMessage({id: 'fusion.settings.email15', defaultMessage: 'Every 15 minutes'})],
                                [String(Preferences.INTERVAL_HOUR), formatMessage({id: 'fusion.settings.emailHour', defaultMessage: 'Every hour'})],
                            ]}
                            onChange={setEmailInterval}
                        />
                    )}
                </>
            )}

            <Section title={formatMessage({id: 'fusion.settings.mentions', defaultMessage: 'What counts as a mention'})}/>
            <Toggle
                title={firstNameTitle}
                desc={formatMessage({id: 'fusion.settings.firstNameDesc', defaultMessage: 'Your username always counts as a mention.'})}
                on={notify.first_name === 'true'}
                onChange={(on) => set({first_name: on ? 'true' : 'false'})}
            />
            <Toggle
                title={formatMessage({id: 'fusion.settings.channelWide', defaultMessage: 'Channel-wide mentions'})}
                desc={formatMessage({id: 'fusion.settings.channelWideDesc', defaultMessage: 'Notify me for @channel, @all and @here.'})}
                on={notify.channel === 'true'}
                onChange={(on) => set({channel: on ? 'true' : 'false'})}
            />
            <KeywordsField
                label={formatMessage({id: 'fusion.settings.keywords', defaultMessage: 'Keywords that notify me'})}
                desc={formatMessage({id: 'fusion.settings.keywordsDesc', defaultMessage: 'Not case-sensitive. A message with one of these notifies you like a mention.'})}
                value={notify.mention_keys || ''}
                onChange={(v) => set({mention_keys: v})}
            />
            <KeywordsField
                label={formatMessage({id: 'fusion.settings.highlightKeys', defaultMessage: 'Keywords that only highlight'})}
                desc={formatMessage({id: 'fusion.settings.highlightKeysDesc', defaultMessage: 'Highlighted in messages, without notifying you.'})}
                value={notify.highlight_keys || ''}
                onChange={(v) => set({highlight_keys: v})}
            />
            {threads && (
                <Toggle
                    title={formatMessage({id: 'fusion.settings.autoFollow', defaultMessage: 'Follow threads I\'m mentioned in by @channel'})}
                    desc={formatMessage({id: 'fusion.settings.autoFollowDesc', defaultMessage: 'Channel-wide mentions in a thread make you follow it.'})}
                    on={notify.channel_mention_auto_follow_threads === 'true'}
                    onChange={(on) => set({channel_mention_auto_follow_threads: on ? 'true' : 'false'})}
                />
            )}

            {config.ExperimentalEnableAutomaticReplies === 'true' && (
                <>
                    <Section title={formatMessage({id: 'fusion.settings.autoReply', defaultMessage: 'Automatic reply'})}/>
                    <Toggle
                        title={formatMessage({id: 'fusion.settings.autoReplyOn', defaultMessage: 'Reply to direct messages automatically'})}
                        desc={formatMessage({id: 'fusion.settings.autoReplyDesc', defaultMessage: 'Sets you out of office, and answers each direct message with the text below.'})}
                        on={notify.auto_responder_active === 'true'}
                        onChange={(on) => set({auto_responder_active: on ? 'true' : 'false', auto_responder_message: autoReply})}
                    />
                    <label className={am('field')}>
                        <span>{formatMessage({id: 'fusion.settings.autoReplyMessage', defaultMessage: 'Message'})}</span>
                        <textarea
                            value={autoReply}
                            maxLength={Constants.MAX_POST_LEN}
                            placeholder={formatMessage({id: 'fusion.settings.autoReplyPlaceholder', defaultMessage: 'Hello, I\'m out of office and unable to respond to messages.'})}
                            onChange={(e) => setAutoReply(e.target.value)}
                            onBlur={() => autoReply !== (notify.auto_responder_message || '') && set({auto_responder_message: autoReply})}
                        />
                    </label>
                </>
            )}

            <div className={am('modal-actions')}>
                <button
                    type='button'
                    className={am('btn')}
                    disabled={testing}
                    onClick={sendTest}
                >
                    {formatMessage({id: 'fusion.settings.testNotification', defaultMessage: 'Send a test notification'})}
                </button>
            </div>
        </>
    );
}
