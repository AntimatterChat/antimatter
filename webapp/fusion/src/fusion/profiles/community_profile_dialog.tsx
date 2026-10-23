// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

import React, {useMemo, useRef, useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import type {CommunityProfile, CommunityProfilePatch} from '@mattermost/types/community_profiles';

import {TeamTypes} from 'mattermost-redux/action_types';
import {updateMe} from 'mattermost-redux/actions/users';
import {Client4} from 'mattermost-redux/client';
import {Preferences} from 'mattermost-redux/constants';
import {getConfig} from 'mattermost-redux/selectors/entities/general';
import {getCurrentTeam} from 'mattermost-redux/selectors/entities/teams';
import {getCurrentUser} from 'mattermost-redux/selectors/entities/users';
import {displayUsername} from 'mattermost-redux/utils/user_utils';

import Avatar from 'fusion/components/avatar';
import {Dialog} from 'fusion/components/layer';
import {useToast} from 'fusion/shell/toast_context';
import {badgeChoice, badgesRequired, getTeamsByUser, TEAM_BADGE_PROP, withBadgeChoice} from 'fusion/teams/teams';
import {am} from 'fusion/utils/class_names';
import {AcceptedProfileImageTypes} from 'utils/constants';

import type {GlobalState} from 'types/store';

import {CommunityScope, communityView, getCommunityProfile, getNameDisplayIn} from './community_profiles';

// CommunityProfileDialog is the person's profile in the current community, from the community menu: their picture
// and nickname there (instead of their global ones in its channels, member list, mentions and cards) and the team
// badge they show there. Direct and group messages keep the profile of My account. Which of their details the
// community sees (their full name, to be shown as their username) is in its privacy settings. Each change is saved as
// it's made, but the nickname, saved with its button.
export default function CommunityProfileDialog({onClose}: {onClose: () => void}) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const toast = useToast();
    const team = useSelector(getCurrentTeam);
    const me = useSelector(getCurrentUser);
    const teamId = team?.id || '';
    const profile = useSelector((state: GlobalState) => getCommunityProfile(state, teamId, me?.id));
    const nameDisplay = useSelector((state: GlobalState) => getNameDisplayIn(state, teamId));
    const maxFileSize = parseInt(useSelector(getConfig).MaxFileSize || '0', 10);
    const savedNickname = profile?.nickname || '';
    const [nickname, setNickname] = useState(savedNickname);
    const [busy, setBusy] = useState(false);
    const [error, setError] = useState('');
    const fileInput = useRef<HTMLInputElement>(null);
    const preview = useMemo(() => (me ? displayUsername(communityView(me, {team_id: teamId, user_id: me.id, update_at: 0, ...profile, nickname: nickname.trim()}), nameDisplay) : ''), [me, teamId, profile, nickname, nameDisplay]);

    if (!team || !me) {
        return null;
    }
    const failed = formatMessage({id: 'fusion.communityProfile.failed', defaultMessage: 'Your profile in this community couldn\'t be saved.'});

    // The server sends the community the new profile too; this shows it at once.
    const saved = (next: CommunityProfile) => dispatch({type: TeamTypes.RECEIVED_COMMUNITY_PROFILE, data: next});
    const run = async (request: () => Promise<CommunityProfile>, done: string) => {
        setBusy(true);
        setError('');
        try {
            saved(await request());
            toast(done);
            return true;
        } catch (err) {
            setError((err as {message?: string}).message || failed);
            return false;
        } finally {
            setBusy(false);
        }
    };
    const patch = (change: CommunityProfilePatch, done: string) => run(() => Client4.patchCommunityProfile(teamId, me.id, change), done);

    const upload = (file: File) => {
        if (!AcceptedProfileImageTypes.includes(file.type)) {
            setError(formatMessage({id: 'fusion.settings.pictureType', defaultMessage: 'Pick a JPEG, PNG or BMP image.'}));
            return;
        }
        if (maxFileSize && file.size > maxFileSize) {
            setError(formatMessage({id: 'fusion.settings.pictureSize', defaultMessage: 'This image is too large.'}));
            return;
        }
        run(() => Client4.uploadCommunityAvatar(teamId, me.id, file), formatMessage({id: 'fusion.communityProfile.pictureSaved', defaultMessage: 'Picture saved for this community'}));
    };

    const nicknameHelp = nameDisplay === Preferences.DISPLAY_PREFER_NICKNAME ? formatMessage({id: 'fusion.communityProfile.nicknameHelp', defaultMessage: 'Shown instead of your name in this community. Empty: your nickname from My account.'}) : formatMessage({id: 'fusion.communityProfile.nicknameHelpNotShown', defaultMessage: 'This community shows people by {display, select, username {username} other {full name}}: your nickname here only shows next to it in mention suggestions.'}, {display: nameDisplay === Preferences.DISPLAY_PREFER_USERNAME ? 'username' : 'full'});
    const title = formatMessage({id: 'fusion.communityProfile.title', defaultMessage: 'Your profile in {community}'}, {community: team.display_name});
    return (
        <Dialog
            label={title}
            onClose={onClose}
        >
            <CommunityScope value={teamId}>
                <div className={am('pane')}>
                    <h2>{title}</h2>
                    <p className={am('lead')}>{formatMessage({id: 'fusion.communityProfile.lead', defaultMessage: 'How you appear in this community\'s channels, member list, mentions and cards. Direct and group messages show your profile from My account.'})}</p>
                    <div className={am('pic-row')}>
                        <Avatar
                            userId={me.id}
                            size='xl'
                            teamId={teamId}
                        />
                        <div className={am('pic-who')}>
                            <div className={am('pic-name')}>{preview}</div>
                            <div className={am('pic-sub')}>{`@${me.username}`}</div>
                            <div className={am('pic-actions')}>
                                <input
                                    ref={fileInput}
                                    type='file'
                                    accept={AcceptedProfileImageTypes.join(',')}
                                    style={{display: 'none'}}
                                    aria-label={formatMessage({id: 'fusion.communityProfile.picture', defaultMessage: 'Picture in this community'})}
                                    onChange={(e) => {
                                        const file = e.target.files?.[0];
                                        e.target.value = '';
                                        if (file) {
                                            upload(file);
                                        }
                                    }}
                                />
                                <button
                                    type='button'
                                    className={am('btn')}
                                    disabled={busy}
                                    onClick={() => fileInput.current?.click()}
                                >
                                    {profile?.avatar_update_at ? formatMessage({id: 'fusion.settings.changePicture', defaultMessage: 'Change picture'}) : formatMessage({id: 'fusion.communityProfile.ownPicture', defaultMessage: 'Use another picture here'})}
                                </button>
                                {Boolean(profile?.avatar_update_at) && (
                                    <button
                                        type='button'
                                        className={am('btn')}
                                        disabled={busy}
                                        onClick={() => run(() => Client4.resetCommunityProfile(teamId, me.id, {avatar: true}), formatMessage({id: 'fusion.communityProfile.pictureRemoved', defaultMessage: 'Your profile picture shows here again'}))}
                                    >
                                        {formatMessage({id: 'fusion.communityProfile.globalPicture', defaultMessage: 'Use my profile picture'})}
                                    </button>
                                )}
                            </div>
                        </div>
                    </div>
                    <label className={am('field')}>
                        <span>{formatMessage({id: 'fusion.communityProfile.nickname', defaultMessage: 'Nickname in this community'})}</span>
                        <input
                            type='text'
                            value={nickname}
                            maxLength={64}
                            placeholder={me.nickname || ''}
                            onChange={(e) => {
                                setError('');
                                setNickname(e.target.value);
                            }}
                        />
                        <small>{nicknameHelp}</small>
                    </label>
                    <TeamBadgeField communityId={teamId}/>
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
                            type='button'
                            className={am('btn')}
                            onClick={onClose}
                        >
                            {formatMessage({id: 'fusion.communityProfile.close', defaultMessage: 'Close'})}
                        </button>
                        <button
                            type='button'
                            className={am('btn', 'primary')}
                            disabled={busy || nickname.trim() === savedNickname}
                            onClick={async () => {
                                const trimmed = nickname.trim();
                                if (await patch({nickname: trimmed}, trimmed ? formatMessage({id: 'fusion.communityProfile.nicknameSaved', defaultMessage: 'Nickname saved for this community'}) : formatMessage({id: 'fusion.communityProfile.nicknameRemoved', defaultMessage: 'Nickname removed from this community'}))) {
                                    setNickname(trimmed);
                                }
                            }}
                        >
                            {formatMessage({id: 'fusion.communityProfile.save', defaultMessage: 'Save nickname'})}
                        </button>
                    </div>
                </div>
            </CommunityScope>
        </Dialog>
    );
}

// TeamBadgeField picks the team whose tag shows next to your name in a community, among your teams of that community
// that have one, as in the mockup. Each community keeps its own choice.
export function TeamBadgeField({communityId}: {communityId: string}) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const toast = useToast();
    const me = useSelector(getCurrentUser);
    const allMyTeams = useSelector((state: GlobalState) => getTeamsByUser(state)[me.id]);
    const myTeams = useMemo(() => (allMyTeams || []).filter((t) => t.tag), [allMyTeams]);
    const required = useSelector(badgesRequired);
    const groups = useSelector((state: GlobalState) => state.entities.groups.groups);
    if (!myTeams.length) {
        return null;
    }

    // Where the community asks everyone to show a badge, there's no "None": without a choice, the first team shows.
    const choice = badgeChoice(me, communityId);
    let chosen = myTeams.some((t) => t.id === choice) ? choice! : '';
    if (required && !chosen) {
        chosen = myTeams[0]?.id || '';
    }
    return (
        <label className={am('field')}>
            <span>{formatMessage({id: 'fusion.settings.teamBadge', defaultMessage: 'Team badge'})}</span>
            <select
                value={chosen}
                onChange={async (e) => {
                    const team = myTeams.find((t) => t.id === e.target.value);
                    const badges = withBadgeChoice(me.props?.[TEAM_BADGE_PROP], communityId, team?.id || '', (id) => groups[id]?.team_id);
                    await dispatch(updateMe({props: {...me.props, [TEAM_BADGE_PROP]: badges}}));
                    toast(team ? formatMessage({id: 'fusion.toast.teamBadge', defaultMessage: 'Team badge: {tag}'}, {tag: team.tag}) : formatMessage({id: 'fusion.toast.teamBadgeHidden', defaultMessage: 'Team badge hidden'}));
                }}
            >
                {!required && <option value=''>{formatMessage({id: 'fusion.settings.teamBadgeNone', defaultMessage: 'None'})}</option>}
                {myTeams.map((t) => (
                    <option
                        key={t.id}
                        value={t.id}
                    >
                        {`${t.tag} — ${t.display_name}`}
                    </option>
                ))}
            </select>
            <small>{required ? formatMessage({id: 'fusion.settings.teamBadgeRequired', defaultMessage: 'Shown next to your name in this community, which asks everyone to show one. Only its teams you belong to are listed; each community keeps its own badge.'}) : formatMessage({id: 'fusion.settings.teamBadgeHelp', defaultMessage: 'Shown next to your name in this community, like a server tag. Only its teams you belong to are listed; each community keeps its own badge.'})}</small>
        </label>
    );
}
