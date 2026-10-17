// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useState} from 'react';
import {useIntl} from 'react-intl';

import type {Command} from '@mattermost/types/integrations';

import {Client4} from 'mattermost-redux/client';

import {am} from 'fusion/utils/class_names';

import {hintArgs, parseCommandLine} from './command_args';
import type {AutocompleteDataJSON, CommandSpec} from './command_args';

// The slash commands of each community, as the server lists them (with their AutocompleteData), once per page load.
const loaded = new Map<string, Promise<CommandSpec[]>>();

export function loadCommandSpecs(teamId: string): Promise<CommandSpec[]> {
    let specs = loaded.get(teamId);
    if (!specs) {
        specs = Client4.getCommandsList(teamId).then((commands) => commands.map((c: Command & {autocomplete_data?: AutocompleteDataJSON | null}) => ({
            trigger: c.trigger,
            hint: c.auto_complete_hint || '',
            description: c.auto_complete_desc || '',
            data: c.autocomplete_data,
        }))).catch(() => {
            loaded.delete(teamId);
            return [];
        });
        loaded.set(teamId, specs);
    }
    return specs;
}

export function useCommandSpecs(teamId: string, wanted: boolean): CommandSpec[] {
    const [specs, setSpecs] = useState<CommandSpec[]>([]);
    useEffect(() => {
        if (!wanted || !teamId) {
            return undefined;
        }
        let live = true;
        loadCommandSpecs(teamId).then((s) => live && setSpecs(s));
        return () => {
            live = false;
        };
    }, [teamId, wanted]);
    return specs;
}

// ArgChips draws a command's hint as its arguments' blocks, in a suggestion row.
export function ArgChips({hint}: {hint: string}) {
    const args = hintArgs(hint);
    if (!args.length) {
        return null;
    }
    return (
        <span className={am('arg-chips')}>
            {args.map((arg, i) => (
                <span
                    key={i}
                    className={am('arg')}
                >
                    {arg.label}
                    {arg.repeat ? '…' : ''}
                </span>
            ))}
        </span>
    );
}

// CommandArgsBar shows, while a slash command is typed, the command and each of its arguments as a block: what has
// been typed in it, and the one being typed highlighted with its help.
export default function CommandArgsBar({pretext, commands}: {pretext: string; commands: CommandSpec[]}) {
    const {formatMessage} = useIntl();
    const line = parseCommandLine(pretext, commands);
    if (!line) {
        return null;
    }
    return (
        <div
            className={am('cmd-args')}
            role='group'
            aria-label={formatMessage({id: 'fusion.ac.commandArgs', defaultMessage: 'Arguments of /{command}'}, {command: line.path.join(' ')})}
        >
            <span className={am('cmd-name')}>{'/' + line.path.join(' ')}</span>
            {line.blocks.map((block, i) => (
                <span
                    key={i}
                    className={am('arg', {on: block.current, filled: Boolean(block.value), optional: block.spec.optional})}
                    aria-current={block.current ? 'true' : undefined}
                >
                    <span className={am('arg-label')}>
                        {block.spec.label}
                        {block.spec.repeat ? '…' : ''}
                    </span>
                    {block.value && <span className={am('arg-value')}>{block.value}</span>}
                </span>
            ))}
            {line.help && <span className={am('arg-help')}>{line.help}</span>}
        </div>
    );
}
