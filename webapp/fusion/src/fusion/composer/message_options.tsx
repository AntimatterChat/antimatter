// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useRef} from 'react';
import {useIntl} from 'react-intl';
import {useSelector} from 'react-redux';

import type {PostPriority} from '@mattermost/types/posts';

import {Posts} from 'mattermost-redux/constants';
import {getConfig} from 'mattermost-redux/selectors/entities/general';
import {isPostPriorityEnabled} from 'mattermost-redux/selectors/entities/posts';

import {isBurnOnReadEnabled} from 'selectors/burn_on_read';

import {useCreateBurnOnReadAccess} from 'components/common/hooks/useCreateBurnOnReadAccess';

import Icon from 'fusion/components/icon';
import type {IconName} from 'fusion/components/icon';
import {Popover} from 'fusion/components/layer';
import {MenuHeading, MenuSeparator} from 'fusion/components/menu';
import {isSpoiler, SPOILER_PROP} from 'fusion/messages/spoiler';
import {am} from 'fusion/utils/class_names';

import type {GlobalState} from 'types/store';
import type {PostDraft} from 'types/store/draft';

import {BurnDurationSlider, burnLimitSeconds, clampBurnSeconds, defaultBurnSeconds, formatBurnDuration, readDurationSeconds, withReadDuration} from './burn_duration';

// A message's options, beside what it says: its priority (important or urgent, with an acknowledgement asked and
// persistent notifications), burning on read (and how long after), and being a spoiler. The composer has one entry
// point for them (its options button), which opens one popover with them all, and shows those set as chips in its top
// row: each chip says its state, opens the popover where it is changed, and has its own remove button.

export type PriorityState = {priority: PostPriority | ''; requested_ack: boolean; persistent_notifications: boolean};

export type OptionSection = 'priority' | 'burn' | 'spoiler';

export type MessageOptions = {

    // What the conversation allows: priority on messages to the channel (not replies), burning where the server and
    // the channel's policies allow it. A spoiler is always allowed.
    canPriority: boolean;
    canBurn: boolean;
    priority: PriorityState;
    burn: boolean;

    // How long it lives once read: picked (burnPicked), else the configured duration; within burnLimit.
    burnSeconds: number;
    burnPicked: boolean;
    burnLimit: number;
    spoiler: boolean;
    persistentMinutes: number;

    // Whether any option is set.
    active: boolean;
    setPriority: (patch: Partial<PriorityState>) => void;
    setBurn: (on: boolean) => void;
    setBurnSeconds: (seconds: number) => void;
    setSpoiler: (on: boolean) => void;
};

// useMessageOptions reads and sets the options of a draft, through the composer's change (which saves the draft).
export function useMessageOptions(draft: PostDraft, change: (patch: Partial<PostDraft>) => void, channelId: string, rootId: string): MessageOptions {
    const priorityEnabled = useSelector(isPostPriorityEnabled);
    const burnEnabledInConfig = useSelector(isBurnOnReadEnabled);

    // Permission policies can also forbid burn-on-read messages in a channel (the server re-checks).
    const burnAllowedByPolicy = useCreateBurnOnReadAccess(channelId);
    const config = useSelector(getConfig);
    const persistentMinutes = parseInt(config.PersistentNotificationIntervalMinutes || '5', 10) || 5;
    const burnLimit = burnLimitSeconds(config);

    const current = draft.metadata?.priority;
    const priority: PriorityState = {
        priority: (current?.priority || '') as PostPriority | '',
        requested_ack: Boolean(current?.requested_ack),
        persistent_notifications: Boolean(current?.persistent_notifications),
    };
    const canPriority = priorityEnabled && !rootId;
    const canBurn = burnEnabledInConfig && burnAllowedByPolicy;
    const burn = draft.type === Posts.POST_TYPES.BURN_ON_READ;
    const picked = readDurationSeconds(draft.props);
    const spoiler = isSpoiler(draft);

    const setPriority = (patch: Partial<PriorityState>) => {
        const next = {...priority, ...patch};
        if (next.priority !== 'urgent') {
            next.persistent_notifications = false;
        }
        change({metadata: {...draft.metadata, priority: next}});
    };

    return {
        canPriority,
        canBurn,
        priority,
        burn,
        burnSeconds: picked === null ? defaultBurnSeconds(config, burnLimit) : clampBurnSeconds(picked, burnLimit),
        burnPicked: picked !== null,
        burnLimit,
        spoiler,
        persistentMinutes,
        active: Boolean(priority.priority || priority.requested_ack) || burn || spoiler,
        setPriority,

        // Off, the message keeps no duration of its own.
        setBurn: (on) => change({type: on ? Posts.POST_TYPES.BURN_ON_READ : undefined, props: on ? draft.props : withReadDuration(draft.props, null)}),
        setBurnSeconds: (seconds) => change({props: withReadDuration(draft.props, clampBurnSeconds(seconds, burnLimit))}),
        setSpoiler: (on) => {
            const props = {...draft.props};
            if (on) {
                props[SPOILER_PROP] = true;
            } else {
                delete props[SPOILER_PROP];
            }
            change({props});
        },
    };
}

type ChipProps = {
    kind: string;
    icon: IconName;
    label: string;
    removeLabel: string;
    onEdit: () => void;
    onRemove: () => void;
};

// OptionChip is a set option in the composer's top row: its state, which opens the options where it's changed, and its
// remove button.
function OptionChip({kind, icon, label, removeLabel, onEdit, onRemove}: ChipProps) {
    const {formatMessage} = useIntl();
    return (
        <span className={am('opt-chip', 'opt-toggle', kind)}>
            <button
                type='button'
                className={am('opt-chip-main')}
                title={formatMessage({id: 'fusion.composer.options', defaultMessage: 'Message options'})}
                aria-haspopup='dialog'
                onClick={onEdit}
            >
                <Icon name={icon}/>
                {label}
            </button>
            <button
                type='button'
                className={am('opt-chip-x')}
                title={removeLabel}
                aria-label={removeLabel}
                onClick={onRemove}
            >
                <Icon name='x'/>
            </button>
        </span>
    );
}

// MessageOptionChips are the chips of the options set, in the order of the popover.
export function MessageOptionChips({options, onEdit}: {options: MessageOptions; onEdit: (section: OptionSection) => void}) {
    const intl = useIntl();
    const {formatMessage} = intl;
    const {priority, setPriority} = options;
    const chips: React.ReactNode[] = [];

    if (priority.priority) {
        const parts = [priority.priority === 'urgent' ? formatMessage({id: 'fusion.composer.urgent', defaultMessage: 'Urgent'}) : formatMessage({id: 'fusion.composer.important', defaultMessage: 'Important'})];
        if (priority.requested_ack) {
            parts.push(formatMessage({id: 'fusion.composer.ackShort', defaultMessage: 'acknowledgement'}));
        }
        if (priority.persistent_notifications) {
            parts.push(formatMessage({id: 'fusion.composer.persistentShort', defaultMessage: 'persistent'}));
        }
        chips.push(
            <OptionChip
                key='priority'
                kind={priority.priority}
                icon='flag'
                label={parts.join(' · ')}
                removeLabel={formatMessage({id: 'fusion.composer.removePriority', defaultMessage: 'Remove priority'})}
                onEdit={() => onEdit('priority')}
                onRemove={() => setPriority({priority: '', requested_ack: false})}
            />,
        );
    } else if (priority.requested_ack) {
        chips.push(
            <OptionChip
                key='ack'
                kind='ack'
                icon='check'
                label={formatMessage({id: 'fusion.composer.ackRequested', defaultMessage: 'Acknowledgement requested'})}
                removeLabel={formatMessage({id: 'fusion.composer.removeAck', defaultMessage: 'Remove acknowledgement'})}
                onEdit={() => onEdit('priority')}
                onRemove={() => setPriority({requested_ack: false})}
            />,
        );
    }
    if (options.burn) {
        chips.push(
            <OptionChip
                key='burn'
                kind='burn'
                icon='flame'
                label={formatMessage({id: 'fusion.composer.burnsAfter', defaultMessage: 'Burns {duration} after reading'}, {duration: formatBurnDuration(options.burnSeconds, intl, 'short')})}
                removeLabel={formatMessage({id: 'fusion.composer.noBurn', defaultMessage: 'Turn off burn on read'})}
                onEdit={() => onEdit('burn')}
                onRemove={() => options.setBurn(false)}
            />,
        );
    }
    if (options.spoiler) {
        chips.push(
            <OptionChip
                key='spoiler'
                kind='spoiler'
                icon='eye-off'
                label={formatMessage({id: 'fusion.composer.spoilerOn', defaultMessage: 'Spoiler'})}
                removeLabel={formatMessage({id: 'fusion.composer.noSpoiler', defaultMessage: 'Don\'t send as a spoiler'})}
                onEdit={() => onEdit('spoiler')}
                onRemove={() => options.setSpoiler(false)}
            />,
        );
    }
    return <>{chips}</>;
}

type SwitchRowProps = {
    section?: OptionSection;
    title: string;
    desc: string;
    on: boolean;
    disabled?: boolean;
    kind?: string;
    onChange: (on: boolean) => void;
};

function SwitchRow({section, title, desc, on, disabled = false, kind, onChange}: SwitchRowProps) {
    return (
        <div className={am('pp-row', {disabled})}>
            <span>
                {title}
                <small>{desc}</small>
            </span>
            <button
                type='button'
                className={am('switch', kind)}
                role='switch'
                data-opt={section}
                aria-checked={on}
                aria-label={title}
                disabled={disabled}
                onClick={() => onChange(!on)}
            />
        </div>
    );
}

type PopoverProps = {
    options: MessageOptions;
    anchor: HTMLElement | null;

    // The option to start on (a chip opened it), else the first.
    section?: OptionSection;
    onClose: () => void;
};

// MessageOptionsPopover sets the message's options, all in one place.
export function MessageOptionsPopover({options, anchor, section, onClose}: PopoverProps) {
    const intl = useIntl();
    const {formatMessage} = intl;
    const ref = useRef<HTMLDivElement>(null);
    const {priority, setPriority} = options;
    const label = formatMessage({id: 'fusion.composer.options', defaultMessage: 'Message options'});

    useEffect(() => {
        const root = ref.current?.closest('.am-pop');
        const target = (section && root?.querySelector<HTMLElement>(`[data-opt="${section}"]`)) || root?.querySelector<HTMLElement>('button:not(:disabled)');
        target?.focus();

        // Once, as it opens.
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, []);

    const levels: Array<[PostPriority | '', string]> = [
        ['', formatMessage({id: 'fusion.composer.standard', defaultMessage: 'Standard'})],
        ['important', formatMessage({id: 'fusion.composer.important', defaultMessage: 'Important'})],
        ['urgent', formatMessage({id: 'fusion.composer.urgent', defaultMessage: 'Urgent'})],
    ];

    return (
        <Popover
            anchor={anchor}
            placement='above'
            className='plus-pop prio-pop msg-opts-pop'
            role='dialog'
            label={label}
            onClose={onClose}
        >
            <div ref={ref}>
                <MenuHeading>{label}</MenuHeading>
                {options.canPriority && (
                    <>
                        <div className={am('pp-row', 'opt-priority')}>
                            <span>{formatMessage({id: 'fusion.composer.priority', defaultMessage: 'Message priority'})}</span>
                            <div
                                className={am('seg')}
                                role='group'
                                aria-label={formatMessage({id: 'fusion.composer.priority', defaultMessage: 'Message priority'})}
                            >
                                {levels.map(([value, text], i) => (
                                    <button
                                        key={value || 'standard'}
                                        type='button'
                                        data-opt={i === 0 ? 'priority' : undefined}
                                        className={am({on: priority.priority === value}, value && `prio-${value}`)}
                                        aria-pressed={priority.priority === value}
                                        onClick={() => setPriority({priority: value})}
                                    >
                                        {text}
                                    </button>
                                ))}
                            </div>
                        </div>
                        <SwitchRow
                            title={formatMessage({id: 'fusion.composer.requestAck', defaultMessage: 'Request acknowledgement'})}
                            desc={formatMessage({id: 'fusion.composer.requestAckDesc', defaultMessage: 'Recipients confirm they have read it'})}
                            on={priority.requested_ack}
                            onChange={(on) => setPriority({requested_ack: on})}
                        />
                        <SwitchRow
                            title={formatMessage({id: 'fusion.composer.persistentTitle', defaultMessage: 'Persistent notifications'})}
                            desc={formatMessage({id: 'fusion.composer.persistentEvery', defaultMessage: 'Repeats every {minutes, plural, one {minute} other {# minutes}} until acknowledged · urgent only'}, {minutes: options.persistentMinutes})}
                            on={priority.persistent_notifications}
                            disabled={priority.priority !== 'urgent'}
                            onChange={(on) => setPriority({persistent_notifications: on})}
                        />
                        <MenuSeparator/>
                    </>
                )}
                {options.canBurn && (
                    <>
                        <SwitchRow
                            section='burn'
                            kind='burn'
                            title={formatMessage({id: 'fusion.composer.burn', defaultMessage: 'Burn on read'})}
                            desc={formatMessage({id: 'fusion.composer.burnAfter', defaultMessage: 'Deleted {duration} after each person opens it'}, {duration: formatBurnDuration(options.burnSeconds, intl)})}
                            on={options.burn}
                            onChange={options.setBurn}
                        />
                        {options.burn && (
                            <BurnDurationSlider
                                seconds={options.burnSeconds}
                                limit={options.burnLimit}
                                onChange={options.setBurnSeconds}
                            />
                        )}
                        <MenuSeparator/>
                    </>
                )}
                <SwitchRow
                    section='spoiler'
                    title={formatMessage({id: 'fusion.composer.spoiler', defaultMessage: 'Send as a spoiler'})}
                    desc={formatMessage({id: 'fusion.composer.spoilerDesc', defaultMessage: 'Hidden until each reader chooses to see it'})}
                    on={options.spoiler}
                    onChange={options.setSpoiler}
                />
                <div className={am('pp-foot')}>
                    <button
                        type='button'
                        className={am('btn', 'primary')}
                        onClick={onClose}
                    >
                        {formatMessage({id: 'fusion.composer.done', defaultMessage: 'Done'})}
                    </button>
                </div>
            </div>
        </Popover>
    );
}
