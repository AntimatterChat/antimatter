// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useMemo} from 'react';
import {useIntl} from 'react-intl';

import type {AgentActivity} from '@mattermost/types/ai_agents';
import type {UserProfile} from '@mattermost/types/users';

import {REDUCED_MOTION_QUERY, useMediaQuery} from 'fusion/hooks/media';
import {am} from 'fusion/utils/class_names';

import {isWorking, needsSomeone, useAgentActivity} from './activity';
import {pickPlayfulWord} from './playful_words';

// How an agent's activity shows (Antimatter), as the user asked:
// - next to its name in the messages, only a small dot while it works: pulsing yellow while thinking or working, a
//   steady dot of the attention colour while it waits for someone or is blocked; never the text;
// - in the member list and the profile card, the same dot starts the status line, followed by what it's doing (or a
//   playful word when it says nothing, or "Waiting for you…");
// - idle, offline and the rest: the agent's plain status, as a person's.
// People who ask for reduced motion get no pulse.

// IntegrationTag is the label after a bot's name: AGENT (purple) for an agent, BOT otherwise.
export function IntegrationTag({agent}: {agent: boolean}) {
    const {formatMessage} = useIntl();
    if (agent) {
        return <span className={am('bot-tag', 'agent-tag')}>{formatMessage({id: 'fusion.agents.tag', defaultMessage: 'AGENT'})}</span>;
    }
    return <span className={am('bot-tag')}>{'BOT'}</span>;
}

function useDotClass(activity: AgentActivity | undefined): string | null {
    const reducedMotion = useMediaQuery(REDUCED_MOTION_QUERY);
    if (!activity) {
        return null;
    }
    if (isWorking(activity.state)) {
        return am('agent-dot', 'agent-busy', {'agent-pulse': !reducedMotion});
    }
    if (needsSomeone(activity.state)) {
        return am('agent-dot', 'agent-waiting');
    }
    return null;
}

function useStateLabel(activity: AgentActivity | undefined): string {
    const {formatMessage} = useIntl();
    switch (activity?.state) {
    case 'thinking':
        return formatMessage({id: 'fusion.agents.state.thinking', defaultMessage: 'Thinking'});
    case 'working':
        return formatMessage({id: 'fusion.agents.state.working', defaultMessage: 'Working'});
    case 'waiting':
        return formatMessage({id: 'fusion.agents.state.waiting', defaultMessage: 'Waiting for someone'});
    case 'blocked':
        return formatMessage({id: 'fusion.agents.state.blocked', defaultMessage: 'Blocked'});
    default:
        return '';
    }
}

// AgentNameDot is what the message header shows after an agent's name while it's busy: the dot alone.
export function AgentNameDot({user}: {user: UserProfile | undefined}) {
    const activity = useAgentActivity(user);
    const className = useDotClass(activity);
    const label = useStateLabel(activity);
    if (!className) {
        return null;
    }
    return (
        <span
            className={className}
            role='img'
            aria-label={label}
            title={label}
            data-testid='agent-dot'
        />
    );
}

// AgentActivityLine is the start of the status line in the member list and the profile card while an agent is busy:
// the dot, then what it's doing. It's nothing otherwise: the plain status shows.
export function AgentActivityLine({user, className}: {user: UserProfile | undefined; className?: string}) {
    const {formatMessage} = useIntl();
    const activity = useAgentActivity(user);
    const dot = useDotClass(activity);

    // A new word for each new activity, not at each render.
    const word = useMemo(() => pickPlayfulWord(), [activity?.update_at, activity?.state]); // eslint-disable-line react-hooks/exhaustive-deps
    if (!activity || !dot) {
        return null;
    }
    let text = activity.text;
    if (!text) {
        text = needsSomeone(activity.state) ? formatMessage({id: 'fusion.agents.waitingForYou', defaultMessage: 'Waiting for you…'}) : formatMessage(word);
    }
    return (
        <span
            className={am('agent-activity', {'agent-activity-waiting': needsSomeone(activity.state)}) + (className ? ` ${className}` : '')}
            data-testid='agent-activity'
        >
            <span
                className={dot}
                aria-hidden={true}
            />
            <span className={am('agent-activity-text')}>{text}</span>
        </span>
    );
}

// useAgentBusy tells whether an agent shows an activity line now (so the caller leaves its plain status out).
export function useAgentBusy(user: UserProfile | undefined): boolean {
    const activity = useAgentActivity(user);
    return Boolean(activity) && (isWorking(activity!.state) || needsSomeone(activity!.state));
}
