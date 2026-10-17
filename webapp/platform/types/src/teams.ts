// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {ServerError} from './errors';
import type {MemberTimeout} from './moderation';
import type {UserProfile} from './users';
import type {RelationOneToOne} from './utilities';

export type TeamMembership = TeamUnread & {
    user_id: string;
    roles: string;
    delete_at: number;
    scheme_admin: boolean;
    scheme_guest: boolean;
    scheme_user: boolean;
};

export type TeamMemberWithError = {
    member: TeamMembership;
    user_id: string;
    error: ServerError;
};

export type TeamType = 'O' | 'I';

export type TeamKind = 'community' | 'corporate';

export type TeamMemberListView = 'choice' | 'roles' | 'teams';

export type Team = {
    id: string;
    create_at: number;
    update_at: number;
    delete_at: number;
    display_name: string;
    name: string;
    description: string;
    email: string;
    type: TeamType;
    company_name: string;
    allowed_domains: string;
    invite_id: string;
    allow_open_invite: boolean;
    scheme_id: string;
    group_constrained: boolean;

    /** Antimatter: role colours show on names unless the team hides them; the team can ask everyone to show their team badge. */
    hide_role_colors?: boolean;
    require_team_badges?: boolean;

    /** Antimatter: the team's onboarding as JSON (a welcome, and questions whose answers give roles and channels). */
    onboarding?: string;

    /** Antimatter: the team's autolinks as JSON (patterns of its messages turned into links when they're posted). */
    autolinks?: string;

    /** Antimatter: everyone in the community is in every channel they may see, and mutes channels rather than leaving them. */
    all_channels_membership?: boolean;

    /** Antimatter: the community's default channel (where newcomers land); empty for Town Square while there is one. */
    default_channel_id?: string;

    /**
     * Antimatter: the kind of community. In a community (the default), members don't archive channels; in a corporate
     * one, they archive those they created (whose admins they are).
     */
    kind?: TeamKind;

    /** Antimatter: how the member list groups the community's members: as each chooses, by role, or by team. */
    member_list_view?: TeamMemberListView;

    /** Data Retention Policy id. Unrelated to access-control policies below. */
    policy_id?: string | null;
    last_team_icon_update?: number;

    /**
     * True when an active team-type access control policy governs membership
     * of this team. Derived by the server on each read; never persisted.
     */
    policy_enforced?: boolean;

    /**
     * Per-action map of the access control policy attached to this team.
     * Populated lazily by the server only on hydrated read paths. Keys are
     * action identifiers (e.g. "membership"); values are true when present.
     * Prefer checking `policy_actions?.membership` over {@link policy_enforced}.
     */
    policy_actions?: Record<string, boolean>;

    /**
     * True when the team's policy opts into the membership sync job's add pass,
     * i.e. matching users are added to the team automatically.
     */
    policy_auto_add?: boolean;

    /** @deprecated Use {@link policy_auto_add}, which this now mirrors. */
    policy_is_active?: boolean;

    /**
     * Transient, per-viewer hint set by the server on public, policy-enforced
     * teams the requesting user qualifies to join and is not already a member of.
     * Never set on private teams; carries no policy detail.
     */
    recommended?: boolean;
};

export type TeamsState = {
    currentTeamId: string;
    teams: Record<string, Team>;
    myMembers: Record<string, TeamMembership>;
    membersInTeam: RelationOneToOne<Team, RelationOneToOne<UserProfile, TeamMembership>>;
    stats: RelationOneToOne<Team, TeamStats>;
    groupsAssociatedToTeam: any;
    totalCount: number;
    contentFlaggingStatus: Record<Team['id'], boolean>;

    // Antimatter: who's timed out in each team whose timeouts were loaded.
    timeouts?: RelationOneToOne<Team, MemberTimeout[]>;
};

export type TeamUnread = {
    team_id: string;

    /** The number of unread mentions in channels on this team, not including DMs and GMs */
    mention_count: number;

    /** The number of unread mentions in root posts in channels on this team, not including DMs and GMs */
    mention_count_root: number;

    /**
     * The number of unread posts in channels on this team, not including DMs and GMs
     *
     * @remarks Note that this differs from ChannelMembership.msg_count and ChannelUnread.msg_count since it tracks
     * unread posts instead of read posts.
     */
    msg_count: number;

    /**
     * The number of unread root posts in channels on this team, not including DMs and GMs
     *
     * @remarks Note that this differs from ChannelMember.msg_count_root and ChannelUnread.msg_count_root since it
     * tracks unread posts instead of read posts.
     */
    msg_count_root: number;

    thread_count?: number;
    thread_mention_count?: number;
    thread_urgent_mention_count?: number;
};

export type GetTeamMembersOpts = {
    sort?: 'Username';
    exclude_deleted_users?: boolean;
};

export type TeamsWithCount = {
    teams: Team[];
    total_count: number;
};

export type TeamStats = {
    team_id: string;
    total_member_count: number;
    active_member_count: number;
};

export type TeamSearchOpts = PagedTeamSearchOpts | NotPagedTeamSearchOpts;
export type PagedTeamSearchOpts = {
    page: number;
    per_page: number;
} & NotPagedTeamSearchOpts;
export type NotPagedTeamSearchOpts = {
    allow_open_invite?: boolean;
    group_constrained?: boolean;
};

export type TeamInviteWithError = {
    email: string;

    // Unlike ServerError, error uses field names directly from model.AppError on the server
    error: {
        id: string;
        message: string;
    };
};

export type MemberInviteProfile = {
    email: string;
    username: string;
    first_name: string;
    last_name: string;
};
