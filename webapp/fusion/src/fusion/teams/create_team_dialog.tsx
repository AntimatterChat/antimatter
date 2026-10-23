// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import type {Team, TeamKind} from '@mattermost/types/teams';

import {createTeam} from 'mattermost-redux/actions/teams';
import {getConfig} from 'mattermost-redux/selectors/entities/general';

import {Dialog} from 'fusion/components/layer';
import {Choice, Toggle} from 'fusion/modals/settings_controls';
import {am} from 'fusion/utils/class_names';
import {getHistory} from 'utils/browser_history';
import Constants from 'utils/constants';

import type {GlobalState} from 'types/store';

// CreateTeamDialog makes a community on this server: its name, its kind (a guild, where everyone is in all the
// channels, or a workspace, where members join the channels they want), and whether anyone on the server can join it.
// Its address is the server's to make from its name. The new community opens, where its admins are offered a layout to
// start from.
export default function CreateTeamDialog({onClose}: {onClose: () => void}) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const defaultKind = useSelector((state: GlobalState) => (getConfig(state).DefaultCommunityKind === 'workspace' ? 'workspace' : 'guild'));
    const [name, setName] = useState('');
    const [kind, setKind] = useState<TeamKind>(defaultKind);
    const [open, setOpen] = useState(true);
    const [busy, setBusy] = useState(false);
    const [error, setError] = useState('');

    const trimmed = name.trim();
    const canCreate = trimmed.length >= Constants.MIN_TEAMNAME_LENGTH && trimmed.length <= Constants.MAX_TEAMNAME_LENGTH && !busy;

    const submit = async () => {
        if (!canCreate) {
            return;
        }
        setBusy(true);
        setError('');
        const result = await dispatch(createTeam({display_name: trimmed, type: open ? 'O' : 'I', allow_open_invite: open, kind} as Team));
        setBusy(false);
        if ('error' in result && result.error) {
            setError((result.error as {message?: string}).message || formatMessage({id: 'fusion.createTeam.failed', defaultMessage: 'The community couldn\'t be created.'}));
            return;
        }
        onClose();
        const created = (result as {data?: Team}).data;
        if (created?.name) {
            getHistory().push(`/${created.name}/channels/${Constants.DEFAULT_CHANNEL}`);
        }
    };

    return (
        <Dialog
            label={formatMessage({id: 'fusion.createTeam.title', defaultMessage: 'Create a community'})}
            onClose={onClose}
            onSubmit={submit}
        >
            <div className={am('pane')}>
                <h2>{formatMessage({id: 'fusion.createTeam.title', defaultMessage: 'Create a community'})}</h2>
                <p className={am('lead')}>{formatMessage({id: 'fusion.createTeam.lead', defaultMessage: 'A community has its own channels and people. You can set it up from a preset once it\'s made.'})}</p>
                <label className={am('field')}>
                    <span>{formatMessage({id: 'fusion.createTeam.name', defaultMessage: 'Name'})}</span>
                    <input
                        type='text'
                        value={name}
                        autoFocus={true}
                        maxLength={Constants.MAX_TEAMNAME_LENGTH}
                        placeholder={formatMessage({id: 'fusion.createTeam.namePlaceholder', defaultMessage: 'e.g. Detector physics'})}
                        onChange={(e) => setName(e.target.value)}
                    />
                </label>
                <Choice<TeamKind>
                    title={formatMessage({id: 'fusion.createTeam.kind', defaultMessage: 'Kind'})}
                    desc={kind === 'guild' ? formatMessage({id: 'fusion.createTeam.guildDesc', defaultMessage: 'A guild, for a public or hobby community: everyone is in all its channels, and only its admins archive them.'}) : formatMessage({id: 'fusion.createTeam.workspaceDesc', defaultMessage: 'A workspace, for a company or an organisation: members join the channels they want, and archive those they created.'})}
                    value={kind}
                    options={[
                        ['guild', formatMessage({id: 'fusion.teamChannels.kind.guild', defaultMessage: 'Guild'})],
                        ['workspace', formatMessage({id: 'fusion.teamChannels.kind.workspace', defaultMessage: 'Workspace'})],
                    ]}
                    onChange={setKind}
                />
                <Toggle
                    title={formatMessage({id: 'fusion.createTeam.open', defaultMessage: 'Anyone on the server can join'})}
                    desc={formatMessage({id: 'fusion.createTeam.openDesc', defaultMessage: 'Otherwise people join by invitation only.'})}
                    on={open}
                    onChange={setOpen}
                />
                {error && (
                    <p
                        className={am('lead')}
                        role='alert'
                    >
                        {error}
                    </p>
                )}
                <div className={am('modal-actions')}>
                    <button
                        type='button'
                        className={am('btn')}
                        onClick={onClose}
                    >
                        {formatMessage({id: 'fusion.createTeam.cancel', defaultMessage: 'Cancel'})}
                    </button>
                    <button
                        type='submit'
                        className={am('btn', 'primary')}
                        disabled={!canCreate}
                    >
                        {formatMessage({id: 'fusion.createTeam.create', defaultMessage: 'Create community'})}
                    </button>
                </div>
            </div>
        </Dialog>
    );
}
