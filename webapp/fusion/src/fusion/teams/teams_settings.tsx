// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useMemo, useState} from 'react';
import {defineMessage, useIntl} from 'react-intl';
import type {MessageDescriptor} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import type {Group} from '@mattermost/types/groups';

import {archiveGroup, getGroups, restoreGroup} from 'mattermost-redux/actions/groups';
import {patchTeam} from 'mattermost-redux/actions/teams';
import {Permissions} from 'mattermost-redux/constants';
import {createSelector} from 'mattermost-redux/selectors/create_selector';
import {haveIGroupPermission, haveITeamPermission} from 'mattermost-redux/selectors/entities/roles';
import {getCurrentTeam} from 'mattermost-redux/selectors/entities/teams';
import {getCurrentUserId} from 'mattermost-redux/selectors/entities/users';

import Icon from 'fusion/components/icon';
import {Toggle} from 'fusion/modals/settings_controls';
import {useToast} from 'fusion/shell/toast_context';
import {am} from 'fusion/utils/class_names';

import type {GlobalState} from 'types/store';

import TeamCard from './team_card';
import TeamEditor from './team_editor';
import {isTeam, loadTeams} from './teams';

export type TeamsFilter = 'all' | 'mine' | 'archived';

const FILTERS: Array<{key: TeamsFilter; label: MessageDescriptor}> = [
    {key: 'all', label: defineMessage({id: 'fusion.teams.filterAll', defaultMessage: 'All'})},
    {key: 'mine', label: defineMessage({id: 'fusion.teams.filterMine', defaultMessage: 'Yours'})},
    {key: 'archived', label: defineMessage({id: 'fusion.teams.filterArchived', defaultMessage: 'Archived'})},
];

// getAllCommunityTeams gives every team of each community, archived ones too, by name.
const getAllCommunityTeams = createSelector(
    'fusionGetAllCommunityTeams',
    (state: GlobalState) => state.entities.groups.groups,
    (groups): Record<string, Group[]> => {
        const byCommunity: Record<string, Group[]> = {};
        for (const group of Object.values(groups)) {
            if (isTeam(group)) {
                (byCommunity[group.team_id!] ||= []).push(group);
            }
        }
        for (const teams of Object.values(byCommunity)) {
            teams.sort((a, b) => a.display_name.localeCompare(b.display_name));
        }
        return byCommunity;
    },
);

// teamsShown filters a community's teams: all those not archived, yours, or the archived ones, matching a search (their
// name, @mention or tag).
export function teamsShown(teams: Group[], filter: TeamsFilter, me: string, search: string): Group[] {
    const term = search.trim().toLowerCase();
    return teams.filter((team) => {
        if (filter === 'archived' ? team.delete_at === 0 : team.delete_at !== 0) {
            return false;
        }
        if (filter === 'mine' && !(team.member_ids || []).includes(me)) {
            return false;
        }
        return !term || [team.display_name, team.name || '', team.tag || ''].some((text) => text.toLowerCase().includes(term));
    });
}

type Props = {
    setUnsaved?: (unsaved: boolean) => void;
};

// TeamsSettings is the Teams tab of the community settings (and the Teams dialog of those who manage nothing in it):
// the community's teams, to find (by name, yours, archived), open (their card), create, edit (name, @mention, badge,
// lead, channel, members) and archive or restore, as the server lets each one; and, for its admins, whether everyone
// shows a team badge there.
export default function TeamsSettings({setUnsaved}: Props) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const toast = useToast();
    const community = useSelector(getCurrentTeam);
    const communityId = community?.id || '';
    const me = useSelector(getCurrentUserId);
    const all = useSelector((state: GlobalState) => getAllCommunityTeams(state)[communityId]);
    const admin = useSelector((state: GlobalState) => Boolean(communityId) && haveITeamPermission(state, communityId, Permissions.MANAGE_TEAM));
    const canCreate = useSelector((state: GlobalState) => Boolean(communityId) && haveITeamPermission(state, communityId, Permissions.MANAGE_TEAM_ROLES));
    const groupRights = useSelector((state: GlobalState) => Object.fromEntries((all || []).map((team) => [team.id, {
        edit: haveIGroupPermission(state, team.id, Permissions.EDIT_CUSTOM_GROUP),
        archive: haveIGroupPermission(state, team.id, Permissions.DELETE_CUSTOM_GROUP),
        restore: haveIGroupPermission(state, team.id, Permissions.RESTORE_CUSTOM_GROUP),
    }])), (a, b) => JSON.stringify(a) === JSON.stringify(b));
    const [filter, setFilter] = useState<TeamsFilter>('all');
    const [search, setSearch] = useState('');
    const [editing, setEditing] = useState<string | 'new' | null>(null);
    const [open, setOpen] = useState<{team: Group; anchor: HTMLElement} | null>(null);

    // The archived teams aren't loaded with the others.
    useEffect(() => {
        if (filter === 'archived' && communityId) {
            dispatch(getGroups({filter_archived: true, in_team_id: communityId, is_team: true, include_member_ids: true, per_page: 200} as Parameters<typeof getGroups>[0]));
        }
    }, [filter, communityId, dispatch]);

    const shown = useMemo(() => teamsShown(all || [], filter, me, search), [all, filter, me, search]);

    if (!community) {
        return null;
    }

    const fail = (result: unknown) => {
        const error = (result as {error?: {message?: string}}).error;
        if (error) {
            toast(error.message || formatMessage({id: 'fusion.teams.saveFailed', defaultMessage: 'The team couldn\'t be saved.'}));
            return true;
        }
        return false;
    };
    const archive = async (team: Group) => {
        if (!fail(await dispatch(archiveGroup(team.id)))) {
            toast(formatMessage({id: 'fusion.teams.archived', defaultMessage: 'Archived {team}'}, {team: team.display_name}));
            dispatch(loadTeams());
        }
    };
    const restore = async (team: Group) => {
        if (!fail(await dispatch(restoreGroup(team.id)))) {
            toast(formatMessage({id: 'fusion.teams.restored', defaultMessage: 'Restored {team}'}, {team: team.display_name}));
            dispatch(loadTeams());
        }
    };

    const editedTeam = editing && editing !== 'new' ? all?.find((t) => t.id === editing) : undefined;
    return (
        <div className={am('team-teams-page')}>
            <p className={am('lead')}>{formatMessage({id: 'fusion.teamSettings.teamsLead', defaultMessage: 'Teams have a badge their members can show next to their name, a lead, a channel and an @mention.'})}</p>
            {admin && (
                <Toggle
                    title={formatMessage({id: 'fusion.roles.badges', defaultMessage: 'Everyone shows a team badge'})}
                    desc={formatMessage({id: 'fusion.roles.badgesDesc', defaultMessage: 'For example in a company: in this community, people who chose no badge show the tag of their first team here.'})}
                    on={Boolean(community.require_team_badges)}
                    onChange={(on) => dispatch(patchTeam({id: community.id, require_team_badges: on}))}
                />
            )}
            <div className={am('team-teams-head')}>
                <input
                    type='search'
                    className={am('grow')}
                    value={search}
                    placeholder={formatMessage({id: 'fusion.teams.search', defaultMessage: 'Find a team'})}
                    aria-label={formatMessage({id: 'fusion.teams.search', defaultMessage: 'Find a team'})}
                    onChange={(e) => setSearch(e.target.value)}
                />
                <div
                    className={am('seg')}
                    role='group'
                    aria-label={formatMessage({id: 'fusion.teams.filter', defaultMessage: 'Show'})}
                >
                    {FILTERS.map((f) => (
                        <button
                            key={f.key}
                            type='button'
                            className={am({on: filter === f.key})}
                            aria-pressed={filter === f.key}
                            onClick={() => setFilter(f.key)}
                        >
                            {formatMessage(f.label)}
                        </button>
                    ))}
                </div>
            </div>
            <div className={am('role-list')}>
                {shown.map((team) => {
                    const rights = groupRights[team.id] || {edit: false, archive: false, restore: false};
                    const archived = team.delete_at !== 0;
                    return (
                        <div
                            key={team.id}
                            className={am('role-row', 'team-row')}
                        >
                            <button
                                type='button'
                                className={am('team-row-open')}
                                aria-haspopup='dialog'
                                disabled={archived}
                                onClick={(e) => {
                                    const anchor = e.currentTarget;
                                    setOpen((current) => (current?.team.id === team.id ? null : {team, anchor}));
                                }}
                            >
                                <span className={am('team-tag', 'static')}>{team.tag || team.display_name.slice(0, 2).toUpperCase()}</span>
                                <span className={am('grow')}>
                                    <b>{team.display_name}</b>
                                    <span>
                                        {formatMessage({id: 'fusion.teams.members', defaultMessage: '{count, plural, one {# member} other {# members}}'}, {count: team.member_ids?.length || team.member_count || 0})}
                                        {team.name && ` · @${team.name}`}
                                    </span>
                                </span>
                            </button>
                            {!archived && rights.edit && (
                                <button
                                    type='button'
                                    className={am('icon-btn')}
                                    title={formatMessage({id: 'fusion.teams.edit', defaultMessage: 'Edit the team'})}
                                    aria-label={formatMessage({id: 'fusion.teams.edit', defaultMessage: 'Edit the team'})}
                                    onClick={() => setEditing(editing === team.id ? null : team.id)}
                                >
                                    <Icon
                                        name='pen'
                                        size='sm'
                                    />
                                </button>
                            )}
                            {!archived && rights.archive && (
                                <button
                                    type='button'
                                    className={am('icon-btn', 'danger')}
                                    title={formatMessage({id: 'fusion.teams.archive', defaultMessage: 'Archive the team'})}
                                    aria-label={formatMessage({id: 'fusion.teams.archive', defaultMessage: 'Archive the team'})}
                                    onClick={() => archive(team)}
                                >
                                    <Icon
                                        name='trash'
                                        size='sm'
                                    />
                                </button>
                            )}
                            {archived && rights.restore && (
                                <button
                                    type='button'
                                    className={am('btn', 'small')}
                                    onClick={() => restore(team)}
                                >
                                    {formatMessage({id: 'fusion.teams.restore', defaultMessage: 'Restore'})}
                                </button>
                            )}
                        </div>
                    );
                })}
                {shown.length === 0 && (
                    <p className={am('lead')}>
                        {search.trim() ? formatMessage({id: 'fusion.teams.noMatch', defaultMessage: 'No team matches.'}) : formatMessage({id: 'fusion.teamSettings.noTeams', defaultMessage: 'No teams yet.'})}
                    </p>
                )}
            </div>
            {editing && (
                <TeamEditor
                    key={editing}
                    team={editedTeam}
                    communityId={community.id}
                    setUnsaved={setUnsaved}
                    onDone={() => setEditing(null)}
                    onCancel={() => setEditing(null)}
                />
            )}
            {!editing && canCreate && (
                <div className={am('team-teams-foot')}>
                    <button
                        type='button'
                        className={am('btn')}
                        onClick={() => {
                            setFilter('all');
                            setEditing('new');
                        }}
                    >
                        <Icon
                            name='plus'
                            size='sm'
                        />
                        {formatMessage({id: 'fusion.teams.createTeam', defaultMessage: 'Create team'})}
                    </button>
                </div>
            )}
            {open && (
                <TeamCard
                    team={open.team}
                    anchor={open.anchor}
                    onClose={() => setOpen(null)}
                />
            )}
        </div>
    );
}
