// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useIntl} from 'react-intl';

import {am} from 'fusion/utils/class_names';

type Props = {
    children: React.ReactNode;

    // What to show instead of the children once one of them throws while drawing.
    fallback: React.ReactNode;

    // Drawing again is tried when this changes (a message edited, another page opened).
    resetKey?: unknown;
};

type State = {
    failed: boolean;
    key: unknown;
};

// FusionErrorBoundary keeps an error while drawing one part of the Fusion UI to that part: React would otherwise take
// the whole page down (a blank window) until the reload, and for a message, until it's deleted.
export default class FusionErrorBoundary extends React.PureComponent<Props, State> {
    constructor(props: Props) {
        super(props);
        this.state = {failed: false, key: props.resetKey};
    }

    static getDerivedStateFromError(): Partial<State> {
        return {failed: true};
    }

    static getDerivedStateFromProps(props: Props, state: State): Partial<State> | null {
        return props.resetKey === state.key ? null : {failed: false, key: props.resetKey};
    }

    render() {
        return this.state.failed ? this.props.fallback : this.props.children;
    }
}

// MessageError stands in for a message that couldn't be drawn: one muted line in its place.
export function MessageError({postId}: {postId: string}) {
    const {formatMessage} = useIntl();
    return (
        <div
            className={am('msg', 'render-error')}
            id={`post_${postId}`}
            role='note'
        >
            {formatMessage({id: 'fusion.error.message', defaultMessage: 'This message can\'t be shown.'})}
        </div>
    );
}

// PageError stands in for the whole Fusion UI when it couldn't be drawn, with a way back in.
export function PageError() {
    const {formatMessage} = useIntl();
    return (
        <div
            className={am('crash')}
            role='alert'
        >
            <div className={am('crash-card')}>
                <h1>{formatMessage({id: 'fusion.error.page', defaultMessage: 'Something went wrong.'})}</h1>
                <p>{formatMessage({id: 'fusion.error.pageHint', defaultMessage: 'This page couldn\'t be shown. Reloading usually fixes it.'})}</p>
                <button
                    type='button'
                    className={am('crash-reload')}
                    onClick={() => window.location.reload()}
                >
                    {formatMessage({id: 'fusion.error.reload', defaultMessage: 'Reload'})}
                </button>
            </div>
        </div>
    );
}
