// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {request, type APIRequestContext} from 'playwright/test';

import {env} from './env';

// Inside a test, new request contexts take the project's storage state (the admin's session cookies), and the server
// prefers a session cookie to the Authorization header: every API context starts without cookies.
export const NO_COOKIES = {cookies: [], origins: []};

export type User = {id: string; username: string; first_name: string; last_name: string; nickname: string; roles: string};
export type Team = {id: string; name: string; display_name: string};
export type Channel = {id: string; name: string; display_name: string; team_id: string};
export type Post = {id: string; message: string; root_id: string; delete_at: number; type: string};

// Api is a logged-in client of the server's REST API (v4), for setting up and cleaning up what the tests need.
export class Api {
    private constructor(readonly context: APIRequestContext, readonly token: string, readonly me: User) {}

    // login logs in with a password. The context keeps the session cookies, so its storage state logs a browser in.
    static async login(loginId: string, password: string): Promise<Api> {
        const context = await request.newContext({baseURL: env.baseURL, storageState: NO_COOKIES, extraHTTPHeaders: {'X-Requested-With': 'XMLHttpRequest'}});
        const res = await context.post('/api/v4/users/login', {data: {login_id: loginId, password}});
        if (!res.ok()) {
            await context.dispose();
            throw new Error(`Logging in as ${loginId} on ${env.baseURL} failed: ${res.status()} ${await res.text()}`);
        }
        return new Api(context, res.headers().token, await res.json());
    }

    // withToken uses a session token the setup saved.
    static async withToken(token: string): Promise<Api> {
        const context = await request.newContext({baseURL: env.baseURL, storageState: NO_COOKIES, extraHTTPHeaders: {Authorization: `Bearer ${token}`, 'X-Requested-With': 'XMLHttpRequest'}});
        const res = await context.get('/api/v4/users/me');
        if (!res.ok()) {
            await context.dispose();
            throw new Error(`The saved session is no longer valid: ${res.status()}`);
        }
        return new Api(context, token, await res.json());
    }

    private headers() {
        return {Authorization: `Bearer ${this.token}`};
    }

    async call<T>(method: 'GET' | 'POST' | 'PUT' | 'DELETE', url: string, data?: unknown): Promise<T> {
        const res = await this.context.fetch(`/api/v4${url}`, {method, headers: this.headers(), data});
        if (!res.ok()) {
            throw new Error(`${method} ${url}: ${res.status()} ${await res.text()}`);
        }
        const text = await res.text();
        return (text ? JSON.parse(text) : undefined) as T;
    }

    async status(method: 'DELETE' | 'POST', url: string): Promise<number> {
        const res = await this.context.fetch(`/api/v4${url}`, {method, headers: this.headers()});
        return res.status();
    }

    myTeams() {
        return this.call<Team[]>('GET', '/users/me/teams');
    }

    teamByName(name: string) {
        return this.call<Team>('GET', `/teams/name/${encodeURIComponent(name)}`);
    }

    createChannel(teamId: string, name: string, displayName: string) {
        return this.call<Channel>('POST', '/channels', {team_id: teamId, name, display_name: displayName, type: 'O', purpose: 'Created by the Fusion Playwright suite; archived when it ends.'});
    }

    // removeChannel deletes the channel for good where the server allows it (ServiceSettings.EnableAPIChannelDeletion),
    // and archives it otherwise.
    async removeChannel(channelId: string) {
        if (await this.status('DELETE', `/channels/${channelId}?permanent=true`) >= 400) {
            await this.call('DELETE', `/channels/${channelId}`);
        }
    }

    addToChannel(channelId: string, userId: string) {
        return this.call('POST', `/channels/${channelId}/members`, {user_id: userId});
    }

    async posts(channelId: string): Promise<Post[]> {
        const list = await this.call<{order: string[]; posts: Record<string, Post>}>('GET', `/channels/${channelId}/posts?per_page=200`);
        return list.order.map((id) => list.posts[id]);
    }

    createPost(channelId: string, message: string, rootId = '') {
        return this.call<Post>('POST', '/posts', {channel_id: channelId, message, root_id: rootId});
    }

    deletePost(postId: string) {
        return this.call('DELETE', `/posts/${postId}`);
    }

    // clearChannel deletes every post in a channel the suite owns, replies first.
    async clearChannel(channelId: string) {
        const posts = (await this.posts(channelId)).filter((p) => !p.delete_at);
        posts.sort((a, b) => Number(Boolean(b.root_id)) - Number(Boolean(a.root_id)));
        for (const post of posts) {
            // A root deleted before its reply takes the reply along: a 404 then is fine.
            await this.status('DELETE', `/posts/${post.id}`);
        }
    }

    createUser(username: string, password: string) {
        return this.call<User>('POST', '/users', {username, password, email: `${username}@example.com`, first_name: 'E2E', last_name: 'Member'});
    }

    addToTeam(teamId: string, userId: string) {
        return this.call('POST', `/teams/${teamId}/members`, {team_id: teamId, user_id: userId});
    }

    // removeUser deletes the user for good where the server allows it (ServiceSettings.EnableAPIUserDeletion), and
    // deactivates them otherwise.
    async removeUser(userId: string) {
        if (await this.status('DELETE', `/users/${userId}?permanent=true`) >= 400) {
            await this.call('DELETE', `/users/${userId}`);
        }
    }

    savePreferences(preferences: Array<{category: string; name: string; value: string}>) {
        return this.call('PUT', `/users/${this.me.id}/preferences`, preferences.map((p) => ({...p, user_id: this.me.id})));
    }

    async logout() {
        await this.status('POST', '/users/logout');
        await this.context.dispose();
    }
}

// displayName is how Fusion names a person by default: their full name, or else their username.
export function displayName(user: User): string {
    const full = [user.first_name, user.last_name].filter(Boolean).join(' ');
    return full || user.username;
}
