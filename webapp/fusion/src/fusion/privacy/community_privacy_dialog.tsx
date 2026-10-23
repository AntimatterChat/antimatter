// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import {getTeam} from 'mattermost-redux/selectors/entities/teams';

import {Dialog} from 'fusion/components/layer';
import {Section, Select} from 'fusion/modals/settings_controls';
import {am} from 'fusion/utils/class_names';

import type {GlobalState} from 'types/store';

import type {DMPrivacy} from './blocking';
import {getDMPrivacy, isUserBlockingEnabled, setDMPrivacy, useDMPrivacyOptions} from './blocking';
import ProfileFieldToggles from './profile_fields';
import {getMyProfilePrivacy, hiddenIn, setHiddenFields} from './profile_privacy';

// CommunityPrivacyDialog is a community's privacy settings, from its menu: how you take direct messages from its
// people, and which of your profile details they see. Each follows your defaults (Settings › Privacy & safety) until
// you choose otherwise here. Someone who's also in another of your communities gets the most private of them.
export default function CommunityPrivacyDialog({teamId, onClose}: {teamId: string; onClose: () => void}) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const team = useSelector((state: GlobalState) => getTeam(state, teamId));
    const blocking = useSelector(isUserBlockingEnabled);
    const all = useSelector((state: GlobalState) => getDMPrivacy(state)) || 'everyone';
    const here = useSelector((state: GlobalState) => getDMPrivacy(state, teamId));
    const privacy = useSelector(getMyProfilePrivacy);
    const options = useDMPrivacyOptions();
    const own = privacy[teamId];
    const name = team?.display_name || '';
    const title = formatMessage({id: 'fusion.communityPrivacy.title', defaultMessage: 'Privacy in {community}'}, {community: name});

    return (
        <Dialog
            label={title}
            onClose={onClose}
        >
            <div className={am('pane')}>
                <h2>{title}</h2>
                <p className={am('lead')}>{formatMessage({id: 'fusion.communityPrivacy.lead', defaultMessage: 'What the people you meet here can do and see. Someone who\'s also in another of your communities gets the most private of your choices. Nobody is told what you choose.'})}</p>

                {blocking && (
                    <>
                        <Section title={formatMessage({id: 'fusion.privacy.directMessages', defaultMessage: 'Direct messages'})}/>
                        <Select<DMPrivacy | ''>
                            title={formatMessage({id: 'fusion.privacy.dmTitle', defaultMessage: 'From people you don\'t talk with yet'})}
                            desc={formatMessage({id: 'fusion.privacy.dmChoiceDesc', defaultMessage: 'Asked first, their messages wait among your message requests, without notifying you, until you accept them. Refused, they can\'t message you. Bots and system admins always get through.'})}
                            value={here}
                            options={[
                                ['', formatMessage({id: 'fusion.communityPrivacy.asDefault', defaultMessage: 'As my default ({choice})'}, {choice: options.find(([key]) => key === all)?.[1] || ''})],
                                ...options,
                            ]}
                            onChange={(value) => dispatch(setDMPrivacy(teamId, value))}
                        />
                    </>
                )}

                <Section title={formatMessage({id: 'fusion.profilePrivacy.section', defaultMessage: 'Your profile'})}/>
                <p className={am('lead')}>
                    {own ? formatMessage({id: 'fusion.communityPrivacy.profileOwn', defaultMessage: 'Chosen for this community. Your username and picture always show.'}) : formatMessage({id: 'fusion.communityPrivacy.profileDefault', defaultMessage: 'As your defaults until you change something here. Your username and picture always show.'})}
                </p>
                <ProfileFieldToggles
                    hidden={hiddenIn(privacy, teamId)}
                    onChange={(hidden) => dispatch(setHiddenFields(teamId, hidden))}
                />

                <div className={am('modal-actions')}>
                    {own && (
                        <button
                            type='button'
                            className={am('btn')}
                            onClick={() => dispatch(setHiddenFields(teamId, null))}
                        >
                            {formatMessage({id: 'fusion.communityPrivacy.useDefaults', defaultMessage: 'Use my defaults'})}
                        </button>
                    )}
                    <button
                        type='button'
                        className={am('btn', 'primary')}
                        onClick={onClose}
                    >
                        {formatMessage({id: 'fusion.communityPrivacy.done', defaultMessage: 'Done'})}
                    </button>
                </div>
            </div>
        </Dialog>
    );
}
