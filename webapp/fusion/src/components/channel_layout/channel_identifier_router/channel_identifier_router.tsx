// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {History} from 'history';
import React from 'react';
import type {match} from 'react-router-dom';

import FusionChannelView from 'fusion/channel/channel_view';
import type {PermalinkAnchorState} from 'fusion/utils/paths';
import {getHistory} from 'utils/browser_history';
import Constants from 'utils/constants';

export type Match = match<{
    identifier: string;
    team: string;
    postid?: string;
    path: string;
}>;

export type MatchAndHistory = Pick<Props, 'match' | 'history'>;

type Props = {
    match: Match;
    actions: {
        onChannelByIdentifierEnter: (props: MatchAndHistory) => void;
        focusPost: (postId: string, returnTo: string, currentUserId: string) => void;
    };
    history: History;

    // Where a permalink came from, for its error page.
    returnTo?: string;
    currentUserId?: string;
};

// A permalink's address has a post and no conversation.
function isPermalink(props: Props): boolean {
    return !props.match.params.identifier && Boolean(props.match.params.postid);
}

// ChannelIdentifierRouter opens the conversation of /<team>/channels/<name>[/<post id>], and resolves a permalink
// (/<team>/pl/<post id>) to it. It is one component for both addresses, so the Fusion conversation view (header,
// messages, composer) stays in place while a permalink resolves: the current conversation shows until the linked
// one is known, as when switching channels, instead of an empty page.
export default class ChannelIdentifierRouter extends React.PureComponent<Props> {
    constructor(props: Props) {
        super(props);

        this.state = {
            prevProps: props,
        };
    }

    private replaceUrlTimeout!: NodeJS.Timeout;

    componentDidUpdate(prevProps: Props) {
        if (this.isPermalink()) {
            if (!isPermalink(prevProps) || this.props.match.params.postid !== prevProps.match.params.postid) {
                // A highlight still fading in the conversation shown mustn't send it back there.
                clearTimeout(this.replaceUrlTimeout);
                this.focusPermalink();
            }
            return;
        }
        if (this.props.match.params.team !== prevProps.match.params.team ||
            this.props.match.params.identifier !== prevProps.match.params.identifier) {
            clearTimeout(this.replaceUrlTimeout);
            this.props.actions.onChannelByIdentifierEnter(this.props);
            this.replaceUrlIfPermalink();
        } else if (prevProps.match.params.postid && !this.props.match.params.postid) {
            // The reader left the message (Jump to present, or a message sent): its highlight's end mustn't bring it
            // back.
            clearTimeout(this.replaceUrlTimeout);
        }
    }
    componentDidMount() {
        if (this.isPermalink()) {
            this.focusPermalink();
            return;
        }
        this.props.actions.onChannelByIdentifierEnter(this.props);
        this.replaceUrlIfPermalink();
    }

    componentWillUnmount() {
        clearTimeout(this.replaceUrlTimeout);
    }

    isPermalink = () => isPermalink(this.props);

    // focusPost finds the post's conversation (joining it, or asking to), selects it and replaces the address with
    // the conversation's, with the post in it.
    focusPermalink = () => {
        this.props.actions.focusPost(this.props.match.params.postid!, this.props.returnTo || '', this.props.currentUserId || '');
    };

    // A few seconds after a message opens from a link, the address drops it; the list stays around it (the
    // address's state keeps it), without its highlight.
    replaceUrlIfPermalink = () => {
        const postId = this.props.match.params.postid;
        if (postId) {
            this.replaceUrlTimeout = setTimeout(() => {
                const channelUrl = this.props.match.url.split('/').slice(0, -1).join('/');
                const state: PermalinkAnchorState = {permalinkAnchor: postId};
                getHistory().replace(channelUrl, state);
            }, Constants.PERMALINK_FADEOUT);
        }
    };

    render() {
        return <FusionChannelView resolvingPermalink={this.isPermalink()}/>;
    }
}
