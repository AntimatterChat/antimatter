// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useState} from 'react';
import {defineMessage, useIntl} from 'react-intl';
import type {MessageDescriptor} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import type {TeamInviteLink} from '@mattermost/types/moderation';

import {getMissingProfilesByIds} from 'mattermost-redux/actions/users';
import {Client4} from 'mattermost-redux/client';
import {getCurrentTeam} from 'mattermost-redux/selectors/entities/teams';

import {Dialog} from 'fusion/components/layer';
import {useDisplayName, useUser} from 'fusion/hooks/users';
import {Select} from 'fusion/modals/settings_controls';
import {useToast} from 'fusion/shell/toast_context';
import {am} from 'fusion/utils/class_names';
import {getSiteURL} from 'utils/url';
import {copyToClipboard} from 'utils/utils';

const MINUTE = 60 * 1000;
const HOUR = 60 * MINUTE;

// When a new link ends, as Discord's (7 days first): or never (0).
export const LINK_EXPIRIES: Array<{ms: number; label: MessageDescriptor}> = [
    {ms: 7 * 24 * HOUR, label: defineMessage({id: 'fusion.inviteLinks.expire.7d', defaultMessage: '7 days'})},
    {ms: 24 * HOUR, label: defineMessage({id: 'fusion.inviteLinks.expire.1d', defaultMessage: '1 day'})},
    {ms: 12 * HOUR, label: defineMessage({id: 'fusion.inviteLinks.expire.12h', defaultMessage: '12 hours'})},
    {ms: 6 * HOUR, label: defineMessage({id: 'fusion.inviteLinks.expire.6h', defaultMessage: '6 hours'})},
    {ms: HOUR, label: defineMessage({id: 'fusion.inviteLinks.expire.1h', defaultMessage: '1 hour'})},
    {ms: 30 * MINUTE, label: defineMessage({id: 'fusion.inviteLinks.expire.30m', defaultMessage: '30 minutes'})},
    {ms: 0, label: defineMessage({id: 'fusion.inviteLinks.expire.never', defaultMessage: 'Never'})},
];

// How many people a new link lets in: no limit (0), or a number.
export const LINK_MAX_USES = [0, 1, 5, 10, 25, 50, 100];

// inviteLinkURL is the address a link's code makes, the same as for the community's permanent invite link.
export function inviteLinkURL(link: TeamInviteLink): string {
    return `${getSiteURL()}/signup_user_complete/?id=${link.id}`;
}

// LinkRow is one of the community's links: its address, who made it, its uses and its end; copying and revoking it.
function LinkRow({link, onRevoked}: {link: TeamInviteLink; onRevoked: (id: string) => void}) {
    const {formatMessage, formatDate, formatTime} = useIntl();
    const toast = useToast();
    const creator = useDisplayName(useUser(link.creator_id));
    const [busy, setBusy] = useState(false);
    const url = inviteLinkURL(link);
    const now = Date.now();
    const ended = link.expire_at !== 0 && link.expire_at <= now;
    const usedUp = link.max_uses > 0 && link.uses >= link.max_uses;

    let state: string;
    if (ended) {
        state = formatMessage({id: 'fusion.inviteLinks.expired', defaultMessage: 'Expired'});
    } else if (usedUp) {
        state = formatMessage({id: 'fusion.inviteLinks.usedUp', defaultMessage: 'Used up'});
    } else if (link.expire_at) {
        state = formatMessage({id: 'fusion.inviteLinks.expires', defaultMessage: 'Expires {date} at {time}'}, {date: formatDate(link.expire_at, {month: 'short', day: 'numeric'}), time: formatTime(link.expire_at)});
    } else {
        state = formatMessage({id: 'fusion.inviteLinks.neverExpires', defaultMessage: 'Never expires'});
    }
    const uses = link.max_uses ? formatMessage({id: 'fusion.inviteLinks.usesOf', defaultMessage: '{uses} of {max, plural, one {# use} other {# uses}}'}, {uses: link.uses, max: link.max_uses}) : formatMessage({id: 'fusion.inviteLinks.uses', defaultMessage: '{uses, plural, one {# use} other {# uses}}'}, {uses: link.uses});

    const revoke = async () => {
        setBusy(true);
        try {
            await Client4.revokeTeamInviteLink(link.team_id, link.id);
            toast(formatMessage({id: 'fusion.inviteLinks.revoked', defaultMessage: 'Link revoked'}));
            onRevoked(link.id);
        } catch (error) {
            setBusy(false);
            toast((error as {message?: string}).message || formatMessage({id: 'fusion.inviteLinks.revokeFailed', defaultMessage: 'The link couldn\'t be revoked.'}));
        }
    };

    return (
        <div
            className={am('toggle-row', 'invite-link-row', {off: ended || usedUp})}
            data-link={link.id}
        >
            <div className={am('txt')}>
                <b className={am('mono')}>{url}</b>
                <span>{[formatMessage({id: 'fusion.inviteLinks.by', defaultMessage: 'By {name}'}, {name: creator || link.creator_id}), uses, state].join(' · ')}</span>
            </div>
            <div className={am('invite-link-actions')}>
                {!ended && !usedUp && (
                    <button
                        type='button'
                        className={am('btn')}
                        aria-label={formatMessage({id: 'fusion.inviteLinks.copyLabel', defaultMessage: 'Copy the link {url}'}, {url})}
                        onClick={() => {
                            copyToClipboard(url);
                            toast(formatMessage({id: 'fusion.inviteLinks.copied', defaultMessage: 'Link copied'}));
                        }}
                    >
                        {formatMessage({id: 'fusion.inviteLinks.copy', defaultMessage: 'Copy'})}
                    </button>
                )}
                <button
                    type='button'
                    className={am('btn', 'danger')}
                    disabled={busy}
                    aria-label={formatMessage({id: 'fusion.inviteLinks.revokeLabel', defaultMessage: 'Revoke the link {url}'}, {url})}
                    onClick={revoke}
                >
                    {formatMessage({id: 'fusion.inviteLinks.revoke', defaultMessage: 'Revoke'})}
                </button>
            </div>
        </div>
    );
}

// InviteLinksDialog is the community's invite links, reached from Invite people by those allowed to invite (the one
// place for them): links that end and allow a number of uses, besides the community's permanent invite link (Invite
// people's Copy invite link). Its admins see and revoke everyone's links; the others, their own.
export default function InviteLinksDialog({onExited}: {onExited: () => void}) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const toast = useToast();
    const team = useSelector(getCurrentTeam);
    const teamId = team?.id || '';
    const [links, setLinks] = useState<TeamInviteLink[] | null>(null);
    const [failed, setFailed] = useState(false);
    const [expiry, setExpiry] = useState(String(LINK_EXPIRIES[0].ms));
    const [maxUses, setMaxUses] = useState('0');
    const [busy, setBusy] = useState(false);
    const [error, setError] = useState('');

    useEffect(() => {
        if (!teamId) {
            return undefined;
        }
        let live = true;
        Client4.getTeamInviteLinks(teamId).
            then((list) => {
                if (!live) {
                    return;
                }
                setLinks(list);
                const creators = [...new Set(list.map((l) => l.creator_id))];
                if (creators.length) {
                    dispatch(getMissingProfilesByIds(creators));
                }
            }).
            catch(() => live && setFailed(true));
        return () => {
            live = false;
        };
    }, [teamId, dispatch]);

    const create = async () => {
        if (!teamId || busy) {
            return;
        }
        setBusy(true);
        setError('');
        const ms = Number(expiry);
        try {
            const link = await Client4.createTeamInviteLink(teamId, {expire_at: ms ? Date.now() + ms : 0, max_uses: Number(maxUses)});
            setLinks((list) => [link, ...(list || [])]);
            copyToClipboard(inviteLinkURL(link));
            toast(formatMessage({id: 'fusion.inviteLinks.created', defaultMessage: 'Link created and copied'}));
        } catch (err) {
            setError((err as {message?: string}).message || formatMessage({id: 'fusion.inviteLinks.createFailed', defaultMessage: 'The link couldn\'t be created.'}));
        }
        setBusy(false);
    };

    const title = formatMessage({id: 'fusion.inviteLinks.title', defaultMessage: 'Invite links'});
    return (
        <Dialog
            label={title}
            onClose={onExited}
            onSubmit={create}
            small={false}
            className={am('wide')}
        >
            <div className={am('pane', 'invite-links')}>
                <h2>{title}</h2>
                <p className={am('lead')}>{formatMessage({id: 'fusion.inviteLinks.lead', defaultMessage: 'Links anyone can use to join {team}, besides its permanent invite link. Each can expire and allow a number of uses; revoke one to stop it. Someone banned from the community can\'t join by any of them.'}, {team: team?.display_name})}</p>
                <Select
                    title={formatMessage({id: 'fusion.inviteLinks.expireAfter', defaultMessage: 'Expire after'})}
                    value={expiry}
                    options={LINK_EXPIRIES.map((e): [string, string] => [String(e.ms), formatMessage(e.label)])}
                    onChange={setExpiry}
                />
                <Select
                    title={formatMessage({id: 'fusion.inviteLinks.maxUses', defaultMessage: 'Maximum number of uses'})}
                    value={maxUses}
                    options={LINK_MAX_USES.map((n): [string, string] => [String(n), n ? String(n) : formatMessage({id: 'fusion.inviteLinks.noLimit', defaultMessage: 'No limit'})])}
                    onChange={setMaxUses}
                />
                {error && (
                    <p
                        className={am('field-error')}
                        role='alert'
                    >
                        {error}
                    </p>
                )}
                <div className={am('modal-actions')}>
                    <button
                        type='submit'
                        className={am('btn', 'primary')}
                        disabled={busy || !teamId}
                    >
                        {formatMessage({id: 'fusion.inviteLinks.create', defaultMessage: 'Create link'})}
                    </button>
                </div>

                <h4 className={am('section-title')}>{formatMessage({id: 'fusion.inviteLinks.listTitle', defaultMessage: 'Links'})}</h4>
                {failed && <p className={am('field-error')}>{formatMessage({id: 'fusion.inviteLinks.loadFailed', defaultMessage: 'The links couldn\'t be loaded.'})}</p>}
                {!failed && links === null && <p className={am('lead')}>{'…'}</p>}
                {links?.length === 0 && <p className={am('lead')}>{formatMessage({id: 'fusion.inviteLinks.none', defaultMessage: 'No links yet.'})}</p>}
                {links?.map((link) => (
                    <LinkRow
                        key={link.id}
                        link={link}
                        onRevoked={(id) => setLinks((list) => (list || []).filter((l) => l.id !== id))}
                    />
                ))}
            </div>
        </Dialog>
    );
}
