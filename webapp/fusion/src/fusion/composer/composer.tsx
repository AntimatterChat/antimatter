// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useCallback, useEffect, useRef, useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector, useStore} from 'react-redux';

import type {FileInfo} from '@mattermost/types/files';
import type {SchedulingInfo} from '@mattermost/types/schedule_post';

import {savePreferences} from 'mattermost-redux/actions/preferences';
import {getConfig} from 'mattermost-redux/selectors/entities/general';
import {getBool, getWysiwygEditorPreference} from 'mattermost-redux/selectors/entities/preferences';
import {isScheduledPostsEnabled} from 'mattermost-redux/selectors/entities/scheduled_posts';
import {getCurrentUserId} from 'mattermost-redux/selectors/entities/users';
import {getFormattedFileSize} from 'mattermost-redux/utils/file_utils';

import {uploadFile} from 'actions/file_actions';
import {emitLocalUserTypingEvent} from 'actions/global_actions';
import {editLatestPost, onSubmit} from 'actions/views/create_comment';
import type {SubmitPostReturnType} from 'actions/views/create_comment';
import {removeDraft, updateDraft} from 'actions/views/drafts';
import {replyToLatestPostInChannel} from 'actions/views/rhs';

import {getComposerPlaceholder} from 'components/advanced_text_editor/composer_placeholder';
import WysiwygEditor from 'components/advanced_text_editor/wysiwyg_editor/wysiwyg_editor';
import type {WysiwygEditorHandle} from 'components/advanced_text_editor/wysiwyg_editor/wysiwyg_editor';
import Markdown from 'components/markdown';

import Icon from 'fusion/components/icon';
import {Popover} from 'fusion/components/layer';
import {
    MENTION_PREFERENCE_CATEGORY,
    MENTION_PREFERENCE_NAME,
    MentionSwitch,
    QuotedMessage,
    REPLY_EVENT,
    mentionsQuotedAuthor,
    replyCandidates,
    replyProps,
    replyToId,
    useInlineRepliesEnabled,
    useQuoted,
} from 'fusion/messages/inline_reply';
import type {ReplyEventDetail} from 'fusion/messages/inline_reply';
import EmojiPicker from 'fusion/popovers/emoji_picker';
import {MENTION_EVENT} from 'fusion/popovers/user_menu';
import {useDialogs} from 'fusion/shell/dialogs_context';
import {useToast} from 'fusion/shell/toast_context';
import {CommunityRulesGate} from 'fusion/teams/community_rules_dialog';
import {a11yRegion} from 'fusion/utils/a11y_regions';
import {am} from 'fusion/utils/class_names';
import PluggableErrorBoundary from 'plugins/pluggable/error_boundary';
import Constants from 'utils/constants';
import {generateId} from 'utils/utils';

import type {GlobalState} from 'types/store';
import type {PostDraft} from 'types/store/draft';
import type {AIActionMenuItemComponent, FilesWillUploadHook, PostEditorActionComponent} from 'types/store/plugins';

import AiRewriteMenu, {useAIRewriteAvailable} from './ai_rewrite';
import Autocomplete from './autocomplete';
import type {AutocompleteHandle} from './autocomplete';
import AwayNote from './away_note';
import {COMPOSER_MODE_EVENT, ComposerModeBar, ComposerModeBody, modeText, useComposerModes} from './composer_modes';
import type {ComposerModeEventDetail} from './composer_modes';
import {confirmChannelWideMentions} from './confirm_mentions';
import {useDraftAutosave} from './draft_autosave';
import {FORMATS, applyFormat, formatRich} from './formats';
import {useShowFormatting} from './formatting_preference';
import {MessageOptionChips, MessageOptionsPopover, useMessageOptions} from './message_options';
import type {OptionSection} from './message_options';
import PlusMenu from './plus_menu';
import {SchedulePopover, ScheduledNote} from './schedule';
import SendButton from './send_button';
import SleepNote from './sleep_note';
import Typing from './typing';

type Props = {
    channelId: string;

    // Replies in a thread have a root; messages to the channel don't.
    rootId?: string;
    placeholder: string;

    // Called once a message is posted (not a command), e.g. by an app's panel to open the conversation it starts.
    afterSubmit?: (response: SubmitPostReturnType) => void;

    // Antimatter forums: merged onto the sent message's props (the New topic form's title, props.am_thread_title),
    // for every way the draft can be sent (now or scheduled).
    extraProps?: Record<string, string>;

    // Antimatter forums: gives a caller a way to send the current draft itself (the New topic form sends from its
    // title field with Ctrl/Cmd+Enter, without its own send button). Called once the composer is ready.
    onReady?: (submit: () => void) => void;
};

type Pending = {clientId: string; name: string; progress: number};

// The conversation view sends files dropped on it to its composer with this event.
export const DROP_FILES_EVENT = 'am-drop-files';

// Under this width (a phone, a narrow panel) the message options leave the composer's row for its "+" menu, so that
// the text box keeps its room.
export const NARROW_COMPOSER = 400;

// Composer writes messages: the mockup's compose box around the classic web app's drafts and submit logic
// (slash commands, reactions, message priority).
const NO_HOOKS: FilesWillUploadHook[] = [];
const NO_EDITOR_ACTIONS: PostEditorActionComponent[] = [];
const NO_AI_ITEMS: AIActionMenuItemComponent[] = [];

// Composer is the message box; in a community's channel, the notice asking to agree to its rules until one does
// (Antimatter: the server refuses the messages until then).
export default function Composer(props: Props) {
    return (
        <CommunityRulesGate channelId={props.channelId}>
            <MessageBox {...props}/>
        </CommunityRulesGate>
    );
}

function MessageBox({channelId, rootId = '', placeholder, afterSubmit, extraProps, onReady}: Props) {
    const intl = useIntl();
    const {formatMessage} = intl;
    const dialogs = useDialogs();
    const dispatch = useDispatch();
    const store = useStore<GlobalState>();
    const schedulingEnabled = useSelector(isScheduledPostsEnabled);
    const toast = useToast();
    const maxFileSize = useSelector((state: GlobalState) => parseInt(getConfig(state).MaxFileSize || '0', 10) || 0);
    const maxPostSize = useSelector((state: GlobalState) => parseInt(getConfig(state).MaxPostSize || '', 10) || Constants.DEFAULT_CHARACTER_LIMIT);
    const [sendError, setSendError] = useState('');

    // The formatting preview shows the message as it will look, under what's being written (Ctrl/Cmd+Alt+P).
    const [previewing, setPreviewing] = useState(false);
    const requests = useRef<Record<string, XMLHttpRequest>>({});
    const uploadHooks = useSelector((state: GlobalState) => state.plugins.components.FilesWillUploadHook) || NO_HOOKS;
    const editorActions = useSelector((state: GlobalState) => state.plugins.components.PostEditorAction) || NO_EDITOR_ACTIONS;
    const aiItems = useSelector((state: GlobalState) => state.plugins.components.AIActionMenuItem) || NO_AI_ITEMS;

    // Plugins can change the placeholder (registerComposerPlaceholder).
    const channelPlaceholder = useSelector((state: GlobalState) => getComposerPlaceholder(state, channelId, placeholder, intl));
    const ctrlSend = useSelector((state: GlobalState) => getBool(state, Constants.Preferences.CATEGORY_ADVANCED_SETTINGS, 'send_on_ctrl_enter', false));
    const inlineReplies = useInlineRepliesEnabled();
    const mentionByDefault = useSelector((state: GlobalState) => getBool(state, MENTION_PREFERENCE_CATEGORY, MENTION_PREFERENCE_NAME, true));
    const currentUserId = useSelector(getCurrentUserId);
    const {draft, setDraft, save, cancel: cancelSave, key} = useDraftAutosave(channelId, rootId);
    const [showFormatting, setShowFormatting] = useShowFormatting();
    const [pending, setPending] = useState<Pending[]>([]);
    const [menu, setMenu] = useState<'plus' | 'emoji' | 'options' | 'more' | 'schedule' | 'ai' | null>(null);
    const textareaRef = useRef<HTMLTextAreaElement>(null);

    // The rich text editor (beta) shows the formatting as you write, in place of the plain text box, when chosen in
    // the settings; the draft stays Markdown either way.
    const richText = useSelector(getWysiwygEditorPreference);
    const richRef = useRef<WysiwygEditorHandle>(null);
    const fileRef = useRef<HTMLInputElement>(null);
    const plusRef = useRef<HTMLButtonElement>(null);
    const emojiRef = useRef<HTMLButtonElement>(null);

    // The message's options open from their button, or from the chip of one of them (on that one), above the button:
    // a chip goes when its option is turned off.
    const optionsRef = useRef<HTMLButtonElement>(null);
    const [optionsSection, setOptionsSection] = useState<OptionSection | undefined>();

    // AI rewriting, once an admin picked a model connection for it: busy while it writes, and the text before it, to
    // undo.
    // The menu also holds apps' AI items (the AI chat app's prompts), which don't need it.
    const rewriteAvailable = useAIRewriteAvailable();
    const aiAvailable = rewriteAvailable || aiItems.length > 0;
    const aiRef = useRef<HTMLButtonElement>(null);
    const [aiBusy, setAiBusy] = useState(false);
    const [aiOriginal, setAiOriginal] = useState<string | null>(null);

    // The schedule picker opens from the send button's chevron ("Send later"), next to which it shows.
    const sendLaterRef = useRef<HTMLButtonElement>(null);
    const moreRef = useRef<HTMLButtonElement>(null);
    const autocompleteRef = useRef<AutocompleteHandle>(null);
    const formRef = useRef<HTMLFormElement>(null);
    const [narrow, setNarrow] = useState(false);
    useEffect(() => {
        const el = formRef.current;
        if (!el || typeof ResizeObserver === 'undefined') {
            return undefined;
        }
        const check = () => setNarrow(el.clientWidth > 0 && el.clientWidth < NARROW_COMPOSER);
        check();
        const observer = new ResizeObserver(check);
        observer.observe(el);
        return () => observer.disconnect();
    }, []);

    // Apps' composer modes (a poll…): the one the composer is in, if any, and what Send does in it. A mode doesn't
    // follow the conversation, nor stay once its app is gone.
    const modes = useComposerModes(channelId, rootId);
    const [modeId, setModeId] = useState<string | null>(null);
    const mode = modeId ? modes.find((m) => m.id === modeId) : undefined;
    const shownPlaceholder = (mode && modeText(mode.placeholder, intl)) || channelPlaceholder;
    const modeSubmit = useRef<((message: string) => Promise<string | void>) | null>(null);
    const [modeBusy, setModeBusy] = useState(false);
    const setModeSubmit = useCallback((fn: ((message: string) => Promise<string | void>) | null) => {
        modeSubmit.current = fn;
    }, []);
    const enterMode = (id: string) => {
        setMenu(null);
        setSendError('');
        modeSubmit.current = null;
        setModeId(id);
        requestAnimationFrame(() => (richRef.current || textareaRef.current)?.focus());
    };
    const leaveMode = () => {
        setModeId(null);
        setSendError('');
        modeSubmit.current = null;
        requestAnimationFrame(() => (richRef.current || textareaRef.current)?.focus());
    };
    const enterModeRef = useRef(enterMode);
    enterModeRef.current = enterMode;
    useEffect(() => {
        const onMode = (e: Event) => {
            const detail = (e as CustomEvent<ComposerModeEventDetail>).detail;
            if (detail?.id && detail.channelId === channelId && (detail.rootId || '') === rootId) {
                enterModeRef.current(detail.id);
            }
        };
        window.addEventListener(COMPOSER_MODE_EVENT, onMode);
        return () => window.removeEventListener(COMPOSER_MODE_EVENT, onMode);
    }, [channelId, rootId]);

    // The draft switches with the conversation (useDraftAutosave); the uploads in progress don't follow it.
    useEffect(() => {
        setPending([]);
        setModeId(null);
    }, [channelId, rootId]);

    const grow = useCallback(() => {
        const el = textareaRef.current;
        if (el) {
            el.style.height = 'auto';
            el.style.height = Math.min(el.scrollHeight, 220) + 'px';
        }
    }, []);
    useEffect(grow, [draft.message, grow]);

    // Its height follows its width too: a window opened or resized after it was measured (a popout) wraps the text
    // differently.
    useEffect(() => {
        const el = textareaRef.current;
        if (!el || typeof ResizeObserver === 'undefined') {
            return undefined;
        }
        let width = el.clientWidth;
        const observer = new ResizeObserver(() => {
            if (el.clientWidth !== width) {
                width = el.clientWidth;
                grow();
            }
        });
        observer.observe(el);
        return () => observer.disconnect();
    }, [grow]);

    const change = (patch: Partial<PostDraft>) => {
        if (sendError && 'message' in patch) {
            setSendError('');
        }
        save({...draft, channelId, rootId, createAt: draft.createAt || Date.now(), ...patch});
    };

    // An inline reply quotes the message in the draft's reply_to prop, which the message is sent with, and notifies
    // its author unless the reply_to_mention prop says not to: the reply bar's switch, which starts as last set.
    const replyTo = inlineReplies ? replyToId(draft) : '';
    const quoted = useQuoted(replyTo, undefined);
    const setReplyTo = (postId: string) => change({props: replyProps(draft.props, postId, mentionByDefault)});
    const setMention = (on: boolean) => {
        change({props: replyProps(draft.props, replyTo, on)});
        dispatch(savePreferences(currentUserId, [{user_id: currentUserId, category: MENTION_PREFERENCE_CATEGORY, name: MENTION_PREFERENCE_NAME, value: String(on)}]));
    };

    // "Reply" on a message of this conversation quotes it here.
    const setReplyToRef = useRef(setReplyTo);
    setReplyToRef.current = setReplyTo;
    useEffect(() => {
        const onReply = (e: Event) => {
            const detail = (e as CustomEvent<ReplyEventDetail>).detail;
            if (detail.channelId === channelId && detail.rootId === rootId) {
                setReplyToRef.current(detail.postId);
                (richRef.current || textareaRef.current)?.focus();
            }
        };
        window.addEventListener(REPLY_EVENT, onReply);
        return () => window.removeEventListener(REPLY_EVENT, onReply);
    }, [channelId, rootId]);

    // Shift+Up in an empty composer quotes the message above the one quoted (the last one at first), and Shift+Down the
    // one below, or none past the last.
    const moveReplyTo = (older: boolean) => {
        const candidates = replyCandidates(store.getState(), channelId, rootId);
        const at = replyTo ? candidates.indexOf(replyTo) : -1;
        let next = '';
        if (older) {
            next = candidates[at + 1] || (at === -1 ? '' : replyTo);
        } else if (at > 0) {
            next = candidates[at - 1];
        }
        setReplyTo(next);
    };

    // In a mode, Send is the mode's: what it sends takes the text, and the composer goes back to a plain message. The
    // rest of the draft (files, options) stays for the next one.
    const submitMode = async () => {
        if (modeBusy || !modeSubmit.current) {
            return;
        }
        setSendError('');
        setModeBusy(true);
        let error: string | void;
        try {
            error = await modeSubmit.current(draft.message);
        } catch (err) {
            error = (err as Error)?.message || formatMessage({id: 'fusion.composer.sendFailed', defaultMessage: 'The message couldn\'t be sent.'});
        }
        setModeBusy(false);
        if (error) {
            setSendError(error);
            return;
        }
        change({message: ''});
        leaveMode();
    };

    // sendDraft sends the draft, now or at a scheduled time, once a message to a whole channel of many people (@all,
    // @channel, @here) is confirmed. The composer empties at once, the draft goes (on the server too, or it would come
    // back from there), and the composer gets it back if sending fails.
    const sendDraft = async ({after, schedulingInfo}: {after?: Props['afterSubmit']; schedulingInfo?: SchedulingInfo} = {}): Promise<{sent: boolean; error?: {message?: string}}> => {
        const {data: confirmed} = await dispatch(confirmChannelWideMentions(channelId, draft.message));
        if (!confirmed) {
            requestAnimationFrame(() => textareaRef.current?.focus());
            return {sent: false};
        }
        const toSend: PostDraft = {...draft, channelId, rootId, props: extraProps ? {...draft.props, ...extraProps} : draft.props};
        const empty: PostDraft = {message: '', fileInfos: [], uploadsInProgress: [], channelId, rootId, createAt: 0, updateAt: 0};
        setDraft(empty);
        cancelSave();
        dispatch(removeDraft(key, channelId, rootId));
        const result = await dispatch(onSubmit(channelId, rootId, toSend, after ? {afterSubmit: after} : {}, schedulingInfo));
        if (result && 'error' in result && result.error) {
            setDraft(toSend);
            return {sent: false, error: result.error as {message?: string}};
        }
        return {sent: true};
    };

    const submit = async () => {
        if (mode) {
            await submitMode();
            return;
        }
        const message = draft.message;
        if (!message.trim() && !draft.fileInfos.length) {
            return;
        }
        if (pending.length) {
            return;
        }
        if (message.length > maxPostSize) {
            setSendError(formatMessage({id: 'fusion.composer.tooLong', defaultMessage: 'This message is {over, plural, one {# character} other {# characters}} too long: the limit is {max}.'}, {over: message.length - maxPostSize, max: maxPostSize}));
            return;
        }
        setSendError('');
        const {error} = await sendDraft({after: afterSubmit});
        if (error) {
            setSendError(error.message || formatMessage({id: 'fusion.composer.sendFailed', defaultMessage: 'The message couldn\'t be sent.'}));
        }
    };

    // Scheduling sends the draft later instead of now (Mattermost's scheduled messages).
    const schedule = async (at: number, sleeper?: {name: string; time: string}) => {
        setMenu(null);
        if (!draft.message.trim() && !draft.fileInfos.length) {
            toast(formatMessage({id: 'fusion.toast.scheduleEmpty', defaultMessage: 'Write your message first, then schedule it'}));
            textareaRef.current?.focus();
            return;
        }
        if (pending.length) {
            return;
        }
        const {sent, error} = await sendDraft({schedulingInfo: {scheduled_at: at}});
        if (error) {
            toast(formatMessage({id: 'fusion.toast.scheduleFailed', defaultMessage: 'The message could not be scheduled'}));
        }
        if (!sent) {
            return;
        }
        const when = intl.formatDate(at, {weekday: 'short', hour: 'numeric', minute: '2-digit'});
        if (sleeper) {
            const hours = Math.max(1, Math.round((at - Date.now()) / 36e5));
            toast(formatMessage({id: 'fusion.toast.scheduledSleeper', defaultMessage: 'Scheduled — {name} gets it at {time} their time (in about {hours} h)'}, {name: sleeper.name, time: sleeper.time, hours}));
        } else {
            toast(formatMessage({id: 'fusion.toast.scheduled', defaultMessage: 'Scheduled for {when}'}, {when}));
        }
    };

    // The messages you sent, for Ctrl/Cmd+Up and Down; a new message starts after the last one.
    const sentHistory = useSelector((state: GlobalState) => state.entities.posts.messagesHistory.messages);
    const historyIndex = useRef(sentHistory.length);
    useEffect(() => {
        historyIndex.current = sentHistory.length;
    }, [sentHistory.length]);

    const selectedText = () => ({start: textareaRef.current?.selectionStart ?? draft.message.length, end: textareaRef.current?.selectionEnd ?? draft.message.length});

    const onKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
        if (autocompleteRef.current?.handleKeyDown(e)) {
            return;
        }

        if (e.key === 'Escape' && replyTo) {
            e.preventDefault();
            e.stopPropagation();
            setReplyTo('');
            return;
        }
        if (e.key === 'Escape' && mode) {
            e.preventDefault();
            e.stopPropagation();
            leaveMode();
            return;
        }
        if (inlineReplies && (e.key === 'ArrowUp' || e.key === 'ArrowDown') && e.shiftKey && !draft.message && !e.altKey && !e.ctrlKey && !e.metaKey) {
            e.preventDefault();
            moveReplyTo(e.key === 'ArrowUp');
            return;
        }

        // Up in an empty composer edits your last message, as in the classic web app.
        if (e.key === 'ArrowUp' && !draft.message && !e.shiftKey && !e.altKey && !e.ctrlKey && !e.metaKey) {
            e.preventDefault();
            dispatch(editLatestPost(channelId, rootId));
            return;
        }

        // Shift+Up replies to the latest message in its thread (inline replies have their own above).
        if (e.key === 'ArrowUp' && e.shiftKey && !draft.message && !rootId && !inlineReplies && !e.altKey && !e.ctrlKey && !e.metaKey) {
            e.preventDefault();
            dispatch(replyToLatestPostInChannel(channelId));
            return;
        }

        // Ctrl/Cmd+Up and Down go through the messages you sent, while the composer is empty or holds one of them.
        const withCtrlOnly = (e.ctrlKey || e.metaKey) && !e.shiftKey && !e.altKey;
        if (withCtrlOnly && (e.key === 'ArrowUp' || e.key === 'ArrowDown') && (!draft.message || draft.message === sentHistory[historyIndex.current])) {
            e.preventDefault();
            const next = historyIndex.current + (e.key === 'ArrowUp' ? -1 : 1);
            if (next < 0 || next > sentHistory.length) {
                return;
            }
            historyIndex.current = next;
            change({message: sentHistory[next] || ''});
            return;
        }

        if ((e.ctrlKey || e.metaKey) && e.altKey && (e.key.toLowerCase() === 'p' || e.code === 'KeyP')) {
            e.preventDefault();
            setPreviewing(!previewing);
            return;
        }

        // Ctrl/Cmd+Shift+\ reacts to the latest message of the conversation.
        if ((e.ctrlKey || e.metaKey) && e.shiftKey && (e.key === '\\' || e.key === '|' || e.code === 'Backslash')) {
            e.preventDefault();
            const scope = rootId ? textareaRef.current?.closest('.am-rhs, .am-thread') : document.getElementById('am-view');
            const messages = scope?.querySelectorAll<HTMLElement>('.am-msg:not(.am-sysline)');
            messages?.[messages.length - 1]?.querySelector<HTMLButtonElement>('.am-react-btn')?.click();
            return;
        }
        if (e.key === 'Enter' && !e.nativeEvent.isComposing) {
            const withCtrl = e.ctrlKey || e.metaKey;
            if ((ctrlSend && withCtrl) || (!ctrlSend && !e.shiftKey && !withCtrl && !e.altKey)) {
                e.preventDefault();
                submit();
            }
        }
    };

    // The rich text editor sends and edits the last message itself; these keys are the composer's.
    const submitRef = useRef(submit);
    submitRef.current = submit;

    // Antimatter forums: the New topic form sends the current draft from outside the composer (Ctrl/Cmd+Enter in
    // its title field).
    useEffect(() => {
        onReady?.(() => submitRef.current());

        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, []);
    const onRichKeyDown = (e: React.KeyboardEvent<HTMLDivElement>) => {
        if (e.key === 'Escape' && replyTo) {
            e.preventDefault();
            e.stopPropagation();
            setReplyTo('');
        } else if (e.key === 'Escape' && mode) {
            e.preventDefault();
            e.stopPropagation();
            leaveMode();
        } else if ((e.ctrlKey || e.metaKey) && e.altKey && (e.key.toLowerCase() === 'p' || e.code === 'KeyP')) {
            e.preventDefault();
            setPreviewing(!previewing);
        }
    };

    const insert = (text: string) => {
        if (richRef.current) {
            richRef.current.insertText(text);
            return;
        }
        const el = textareaRef.current;
        const start = el?.selectionStart ?? draft.message.length;
        const end = el?.selectionEnd ?? draft.message.length;
        const value = draft.message.slice(0, start) + text + draft.message.slice(end);
        change({message: value});
        requestAnimationFrame(() => {
            el?.focus();
            el?.setSelectionRange(start + text.length, start + text.length);
        });
    };

    // "Mention" in a person's menu writes @username into the conversation's composer.
    const insertRef = useRef(insert);
    insertRef.current = insert;
    useEffect(() => {
        if (rootId) {
            return undefined;
        }
        const onMention = (e: Event) => insertRef.current((e as CustomEvent<string>).detail);
        window.addEventListener(MENTION_EVENT, onMention);
        return () => window.removeEventListener(MENTION_EVENT, onMention);
    }, [rootId]);

    const format = (id: string) => {
        const f = FORMATS.find((x) => x && x.id === id);
        const editor = richRef.current?.getEditor();
        if (editor) {
            formatRich(editor, id);
            return;
        }
        const el = textareaRef.current;
        if (!f || !el) {
            return;
        }
        const next = applyFormat(draft.message, el.selectionStart, el.selectionEnd, f);
        change({message: next.value});
        requestAnimationFrame(() => {
            el.focus();
            el.setSelectionRange(next.start, next.end);
        });
    };

    // Uploads check the server's limits first, as the classic editor: the size of each file, and the files per message.
    const upload = (picked: FileList | File[]) => {
        // Plugins see the files first (registerFilesWillUploadHook): they can change them, or refuse with a message.
        let files = Array.from(picked);
        for (const h of uploadHooks) {
            const result = h.hook?.(files, (more: File[]) => uploadRef.current(more));
            if (result?.message) {
                toast(result.message);
            }
            files = result?.files || [];
        }
        let room = Constants.MAX_UPLOAD_FILES - draft.fileInfos.length - pending.length;
        files.forEach((file) => {
            if (maxFileSize && file.size > maxFileSize) {
                toast(formatMessage({id: 'fusion.composer.tooBig', defaultMessage: '{name} is too big: files can be up to {size}'}, {name: file.name, size: getFormattedFileSize(maxFileSize)}));
                return;
            }
            if (room <= 0) {
                toast(formatMessage({id: 'fusion.composer.tooMany', defaultMessage: 'A message can carry up to {count} files'}, {count: Constants.MAX_UPLOAD_FILES}));
                return;
            }
            room -= 1;
            const clientId = generateId();
            setPending((p) => [...p, {clientId, name: file.name, progress: 0}]);
            requests.current[clientId] = dispatch(uploadFile({
                file,
                name: file.name,
                type: file.type,
                rootId,
                channelId,
                clientId,
                onProgress: (info) => setPending((p) => p.map((x) => (x.clientId === clientId ? {...x, progress: info.percent || 0} : x))),
                onSuccess: (data: {file_infos: FileInfo[]}) => {
                    setPending((p) => p.filter((x) => x.clientId !== clientId));
                    setDraft((d) => {
                        const next = {...d, fileInfos: [...d.fileInfos, ...data.file_infos]};
                        dispatch(updateDraft(key, next, rootId, true));
                        return next;
                    });
                },
                onError: (error: string | {message?: string}) => {
                    setPending((p) => p.filter((x) => x.clientId !== clientId));
                    const message = typeof error === 'string' ? error : error?.message;
                    if (message) {
                        toast(formatMessage({id: 'fusion.composer.uploadFailed', defaultMessage: '{name} couldn\'t be uploaded: {message}'}, {name: file.name, message}));
                    }
                },
            })) as unknown as XMLHttpRequest;
        });
    };
    const cancelUpload = (clientId: string) => {
        requests.current[clientId]?.abort();
        delete requests.current[clientId];
        setPending((p) => p.filter((x) => x.clientId !== clientId));
    };

    // Files dropped anywhere on the conversation go to its composer, as in the mockup.
    const uploadRef = useRef(upload);
    uploadRef.current = upload;
    useEffect(() => {
        if (rootId) {
            return undefined;
        }
        const onDrop = (e: Event) => uploadRef.current((e as CustomEvent<File[]>).detail);
        window.addEventListener(DROP_FILES_EVENT, onDrop);
        return () => window.removeEventListener(DROP_FILES_EVENT, onDrop);
    }, [rootId]);

    // Priority, burning on read and spoilers: set in one popover, shown as chips.
    const options = useMessageOptions(draft, change, channelId, rootId);
    const openOptions = (section?: OptionSection) => {
        setOptionsSection(section);
        setMenu(menu === 'options' && !section ? null : 'options');
    };
    const chips = (
        <MessageOptionChips
            options={options}
            onEdit={openOptions}
        />
    );
    const files = draft.fileInfos.map((f) => (
        <span
            key={f.id}
            className={am('opt-chip')}
        >
            <Icon name='attach'/>
            {f.name}
            <button
                type='button'
                aria-label={formatMessage({id: 'fusion.composer.removeFile', defaultMessage: 'Remove {name}'}, {name: f.name})}
                onClick={() => change({fileInfos: draft.fileInfos.filter((x) => x.id !== f.id)})}
            >
                <Icon name='x'/>
            </button>
        </span>
    ));
    const uploads = pending.map((p) => (
        <span
            key={p.clientId}
            className={am('opt-chip', 'uploading')}
            style={{'--am-progress': `${Math.round(p.progress)}%`} as React.CSSProperties}
        >
            <Icon name='upload'/>
            {`${p.name} · ${Math.round(p.progress)}%`}
            <button
                type='button'
                aria-label={formatMessage({id: 'fusion.composer.cancelUpload', defaultMessage: 'Cancel the upload of {name}'}, {name: p.name})}
                onClick={() => cancelUpload(p.clientId)}
            >
                <Icon name='x'/>
            </button>
        </span>
    ));

    let group = 0;
    return (
        <form
            ref={formRef}
            {...a11yRegion(am('composer'), 'composer')}

            // Reaching the composer with F6 goes on to its text box.
            onFocus={(e) => {
                if (e.target === e.currentTarget) {
                    (richRef.current || textareaRef.current)?.focus();
                }
            }}
            onSubmit={(e) => {
                e.preventDefault();
                submit();
            }}
            onDragOver={(e) => e.preventDefault()}
            onDrop={(e) => {
                if (e.dataTransfer.files.length) {
                    e.preventDefault();
                    upload(e.dataTransfer.files);
                }
            }}
        >
            {!richText && (
                <Autocomplete
                    ref={autocompleteRef}
                    channelId={channelId}
                    rootId={rootId}
                    value={draft.message}
                    textareaRef={textareaRef}
                    onChange={(message) => change({message})}
                />
            )}
            {!rootId && <AwayNote channelId={channelId}/>}
            {!rootId && (
                <SleepNote
                    channelId={channelId}
                    onSendAt={schedulingEnabled ? schedule : undefined}
                />
            )}
            {schedulingEnabled && <ScheduledNote id={rootId || channelId}/>}
            {replyTo && (
                <div className={am('replying')}>
                    <QuotedMessage quoted={quoted}/>
                    <MentionSwitch
                        quoted={quoted}
                        on={mentionsQuotedAuthor(draft)}
                        onChange={setMention}
                    />
                    <button
                        type='button'
                        title={formatMessage({id: 'fusion.composer.cancelReply', defaultMessage: 'Cancel reply'})}
                        aria-label={formatMessage({id: 'fusion.composer.cancelReply', defaultMessage: 'Cancel reply'})}
                        onClick={() => {
                            setReplyTo('');
                            (richRef.current || textareaRef.current)?.focus();
                        }}
                    >
                        <Icon
                            name='x'
                            size='sm'
                        />
                    </button>
                </div>
            )}
            <div className={am('compose-box', {'in-mode': Boolean(mode)})}>
                {mode && (
                    <ComposerModeBar
                        mode={mode}
                        onCancel={leaveMode}
                    />
                )}
                {!mode && (options.active || files.length > 0 || uploads.length > 0) && <div className={am('opt-chips')}>{chips}{files}{uploads}</div>}
                {previewing && draft.message.trim() && (
                    <div
                        className={am('md-preview')}
                        aria-label={formatMessage({id: 'fusion.composer.previewLabel', defaultMessage: 'Show how the message will look'})}
                    >
                        <Markdown message={draft.message}/>
                    </div>
                )}
                {aiBusy && <div className={am('ai-note', 'busy')}>{formatMessage({id: 'fusion.ai.writing', defaultMessage: 'The AI is rewriting your message…'})}</div>}
                {aiOriginal !== null && !aiBusy && draft.message && (
                    <div className={am('ai-note')}>
                        <Icon
                            name='sparkle'
                            size='sm'
                        />
                        <span>{formatMessage({id: 'fusion.ai.rewritten', defaultMessage: 'Rewritten by AI'})}</span>
                        <button
                            type='button'
                            onClick={() => {
                                change({message: aiOriginal});
                                setAiOriginal(null);
                            }}
                        >
                            {formatMessage({id: 'fusion.ai.undo', defaultMessage: 'Undo'})}
                        </button>
                    </div>
                )}
                {sendError && (
                    <div
                        className={am('send-error')}
                        role='alert'
                    >
                        {sendError}
                    </div>
                )}
                {draft.message.length > maxPostSize * 0.9 && (
                    <span
                        className={am('len-count', {over: draft.message.length > maxPostSize})}
                        title={formatMessage({id: 'fusion.composer.lengthTitle', defaultMessage: 'Messages can be up to {max} characters'}, {max: maxPostSize})}
                    >
                        {`${draft.message.length} / ${maxPostSize}`}
                    </span>
                )}
                <div className={am('compose-row')}>
                    {!mode && (
                        <button
                            ref={plusRef}
                            type='button'
                            className={am('plus-btn')}
                            aria-label={formatMessage({id: 'fusion.composer.plus', defaultMessage: 'Add a file and more'})}
                            aria-haspopup='menu'
                            aria-expanded={menu === 'plus'}
                            onClick={() => setMenu(menu === 'plus' ? null : 'plus')}
                        >
                            <span><Icon name='plus'/></span>
                        </button>
                    )}
                    {richText && (
                        <div
                            className={am('rich-input')}

                            // Pasted files are attached, as in the text box, and the editor doesn't get the paste:
                            // it would add the text that comes with them (a file's name) or show an image inline.
                            onPasteCapture={(e) => {
                                if (e.clipboardData.files.length) {
                                    e.preventDefault();
                                    e.stopPropagation();
                                    upload(e.clipboardData.files);
                                }
                            }}
                        >
                            <WysiwygEditor
                                key={rootId || channelId}
                                ref={richRef}
                                value={draft.message}
                                placeholder={shownPlaceholder}
                                channelId={channelId}
                                rootId={rootId}
                                useCtrlSend={ctrlSend}
                                onChange={(message) => {
                                    change({message});
                                    emitLocalUserTypingEvent(channelId, rootId);
                                }}
                                onSubmit={() => submitRef.current()}
                                onKeyDown={onRichKeyDown}
                            />
                        </div>
                    )}
                    {!richText && (
                        <textarea
                            ref={textareaRef}
                            rows={1}
                            value={draft.message}
                            placeholder={shownPlaceholder}
                            aria-label={shownPlaceholder}
                            onChange={(e) => {
                                change({message: e.target.value});

                                // Others see that we're typing, as often as the server allows.
                                emitLocalUserTypingEvent(channelId, rootId);
                            }}
                            onKeyDown={onKeyDown}
                            onPaste={(e) => {
                                if (e.clipboardData.files.length) {
                                    e.preventDefault();
                                    upload(e.clipboardData.files);
                                }
                            }}
                        />
                    )}
                    <button
                        type='button'
                        className={am('icon-btn', {on: showFormatting})}
                        aria-pressed={showFormatting}
                        title={showFormatting ? formatMessage({id: 'fusion.composer.hideFormatting', defaultMessage: 'Hide formatting'}) : formatMessage({id: 'fusion.composer.showFormatting', defaultMessage: 'Show formatting'})}
                        aria-label={formatMessage({id: 'fusion.composer.showFormatting', defaultMessage: 'Show formatting'})}
                        onClick={() => setShowFormatting(!showFormatting)}
                    >
                        <Icon name='type'/>
                    </button>
                    <button
                        ref={emojiRef}
                        type='button'
                        className={am('icon-btn')}
                        title={formatMessage({id: 'fusion.composer.emoji', defaultMessage: 'Emoji'})}
                        aria-label={formatMessage({id: 'fusion.composer.emoji', defaultMessage: 'Emoji'})}
                        aria-haspopup='dialog'
                        onClick={() => setMenu(menu === 'emoji' ? null : 'emoji')}
                    >
                        <Icon name='smile'/>
                    </button>
                    {!mode && !narrow && (
                        <button
                            ref={optionsRef}
                            type='button'
                            className={am('icon-btn', 'opts-btn', {on: options.active})}
                            title={formatMessage({id: 'fusion.composer.options', defaultMessage: 'Message options'})}
                            aria-label={formatMessage({id: 'fusion.composer.options', defaultMessage: 'Message options'})}
                            aria-haspopup='dialog'
                            aria-expanded={menu === 'options'}
                            onClick={() => openOptions()}
                        >
                            <Icon name='sliders'/>
                        </button>
                    )}
                    <SendButton
                        disabled={pending.length > 0 || modeBusy}
                        label={mode ? modeText(mode.sendLabel, intl) : undefined}
                        laterRef={sendLaterRef}
                        laterOpen={menu === 'schedule'}
                        onLater={schedulingEnabled && !mode ? () => setMenu(menu === 'schedule' ? null : 'schedule') : undefined}
                    />
                </div>
                {mode && (
                    <ComposerModeBody
                        mode={mode}
                        channelId={channelId}
                        rootId={rootId || undefined}
                        message={draft.message}
                        setSubmit={setModeSubmit}
                        onCancel={leaveMode}
                    />
                )}
                {showFormatting && (
                    <div
                        className={am('fmt-bar')}

                        // The buttons leave the focus in the text, where the format applies and the writing goes on.
                        onMouseDown={(e) => e.preventDefault()}
                        role='toolbar'
                        aria-label={formatMessage({id: 'fusion.composer.formattingBar', defaultMessage: 'Formatting'})}
                    >
                        {aiAvailable && (
                            <>
                                <button
                                    ref={aiRef}
                                    type='button'
                                    className={am('ai-btn', {on: menu === 'ai' || aiBusy})}
                                    title={formatMessage({id: 'fusion.ai.label', defaultMessage: 'Rewrite with AI'})}
                                    aria-label={formatMessage({id: 'fusion.ai.label', defaultMessage: 'Rewrite with AI'})}
                                    aria-haspopup='menu'
                                    disabled={aiBusy}
                                    onClick={() => setMenu(menu === 'ai' ? null : 'ai')}
                                >
                                    <Icon
                                        name='sparkle'
                                        size='sm'
                                    />
                                    {formatMessage({id: 'fusion.ai.button', defaultMessage: 'AI'})}
                                </button>
                                <span className={am('fmt-sep')}/>
                            </>
                        )}
                        {FORMATS.map((f, i) => {
                            if (!f) {
                                group += 1;
                                return (
                                    <span
                                        key={`sep-${i}`}
                                        className={am('fmt-sep', `g${group}`)}
                                    />
                                );
                            }
                            return (
                                <button
                                    key={f.id}
                                    type='button'
                                    className={am(`g${group}`)}
                                    title={formatMessage(f.label)}
                                    aria-label={formatMessage(f.label)}
                                    onClick={() => format(f.id)}
                                >
                                    <Icon name={f.id}/>
                                </button>
                            );
                        })}
                        <button
                            ref={moreRef}
                            type='button'
                            className={am('fmt-more')}
                            title={formatMessage({id: 'fusion.composer.moreFormatting', defaultMessage: 'More formatting'})}
                            aria-label={formatMessage({id: 'fusion.composer.moreFormatting', defaultMessage: 'More formatting'})}
                            aria-haspopup='menu'
                            onClick={() => setMenu(menu === 'more' ? null : 'more')}
                        >
                            <Icon name='dots'/>
                        </button>
                        <button
                            type='button'
                            className={am({on: previewing})}
                            title={formatMessage({id: 'fusion.composer.preview', defaultMessage: 'Preview (Ctrl+Alt+P)'})}
                            aria-label={formatMessage({id: 'fusion.composer.previewLabel', defaultMessage: 'Show how the message will look'})}
                            aria-pressed={previewing}
                            onClick={() => setPreviewing(!previewing)}
                        >
                            <Icon name='text'/>
                        </button>
                        <button
                            type='button'
                            className={am('fmt-help')}
                            title={formatMessage({id: 'fusion.composer.help', defaultMessage: 'Help with formatting, mentions, files and commands'})}
                            aria-label={formatMessage({id: 'fusion.composer.help', defaultMessage: 'Help with formatting, mentions, files and commands'})}
                            onClick={() => dialogs.help('messaging')}
                        >
                            {'?'}
                        </button>
                        {editorActions.map((item) => {
                            const Component = item.component as React.ComponentType<Record<string, unknown>> | undefined;
                            return Component ? (
                                <PluggableErrorBoundary
                                    key={item.id}
                                    pluginId={item.pluginId}
                                >
                                    <Component
                                        draft={draft}
                                        getSelectedText={selectedText}
                                        updateText={(message: string) => change({message})}
                                    />
                                </PluggableErrorBoundary>
                            ) : null;
                        })}
                    </div>
                )}
            </div>
            <Typing
                channelId={channelId}
                rootId={rootId}
            />
            <input
                ref={fileRef}
                type='file'
                multiple={true}
                hidden={true}
                onChange={(e) => {
                    if (e.target.files) {
                        upload(e.target.files);
                    }
                    e.target.value = '';
                }}
            />
            {menu === 'plus' && (
                <PlusMenu
                    anchor={plusRef.current}
                    channelId={channelId}
                    rootId={rootId}
                    modes={modes}
                    onMode={enterMode}
                    onOptions={narrow ? () => openOptions() : undefined}
                    onPickFile={() => fileRef.current?.click()}
                    onUpload={upload}
                    onClose={() => setMenu(null)}
                />
            )}
            {menu === 'ai' && (
                <AiRewriteMenu
                    anchor={aiRef.current}
                    canRewrite={rewriteAvailable}
                    pluginItems={aiItems}
                    pluginProps={{draft, getSelectedText: selectedText, updateText: (message: string) => change({message}), channelId, isRHS: Boolean(rootId)}}
                    message={draft.message}
                    rootId={rootId}
                    onRewritten={(message, original) => {
                        change({message});
                        setAiOriginal(original);
                        requestAnimationFrame(() => textareaRef.current?.focus());
                    }}
                    onError={setSendError}
                    onBusy={setAiBusy}
                    onClose={() => setMenu(null)}
                />
            )}
            {menu === 'schedule' && (
                <SchedulePopover
                    anchor={sendLaterRef.current}
                    onSchedule={(at) => schedule(at)}
                    onClose={() => setMenu(null)}
                />
            )}
            {menu === 'emoji' && (
                <EmojiPicker
                    anchor={emojiRef.current}
                    keepOpen={true}
                    compose={{channelId, rootId, insertText: insert}}
                    onPick={(name) => insert(`:${name}: `)}
                    onClose={() => setMenu(null)}
                />
            )}
            {menu === 'more' && (
                <Popover
                    anchor={moreRef.current}
                    placement='above'
                    className='plus-pop fmt-pop'
                    role='menu'
                    label={formatMessage({id: 'fusion.composer.moreFormatting', defaultMessage: 'More formatting'})}
                    onClose={() => setMenu(null)}
                >
                    <div className={am('fmt-grid')}>
                        {FORMATS.filter((f, i) => f && FORMATS.slice(0, i).filter((x) => x === null).length >= 2).map((f) => (
                            <button
                                key={f!.id}
                                type='button'
                                role='menuitem'
                                title={formatMessage(f!.label)}
                                aria-label={formatMessage(f!.label)}
                                onClick={() => format(f!.id)}
                            >
                                <Icon name={f!.id}/>
                            </button>
                        ))}
                    </div>
                </Popover>
            )}
            {menu === 'options' && (
                <MessageOptionsPopover
                    key={optionsSection || 'all'}
                    options={options}
                    anchor={narrow ? plusRef.current : optionsRef.current}
                    section={optionsSection}
                    onClose={() => setMenu(null)}
                />
            )}
        </form>
    );
}
