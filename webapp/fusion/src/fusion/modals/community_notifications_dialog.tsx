// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import {Preferences} from 'mattermost-redux/constants';
import {getConfig} from 'mattermost-redux/selectors/entities/general';
import {getCollapsedThreadsPreference, isCollapsedThreadsAllowed} from 'mattermost-redux/selectors/entities/preferences';
import {getTeam} from 'mattermost-redux/selectors/entities/teams';
import {getCurrentUser} from 'mattermost-redux/selectors/entities/users';

import {Dialog} from 'fusion/components/layer';
import {am} from 'fusion/utils/class_names';
import {notificationSoundKeys, tryNotificationSound} from 'utils/notification_sounds';

import type {GlobalState} from 'types/store';

import type {CommunityNotifyKey} from './community_notify';
import {getCommunityNotifyProps, resetCommunityNotifyProps, setCommunityNotifyProp} from './community_notify';
import {Section, Select} from './settings_controls';

// CommunityNotificationsDialog is a community's notification settings, from its menu: each follows the person's own
// (the Notifications settings) until they choose otherwise for this community. A channel's own notification
// preferences still come first; direct and group messages, in no community, follow the person's own settings.
export default function CommunityNotificationsDialog({teamId, onClose}: {teamId: string; onClose: () => void}) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const me = useSelector(getCurrentUser);
    const team = useSelector((state: GlobalState) => getTeam(state, teamId));
    const config = useSelector(getConfig);
    const threads = useSelector((state: GlobalState) => isCollapsedThreadsAllowed(state) && getCollapsedThreadsPreference(state) === Preferences.COLLAPSED_REPLY_THREADS_ON);
    const overrides = useSelector((state: GlobalState) => getCommunityNotifyProps(state, teamId));
    const mine = me.notify_props;

    const levels: Record<string, string> = {
        all: formatMessage({id: 'fusion.settings.levelAll', defaultMessage: 'All'}),
        mention: formatMessage({id: 'fusion.settings.levelMentions', defaultMessage: 'Mentions'}),
        none: formatMessage({id: 'fusion.settings.levelNone', defaultMessage: 'Nothing'}),
    };
    const onOff: Record<string, string> = {
        true: formatMessage({id: 'fusion.communityNotifications.on', defaultMessage: 'On'}),
        false: formatMessage({id: 'fusion.communityNotifications.off', defaultMessage: 'Off'}),
    };
    const threadLevels: Record<string, string> = {
        all: formatMessage({id: 'fusion.communityNotifications.everyReply', defaultMessage: 'Every reply'}),
        mention: formatMessage({id: 'fusion.settings.levelMentions', defaultMessage: 'Mentions'}),
    };
    const sounds: Record<string, string> = Object.fromEntries(notificationSoundKeys.map((k) => [k, k]));

    // Yours: what the person's own settings say, as the first option.
    const own = (key: CommunityNotifyKey, labels: Record<string, string>, fallback: string): string => {
        const value = (mine as unknown as Record<string, string | undefined>)[key];
        return labels[value && labels[value] ? value : fallback];
    };

    const row = (key: CommunityNotifyKey, title: string, labels: Record<string, string>, fallback: string, onPick?: (value: string) => void) => (
        <Select<string>
            key={key}
            title={title}
            value={overrides[key] || ''}
            options={[
                ['', formatMessage({id: 'fusion.communityNotifications.asMine', defaultMessage: 'As in my settings ({choice})'}, {choice: own(key, labels, fallback)})],
                ...Object.entries(labels),
            ]}
            onChange={(value) => {
                onPick?.(value);
                dispatch(setCommunityNotifyProp(teamId, key, value));
            }}
        />
    );

    const soundOn = (overrides.desktop_sound || mine.desktop_sound) !== 'false';
    const name = team?.display_name || '';

    return (
        <Dialog
            label={formatMessage({id: 'fusion.communityNotifications.title', defaultMessage: 'Notifications in {community}'}, {community: name})}
            onClose={onClose}
        >
            <div className={am('pane')}>
                <h2>{formatMessage({id: 'fusion.communityNotifications.title', defaultMessage: 'Notifications in {community}'}, {community: name})}</h2>
                <p className={am('lead')}>{formatMessage({id: 'fusion.communityNotifications.lead', defaultMessage: 'Its channels notify you as chosen here, and as in your Notifications settings for the rest. A channel\'s own notification preferences still come first; direct and group messages follow your settings.'})}</p>

                <Section title={formatMessage({id: 'fusion.settings.desktop', defaultMessage: 'Desktop and browser'})}/>
                {row('desktop', formatMessage({id: 'fusion.settings.desktopLevel', defaultMessage: 'Notify me about'}), levels, 'mention')}
                {threads && row('desktop_threads', formatMessage({id: 'fusion.settings.threadReplies', defaultMessage: 'Replies to threads you follow'}), threadLevels, 'mention')}
                {row('desktop_sound', formatMessage({id: 'fusion.settings.desktopSound', defaultMessage: 'Notification sound'}), onOff, 'true')}
                {soundOn && row('desktop_notification_sound', formatMessage({id: 'fusion.settings.messageSound', defaultMessage: 'Message sound'}), sounds, 'Bing', (value) => {
                    if (value) {
                        tryNotificationSound(value);
                    }
                })}

                {config.SendPushNotifications === 'true' && (
                    <>
                        <Section title={formatMessage({id: 'fusion.settings.mobile', defaultMessage: 'Mobile'})}/>
                        {row('push', formatMessage({id: 'fusion.settings.pushLevel', defaultMessage: 'Push notifications about'}), levels, 'mention')}
                        {threads && row('push_threads', formatMessage({id: 'fusion.settings.pushThreads', defaultMessage: 'Replies to threads you follow'}), threadLevels, 'mention')}
                    </>
                )}

                {config.SendEmailNotifications === 'true' && (
                    <>
                        <Section title={formatMessage({id: 'fusion.settings.emailSection', defaultMessage: 'Email'})}/>
                        {row('email', formatMessage({id: 'fusion.settings.email', defaultMessage: 'Email notifications'}), onOff, 'false')}
                    </>
                )}

                <Section title={formatMessage({id: 'fusion.settings.mentions', defaultMessage: 'What counts as a mention'})}/>
                {row('channel', formatMessage({id: 'fusion.settings.channelWide', defaultMessage: 'Channel-wide mentions'}), onOff, 'false')}

                <div className={am('modal-actions')}>
                    {Object.keys(overrides).length > 0 && (
                        <button
                            type='button'
                            className={am('btn')}
                            onClick={() => dispatch(resetCommunityNotifyProps(teamId))}
                        >
                            {formatMessage({id: 'fusion.communityNotifications.reset', defaultMessage: 'Use my settings everywhere'})}
                        </button>
                    )}
                    <button
                        type='button'
                        className={am('btn', 'primary')}
                        onClick={onClose}
                    >
                        {formatMessage({id: 'fusion.communityNotifications.done', defaultMessage: 'Done'})}
                    </button>
                </div>
            </div>
        </Dialog>
    );
}
