// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import {getMissingProfilesByIds} from 'mattermost-redux/actions/users';

import Avatar from 'fusion/components/avatar';
import {useDisplayName, useUser} from 'fusion/hooks/users';
import {Choice, Section} from 'fusion/modals/settings_controls';
import {am} from 'fusion/utils/class_names';

import type {GlobalState} from 'types/store';

import {DM_PRIVACY_DEFAULT, getBlockLists, getDMPrivacy, isUserBlockingEnabled, setDMPrivacy, useDMPrivacyOptions} from './blocking';
import {useBlockActions} from './message_requests';
import ProfileFieldToggles from './profile_fields';
import {PROFILE_DEFAULT, getMyProfilePrivacy, hiddenIn, setHiddenFields} from './profile_privacy';

// PersonRow is someone you block or ignore, with the button that stops it.
function PersonRow({userId}: {userId: string}) {
    const {formatMessage} = useIntl();
    const user = useUser(userId);
    const name = useDisplayName(user);
    const {level, toggleIgnore, toggleBlock} = useBlockActions(userId);
    return (
        <div className={am('toggle-row', 'person-row')}>
            <Avatar
                userId={userId}
                size='sm'
            />
            <div className={am('txt')}>
                <b>{name || userId}</b>
                {user && <span>{'@' + user.username}</span>}
            </div>
            <button
                type='button'
                className={am('btn')}
                onClick={level === 'block' ? toggleBlock : toggleIgnore}
            >
                {level === 'block' ? formatMessage({id: 'fusion.privacy.unblock', defaultMessage: 'Unblock'}) : formatMessage({id: 'fusion.privacy.unignore', defaultMessage: 'Stop ignoring'})}
            </button>
        </div>
    );
}

// PrivacyPane is the "Privacy & safety" settings, as Discord's: your defaults for all communities (who can message you
// directly, which of your profile details people see), and the people you block or ignore. Each community's own
// choices are in its menu's Privacy settings. The admin can turn blocking and message requests off
// (EnableUserBlocking); the profile details stay.
export default function PrivacyPane() {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const all = useSelector((state: GlobalState) => getDMPrivacy(state)) || 'everyone';
    const profile = useSelector(getMyProfilePrivacy);
    const {blocked, ignored} = useSelector(getBlockLists);
    const blocking = useSelector(isUserBlockingEnabled);
    const options = useDMPrivacyOptions();

    // The people listed may be strangers to this session.
    useEffect(() => {
        if (blocked.length || ignored.length) {
            dispatch(getMissingProfilesByIds([...blocked, ...ignored]));
        }
    }, [dispatch, blocked, ignored]);

    return (
        <>
            <h2>{formatMessage({id: 'fusion.settings.privacy', defaultMessage: 'Privacy & safety'})}</h2>
            <p className={am('lead')}>
                {blocking ? formatMessage({id: 'fusion.privacy.leadDefaults', defaultMessage: 'Who can message you directly and what people see of you, in all your communities, and the people you ignore or block. A community can differ: its menu has its own Privacy settings. Nobody is told what you choose here.'}) : formatMessage({id: 'fusion.privacy.leadProfileOnly', defaultMessage: 'What people see of you in all your communities. A community can differ: its menu has its own Privacy settings. Nobody is told what you choose here.'})}
            </p>

            {blocking && (
                <>
                    <Section title={formatMessage({id: 'fusion.privacy.directMessages', defaultMessage: 'Direct messages'})}/>
                    <Choice
                        title={formatMessage({id: 'fusion.privacy.dmTitle', defaultMessage: 'From people you don\'t talk with yet'})}
                        desc={formatMessage({id: 'fusion.privacy.dmChoiceDesc', defaultMessage: 'Asked first, their messages wait among your message requests, without notifying you, until you accept them. Refused, they can\'t message you. Bots and system admins always get through.'})}
                        value={all}
                        options={options}
                        onChange={(value) => dispatch(setDMPrivacy(DM_PRIVACY_DEFAULT, value))}
                    />
                </>
            )}

            <Section title={formatMessage({id: 'fusion.profilePrivacy.section', defaultMessage: 'Your profile'})}/>
            <p className={am('lead')}>{formatMessage({id: 'fusion.profilePrivacy.lead', defaultMessage: 'The details people see on your profile. Someone who shares several communities with you sees only what all of them show. Your username and picture always show; system admins see everything.'})}</p>
            <ProfileFieldToggles
                hidden={hiddenIn(profile, PROFILE_DEFAULT)}
                onChange={(hidden) => dispatch(setHiddenFields(PROFILE_DEFAULT, hidden))}
            />

            {blocking && (
                <>
                    <Section title={formatMessage({id: 'fusion.privacy.blocked', defaultMessage: 'Blocked'})}/>
                    <p className={am('lead')}>{formatMessage({id: 'fusion.privacy.blockedLead', defaultMessage: 'They can\'t message you or start a group message with you, nothing they write notifies you, and their messages are folded.'})}</p>
                    {blocked.map((userId) => (
                        <PersonRow
                            key={userId}
                            userId={userId}
                        />
                    ))}
                    {!blocked.length && <div className={am('empty')}>{formatMessage({id: 'fusion.privacy.noneBlocked', defaultMessage: 'You haven\'t blocked anyone.'})}</div>}

                    <Section title={formatMessage({id: 'fusion.privacy.ignored', defaultMessage: 'Ignored'})}/>
                    <p className={am('lead')}>{formatMessage({id: 'fusion.privacy.ignoredLead', defaultMessage: 'Nothing they write notifies you, their direct messages wait among your message requests, and their messages are folded.'})}</p>
                    {ignored.map((userId) => (
                        <PersonRow
                            key={userId}
                            userId={userId}
                        />
                    ))}
                    {!ignored.length && <div className={am('empty')}>{formatMessage({id: 'fusion.privacy.noneIgnored', defaultMessage: 'You aren\'t ignoring anyone.'})}</div>}
                </>
            )}
        </>
    );
}
