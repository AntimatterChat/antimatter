// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useMemo} from 'react';
import {useIntl} from 'react-intl';
import type {IntlShape} from 'react-intl';
import {useDispatch} from 'react-redux';

import type {PropertyFieldOption} from '@mattermost/types/properties';
import type {UserAttributeValue} from '@mattermost/types/properties_user';

import {getMissingProfilesByIds} from 'mattermost-redux/actions/users';

import Avatar from 'fusion/components/avatar';
import Icon from 'fusion/components/icon';
import PeopleSearch, {PersonName} from 'fusion/members/people_search';
import {am} from 'fusion/utils/class_names';

// What the attribute inputs and texts need of an attribute: the server's user attributes and the communities' alike.
export type AttributeField = {
    id: string;
    name: string;
    type: string;
    attrs?: {
        display_name?: string;
        value_type?: string;
        options?: PropertyFieldOption[];
        visibility?: string;
        managed?: string;
    };
};

const INPUT_TYPES: Record<string, string> = {email: 'email', url: 'url', phone: 'tel'};

// The longest text value the server keeps.
export const ATTRIBUTE_TEXT_MAX = 64;

export function attributeLabel(field: AttributeField): string {
    return field.attrs?.display_name?.trim() || field.name;
}

export function isEmptyValue(value: UserAttributeValue | undefined): boolean {
    return !value || !value.length;
}

function asList(value: UserAttributeValue | undefined): string[] {
    if (!value) {
        return [];
    }
    return Array.isArray(value) ? value : [value];
}

// usePeople loads the profiles of the people a person attribute names, for their names.
function usePeople(field: AttributeField, value: UserAttributeValue | undefined) {
    const dispatch = useDispatch();
    const ids = field.type === 'user' || field.type === 'multiuser' ? asList(value) : [];
    const key = ids.join(',');
    useEffect(() => {
        if (key) {
            dispatch(getMissingProfilesByIds(key.split(',')));
        }
    }, [key, dispatch]);
}

// formatAttributeDate shows a YYYY-MM-DD value as a date in the reader's language.
export function formatAttributeDate(intl: IntlShape, value: string): string {
    const date = new Date(`${value}T00:00:00`);
    return Number.isNaN(date.getTime()) ? value : intl.formatDate(date, {year: 'numeric', month: 'short', day: 'numeric'});
}

// attributeValueText is a value as text: the names of the chosen options, a date, or the text itself. People are
// shown by AttributeValue instead.
export function attributeValueText(intl: IntlShape, field: AttributeField, value: UserAttributeValue | undefined): string {
    const list = asList(value);
    switch (field.type) {
    case 'select':
    case 'multiselect':
    case 'rank':
        return list.map((id) => field.attrs?.options?.find((option) => option.id === id)?.name ?? '').filter(Boolean).join(', ');
    case 'date':
        return list.map((d) => formatAttributeDate(intl, d)).join(', ');
    default:
        return list.join(', ');
    }
}

// AttributeValue shows a value: people by their names, the rest as text.
export function AttributeValue({field, value}: {field: AttributeField; value: UserAttributeValue | undefined}) {
    const intl = useIntl();
    usePeople(field, value);
    if (field.type === 'user' || field.type === 'multiuser') {
        const ids = asList(value);
        return (
            <>
                {ids.map((id, i) => (
                    <React.Fragment key={id}>
                        {i > 0 && ', '}
                        <PersonName userId={id}/>
                    </React.Fragment>
                ))}
            </>
        );
    }
    return <>{attributeValueText(intl, field, value)}</>;
}

type InputProps = {
    field: AttributeField;
    value: UserAttributeValue | undefined;
    onChange: (value: UserAttributeValue) => void;

    // The community whose people a person attribute offers.
    teamId: string;
    disabled?: boolean;
};

// AttributeInput edits a value, as fits its attribute: a text (an email, a URL, a phone number), a choice or
// several, a date, a person or several.
export function AttributeInput({field, value, onChange, teamId, disabled = false}: InputProps) {
    const {formatMessage} = useIntl();
    const label = attributeLabel(field);
    const options = field.attrs?.options || [];
    const list = useMemo(() => asList(value), [value]);
    usePeople(field, value);

    switch (field.type) {
    case 'select':
        return (
            <select
                className={am('select')}
                value={list[0] || ''}
                disabled={disabled}
                aria-label={label}
                onChange={(e) => onChange(e.target.value)}
            >
                <option value=''>{formatMessage({id: 'fusion.attributes.none', defaultMessage: '—'})}</option>
                {options.map((option) => (
                    <option
                        key={option.id}
                        value={option.id}
                    >
                        {option.name}
                    </option>
                ))}
            </select>
        );
    case 'multiselect':
        return (
            <div
                className={am('attr-choices')}
                role='group'
                aria-label={label}
            >
                {options.map((option) => {
                    const on = list.includes(option.id);
                    return (
                        <button
                            key={option.id}
                            type='button'
                            role='checkbox'
                            aria-checked={on}
                            disabled={disabled}
                            className={am('onboarding-option', {on})}
                            onClick={() => onChange(on ? list.filter((id) => id !== option.id) : [...list, option.id])}
                        >
                            {option.name}
                        </button>
                    );
                })}
            </div>
        );
    case 'date':
        return (
            <input
                type='date'
                value={list[0] || ''}
                disabled={disabled}
                aria-label={label}
                onChange={(e) => onChange(e.target.value)}
            />
        );
    case 'user':
    case 'multiuser': {
        const multiple = field.type === 'multiuser';
        return (
            <div className={am('attr-people')}>
                {(multiple || !list.length) && !disabled && (
                    <PeopleSearch
                        teamId={teamId}
                        exclude={list}
                        placeholder={formatMessage({id: 'fusion.attributes.findPerson', defaultMessage: 'Find someone…'})}
                        onPick={(user) => onChange(multiple ? [...list, user.id] : user.id)}
                    />
                )}
                {list.length > 0 && (
                    <div className={am('role-people')}>
                        {list.map((id) => (
                            <span
                                key={id}
                                className={am('role-person')}
                            >
                                <Avatar
                                    userId={id}
                                    size='sm'
                                />
                                <PersonName userId={id}/>
                                {!disabled && (
                                    <button
                                        type='button'
                                        aria-label={formatMessage({id: 'fusion.attributes.removePerson', defaultMessage: 'Remove'})}
                                        onClick={() => onChange(multiple ? list.filter((other) => other !== id) : '')}
                                    >
                                        <Icon
                                            name='x'
                                            size='xs'
                                        />
                                    </button>
                                )}
                            </span>
                        ))}
                    </div>
                )}
            </div>
        );
    }
    default:
        return (
            <input
                type={INPUT_TYPES[field.attrs?.value_type || ''] || 'text'}
                value={list[0] || ''}
                maxLength={ATTRIBUTE_TEXT_MAX}
                disabled={disabled}
                aria-label={label}
                onChange={(e) => onChange(e.target.value)}
            />
        );
    }
}
