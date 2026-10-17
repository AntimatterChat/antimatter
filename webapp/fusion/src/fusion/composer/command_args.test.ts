// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {hintArgs, parseCommandLine, splitWords} from './command_args';
import type {CommandSpec} from './command_args';

const commands: CommandSpec[] = [
    {trigger: 'invite', hint: '@[username]... #[channel]...', description: 'Invite people'},
    {trigger: 'join', hint: '~[channel]', description: 'Join a channel'},
    {trigger: 'shrug', hint: '[message]', description: 'Shrug'},
    {trigger: 'echo', hint: '\'message\' [delay in seconds]', description: 'Echo'},
    {
        trigger: 'remote',
        hint: '[action]',
        description: 'Secure connections',
        data: {
            Trigger: 'remote',
            HelpText: 'Manage secure connections',
            SubCommands: [
                {Trigger: 'create', Arguments: [
                    {Name: 'name', HelpText: 'The connection\'s name', Type: 'TextInput', Required: true, Data: {Hint: '[name]'}},
                    {Name: 'password', HelpText: 'Its password', Type: 'TextInput', Required: false, Data: {Hint: '[password]'}},
                ]},
                {Trigger: 'remove', Arguments: [{HelpText: 'The connection', Type: 'StaticList', Required: true, Data: {PossibleArguments: [{Item: 'lab'}, {Item: 'hq'}]}}]},
            ],
        },
    },
];

const blocks = (pretext: string) => parseCommandLine(pretext, commands)?.blocks.map((b) => [b.spec.label, b.value, b.current]);

describe('fusion/composer/command_args', () => {
    test('a hint gives the arguments, # for channels, … for the ones that repeat', () => {
        expect(hintArgs('@[username]... #[channel]...')).toEqual([
            {label: '@username', sign: '@', optional: false, repeat: true},
            {label: '#channel', sign: '#', optional: false, repeat: true},
        ]);
        expect(hintArgs('~[channel]').map((a) => a.label)).toEqual(['#channel']);
        expect(hintArgs('\'message\' [delay in seconds]').map((a) => a.label)).toEqual(['message', 'delay in seconds']);
        expect(hintArgs('')).toEqual([]);
    });

    test('quoted words stay one word', () => {
        expect(splitWords('"a b" c').map((w) => w.text)).toEqual(['"a b"', 'c']);
    });

    test('only for a known command followed by a space', () => {
        expect(parseCommandLine('/invite', commands)).toBeNull();
        expect(parseCommandLine('/nothing here', commands)).toBeNull();
        expect(parseCommandLine('hello /invite x', commands)).toBeNull();
    });

    test('people and channels go to their arguments, whatever their order', () => {
        expect(blocks('/invite ')).toEqual([['@username', '', true], ['#channel', '', false]]);
        expect(blocks('/invite @bob ')).toEqual([['@username', '@bob', true], ['#channel', '', false]]);
        expect(blocks('/invite @bob @maya #beam')).toEqual([['@username', '@bob @maya', false], ['#channel', '#beam', true]]);
        expect(blocks('/invite #beam @bob')).toEqual([['@username', '@bob', true], ['#channel', '#beam', false]]);
    });

    test('~channel and #channel fill the same argument', () => {
        expect(blocks('/join ~town')).toEqual([['#channel', '~town', true]]);
        expect(blocks('/join #town')).toEqual([['#channel', '#town', true]]);
    });

    test('the last argument takes the rest of the line', () => {
        expect(blocks('/shrug well, nothing to do')).toEqual([['message', 'well, nothing to do', true]]);
        expect(blocks('/echo "hello there" 5')).toEqual([['message', '"hello there"', false], ['delay in seconds', '5', true]]);
        expect(blocks('/echo "hello there" ')).toEqual([['message', '"hello there"', false], ['delay in seconds', '', true]]);
    });

    test('subcommands, then named arguments, with the help of the one being typed', () => {
        expect(blocks('/remote ')).toEqual([['create | remove', '', true]]);
        expect(blocks('/remote cr')).toEqual([['create | remove', 'cr', true]]);

        const line = parseCommandLine('/remote create --name lab --pass', commands);
        expect(line?.path).toEqual(['remote', 'create']);
        expect(line?.blocks.map((b) => [b.spec.label, b.value, b.current, b.spec.optional])).toEqual([
            ['--name name', 'lab', false, false],
            ['--password password', '', true, true],
        ]);
        expect(line?.help).toBe('Its password');

        expect(parseCommandLine('/remote create --name ', commands)?.help).toBe('The connection\'s name');
        expect(blocks('/remote remove ')).toEqual([['lab | hq', '', true]]);
    });
});
