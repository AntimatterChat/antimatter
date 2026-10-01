// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {IconName} from 'fusion/components/icon';

export type KnownApp = {

    // The app's colour in the app rail, a modifier of .am-app-ic (see styles/_09_apps.scss).
    tone?: string;

    // The mockup's icon for the app, drawn instead of the plugin's own.
    icon: IconName;
};

// The apps the mockup draws in the app rail, by plugin id. The plugins not listed keep their own icons.
// The Antimatter plugins' ids are not final yet: this is the one place to change them.
const KNOWN_APPS: Record<string, KnownApp> = {
    focalboard: {tone: 'boards', icon: 'kanban'},
    playbooks: {tone: 'playbooks', icon: 'runbook'},
    jira: {tone: 'jira', icon: 'ticket'},
    github: {icon: 'code'},
    'com.github.manland.mattermost-plugin-gitlab': {icon: 'code'},
    'mattermost-ai': {tone: 'ai', icon: 'sparkle'},
    'com.antimatterchat.ai': {tone: 'ai', icon: 'sparkle'},
    'com.antimatterchat.mail': {tone: 'mail', icon: 'mail'},
    'com.antimatterchat.calendar': {tone: 'calendar', icon: 'calendar'},
    'com.antimatterchat.notes': {tone: 'notes', icon: 'pad'},
    'com.antimatterchat.whiteboard': {tone: 'whiteboard', icon: 'draw'},
};

export function getKnownApp(pluginId: string): KnownApp | undefined {
    return Object.prototype.hasOwnProperty.call(KNOWN_APPS, pluginId) ? KNOWN_APPS[pluginId] : undefined;
}
