// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {lazy} from 'react';
import {Route, Switch, Redirect} from 'react-router-dom';

import {makeAsyncComponent} from 'components/async_load';
import ChannelIdentifierRouter from 'components/channel_layout/channel_identifier_router';
import LoadingScreen from 'components/loading_screen';

import ForumTopicRoute from 'fusion/forum/topic_route';
import {SCHEDULED_POST_URL_SUFFIX} from 'utils/constants';
import {IDENTIFIER_PATH_PATTERN, ID_PATH_PATTERN, TEAM_NAME_PATH_PATTERN} from 'utils/path';

import type {OwnProps, PropsFromRedux} from './index';

const GlobalThreads = makeAsyncComponent('GlobalThreads', lazy(() => import('components/threading/global_threads')),
    (
        <div className='app__content'>
            <LoadingScreen/>
        </div>
    ),
);
const Drafts = makeAsyncComponent('Drafts', lazy(() => import('components/drafts')),
    (
        <div className='app__content'>
            <LoadingScreen/>
        </div>
    ),
);
const Recaps = makeAsyncComponent('Recaps', lazy(() => import('components/recaps')),
    (
        <div className='app__content'>
            <LoadingScreen/>
        </div>
    ),
);
const Unreads = makeAsyncComponent('Unreads', lazy(() => import('fusion/unreads/unreads_view')),
    (
        <div className='app__content'>
            <LoadingScreen/>
        </div>
    ),
);
const PlaybookRunner = makeAsyncComponent('PlaybookRunner', lazy(() => import('components/channel_layout/playbook_runner')));

type Props = PropsFromRedux & OwnProps;

type State = {
    returnTo: string;
    lastReturnTo: string;
};

export default class CenterChannel extends React.PureComponent<Props, State> {
    constructor(props: Props) {
        super(props);
        this.state = {
            returnTo: '',
            lastReturnTo: '',
        };
    }

    static getDerivedStateFromProps(nextProps: Props, prevState: State) {
        if (prevState.lastReturnTo !== nextProps.location.pathname && nextProps.location.pathname.includes('/pl/')) {
            return {
                lastReturnTo: nextProps.location.pathname,
                returnTo: prevState.lastReturnTo,
            };
        }
        return {lastReturnTo: nextProps.location.pathname};
    }

    async componentDidMount() {
        const {actions} = this.props;
        await actions.getProfiles();
    }

    render() {
        const {lastChannelPath, isCollapsedThreadsEnabled} = this.props;
        const url = this.props.match.url;

        // The Fusion UI's main column (see FusionApp) holds the route's view directly.
        return (
            <div className='am-center'>
                <Switch>
                    {/* A forum topic's own page (Antimatter channel types, design §4.4), ahead of the plain channel
                        route below so a topic's address doesn't fall into it as a highlighted post. */}
                    <Route
                        path={`/:team(${TEAM_NAME_PATH_PATTERN})/channels/:identifier(${IDENTIFIER_PATH_PATTERN})/topics/:rootId(${ID_PATH_PATTERN})`}
                        component={ForumTopicRoute}
                    />
                    {/* A permalink and a conversation are one route, so that the conversation view stays mounted
                        while a permalink resolves to its conversation: no empty page between the two. */}
                    <Route
                        path={[
                            `${url}/pl/:postid(${ID_PATH_PATTERN})`,
                            `/:team(${TEAM_NAME_PATH_PATTERN})/:path(channels|messages)/:identifier(${IDENTIFIER_PATH_PATTERN})/:postid(${ID_PATH_PATTERN})?`,
                        ]}
                        render={() => <ChannelIdentifierRouter returnTo={this.state.returnTo}/>}
                    />
                    <Route
                        path={`/:team(${TEAM_NAME_PATH_PATTERN})/_playbooks/:playbookId(${ID_PATH_PATTERN})/run`}
                    >
                        <PlaybookRunner/>
                    </Route>
                    {isCollapsedThreadsEnabled ? (
                        <Route
                            path={`/:team(${TEAM_NAME_PATH_PATTERN})/threads/:threadIdentifier(${ID_PATH_PATTERN})?`}
                            component={GlobalThreads}
                        />
                    ) : null}
                    <Route
                        path={`/:team(${TEAM_NAME_PATH_PATTERN})/recaps`}
                        component={Recaps}
                    />
                    <Route
                        path={`/:team(${TEAM_NAME_PATH_PATTERN})/drafts`}
                        component={Drafts}
                    />
                    <Route
                        path={`/:team(${TEAM_NAME_PATH_PATTERN})/${SCHEDULED_POST_URL_SUFFIX}`}
                        component={Drafts}
                    />
                    <Route
                        path={`/:team(${TEAM_NAME_PATH_PATTERN})/unreads`}
                        component={Unreads}
                    />

                    <Redirect to={lastChannelPath}/>
                </Switch>
            </div>
        );
    }
}
