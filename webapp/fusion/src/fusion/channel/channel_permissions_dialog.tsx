// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useRef, useState} from 'react';
import {useIntl} from 'react-intl';
import {useSelector} from 'react-redux';

import type {Channel} from '@mattermost/types/channels';

import {Client4} from 'mattermost-redux/client';
import {getManagedCategoryMappings} from 'mattermost-redux/selectors/entities/channel_categories';

import {Dialog} from 'fusion/components/layer';
import {useToast} from 'fusion/shell/toast_context';
import {getTeamRoles} from 'fusion/teams/roles';
import {am} from 'fusion/utils/class_names';

import type {GlobalState} from 'types/store';

import {OverridesEditor, useOverrideList} from './overrides_editor';
import type {Override} from './overrides_editor';

export {lineState, setLine} from './overrides_editor';

// ChannelPermissionsDialog is a channel's permissions, as Discord's: who can see it (for a private channel, possibly
// some roles only), and what roles or people may do here on top of, or less than, what they have elsewhere. A channel
// filed in a category for everyone inherits the category's overrides; its own win, line by line.
export default function ChannelPermissionsDialog({channel, onClose}: {channel: Channel; onClose: () => void}) {
    const {formatMessage} = useIntl();
    const toast = useToast();
    const roles = useSelector((state: GlobalState) => getTeamRoles(state, channel.team_id));
    const category = useSelector((state: GlobalState) => getManagedCategoryMappings(state, channel.team_id)?.[channel.id] || '');
    const [access, setAccess] = useState<string[]>([]);
    const [inherited, setInherited] = useState<Override[]>([]);
    const isPrivate = channel.type === 'P';

    const list = useOverrideList(
        () => Promise.all([
            Client4.getChannelOverrides(channel.id),
            isPrivate ? Client4.getChannelRoleAccess(channel.id) : Promise.resolve({group_ids: []}),
        ]).then(([overrides, a]) => {
            setAccess(a.group_ids);
            return overrides;
        }),
        (o) => Client4.saveChannelOverride(channel.id, {target_type: o.target_type, target_id: o.target_id, allow: o.allow, deny: o.deny}),
        [channel.id, isPrivate],
    );

    // The category's overrides, shown beside the channel's own.
    useEffect(() => {
        if (!category) {
            setInherited([]);
            return;
        }
        const key = category.trim().toLowerCase();
        Client4.getCategoryOverrides(channel.team_id).
            then((all) => setInherited(all.filter((o) => o.category === key))).
            catch(() => setInherited([]));
    }, [category, channel.team_id]);

    const fail = (error: unknown) => toast((error as {message?: string}).message || formatMessage({id: 'fusion.overrides.failed', defaultMessage: 'The permissions couldn\'t be saved.'}));

    const accessQueue = useRef<Promise<unknown>>(Promise.resolve());
    const accessLatest = useRef<string[]>([]);
    accessLatest.current = access;
    const toggleAccess = (groupId: string) => {
        accessQueue.current = accessQueue.current.then(async () => {
            const current = accessLatest.current;
            const next = current.includes(groupId) ? current.filter((id) => id !== groupId) : [...current, groupId];
            accessLatest.current = next;
            setAccess(next);
            try {
                const result = await Client4.setChannelRoleAccess(channel.id, next);
                accessLatest.current = result.group_ids;
                setAccess(result.group_ids);
            } catch (error) {
                accessLatest.current = current;
                setAccess(current);
                fail(error);
            }
        });
    };

    return (
        <Dialog
            label={formatMessage({id: 'fusion.overrides.title', defaultMessage: 'Permissions of #{channel}'}, {channel: channel.display_name})}
            onClose={onClose}
            small={false}
            className={am('wide')}
        >
            <div className={am('pane', 'roles-pane')}>
                <h2>{formatMessage({id: 'fusion.overrides.title', defaultMessage: 'Permissions of #{channel}'}, {channel: channel.display_name})}</h2>
                <p className={am('lead')}>{formatMessage({id: 'fusion.overrides.lead', defaultMessage: 'Here, roles and people can be allowed more, or less, than they have elsewhere. A person\'s own setting wins over their roles\'; the community\'s admins aren\'t affected.'})}</p>
                {list.loadFailed && (
                    <p
                        className={am('lead')}
                        role='alert'
                    >
                        {formatMessage({id: 'fusion.overrides.loadFailed', defaultMessage: 'The channel\'s permissions couldn\'t be loaded. Close this and try again.'})}
                    </p>
                )}
                <h3 className={am('section-title')}>{formatMessage({id: 'fusion.overrides.whoSees', defaultMessage: 'Who can see it'})}</h3>
                {isPrivate ? (
                    <div className={am('role-access')}>
                        <p className={am('lead')}>{access.length ? formatMessage({id: 'fusion.overrides.rolesOnly', defaultMessage: 'Only people with these roles: they join it, and others leave.'}) : formatMessage({id: 'fusion.overrides.invited', defaultMessage: 'Anyone invited can join. Pick roles to limit it to the people who have them.'})}</p>
                        {roles.map((role) => (
                            <label
                                key={role.id}
                                className={am('check-row')}
                            >
                                <input
                                    type='checkbox'
                                    checked={access.includes(role.id)}
                                    disabled={!list.editable}
                                    onChange={() => toggleAccess(role.id)}
                                />
                                <i
                                    className={am('role-dot')}
                                    style={{background: role.color || 'var(--am-muted)'}}
                                />
                                {role.display_name}
                            </label>
                        ))}
                        {!roles.length && <p className={am('lead')}>{formatMessage({id: 'fusion.overrides.noRoles', defaultMessage: 'The community has no roles yet.'})}</p>}
                    </div>
                ) : (
                    <p className={am('lead')}>{formatMessage({id: 'fusion.overrides.public', defaultMessage: 'Everyone in the community: it\'s public. Make it private to keep it for some roles.'})}</p>
                )}
                <h3 className={am('section-title')}>{formatMessage({id: 'fusion.overrides.what', defaultMessage: 'What they can do here'})}</h3>
                {inherited.length > 0 && <p className={am('lead')}>{formatMessage({id: 'fusion.overrides.inheritsFrom', defaultMessage: 'It inherits the permissions of the category {category}; what\'s set here wins.'}, {category})}</p>}
                <OverridesEditor
                    teamId={channel.team_id}
                    overrides={list.overrides}
                    inherited={inherited}
                    editable={list.editable}
                    onChange={list.change}
                    onAdd={list.add}
                />
                <div className={am('modal-actions')}>
                    <button
                        type='button'
                        className={am('btn', 'primary')}
                        onClick={onClose}
                    >
                        {formatMessage({id: 'fusion.overrides.done', defaultMessage: 'Done'})}
                    </button>
                </div>
            </div>
        </Dialog>
    );
}
