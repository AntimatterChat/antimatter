// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {lazy} from 'react';
import type {RouteComponentProps} from 'react-router-dom';
import {Route} from 'react-router-dom';

import {makeAsyncComponent} from 'components/async_load';
import LoggedIn from 'components/logged_in';

const OnBoardingTaskList = makeAsyncComponent('OnboardingTaskList', lazy(() => import('components/onboarding_tasklist')));

type Props = {
    component: React.ComponentType<RouteComponentProps<any>>;
    path: string | string[];
};

export default function LoggedInRoute(props: Props) {
    const {component: Component, ...rest} = props;

    return (
        <Route
            {...rest}
            render={(routeProps) => (
                <LoggedIn {...routeProps}>
                    <OnBoardingTaskList/>
                    <Component {...(routeProps)}/>
                </LoggedIn>
            )}
        />
    );
}
