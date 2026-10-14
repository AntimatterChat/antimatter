// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useMemo, useRef, useState} from 'react';
import {useIntl} from 'react-intl';

import type {UserAttributeValue} from '@mattermost/types/properties_user';
import type {UserProfile} from '@mattermost/types/users';

import Icon from 'fusion/components/icon';
import {Popover} from 'fusion/components/layer';
import {MenuItem, MenuSeparator} from 'fusion/components/menu';
import {am} from 'fusion/utils/class_names';

import {attributeValueText} from './attribute_inputs';
import {listedAttribute, useCommunityAttributeValues, useCommunityAttributes} from './community_attributes';

export type MemberListAttribute = {

    // The listed attribute's value of someone, as text ('' when none).
    textOf: (userId: string) => string;

    // Whether someone is shown under the filter chosen.
    shown: (userId: string) => boolean;

    // The filter button, when the attribute can filter the list.
    filter: React.ReactNode;
};

const asList = (value: UserAttributeValue | undefined) => (Array.isArray(value) ? value : [value].filter(Boolean) as string[]);

// useMemberListAttribute is the community attribute its admins chose to show in the member list (as Company): each
// member's value, shown under their name, and a filter of the list by it. Choices filter by option; a text filters by
// the values the members listed hold.
export function useMemberListAttribute(teamId: string | undefined, profiles: UserProfile[]): MemberListAttribute {
    const intl = useIntl();
    const field = listedAttribute(useCommunityAttributes(teamId));
    const ids = useMemo(() => profiles.map((p) => p.id), [profiles]);
    const values = useCommunityAttributeValues(field ? teamId : undefined, ids);
    const [chosen, setChosen] = useState<string | null>(null);
    const [open, setOpen] = useState(false);
    const button = useRef<HTMLButtonElement>(null);

    const people = field?.type === 'user' || field?.type === 'multiuser';
    const textOf = (userId: string) => (field && !people ? attributeValueText(intl, field, values[userId]?.[field.id]) : '');

    // What can be filtered by: the choices, or the texts held.
    let choices: Array<[string, string]> = [];
    if (field?.type === 'select' || field?.type === 'multiselect') {
        choices = (field.attrs?.options || []).map((option) => [option.id, option.name]);
    } else if (field && (field.type === 'text' || field.type === 'date')) {
        const held = new Set(ids.map((id) => asList(values[id]?.[field.id])[0]).filter(Boolean));
        choices = [...held].sort().map((value) => [value, attributeValueText(intl, field, value)]);
    }
    const active = chosen !== null && choices.some(([value]) => value === chosen);
    const shown = (userId: string) => !active || !field || asList(values[userId]?.[field.id]).includes(chosen!);

    let filter: React.ReactNode = null;
    if (field && choices.length > 1) {
        const label = intl.formatMessage({id: 'fusion.members.filterBy', defaultMessage: 'Filter by {attribute}'}, {attribute: field.name});
        filter = (
            <>
                <button
                    ref={button}
                    type='button'
                    className={am('icon-btn', 'mem-add', {on: active})}
                    title={active ? `${label}: ${choices.find(([value]) => value === chosen)?.[1]}` : label}
                    aria-label={label}
                    aria-haspopup='menu'
                    aria-expanded={open}
                    onClick={() => setOpen(!open)}
                >
                    <Icon
                        name='tag'
                        size='sm'
                    />
                </button>
                {open && (
                    <Popover
                        anchor={button.current}
                        placement='below'
                        className='plus-pop'
                        role='menu'
                        label={label}
                        onClose={() => setOpen(false)}
                    >
                        <MenuItem
                            role='menuitemradio'
                            checked={!active}
                            label={intl.formatMessage({id: 'fusion.members.filterAll', defaultMessage: 'Everyone'})}
                            onClick={() => {
                                setChosen(null);
                                setOpen(false);
                            }}
                        />
                        <MenuSeparator/>
                        {choices.map(([value, text]) => (
                            <MenuItem
                                key={value}
                                role='menuitemradio'
                                checked={active && chosen === value}
                                label={text}
                                onClick={() => {
                                    setChosen(value);
                                    setOpen(false);
                                }}
                            />
                        ))}
                    </Popover>
                )}
            </>
        );
    }
    return {textOf, shown, filter};
}
