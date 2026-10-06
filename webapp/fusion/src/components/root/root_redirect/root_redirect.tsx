// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect} from 'react';
import {Redirect, useHistory, useLocation} from 'react-router-dom';

import type {ActionResult} from 'mattermost-redux/types/actions';

import * as GlobalActions from 'actions/global_actions';

export type Props = {
    isElegibleForFirstAdmingOnboarding: boolean;
    currentUserId: string;
    location?: Location;
    isFirstAdmin: boolean;
    areThereTeams: boolean;
    actions: {
        getFirstAdminSetupComplete: () => Promise<ActionResult>;
    };
};

export default function RootRedirect(props: Props) {
    const history = useHistory();
    const location = useLocation();

    useEffect(() => {
        if (props.currentUserId) {
            if (props.isElegibleForFirstAdmingOnboarding) {
                props.actions.getFirstAdminSetupComplete().then((firstAdminCompletedSignup) => {
                    // root.tsx ensures admin profiles are eventually loaded
                    if (firstAdminCompletedSignup.data === false && props.isFirstAdmin && !props.areThereTeams) {
                        history.push('/preparing-workspace');
                    } else {
                        GlobalActions.redirectUserToDefaultTeam(new URLSearchParams(location.search));
                    }
                });
            } else {
                GlobalActions.redirectUserToDefaultTeam(new URLSearchParams(location.search));
            }
        }
    }, [props.currentUserId, props.isElegibleForFirstAdmingOnboarding]);

    if (props.currentUserId) {
        // Ideally, this would be a Redirect like below, but since we need to call an action, this redirect is done above
        return null;
    }

    return (
        <Redirect
            to={{
                ...props.location,
                pathname: isInviteAfterLogout(location) ? '/signup_user_complete' : '/login',
            }}
        />
    );
}

// Logging out reloads the root of the site, keeping only the query string (see store/index.ts). When
// someone logs out to accept an invitation sent to another account (see Signup), the invitation's token
// is in that query string: take them back to it rather than to the login page.
function isInviteAfterLogout({pathname, search}: {pathname: string; search: string}) {
    return pathname === '/' && new URLSearchParams(search).has('t');
}
