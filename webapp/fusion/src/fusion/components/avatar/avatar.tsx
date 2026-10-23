// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useState} from 'react';

import {useFocusLabel, useFocusUntil} from 'fusion/focus/focus';
import {useDisplayName, useUser, useUserStatus, initials} from 'fusion/hooks/users';
import {avatarURLIn, useCommunityProfile} from 'fusion/profiles/community_profiles';
import {am} from 'fusion/utils/class_names';

export type AvatarSize = '' | 'sm' | 'xs' | 'lg' | 'xl' | 'rail' | 'me';

type Props = {
    userId: string;
    size?: AvatarSize;
    status?: boolean;
    speaking?: boolean;
    className?: string;

    /** The community whose picture of theirs shows ('' for the global one); by default, the community scope. */
    teamId?: string;
};

// A stable hue per user, for the initials shown until (or instead of) the profile picture.
export function hueFor(id: string) {
    let h = 0;
    for (let i = 0; i < id.length; i++) {
        h = ((h * 31) + id.charCodeAt(i)) % 360;
    }
    return h;
}

// Avatar shows a user's profile picture (in a community, theirs for it when they set one), with their status dot when
// asked for: purple while they focus (focus mode), with its end in its tooltip.
export default function Avatar({userId, size = '', status = false, speaking = false, className, teamId}: Props) {
    const user = useUser(userId);
    const name = useDisplayName(user, teamId);
    const [profile, scope] = useCommunityProfile(userId, teamId);
    const userStatus = useUserStatus(status ? userId : undefined);
    const focusLabel = useFocusLabel(useFocusUntil(status ? userId : undefined));
    const [failed, setFailed] = useState(false);

    return (
        <span
            className={am('av', size, {speaking}) + (className ? ' ' + className : '')}
            style={{'--am-h': hueFor(userId)} as React.CSSProperties}
            data-name={name}
            aria-hidden='true'
        >
            {user && !failed ? (
                <img
                    src={avatarURLIn(user, profile, scope)}
                    alt=''
                    loading='lazy'
                    onError={() => setFailed(true)}
                />
            ) : initials(name)}
            {status && (
                <span
                    className={am('st', focusLabel ? 'focus' : userStatus)}
                    data-am-tip={focusLabel || undefined}
                />
            )}
        </span>
    );
}
