// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect} from 'react';
import {useIntl} from 'react-intl';
import {shallowEqual, useDispatch, useSelector} from 'react-redux';

import {getMissingProfilesByIds} from 'mattermost-redux/actions/users';

import Avatar from 'fusion/components/avatar';
import {useDisplayName, useUser} from 'fusion/hooks/users';
import {Choice, Section, Select} from 'fusion/modals/settings_controls';
import {getSortedMyTeams} from 'fusion/selectors';
import {am} from 'fusion/utils/class_names';

import type {GlobalState} from 'types/store';

import type {DMPrivacy} from './blocking';
import {DM_PRIVACY_DEFAULT, getBlockLists, getDMPrivacy, setDMPrivacy} from './blocking';
import {useBlockActions} from './message_requests';

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

// PrivacyPane is the "Privacy & safety" settings, as Discord's: who can message you directly, in all communities and in
// each of them, and the people you block or ignore. The admin can turn it all off (EnableUserBlocking).
export default function PrivacyPane() {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const all = useSelector((state: GlobalState) => getDMPrivacy(state)) || 'everyone';
    const communities = useSelector(getSortedMyTeams);
    const choices = useSelector((state: GlobalState) => communities.map((team) => getDMPrivacy(state, team.id)), shallowEqual);
    const {blocked, ignored} = useSelector(getBlockLists);

    // The people listed may be strangers to this session.
    useEffect(() => {
        if (blocked.length || ignored.length) {
            dispatch(getMissingProfilesByIds([...blocked, ...ignored]));
        }
    }, [dispatch, blocked, ignored]);

    const everyone = formatMessage({id: 'fusion.privacy.everyone', defaultMessage: 'Accept from everyone'});
    const ask = formatMessage({id: 'fusion.privacy.ask', defaultMessage: 'Ask me first'});
    const options: Array<[DMPrivacy, string]> = [['everyone', everyone], ['ask', ask]];

    return (
        <>
            <h2>{formatMessage({id: 'fusion.settings.privacy', defaultMessage: 'Privacy & safety'})}</h2>
            <p className={am('lead')}>{formatMessage({id: 'fusion.privacy.lead', defaultMessage: 'Who can message you directly, and the people you ignore or block. Nobody is told what you choose here.'})}</p>

            <Section title={formatMessage({id: 'fusion.privacy.directMessages', defaultMessage: 'Direct messages'})}/>
            <Choice
                title={formatMessage({id: 'fusion.privacy.dmTitle', defaultMessage: 'From people you don\'t talk with yet'})}
                desc={formatMessage({id: 'fusion.privacy.dmDesc', defaultMessage: 'Asked first, their messages wait among your message requests, without notifying you, until you accept them. Bots and system admins always get through.'})}
                value={all}
                options={options}
                onChange={(value) => dispatch(setDMPrivacy(DM_PRIVACY_DEFAULT, value))}
            />
            {communities.length > 0 && (
                <>
                    <Section title={formatMessage({id: 'fusion.privacy.perCommunity', defaultMessage: 'In each community'})}/>
                    <p className={am('lead')}>{formatMessage({id: 'fusion.privacy.perCommunityLead', defaultMessage: 'Someone gets through when a community you share with them accepts them. With no community in common, the choice above decides.'})}</p>
                    {communities.map((team, i) => (
                        <Select<DMPrivacy | ''>
                            key={team.id}
                            title={team.display_name}
                            value={choices[i]}
                            options={[
                                ['', formatMessage({id: 'fusion.privacy.asAbove', defaultMessage: 'As above ({choice})'}, {choice: all === 'ask' ? ask : everyone})],
                                ...options,
                            ]}
                            onChange={(value) => dispatch(setDMPrivacy(team.id, value))}
                        />
                    ))}
                </>
            )}

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
    );
}
