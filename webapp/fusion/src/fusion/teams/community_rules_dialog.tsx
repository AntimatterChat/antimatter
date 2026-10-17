// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import {fetchCommunityRules} from 'mattermost-redux/actions/teams';
import {getChannel} from 'mattermost-redux/selectors/entities/channels';
import {getTeam} from 'mattermost-redux/selectors/entities/teams';

import Markdown from 'components/markdown';

import Icon from 'fusion/components/icon';
import {Dialog} from 'fusion/components/layer';
import {useToast} from 'fusion/shell/toast_context';
import {am} from 'fusion/utils/class_names';

import type {GlobalState} from 'types/store';

import {RulesChangedError, acceptCommunityRules, getCommunityRules, mustAcceptCommunityRules} from './community_rules';

// CommunityRulesDialog shows a community's rules with "I agree": when someone opens the community (on joining it,
// and when its admins ask everyone to agree again), and from the notice in place of the message box.
export function CommunityRulesDialog({teamId, onClose}: {teamId: string; onClose: () => void}) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const toast = useToast();
    const team = useSelector((state: GlobalState) => getTeam(state, teamId));
    const rules = useSelector((state: GlobalState) => getCommunityRules(state, teamId));
    const [busy, setBusy] = useState(false);
    const [error, setError] = useState('');

    // The rules are shown as last loaded, and loaded afresh in case they changed.
    useEffect(() => {
        dispatch(fetchCommunityRules(teamId));
    }, [teamId, dispatch]);

    const name = team?.display_name || '';
    const title = formatMessage({id: 'fusion.communityRules.dialogTitle', defaultMessage: 'Rules of {community}'}, {community: name});

    const agree = async () => {
        if (!rules) {
            return;
        }
        setBusy(true);
        setError('');
        const result = await dispatch(acceptCommunityRules(teamId, rules.version));
        setBusy(false);
        if ('error' in result && result.error) {
            setError(result.error instanceof RulesChangedError ?
                formatMessage({id: 'fusion.communityRules.changed', defaultMessage: 'These rules just changed: read them again, then agree.'}) :
                formatMessage({id: 'fusion.communityRules.agreeFailed', defaultMessage: 'Your agreement couldn\'t be saved. Try again.'}));
            return;
        }
        toast(formatMessage({id: 'fusion.communityRules.agreed', defaultMessage: 'Thanks! You can post in {community} now'}, {community: name}));
        onClose();
    };

    return (
        <Dialog
            label={title}
            onClose={onClose}
            small={false}
            className={am('wide')}
        >
            <div className={am('pane', 'community-rules')}>
                <h2>{title}</h2>
                <p className={am('lead')}>
                    {formatMessage({id: 'fusion.communityRules.dialogLead', defaultMessage: 'Read and agree to the rules of this community to post in its channels. You can read everything meanwhile.'})}
                </p>
                <div className={am('community-rules-text')}>
                    {rules ? <Markdown message={rules.text}/> : formatMessage({id: 'fusion.communityRules.loading', defaultMessage: 'Loading the rules…'})}
                </div>
                {error && (
                    <div
                        className={am('field-error')}
                        role='alert'
                    >
                        {error}
                    </div>
                )}
                <div className={am('modal-actions')}>
                    <button
                        type='button'
                        className={am('btn')}
                        disabled={busy}
                        onClick={onClose}
                    >
                        {formatMessage({id: 'fusion.communityRules.notNow', defaultMessage: 'Not now'})}
                    </button>
                    <button
                        type='button'
                        className={am('btn', 'primary')}
                        disabled={busy || !rules?.must_accept}
                        onClick={agree}
                    >
                        <Icon name='check'/>
                        {formatMessage({id: 'fusion.communityRules.agree', defaultMessage: 'I agree'})}
                    </button>
                </div>
            </div>
        </Dialog>
    );
}

// CommunityRulesNotice takes the place of the message box in a community's channels until one agrees to its rules.
export function CommunityRulesNotice({teamId}: {teamId: string}) {
    const {formatMessage} = useIntl();
    const team = useSelector((state: GlobalState) => getTeam(state, teamId));
    const [reading, setReading] = useState(false);
    return (
        <div className={am('composer')}>
            <div className={am('note-box', 'request-bar', 'community-rules-bar')}>
                <Icon name='shield'/>
                <div className={am('grow')}>
                    <b>{formatMessage({id: 'fusion.communityRules.noticeTitle', defaultMessage: 'Agree to the rules to post here'})}</b>
                    <span>
                        {formatMessage({id: 'fusion.communityRules.noticeText', defaultMessage: '{community} asks everyone to agree to its rules before they post. You can read everything meanwhile.'}, {community: team?.display_name || ''})}
                    </span>
                </div>
                <div className={am('request-actions')}>
                    <button
                        type='button'
                        className={am('btn', 'primary')}
                        onClick={() => setReading(true)}
                    >
                        {formatMessage({id: 'fusion.communityRules.read', defaultMessage: 'Read and agree'})}
                    </button>
                </div>
            </div>
            {reading && (
                <CommunityRulesDialog
                    teamId={teamId}
                    onClose={() => setReading(false)}
                />
            )}
        </div>
    );
}

// CommunityRulesGate shows its message box, or, in a community's channel whose rules one must agree to first, the
// notice instead. Direct and group messages have no community, so no rules.
export function CommunityRulesGate({channelId, children}: {channelId: string; children: React.ReactNode}) {
    const teamId = useSelector((state: GlobalState) => getChannel(state, channelId)?.team_id || '');
    const mustAccept = useSelector((state: GlobalState) => mustAcceptCommunityRules(state, teamId));
    if (mustAccept) {
        return <CommunityRulesNotice teamId={teamId}/>;
    }
    return <>{children}</>;
}
