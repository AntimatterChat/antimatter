// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {ServerCategory} from '@mattermost/types/channel_categories';
import type {Channel, ChannelPresetId} from '@mattermost/types/channels';
import type {TeamKind} from '@mattermost/types/teams';

import {saveServerCategories} from 'mattermost-redux/actions/channel_categories';
import {createChannel} from 'mattermost-redux/actions/channels';
import {createGroupWithUserIds, patchGroup} from 'mattermost-redux/actions/groups';
import {patchTeam} from 'mattermost-redux/actions/teams';
import {Client4} from 'mattermost-redux/client';
import {General, Permissions} from 'mattermost-redux/constants';
import {getManagedCategoryMappings} from 'mattermost-redux/selectors/entities/channel_categories';
import {getMyChannels} from 'mattermost-redux/selectors/entities/channels';
import {getChannelTypeInfo} from 'mattermost-redux/selectors/entities/channel_types';
import {getTeam} from 'mattermost-redux/selectors/entities/teams';

import {isChannelPresetId} from 'fusion/channel/channel_type_presets';
import {fileChannel, findServerCategory, getServerCategories} from 'fusion/sidebar/server_categories';

import type {ActionFuncAsync, GlobalState} from 'types/store';

import {getTeamRoles} from './roles';
import {loadTeams} from './teams';

// A team template: what makes a team's layout, to export from one team and apply to another, as Discord's server
// templates. The presets are templates too. Channels are matched by name, roles by their display name: applying a
// template adds what's missing and sets what's there, and removes nothing but server categories it doesn't list.
export type TeamTemplate = {
    version: 1;
    server_categories: Array<{name: string; locked: boolean; channels: string[]}>;

    // preset (Antimatter channel types): the channel's kind of channel, e.g. 'forum_qa'. Left out, or unknown on
    // import, makes a plain text Chat channel, as before channel types. The channel's other contents are unchanged.
    channels: Array<{name: string; display_name: string; type: 'O' | 'P'; purpose?: string; header?: string; preset?: ChannelPresetId}>;
    roles: Array<{display_name: string; color?: string; hoist?: boolean; self_assignable?: boolean; permissions?: string[]}>;
    onboarding?: string;

    // The kind of community it makes (see Team.kind): what its members may do, such as archiving channels.
    kind?: TeamKind;
};

export type Preset = 'hybrid' | 'guild' | 'workspace';

// The channel new communities get for their join and leave messages (the server's model.WelcomeChannelName).
export const WELCOME_CHANNEL = 'welcome';

// What the presets' moderators may do: manage messages (edit and delete others', put them on top), rename channels
// and manage their members, and remove people, which timeouts take too (the server's CheckCanModerateMember). The
// server brings the presets' roles made before "Remove people" was in it up to this (doModeratorPresetMigration).
export const MODERATOR_PERMISSIONS = [
    Permissions.DELETE_OTHERS_POSTS,
    Permissions.EDIT_OTHERS_POSTS,
    Permissions.MANAGE_PUBLIC_CHANNEL_PROPERTIES,
    Permissions.MANAGE_PRIVATE_CHANNEL_PROPERTIES,
    Permissions.MANAGE_PUBLIC_CHANNEL_MEMBERS,
    Permissions.MANAGE_PRIVATE_CHANNEL_MEMBERS,
    Permissions.REMOVE_USER_FROM_TEAM,
];

// The presets of the roles and categories plan. Hybrid, Antimatter's default: curated categories for everyone and an
// open one where members file their channels. Guild, Discord's: categories admins keep. Both make a guild. Workspace,
// Mattermost's: everyone's own categories only, in a workspace, where members archive the channels they created. The
// Information category holds the Welcome channel, the others General, where people land.
export const PRESETS: Record<Preset, TeamTemplate> = {
    hybrid: {
        version: 1,
        server_categories: [{name: 'Information', locked: true, channels: [WELCOME_CHANNEL]}, {name: 'Community', locked: false, channels: [General.DEFAULT_CHANNEL, 'off-topic']}],
        channels: [],
        roles: [{display_name: 'Moderators', color: '#e67e22', hoist: true, permissions: MODERATOR_PERMISSIONS}],
        kind: 'guild',
    },
    guild: {
        version: 1,
        server_categories: [{name: 'Information', locked: true, channels: [WELCOME_CHANNEL]}, {name: 'Channels', locked: true, channels: [General.DEFAULT_CHANNEL, 'off-topic']}],
        channels: [],
        roles: [{display_name: 'Moderators', color: '#e67e22', hoist: true, permissions: MODERATOR_PERMISSIONS}],
        kind: 'guild',
    },
    workspace: {
        version: 1,
        server_categories: [],
        channels: [],
        roles: [],
        kind: 'workspace',
    },
};

// templateKind reads a template's kind, the kinds' former names (community, workplace) standing for them.
export function templateKind(kind: unknown): TeamKind | undefined {
    switch (kind) {
    case 'guild':
    case 'community':
        return 'guild';
    case 'workspace':
    case 'workplace':
        return 'workspace';
    }
    return undefined;
}

// exportTemplate captures a team's layout: its server categories and their channels, its public and private channels,
// its roles and their permissions (but its default role, which every community has), and its onboarding.
export function exportTemplate(teamId: string): ActionFuncAsync<TeamTemplate> {
    return async (dispatch, getState) => {
        const state = getState();
        const channels = getMyChannels(state).filter((c) => c.team_id === teamId && (c.type === 'O' || c.type === 'P') && c.delete_at === 0);
        const byId = new Map(channels.map((c) => [c.id, c]));
        const mappings = getManagedCategoryMappings(state, teamId) || {};
        const defaultRoleId = getTeam(state, teamId)?.default_role_id;
        const roles = getTeamRoles(state, teamId).filter((r) => r.id !== defaultRoleId);

        // A role whose permissions can't be read is exported without them: applying it leaves them as they are.
        const permissions = await Promise.all(roles.map((r) => Client4.getTeamRole(r.id).then((role) => role.permissions).catch(() => undefined)));
        return {
            data: {
                version: 1,
                server_categories: getServerCategories(state, teamId).map((c) => ({
                    name: c.name,
                    locked: c.locked,
                    channels: Object.entries(mappings).filter(([, name]) => name.toLowerCase() === c.name.toLowerCase()).map(([id]) => byId.get(id)?.name).filter((n): n is string => Boolean(n)),
                })),
                channels: channels.map((c) => {
                    // Plain text Chat, the default every channel had before channel types, is left out: it exports
                    // the same as before them.
                    const preset = getChannelTypeInfo(state, c.id).preset;
                    return {name: c.name, display_name: c.display_name, type: c.type as 'O' | 'P', purpose: c.purpose || undefined, header: c.header || undefined, preset: preset && preset !== 'text_chat' ? preset : undefined};
                }),
                roles: roles.map((r, i) => ({display_name: r.display_name, color: r.color || undefined, hoist: r.hoist, self_assignable: r.self_assignable, permissions: permissions[i]})),
                onboarding: getTeam(state, teamId)?.onboarding || undefined,
                kind: getTeam(state, teamId)?.kind || undefined,
            },
        };
    };
}

// parseTemplate reads a template file, or undefined when it isn't one. It keeps what's well formed of each entry and
// drops the entries it can't use. Templates made before the default channel became general list it as town-square: it
// stands for general.
export function parseTemplate(json: string): TeamTemplate | undefined {
    let raw;
    try {
        raw = JSON.parse(json);
    } catch {
        return undefined;
    }
    if (raw?.version !== 1 || !Array.isArray(raw.server_categories) || !Array.isArray(raw.channels) || !Array.isArray(raw.roles)) {
        return undefined;
    }
    const text = (v: unknown) => (typeof v === 'string' ? v.trim() : '');
    const strings = (v: unknown) => (Array.isArray(v) ? v.filter((x): x is string => typeof x === 'string') : []);
    const channelName = (name: string) => (name === General.LEGACY_DEFAULT_CHANNEL ? General.DEFAULT_CHANNEL : name);
    const objects = (v: unknown[]) => v.filter((x): x is Record<string, unknown> => Boolean(x) && typeof x === 'object');
    return {
        version: 1,
        server_categories: objects(raw.server_categories).
            map((c) => ({name: text(c.name), locked: c.locked === true, channels: strings(c.channels).map(channelName)})).
            filter((c) => c.name),
        channels: objects(raw.channels).
            map((c) => ({
                name: channelName(text(c.name)),
                display_name: text(c.display_name),
                type: c.type === 'P' ? 'P' as const : 'O' as const,
                purpose: text(c.purpose) || undefined,
                header: text(c.header) || undefined,
                preset: typeof c.preset === 'string' && isChannelPresetId(c.preset) ? c.preset : undefined,
            })).
            filter((c) => (/^[a-z0-9]+([a-z0-9_-]*[a-z0-9]+)*$/).test(c.name) && c.display_name),
        roles: objects(raw.roles).
            map((r) => ({
                display_name: text(r.display_name),
                color: (/^#[0-9a-fA-F]{6}$/).test(text(r.color)) ? text(r.color) : undefined,
                hoist: r.hoist === true,
                self_assignable: r.self_assignable === true,
                permissions: Array.isArray(r.permissions) ? strings(r.permissions) : undefined,
            })).
            filter((r) => r.display_name),
        onboarding: typeof raw.onboarding === 'string' ? raw.onboarding : undefined,
        kind: templateKind(raw.kind),
    };
}

// applyTemplate sets a team up from a template: the channels it lacks, the roles it lacks (and the looks and
// permissions of those it has), its server categories with their channels filed in, its onboarding and its kind. It
// goes on past what fails, and gives back how many steps did.
export function applyTemplate(teamId: string, template: TeamTemplate): ActionFuncAsync<{failed: number}> {
    return async (dispatch, getState) => {
        let failed = 0;
        const failedIf = (result: unknown) => {
            if (!result || (typeof result === 'object' && 'error' in result && result.error)) {
                failed += 1;
            }
            return result;
        };
        const byName = new Map<string, Channel>(getMyChannels(getState()).filter((c) => c.team_id === teamId).map((c) => [c.name, c]));
        const created = await Promise.all(template.channels.filter((c) => !byName.has(c.name)).map((c) => dispatch(createChannel({
            team_id: teamId, name: c.name, display_name: c.display_name, type: c.type, purpose: c.purpose || '', header: c.header || '',
            ...(c.preset ? {props: {am_preset: c.preset}} : {}),
        } as Channel, ''))));
        for (const result of created) {
            failedIf(result);
            if ('data' in result && result.data) {
                byName.set(result.data.name, result.data);
            }
        }

        const defaultRoleId = getTeam(getState(), teamId)?.default_role_id;
        const existing = getTeamRoles(getState(), teamId).filter((r) => r.id !== defaultRoleId);
        await Promise.all(template.roles.map(async (r, i) => {
            const fields = {display_name: r.display_name, color: r.color || '', hoist: Boolean(r.hoist), self_assignable: Boolean(r.self_assignable)};
            const known = existing.find((e) => e.display_name.toLowerCase() === r.display_name.toLowerCase());
            const result = failedIf(known ? await dispatch(patchGroup(known.id, fields)) : await dispatch(createGroupWithUserIds({
                ...fields,
                name: `${r.display_name.toLowerCase().replace(/[^a-z0-9]+/g, '-')}-${teamId.slice(0, 8)}`,
                source: 'custom',
                allow_reference: true,
                team_id: teamId,
                position: existing.length + i,
                user_ids: [],
            }))) as {data?: {id: string}};
            const id = known?.id || result.data?.id;
            if (id && r.permissions) {
                await Client4.updateTeamRolePermissions(id, r.permissions).catch(() => failedIf(null));
            }
        }));

        const listed = template.server_categories.map(({name, locked}) => ({name, locked}));
        const removed = getServerCategories(getState(), teamId).map((c) => c.name).filter((name) => !findServerCategory(listed as ServerCategory[], name));
        failedIf(await dispatch(saveServerCategories(teamId, {categories: listed, removed})));
        await Promise.all(template.server_categories.flatMap((category) => category.channels.
            map((name) => byName.get(name)).
            filter((channel): channel is Channel => Boolean(channel)).
            map((channel) => dispatch(fileChannel(channel.id, category.name)).then(failedIf))));

        if (template.onboarding !== undefined || template.kind) {
            failedIf(await dispatch(patchTeam({
                id: teamId,
                ...(template.onboarding === undefined ? {} : {onboarding: template.onboarding}),
                ...(template.kind ? {kind: template.kind} : {}),
            })));
        }
        dispatch(loadTeams());
        return {data: {failed}};
    };
}

// isTeamSetUp tells whether a team has categories for everyone or roles (its default role, which any team whose roles
// were managed has, doesn't count).
export function isTeamSetUp(state: GlobalState, teamId: string): boolean {
    const defaultRoleId = getTeam(state, teamId)?.default_role_id;
    return getServerCategories(state, teamId).length > 0 || getTeamRoles(state, teamId).some((r) => r.id !== defaultRoleId);
}
