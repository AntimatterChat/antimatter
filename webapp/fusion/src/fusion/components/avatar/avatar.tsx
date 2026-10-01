// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useState} from 'react';

import {useDisplayName, useUser, useUserStatus, initials} from 'fusion/hooks/users';
import {am} from 'fusion/utils/class_names';
import {imageURLForUser} from 'utils/utils';

export type AvatarSize = '' | 'sm' | 'xs' | 'lg' | 'xl' | 'rail' | 'me';

type Props = {
    userId: string;
    size?: AvatarSize;
    status?: boolean;
    speaking?: boolean;
    className?: string;
};

// A stable hue per user, for the initials shown until (or instead of) the profile picture.
export function hueFor(id: string) {
    let h = 0;
    for (let i = 0; i < id.length; i++) {
        h = ((h * 31) + id.charCodeAt(i)) % 360;
    }
    return h;
}

// Avatar shows a user's profile picture, with their status dot when asked for.
export default function Avatar({userId, size = '', status = false, speaking = false, className}: Props) {
    const user = useUser(userId);
    const name = useDisplayName(user);
    const userStatus = useUserStatus(status ? userId : undefined);
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
                    src={imageURLForUser(user.id, user.last_picture_update)}
                    alt=''
                    loading='lazy'
                    onError={() => setFailed(true)}
                />
            ) : initials(name)}
            {status && <span className={am('st', userStatus)}/>}
        </span>
    );
}
