// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

// The Fusion UI's icon set, from the mockup. Rendered once by the app; icons reference the symbols by id.
const SYMBOLS = [
    '<symbol id="am-i-hash" viewBox="0 0 24 24"><path d="M5 9h15M4 15h15M10 3 8 21M16 3l-2 18"/></symbol>',
    '<symbol id="am-i-speaker" viewBox="0 0 24 24"><path d="M4 10v4h4l5 4V6l-5 4H4z"/><path d="M16.5 9a4.5 4.5 0 0 1 0 6M19 6.5a8 8 0 0 1 0 11"/></symbol>',
    '<symbol id="am-i-lock" viewBox="0 0 24 24"><rect x="5" y="11" width="14" height="10" rx="2"/><path d="M8 11V8a4 4 0 0 1 8 0v3"/></symbol>',
    '<symbol id="am-i-forum" viewBox="0 0 24 24"><path d="M3 6a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v6a2 2 0 0 1-2 2H9l-4 3v-3a2 2 0 0 1-2-2z"/><path d="M19 9a2 2 0 0 1 2 2v6a2 2 0 0 1-2 2v3l-4-3h-3"/></symbol>',
    '<symbol id="am-i-thread" viewBox="0 0 24 24"><path d="M6 3v10a4 4 0 0 0 4 4h9"/><path d="m15 13 4 4-4 4"/></symbol>',
    '<symbol id="am-i-search" viewBox="0 0 24 24"><circle cx="11" cy="11" r="7"/><path d="m20 20-3.5-3.5"/></symbol>',
    '<symbol id="am-i-users" viewBox="0 0 24 24"><circle cx="9" cy="8" r="3.5"/><path d="M3 20a6 6 0 0 1 12 0M16 4.6a3.5 3.5 0 0 1 0 6.8M18 14.2a6 6 0 0 1 3.5 5.8"/></symbol>',
    '<symbol id="am-i-pin" viewBox="0 0 24 24"><path d="M9 3h6l-1 7 3 3H7l3-3z"/><path d="M12 13v8"/></symbol>',
    '<symbol id="am-i-bell" viewBox="0 0 24 24"><path d="M6 16v-5a6 6 0 0 1 12 0v5l2 2H4z"/><path d="M10 21a2 2 0 0 0 4 0"/></symbol>',
    '<symbol id="am-i-inbox" viewBox="0 0 24 24"><path d="m3 13 3-8h12l3 8v6a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1z"/><path d="M3 13h5l1 3h6l1-3h5"/></symbol>',
    '<symbol id="am-i-cog" viewBox="0 0 24 24"><circle cx="12" cy="12" r="3.2"/><path d="M12 2.5v3M12 18.5v3M4.6 4.6l2.1 2.1M17.3 17.3l2.1 2.1M2.5 12h3M18.5 12h3M4.6 19.4l2.1-2.1M17.3 6.7l2.1-2.1"/></symbol>',
    '<symbol id="am-i-shield" viewBox="0 0 24 24"><path d="M12 3 20 6v6c0 5-3.5 8-8 9-4.5-1-8-4-8-9V6z"/><path d="m9 12 2 2 4-4"/></symbol>',
    '<symbol id="am-i-mic" viewBox="0 0 24 24"><rect x="9" y="3" width="6" height="11" rx="3"/><path d="M5.5 11a6.5 6.5 0 0 0 13 0M12 17.5V21"/></symbol>',
    '<symbol id="am-i-mic-off" viewBox="0 0 24 24"><rect x="9" y="3" width="6" height="11" rx="3"/><path d="M5.5 11a6.5 6.5 0 0 0 13 0M12 17.5V21M4 4l16 16"/></symbol>',
    '<symbol id="am-i-headphones" viewBox="0 0 24 24"><path d="M4 15v-3a8 8 0 0 1 16 0v3"/><rect x="3" y="14" width="4" height="7" rx="1.5"/><rect x="17" y="14" width="4" height="7" rx="1.5"/></symbol>',
    '<symbol id="am-i-headphones-off" viewBox="0 0 24 24"><path d="M4 15v-3a8 8 0 0 1 16 0v3"/><rect x="3" y="14" width="4" height="7" rx="1.5"/><rect x="17" y="14" width="4" height="7" rx="1.5"/><path d="M3 3l18 18"/></symbol>',
    '<symbol id="am-i-video" viewBox="0 0 24 24"><rect x="3" y="6" width="12" height="12" rx="2"/><path d="m15 10 6-3v10l-6-3"/></symbol>',
    '<symbol id="am-i-screen" viewBox="0 0 24 24"><rect x="3" y="4" width="18" height="12" rx="2"/><path d="M8 20h8M12 16v4"/></symbol>',
    '<symbol id="am-i-hangup" viewBox="0 0 24 24"><path d="M3 14.5c5-4.7 13-4.7 18 0l-2.2 2.6-3.6-1.1v-2.4a9 9 0 0 0-6.4 0V16l-3.6 1.1z"/></symbol>',
    '<symbol id="am-i-plus" viewBox="0 0 24 24"><path d="M12 5v14M5 12h14"/></symbol>',
    '<symbol id="am-i-apps" viewBox="0 0 24 24"><rect x="4" y="4" width="6" height="6" rx="1.5"/><rect x="14" y="4" width="6" height="6" rx="1.5"/><rect x="4" y="14" width="6" height="6" rx="1.5"/><rect x="14" y="14" width="6" height="6" rx="1.5"/></symbol>',
    '<symbol id="am-i-reply" viewBox="0 0 24 24"><path d="M10 7 4 12l6 5"/><path d="M4 12h10a6 6 0 0 1 6 6v1"/></symbol>',
    '<symbol id="am-i-smile" viewBox="0 0 24 24"><circle cx="12" cy="12" r="9"/><path d="M8.5 14a4 4 0 0 0 7 0M9 9.5h.01M15 9.5h.01"/></symbol>',
    '<symbol id="am-i-dots" viewBox="0 0 24 24"><path d="M5 12h.01M12 12h.01M19 12h.01" stroke-width="3"/></symbol>',
    '<symbol id="am-i-x" viewBox="0 0 24 24"><path d="M6 6l12 12M18 6 6 18"/></symbol>',
    '<symbol id="am-i-chev" viewBox="0 0 24 24"><path d="m6 9 6 6 6-6"/></symbol>',
    '<symbol id="am-i-globe" viewBox="0 0 24 24"><circle cx="12" cy="12" r="9"/><path d="M3 12h18M12 3a14 14 0 0 1 0 18M12 3a14 14 0 0 0 0 18"/></symbol>',
    '<symbol id="am-i-export" viewBox="0 0 24 24"><path d="M14 3h7v7M21 3l-9 9"/><path d="M19 14v5a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V7a2 2 0 0 1 2-2h5"/></symbol>',
    '<symbol id="am-i-pen" viewBox="0 0 24 24"><path d="M4 20h4L19 9l-4-4L4 16z"/><path d="m13 7 4 4"/></symbol>',
    '<symbol id="am-i-send" viewBox="0 0 24 24"><path d="M4 12 20 4l-6 16-3-7z"/><path d="m11 13 9-9"/></symbol>',
    '<symbol id="am-i-compass" viewBox="0 0 24 24"><circle cx="12" cy="12" r="9"/><path d="m15.5 8.5-2 5-5 2 2-5z"/></symbol>',
    '<symbol id="am-i-menu" viewBox="0 0 24 24"><path d="M4 7h16M4 12h16M4 17h16"/></symbol>',
    '<symbol id="am-i-board" viewBox="0 0 24 24"><rect x="3" y="4" width="18" height="16" rx="2"/><path d="M9 4v16M15 4v11"/></symbol>',
    '<symbol id="am-i-playbook" viewBox="0 0 24 24"><path d="M5 4h11l3 3v13H5z"/><path d="M9 10h6M9 14h6M9 18h3"/></symbol>',
    '<symbol id="am-i-check" viewBox="0 0 24 24"><path d="m5 12 5 5 9-10"/></symbol>',
    '<symbol id="am-i-store" viewBox="0 0 24 24"><path d="M4 9 5 4h14l1 5M4 9v11h16V9"/><path d="M4 9a3 3 0 0 0 5.3 1.8A3 3 0 0 0 12 12a3 3 0 0 0 2.7-1.2A3 3 0 0 0 20 9"/></symbol>',
    '<symbol id="am-i-plug" viewBox="0 0 24 24"><path d="M9 3v5M15 3v5M6 8h12v3a6 6 0 0 1-12 0zM12 17v4"/></symbol>',
    '<symbol id="am-i-chat" viewBox="0 0 24 24"><path d="M4 5h16v11H9l-5 4z"/></symbol>',
    '<symbol id="am-i-tag" viewBox="0 0 24 24"><path d="M3 12V4h8l10 10-8 8z"/><path d="M7.5 7.5h.01"/></symbol>',
    '<symbol id="am-i-clock" viewBox="0 0 24 24"><circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/></symbol>',
    '<symbol id="am-i-image" viewBox="0 0 24 24"><rect x="3" y="4" width="18" height="16" rx="2"/><circle cx="8.5" cy="9.5" r="1.5"/><path d="m21 16-5-5-9 9"/></symbol>',
    '<symbol id="am-i-attach" viewBox="0 0 24 24"><path d="m20 11-8.5 8.5a5 5 0 0 1-7-7L13 4a3.5 3.5 0 0 1 5 5l-8.5 8.5a2 2 0 0 1-3-3L14 7"/></symbol>',
    '<symbol id="am-i-follow" viewBox="0 0 24 24"><path d="M6 16v-5a6 6 0 0 1 12 0v5l2 2H4z"/><path d="M10 21a2 2 0 0 0 4 0M9 11l2 2 4-4"/></symbol>',
    '<symbol id="am-i-at" viewBox="0 0 24 24"><circle cx="12" cy="12" r="4"/><path d="M16 8v5a3 3 0 0 0 6 0v-1a10 10 0 1 0-4 8"/></symbol>',
    '<symbol id="am-i-bold" viewBox="0 0 24 24"><path d="M7 5h6a3.5 3.5 0 0 1 0 7H7zM7 12h7a3.5 3.5 0 0 1 0 7H7z"/></symbol>',
    '<symbol id="am-i-italic" viewBox="0 0 24 24"><path d="M10 5h8M6 19h8M14 5l-4 14"/></symbol>',
    '<symbol id="am-i-strike" viewBox="0 0 24 24"><path d="M5 12h14M16 7.5A4 3.5 0 0 0 12 5c-2.5 0-4 1.3-4 3 0 1.3.8 2.3 2.5 3M8 16.5A4 3.5 0 0 0 12 19c2.5 0 4-1.3 4-3"/></symbol>',
    '<symbol id="am-i-heading" viewBox="0 0 24 24"><path d="M6 5v14M18 5v14M6 12h12"/></symbol>',
    '<symbol id="am-i-link" viewBox="0 0 24 24"><path d="M10 14a4 4 0 0 0 5.7 0l3-3a4 4 0 0 0-5.7-5.7l-1 1M14 10a4 4 0 0 0-5.7 0l-3 3a4 4 0 0 0 5.7 5.7l1-1"/></symbol>',
    '<symbol id="am-i-code" viewBox="0 0 24 24"><path d="m9 8-4 4 4 4M15 8l4 4-4 4"/></symbol>',
    '<symbol id="am-i-codeblock" viewBox="0 0 24 24"><rect x="3" y="4" width="18" height="16" rx="2"/><path d="m10 10-2 2 2 2M14 10l2 2-2 2"/></symbol>',
    '<symbol id="am-i-quote" viewBox="0 0 24 24"><path d="M10 7C7 8 5 10.5 5 14v3h5v-5H6.8M19 7c-3 1-5 3.5-5 7v3h5v-5h-3.2"/></symbol>',
    '<symbol id="am-i-sparkle" viewBox="0 0 24 24"><path d="m11 3 1.9 5.1L18 10l-5.1 1.9L11 17l-1.9-5.1L4 10l5.1-1.9zM18.5 14.5l.9 2.1 2.1.9-2.1.9-.9 2.1-.9-2.1-2.1-.9 2.1-.9z"/></symbol>',
    '<symbol id="am-i-flag" viewBox="0 0 24 24"><path d="M5 21V4M5 4h12l-2.5 4.5L17 13H5"/></symbol>',
    '<symbol id="am-i-flame" viewBox="0 0 24 24"><path d="M12 3c1.2 3.4 5.5 5.4 5.5 10.5a5.5 5.5 0 0 1-11 0c0-2.8 1.6-4.4 2.7-5.5 0 2.2 1 3.3 2.2 3.3 0-3.3-.9-5.8.6-8.3z"/></symbol>',
    '<symbol id="am-i-timer" viewBox="0 0 24 24"><path d="M9 2h6M12 9v5l3 2"/><circle cx="12" cy="14" r="8"/></symbol>',
    '<symbol id="am-i-ul" viewBox="0 0 24 24"><path d="M9 6h11M9 12h11M9 18h11"/><path d="M4.5 6h.01M4.5 12h.01M4.5 18h.01" stroke-width="3"/></symbol>',
    '<symbol id="am-i-ol" viewBox="0 0 24 24"><path d="M10 6h10M10 12h10M10 18h10M4 5h1.2v4M3.8 15.2a1.2 1.2 0 0 1 2.3.5L4 18.5h2.2"/></symbol>',
    '<symbol id="am-i-type" viewBox="0 0 24 24"><path d="M3 18 7.5 6 12 18M4.8 14h5.4M14.5 11.5a3 3 0 0 1 5.5 1.6V18M20 14.5h-3a2 2 0 0 0 0 4c1.7 0 3-1.2 3-3"/></symbol>',
    '<symbol id="am-i-poll" viewBox="0 0 24 24"><path d="M5 20V11M12 20V5M19 20v-7"/></symbol>',
    '<symbol id="am-i-upload" viewBox="0 0 24 24"><path d="M12 16V4M7 9l5-5 5 5M5 20h14"/></symbol>',
    '<symbol id="am-i-slash" viewBox="0 0 24 24"><rect x="3" y="3" width="18" height="18" rx="4"/><path d="m14 7-4 10"/></symbol>',
    '<symbol id="am-i-star" viewBox="0 0 24 24"><path d="m12 3.5 2.6 5.3 5.9.9-4.3 4.1 1 5.8L12 16.9l-5.2 2.7 1-5.8-4.3-4.1 5.9-.9z"/></symbol>',
    '<symbol id="am-i-bell-off" viewBox="0 0 24 24"><path d="M6 16v-5a6 6 0 0 1 9.4-4.9M18 11v5l2 2H8M10 21a2 2 0 0 0 4 0M3 3l18 18"/></symbol>',
    '<symbol id="am-i-folder" viewBox="0 0 24 24"><path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/></symbol>',
    '<symbol id="am-i-leave" viewBox="0 0 24 24"><path d="M14 4h4a2 2 0 0 1 2 2v12a2 2 0 0 1-2 2h-4M10 16l-4-4 4-4M6 12h10"/></symbol>',
    '<symbol id="am-i-user-plus" viewBox="0 0 24 24"><circle cx="10" cy="8" r="3.5"/><path d="M4 20a6 6 0 0 1 12 0M19 8v6M16 11h6"/></symbol>',
    '<symbol id="am-i-forward" viewBox="0 0 24 24"><path d="m14 5 6 6-6 6M20 11H9a5 5 0 0 0-5 5v3"/></symbol>',
    '<symbol id="am-i-bookmark" viewBox="0 0 24 24"><path d="M6 3h12v18l-6-4-6 4z"/></symbol>',
    '<symbol id="am-i-copy" viewBox="0 0 24 24"><rect x="8" y="8" width="12" height="12" rx="2"/><path d="M16 8V6a2 2 0 0 0-2-2H6a2 2 0 0 0-2 2v8a2 2 0 0 0 2 2h2"/></symbol>',
    '<symbol id="am-i-trash" viewBox="0 0 24 24"><path d="M4 7h16M10 11v6M14 11v6M6 7l1 13h10l1-13M9 7V4h6v3"/></symbol>',
    '<symbol id="am-i-sticker" viewBox="0 0 24 24"><path d="M5 4h14a1 1 0 0 1 1 1v9l-6 6H5a1 1 0 0 1-1-1V5a1 1 0 0 1 1-1z"/><path d="M14 20v-5a1 1 0 0 1 1-1h5M9 10h.01M15 10h.01M9 14.5a4 4 0 0 0 3 1"/></symbol>',
    '<symbol id="am-i-popout" viewBox="0 0 24 24"><path d="M11 4H6a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-5M14 4h6v6M20 4l-8 8"/></symbol>',
    '<symbol id="am-i-expand" viewBox="0 0 24 24"><path d="M15 3h6v6M9 21H3v-6M21 3l-7 7M3 21l7-7"/></symbol>',
    '<symbol id="am-i-kanban" viewBox="0 0 24 24"><rect x="3" y="4" width="5" height="11" rx="1.5"/><rect x="9.5" y="4" width="5" height="7" rx="1.5"/><rect x="16" y="4" width="5" height="15" rx="1.5"/></symbol>',
    '<symbol id="am-i-runbook" viewBox="0 0 24 24"><path d="m4 6 1.5 1.5L8 5M11 6h9M4 12l1.5 1.5L8 11M11 12h5"/><path d="M4.5 17.5h3M11 17.5h1.5"/><path d="m16 15 5 3-5 3z"/></symbol>',
    '<symbol id="am-i-ticket" viewBox="0 0 24 24"><path d="M3 8V6a1 1 0 0 1 1-1h16a1 1 0 0 1 1 1v2a2.5 2.5 0 0 0 0 5v2a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1v-2a2.5 2.5 0 0 0 0-5z"/><path d="m9 10.5 2 2 4-4"/></symbol>',
    '<symbol id="am-i-mail" viewBox="0 0 24 24"><rect x="3" y="5" width="18" height="14" rx="2"/><path d="m4 7 8 6 8-6"/></symbol>',
    '<symbol id="am-i-calendar" viewBox="0 0 24 24"><rect x="3" y="5" width="18" height="16" rx="2"/><path d="M3 10h18M8 3v4M16 3v4"/><path d="M8 14h3v3H8z"/></symbol>',
    '<symbol id="am-i-pad" viewBox="0 0 24 24"><path d="M5 3h10l4 4v14H5z"/><path d="M15 3v4h4M8 11h8M8 15h8M8 19h5"/></symbol>',
    '<symbol id="am-i-draw" viewBox="0 0 24 24"><rect x="3" y="3" width="8" height="8" rx="1.5"/><circle cx="16.5" cy="16.5" r="4.5"/><path d="M4 20c2-3 4-3 6-6"/></symbol>',
    '<symbol id="am-i-cursor" viewBox="0 0 24 24"><path d="m5 3 14 7-6 2-2 6z"/></symbol>',
    '<symbol id="am-i-rect" viewBox="0 0 24 24"><rect x="4" y="6" width="16" height="12" rx="1.5"/></symbol>',
    '<symbol id="am-i-ellipse" viewBox="0 0 24 24"><ellipse cx="12" cy="12" rx="8.5" ry="6"/></symbol>',
    '<symbol id="am-i-arrow" viewBox="0 0 24 24"><path d="M5 19 19 5M10 5h9v9"/></symbol>',
    '<symbol id="am-i-text" viewBox="0 0 24 24"><path d="M5 6V4h14v2M12 4v16M9 20h6"/></symbol>',
    '<symbol id="am-i-eraser" viewBox="0 0 24 24"><path d="m8 20-4-4L14 6l6 6-8 8zM11 9l6 6M8 20h12"/></symbol>',
    '<symbol id="am-i-undo" viewBox="0 0 24 24"><path d="M9 14 4 9l5-5"/><path d="M4 9h11a5 5 0 0 1 0 10h-3"/></symbol>',
    '<symbol id="am-i-checkbox" viewBox="0 0 24 24"><rect x="4" y="4" width="16" height="16" rx="3"/><path d="m8 12 3 3 5-6"/></symbol>',
    '<symbol id="am-i-pause" viewBox="0 0 24 24"><path d="M8 5v14M16 5v14"/></symbol>',
    '<symbol id="am-i-moon" viewBox="0 0 24 24"><path d="M20 14.5A8 8 0 0 1 9.5 4a8 8 0 1 0 10.5 10.5z"/></symbol>',
    '<symbol id="am-i-phone" viewBox="0 0 24 24"><path d="M5 4h3.5l2 5-2.5 1.5a11 11 0 0 0 5.5 5.5L15 13.5l5 2V19a2 2 0 0 1-2 2A16 16 0 0 1 3 6a2 2 0 0 1 2-2z"/></symbol>',
    '<symbol id="am-i-pin-top" viewBox="0 0 24 24"><path d="M5 3h14M9.5 7h5l-.8 4.5 2.8 2.8h-9.8l2.8-2.8z M12 14.5V21"/></symbol>',
    '<symbol id="am-i-hand" viewBox="0 0 24 24"><path d="M8 13V5.5a1.5 1.5 0 0 1 3 0V11M11 10.5V4a1.5 1.5 0 0 1 3 0v6.5M14 10.5V5.5a1.5 1.5 0 0 1 3 0V13"/><path d="M17 12.5V9.5a1.5 1.5 0 0 1 3 0V14a7 7 0 0 1-7 7h-1a6 6 0 0 1-4.6-2.2L4 15.6a1.5 1.5 0 0 1 2.3-1.9L8 15.5V13"/></symbol>',
    '<symbol id="am-i-fullscreen" viewBox="0 0 24 24"><path d="M4 9V4h5M20 9V4h-5M4 15v5h5M20 15v5h-5"/></symbol>',
    '<symbol id="am-i-mark" viewBox="0 0 64 64">',
    '<ellipse cx="32" cy="32" rx="25" ry="9.5" fill="none" stroke="#A78BFA" stroke-width="4.5" transform="rotate(45 32 32)"/>',
    '<ellipse cx="32" cy="32" rx="25" ry="9.5" fill="none" stroke="#22D3EE" stroke-width="4.5" transform="rotate(-45 32 32)"/>',
    '<path d="M32 20.5Q33.9 30.1 43.5 32Q33.9 33.9 32 43.5Q30.1 33.9 20.5 32Q30.1 30.1 32 20.5Z" fill="#FBBF24" stroke="none"/>',
    '<circle cx="14.32" cy="14.32" r="5.5" fill="#A78BFA" stroke="none"/>',
    '<circle cx="49.68" cy="14.32" r="4.25" fill="#1E1F22" stroke="#22D3EE" stroke-width="2.5"/>',
    '</symbol>',
].join('');

export type IconName = 'hash' | 'speaker' | 'lock' | 'forum' | 'thread' | 'search' | 'users' | 'pin' | 'bell' | 'inbox' | 'cog' | 'shield' | 'mic' | 'mic-off' | 'headphones' | 'headphones-off' | 'video' | 'screen' | 'hangup' | 'plus' | 'apps' | 'reply' | 'smile' | 'dots' | 'x' | 'chev' | 'globe' | 'export' | 'pen' | 'send' | 'compass' | 'menu' | 'board' | 'playbook' | 'check' | 'store' | 'plug' | 'chat' | 'tag' | 'clock' | 'attach' | 'image' | 'follow' | 'at' | 'bold' | 'italic' | 'strike' | 'heading' | 'link' | 'code' | 'codeblock' | 'quote' | 'sparkle' | 'flag' | 'flame' | 'timer' | 'ul' | 'ol' | 'type' | 'poll' | 'upload' | 'slash' | 'star' | 'bell-off' | 'folder' | 'leave' | 'user-plus' | 'forward' | 'bookmark' | 'copy' | 'trash' | 'sticker' | 'popout' | 'expand' | 'kanban' | 'runbook' | 'ticket' | 'mail' | 'calendar' | 'pad' | 'draw' | 'cursor' | 'rect' | 'ellipse' | 'arrow' | 'text' | 'eraser' | 'undo' | 'checkbox' | 'pause' | 'moon' | 'phone' | 'pin-top' | 'fullscreen' | 'hand' | 'mark';

const ICON_NAMES = new Set(Array.from(SYMBOLS.matchAll(/<symbol id="am-i-([^"]+)"/g), (m) => m[1]));

// isIconName tells whether a name from outside the Fusion code (e.g. a plugin's) is one of the icons.
export function isIconName(name: string): name is IconName {
    return ICON_NAMES.has(name);
}

export default function IconSprite() {
    return (
        <svg
            width='0'
            height='0'
            style={{position: 'absolute'}}
            aria-hidden='true'
            dangerouslySetInnerHTML={{__html: `<defs>${SYMBOLS}</defs>`}}
        />
    );
}
