// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useState} from 'react';
import {useSelector} from 'react-redux';

import type {UserProfile} from '@mattermost/types/users';

import {Client4} from 'mattermost-redux/client';
import {getUser} from 'mattermost-redux/selectors/entities/users';

import Avatar from 'fusion/components/avatar';
import Icon from 'fusion/components/icon';
import {useDisplayName} from 'fusion/hooks/users';
import {am} from 'fusion/utils/class_names';

import type {GlobalState} from 'types/store';

type Props = {

    // The team whose people are searched.
    teamId: string;

    // People not to offer (those already there).
    exclude: string[];
    placeholder: string;
    disabled?: boolean;
    onPick: (user: UserProfile) => void;
};

// PeopleSearch finds people of a team as they're typed, to pick one.
export default function PeopleSearch({teamId, exclude, placeholder, disabled = false, onPick}: Props) {
    const [query, setQuery] = useState('');
    const [found, setFound] = useState<UserProfile[]>([]);

    useEffect(() => {
        const term = query.trim();
        if (!term) {
            setFound([]);
            return undefined;
        }
        let stale = false;
        const timer = setTimeout(async () => {
            try {
                const result = await Client4.autocompleteUsers(term, teamId, '', {limit: 8});
                if (!stale) {
                    setFound(result.users.filter((u) => !exclude.includes(u.id) && !u.delete_at));
                }
            } catch {
                // Nothing found then.
            }
        }, 200);
        return () => {
            stale = true;
            clearTimeout(timer);
        };
    }, [query, teamId, exclude]);

    return (
        <>
            <input
                type='text'
                value={query}
                disabled={disabled}
                placeholder={placeholder}
                aria-label={placeholder}
                onChange={(e) => setQuery(e.target.value)}
            />
            {found.length > 0 && (
                <div className={am('role-found')}>
                    {found.map((u) => (
                        <button
                            key={u.id}
                            type='button'
                            onClick={() => {
                                setQuery('');
                                onPick(u);
                            }}
                        >
                            <Avatar
                                userId={u.id}
                                size='sm'
                            />
                            <FoundName user={u}/>
                            <Icon
                                name='plus'
                                size='sm'
                            />
                        </button>
                    ))}
                </div>
            )}
        </>
    );
}

// Someone found may not be among the profiles loaded yet: their name comes from the search.
function FoundName({user}: {user: UserProfile}) {
    return <span className={am('nm')}>{useDisplayName(user)}</span>;
}

// PersonName is someone's name as the reader chose to see names, from the profiles loaded.
export function PersonName({userId}: {userId: string}) {
    const user = useSelector((state: GlobalState) => getUser(state, userId));
    return <span className={am('nm')}>{useDisplayName(user)}</span>;
}
