// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import type {UserPropertyField} from '@mattermost/types/properties_user';
import type {UserProfile} from '@mattermost/types/users';

import {getCustomProfileAttributeValues} from 'mattermost-redux/actions/users';
import {getCustomProfileAttributes} from 'mattermost-redux/selectors/entities/general';
import {getCurrentTeamId} from 'mattermost-redux/selectors/entities/teams';
import {displayLastActiveLabel, getLastActivityForUserId} from 'mattermost-redux/selectors/entities/users';

import ExternalLink from 'components/external_link';

import {AttributeValue, attributeValueText, isEmptyValue} from 'fusion/attributes/attribute_inputs';
import {useCommunityAttributes, useMemberAttributeValues} from 'fusion/attributes/community_attributes';
import {lastOnlineAgo} from 'fusion/channel/header_topic';
import Icon from 'fusion/components/icon';
import type {IconName} from 'fusion/components/icon';
import {useDisplayPrefs} from 'fusion/hooks/display';
import {getUserPropertyFieldLabel} from 'utils/properties';

import type {GlobalState} from 'types/store';

// attributeText is a custom profile field's value as text, with the names of the chosen options.
export function attributeText(field: UserPropertyField, value: string | string[] | undefined): string {
    if (!value || !value.length) {
        return '';
    }
    const name = (id: string) => field.attrs?.options?.find((option) => option.id === id)?.name ?? (field.type === 'text' ? id : '');
    return (Array.isArray(value) ? value : [value]).map(name).filter(Boolean).join(', ');
}

const VALUE_ICONS: Record<string, IconName> = {email: 'mail', phone: 'phone', url: 'link'};

function attributeHref(field: {attrs?: {value_type?: string}}, text: string): string | undefined {
    switch (field.attrs?.value_type) {
    case 'email': return `mailto:${text}`;
    case 'phone': return `tel:${text.replace(/[^\d+]/g, '')}`;
    case 'url': return (/^https?:\/\//i).test(text) ? text : undefined;
    default: return undefined;
    }
}

// ProfileFacts are the extra lines of someone's profile card: their email when the server shares it, the custom
// profile fields the admins defined (the server's, then the current community's), and when they were last online.
// The server only sends admin-only values to the admins and the person themselves; those lines show a lock.
export default function ProfileFacts({user}: {user: UserProfile}) {
    const intl = useIntl();
    const dispatch = useDispatch();
    const {streamer} = useDisplayPrefs();
    const fields = useSelector(getCustomProfileAttributes);
    const lastOnline = useSelector((state: GlobalState) => (displayLastActiveLabel(state, user.id) ? getLastActivityForUserId(state, user.id) : 0));
    const values = user.custom_profile_attributes;
    const teamId = useSelector(getCurrentTeamId);
    const communityFields = useCommunityAttributes(user.is_bot ? undefined : teamId);
    const communityValues = useMemberAttributeValues(user.is_bot ? undefined : teamId, user.id);

    useEffect(() => {
        if (!user.is_bot && fields.length && !values) {
            dispatch(getCustomProfileAttributeValues(user.id));
        }
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [user.id, fields.length, Boolean(values)]);

    const lines: Array<{key: string; icon: IconName; label?: string; text: string; href?: string; node?: React.ReactNode}> = [];
    const adminOnly = intl.formatMessage({id: 'fusion.profile.adminOnly', defaultMessage: 'Only admins and this person see this'});
    if (user.email && !user.is_bot && !streamer) {
        lines.push({key: 'email', icon: 'mail', text: user.email, href: `mailto:${user.email}`});
    }
    if (values && !user.is_bot && !streamer) {
        for (const field of fields) {
            const visibility = field.attrs?.visibility || 'when_set';
            if (visibility === 'hidden' || field.attrs?.access_mode === 'source_only') {
                continue;
            }
            const text = attributeText(field, values[field.id]);
            if (!text && visibility === 'when_set') {
                continue;
            }
            const restricted = visibility === 'admin';
            lines.push({key: field.id, icon: restricted ? 'lock' : VALUE_ICONS[field.attrs?.value_type || ''] || 'tag', label: restricted ? `${getUserPropertyFieldLabel(field)} · ${adminOnly}` : getUserPropertyFieldLabel(field), text: text || '—', href: text ? attributeHref(field, text) : undefined});
        }
    }
    if (communityFields && communityValues && !user.is_bot && !streamer) {
        for (const field of communityFields) {
            const visibility = field.attrs?.visibility || 'when_set';
            const value = communityValues[field.id];
            if (visibility === 'hidden' || (isEmptyValue(value) && visibility !== 'always')) {
                continue;
            }
            const people = field.type === 'user' || field.type === 'multiuser';
            const text = people || isEmptyValue(value) ? '' : attributeValueText(intl, field, value);
            const restricted = visibility === 'admin';
            lines.push({
                key: field.id,
                icon: restricted ? 'lock' : VALUE_ICONS[field.attrs?.value_type || ''] || 'tag',
                label: restricted ? `${field.name} · ${adminOnly}` : field.name,
                text: text || '—',
                href: text ? attributeHref(field, text) : undefined,
                node: people && !isEmptyValue(value) ? (
                    <AttributeValue
                        field={field}
                        value={value}
                    />
                ) : undefined,
            });
        }
    }
    if (lastOnline) {
        lines.push({key: 'last', icon: 'history', text: intl.formatMessage({id: 'fusion.header.lastOnline', defaultMessage: 'Last online {when}'}, {when: lastOnlineAgo(intl, lastOnline)})});
    }
    if (!lines.length) {
        return null;
    }
    return (
        <>
            {lines.map((line) => (
                <span
                    key={line.key}
                    title={line.label}
                >
                    <Icon name={line.icon}/>
                    {line.label && <b>{line.label}</b>}
                    {line.node}
                    {!line.node && line.href?.startsWith('http') && (
                        <ExternalLink
                            href={line.href}
                            location='fusion_profile'
                        >
                            {line.text}
                        </ExternalLink>
                    )}
                    {!line.node && line.href && !line.href.startsWith('http') && <a href={line.href}>{line.text}</a>}
                    {!line.node && !line.href && line.text}
                </span>
            ))}
        </>
    );
}
