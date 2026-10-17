// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

// The arguments of the slash command being typed, each in its own block (as Mattermost's slash command autocomplete
// shows them). A command describes its arguments in one of two ways:
// - its AutocompleteData (the server's model.AutocompleteData, as JSON): subcommands, named arguments (--name value)
//   and positional ones, each with a hint and a help text;
// - or only its hint, e.g. "@[username]... #[channel]...": each bracketed or bare word is an argument, "..." one that
//   repeats, and a leading @, # or ~ the kind of word that goes in it.

export type AutocompleteArgJSON = {
    Name?: string;
    HelpText?: string;
    Type?: string;
    Required?: boolean;
    Data?: {Hint?: string; PossibleArguments?: Array<{Item: string; Hint?: string; HelpText?: string}>} | null;
};

export type AutocompleteDataJSON = {
    Trigger: string;
    Hint?: string;
    HelpText?: string;
    Arguments?: AutocompleteArgJSON[] | null;
    SubCommands?: AutocompleteDataJSON[] | null;
};

export type CommandSpec = {
    trigger: string;
    hint: string;
    description: string;
    data?: AutocompleteDataJSON | null;
};

export type ArgSpec = {

    // What the block says: "#channel", "message", "--name value"...
    label: string;

    // A named argument's name (typed --name).
    name?: string;

    // The sign words of this argument start with (@ people, # channels; ~ counts as #).
    sign?: '@' | '#';
    help?: string;
    optional: boolean;
    repeat: boolean;
};

export type ArgBlock = {spec: ArgSpec; value: string; current: boolean};

export type CommandLine = {

    // The command and the subcommands typed, e.g. ["remote", "create"].
    path: string[];
    blocks: ArgBlock[];

    // The help text of the argument being typed.
    help: string;
};

type Word = {text: string; end: number};

// splitWords splits what follows a command into words, "quoted words" staying one.
export function splitWords(text: string): Word[] {
    const words: Word[] = [];
    const re = /"[^"]*"?|\S+/g;
    let m: RegExpExecArray | null;
    while ((m = re.exec(text)) !== null) {
        words.push({text: m[0], end: m.index + m[0].length});
    }
    return words;
}

function signOf(label: string): ArgSpec['sign'] {
    if (label.startsWith('@')) {
        return '@';
    }
    if (label.startsWith('#') || label.startsWith('~')) {
        return '#';
    }
    return undefined;
}

// hintArgs reads a command's hint: "@[username]... #[channel]..." gives two repeating arguments, "@username" and
// "#channel"; "'message' [delay in seconds]" gives "message" and "delay in seconds".
export function hintArgs(hint: string): ArgSpec[] {
    const tokens = hint.match(/'[^']*'|"[^"]*"|[^\s[]*\[[^\]]*\][^\s]*|\S+/g) || [];
    return tokens.map((token) => {
        const repeat = (/(\.\.\.|…)$/).test(token);
        let label = token.replace(/(\.\.\.|…)$/, '').replace(/[[\]'"]/g, '');
        if (label.startsWith('~')) {
            label = '#' + label.slice(1);
        }
        return {label, sign: signOf(label), optional: false, repeat};
    }).filter((spec) => spec.label);
}

// dataArgs reads the arguments of an AutocompleteData.
export function dataArgs(args: AutocompleteArgJSON[]): ArgSpec[] {
    return args.map((arg) => {
        let value = arg.Data?.Hint || '';
        if (!value && arg.Data?.PossibleArguments?.length) {
            value = arg.Data.PossibleArguments.map((item) => item.Item).slice(0, 4).join(' | ');
        }
        value = value.replace(/[[\]]/g, '');
        if (value.startsWith('~')) {
            value = '#' + value.slice(1);
        }
        const label = arg.Name ? `--${arg.Name}${value ? ' ' + value : ''}` : value || arg.HelpText || '…';
        return {label, name: arg.Name || undefined, sign: arg.Name ? undefined : signOf(value), help: arg.HelpText || '', optional: !arg.Required, repeat: false};
    });
}

// The arguments of the command or subcommand words point at, and how many words the subcommands took.
function resolve(spec: CommandSpec, words: Word[]): {path: string[]; specs: ArgSpec[]; used: number} {
    const path = [spec.trigger];
    let data = spec.data;
    let used = 0;
    while (data && !data.Arguments?.length && data.SubCommands?.length) {
        const word = words[used]?.text.toLowerCase();
        const sub = data.SubCommands.find((s) => s.Trigger.toLowerCase() === word);

        // The subcommand being chosen: one block listing them.
        if (!sub || used === words.length) {
            const label = data.SubCommands.map((s) => s.Trigger).slice(0, 5).join(' | ');
            return {path, specs: [{label, help: data.HelpText || '', optional: false, repeat: false}], used};
        }
        path.push(sub.Trigger);
        data = sub;
        used++;
    }
    if (data?.Arguments?.length) {
        return {path, specs: dataArgs(data.Arguments), used};
    }
    return {path, specs: hintArgs(data?.Hint || spec.hint), used};
}

// parseCommandLine gives the blocks of the command being typed in pretext (the text before the caret): which value
// goes in which argument, and which argument is being typed. Null when pretext isn't a known command followed by a
// space.
export function parseCommandLine(pretext: string, commands: CommandSpec[]): CommandLine | null {
    const m = (/^\/(\S+)\s([\s\S]*)$/).exec(pretext);
    if (!m) {
        return null;
    }
    const spec = commands.find((c) => c.trigger.toLowerCase() === m[1].toLowerCase());
    if (!spec) {
        return null;
    }
    const rest = m[2];
    const allWords = splitWords(rest);
    const {path, specs, used} = resolve(spec, allWords);
    if (!specs.length) {
        return null;
    }
    const words = allWords.slice(used);
    const typingWord = words.length > 0 && words[words.length - 1].end === rest.length;
    const blocks: ArgBlock[] = specs.map((s) => ({spec: s, value: '', current: false}));
    const add = (index: number, text: string) => {
        blocks[index].value = blocks[index].value ? blocks[index].value + ' ' + text : text;
    };

    // Which block each word goes in: --name gives the next word to a named argument, a word with a sign (@, #) goes
    // to the argument of that sign, the others to the positional arguments in turn. A repeating argument, or the last
    // one (a message, say), takes all the words that follow.
    const positional = blocks.map((b, i) => (b.spec.name ? -1 : i)).filter((i) => i >= 0);
    let position = 0;
    let last = -1;
    let pendingName = -1;
    const advancePast = (index: number) => {
        const at = positional.indexOf(index);
        if (at >= position && !blocks[index].spec.repeat && at < positional.length - 1) {
            position = at + 1;
        } else if (at > position) {
            position = at;
        }
    };
    for (const word of words) {
        if (pendingName >= 0) {
            add(pendingName, word.text);
            last = pendingName;
            pendingName = -1;
            continue;
        }
        if (word.text.startsWith('--')) {
            const named = blocks.findIndex((b) => b.spec.name && ('--' + b.spec.name).startsWith(word.text));
            if (named >= 0) {
                last = named;
                if ('--' + blocks[named].spec.name === word.text) {
                    pendingName = named;
                }
                continue;
            }
        }
        const sign = signOf(word.text);
        const signed = sign ? positional.find((i) => blocks[i].spec.sign === sign) : undefined;
        const index = signed ?? (position < positional.length ? positional[position] : -1);
        if (index < 0) {
            continue;
        }
        add(index, word.text);
        last = index;
        advancePast(index);
    }
    const next = position < positional.length ? positional[position] : -1;
    let current = typingWord ? last : next;
    if (pendingName >= 0) {
        current = pendingName;
    }
    if (current >= 0) {
        blocks[current].current = true;
    }
    return {path, blocks, help: current >= 0 ? blocks[current].spec.help || '' : ''};
}
