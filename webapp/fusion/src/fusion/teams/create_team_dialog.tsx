// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch} from 'react-redux';

import type {Team} from '@mattermost/types/teams';

import {checkIfTeamExists, createTeam} from 'mattermost-redux/actions/teams';

import {Dialog} from 'fusion/components/layer';
import {Toggle} from 'fusion/modals/settings_controls';
import {am} from 'fusion/utils/class_names';
import {getHistory} from 'utils/browser_history';
import Constants from 'utils/constants';
import {cleanUpUrlable} from 'utils/url';

export type TeamUrlProblem = 'length' | 'format' | 'reserved' | 'taken';

// teamUrlFromName is the address a team's name suggests: lower case, dashes for the rest, as the classic page made it.
export function teamUrlFromName(name: string): string {
    return cleanUpUrlable(name).slice(0, Constants.MAX_TEAMNAME_LENGTH);
}

// teamUrlProblem tells what's wrong with a team's address, by Mattermost's rules; undefined when it's fine as written.
export function teamUrlProblem(url: string): TeamUrlProblem | undefined {
    if (url.length < Constants.MIN_TEAMNAME_LENGTH || url.length > Constants.MAX_TEAMNAME_LENGTH) {
        return 'length';
    }
    if (cleanUpUrlable(url) !== url || !(/^[a-z]+([a-z\-0-9]+|(__)?)[a-z0-9]+$/).test(url)) {
        return 'format';
    }
    if (Constants.RESERVED_TEAM_NAMES.some((reserved: string) => url.startsWith(reserved))) {
        return 'reserved';
    }
    return undefined;
}

// CreateTeamDialog makes a team: its name, its address (suggested by the name), and whether anyone on the server can
// join it. The new team opens, where its admins are offered a layout to start from.
export default function CreateTeamDialog({onClose}: {onClose: () => void}) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const [name, setName] = useState('');
    const [url, setUrl] = useState('');
    const [urlEdited, setUrlEdited] = useState(false);
    const [open, setOpen] = useState(true);
    const [taken, setTaken] = useState<string | null>(null);
    const [busy, setBusy] = useState(false);
    const [error, setError] = useState('');

    // The address follows the name until it's edited.
    useEffect(() => {
        if (!urlEdited) {
            setUrl(teamUrlFromName(name));
        }
    }, [name, urlEdited]);

    // Whether the address is free, asked once typing stops.
    useEffect(() => {
        setTaken(null);
        if (!url || teamUrlProblem(url)) {
            return undefined;
        }
        let stale = false;
        const timer = setTimeout(async () => {
            const result = await dispatch(checkIfTeamExists(url));
            if (!stale && 'data' in result && result.data) {
                setTaken(url);
            }
        }, 300);
        return () => {
            stale = true;
            clearTimeout(timer);
        };
    }, [url, dispatch]);

    const problem: TeamUrlProblem | undefined = url ? teamUrlProblem(url) || (taken === url ? 'taken' : undefined) : undefined;
    const problems: Record<TeamUrlProblem, string> = {
        length: formatMessage({id: 'fusion.createTeam.urlLength', defaultMessage: 'Between {min} and {max} characters.'}, {min: Constants.MIN_TEAMNAME_LENGTH, max: Constants.MAX_TEAMNAME_LENGTH}),
        format: formatMessage({id: 'fusion.createTeam.urlFormat', defaultMessage: 'Lower case letters, numbers and dashes; it starts with a letter and doesn\'t end with a dash.'}),
        reserved: formatMessage({id: 'fusion.createTeam.urlReserved', defaultMessage: 'It can\'t start with a reserved word.'}),
        taken: formatMessage({id: 'fusion.createTeam.urlTaken', defaultMessage: 'That address is taken.'}),
    };
    const canCreate = Boolean(name.trim()) && name.trim().length <= Constants.MAX_TEAMNAME_LENGTH && Boolean(url) && !problem && !busy;

    const submit = async () => {
        if (!canCreate) {
            return;
        }
        setBusy(true);
        setError('');
        const result = await dispatch(createTeam({display_name: name.trim(), name: url, type: open ? 'O' : 'I', allow_open_invite: open} as Team));
        setBusy(false);
        if ('error' in result && result.error) {
            setError((result.error as {message?: string}).message || formatMessage({id: 'fusion.createTeam.failed', defaultMessage: 'The team couldn\'t be created.'}));
            return;
        }
        onClose();
        getHistory().push(`/${url}/channels/${Constants.DEFAULT_CHANNEL}`);
    };

    return (
        <Dialog
            label={formatMessage({id: 'fusion.createTeam.title', defaultMessage: 'Create a team'})}
            onClose={onClose}
            onSubmit={submit}
        >
            <div className={am('pane')}>
                <h2>{formatMessage({id: 'fusion.createTeam.title', defaultMessage: 'Create a team'})}</h2>
                <p className={am('lead')}>{formatMessage({id: 'fusion.createTeam.lead', defaultMessage: 'A team has its own channels and people. You can set it up from a preset once it\'s made.'})}</p>
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
                <label className={am('field')}>
                    <span>{formatMessage({id: 'fusion.createTeam.url', defaultMessage: 'Address'})}</span>
                    <div className={am('team-url')}>
                        <span className={am('team-url-site')}>{`${window.location.host}/`}</span>
                        <input
                            type='text'
                            value={url}
                            maxLength={Constants.MAX_TEAMNAME_LENGTH}
                            aria-label={formatMessage({id: 'fusion.createTeam.url', defaultMessage: 'Address'})}
                            aria-invalid={Boolean(problem)}
                            aria-describedby='am-create-team-url-note'
                            onChange={(e) => {
                                setUrlEdited(true);
                                setUrl(e.target.value.toLowerCase());
                            }}
                        />
                    </div>
                    <small
                        id='am-create-team-url-note'
                        className={am('field-note', {bad: Boolean(problem)})}
                    >
                        {problem ? problems[problem] : formatMessage({id: 'fusion.createTeam.urlNote', defaultMessage: 'Where the team is found; it can\'t change later.'})}
                    </small>
                </label>
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
                        {formatMessage({id: 'fusion.createTeam.create', defaultMessage: 'Create team'})}
                    </button>
                </div>
            </div>
        </Dialog>
    );
}
