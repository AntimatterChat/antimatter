// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import type {ChangeEventHandler, FormEvent, JSX, MouseEvent} from 'react';
import {FormattedMessage} from 'react-intl';
import type {MessageDescriptor} from 'react-intl';
import {Link} from 'react-router-dom';

import {buttonClassNames} from '@mattermost/shared/components/button';
import type {OutgoingWebhook} from '@mattermost/types/integrations';
import type {Team} from '@mattermost/types/teams';

import {Client4} from 'mattermost-redux/client';

import BackstageHeader from 'components/backstage/components/backstage_header';
import ChannelSelect from 'components/channel_select';
import ExternalLink from 'components/external_link';
import FormError from 'components/form_error';
import SpinnerButton from 'components/spinner_button';

import {DocLinks} from 'utils/constants';
import {localizeMessage} from 'utils/utils';

// The trigger of a message action (Antimatter): the webhook is an action of the message menu, sending the message
// someone picks it for, and never fires on posted messages.
const TRIGGER_MESSAGE_MENU = 2;

// Where an uploaded icon is served from (model.OutgoingWebhookIconPath).
const UPLOADED_ICON_PATH = '/api/v4/hooks/outgoing/';

interface State {
    callbackUrls: string;
    channelId: string;
    clientError: JSX.Element | null;
    contentType: string;
    description: string;
    displayName: string;
    iconURL: string;

    // An icon picked to upload (message actions), and its preview.
    iconFile: File | null;
    iconPreview: string;
    saving: boolean;
    triggerWhen: number;
    triggerWords: string;
    username: string;
}

interface Props {

    /**
     * The current team
     */
    team: Team;

    /**
     * The header text to render, has id and defaultMessage
     */
    header: MessageDescriptor;

    /**
     * The footer text to render, has id and defaultMessage
     */
    footer: MessageDescriptor;

    /**
    * The spinner loading text to render, has id and defaultMessage
    */
    loading: MessageDescriptor;

    /**
     * Any extra component/node to render
     */
    renderExtra: React.ReactNode;

    /**
     * The server error text after a failed action
     */
    serverError: string;

    /**
     * The hook used to set the initial state
     */
    initialHook?: OutgoingWebhook;

    /**
     * The async function to run when the action button is pressed, with the icon picked to upload (message actions)
     */
    action: (hook: OutgoingWebhook, iconFile?: File | null) => Promise<void>;

    /**
     * Whether an icon can be uploaded for a message action (else only its URL can be given)
     */
    canUploadIcon?: boolean;

    /**
     * Whether to allow configuration of the default post username.
     */
    enablePostUsernameOverride: boolean;

    /**
     * Whether to allow configuration of the default post icon.
     */
    enablePostIconOverride: boolean;
}

export default class AbstractOutgoingWebhook extends React.PureComponent<Props, State> {
    constructor(props: Props | Readonly<Props>) {
        super(props);

        this.state = this.getStateFromHook(this.props.initialHook);
    }

    getStateFromHook = (hook?: OutgoingWebhook) => {
        let triggerWords = '';
        if (hook?.trigger_words) {
            let i = 0;
            for (i = 0; i < hook.trigger_words.length; i++) {
                triggerWords += hook.trigger_words[i] + '\n';
            }
        }

        let callbackUrls = '';
        if (hook?.callback_urls) {
            let i = 0;
            for (i = 0; i < hook.callback_urls.length; i++) {
                callbackUrls += hook.callback_urls[i] + '\n';
            }
        }

        return {
            displayName: hook?.display_name || '',
            description: hook?.description || '',
            contentType: hook?.content_type || 'application/x-www-form-urlencoded',
            channelId: hook?.channel_id || '',
            triggerWords,
            triggerWhen: hook?.trigger_when || 0,
            callbackUrls,
            saving: false,
            clientError: null,
            username: hook?.username || '',
            iconURL: hook?.icon_url || '',
            iconFile: null,
            iconPreview: '',
        };
    };

    componentWillUnmount() {
        this.revokeIconPreview();
    }

    revokeIconPreview = () => {
        if (this.state.iconPreview && typeof URL.revokeObjectURL === 'function') {
            URL.revokeObjectURL(this.state.iconPreview);
        }
    };

    isMessageAction = () => this.state.triggerWhen === TRIGGER_MESSAGE_MENU;

    handleSubmit = (e: MouseEvent<HTMLElement> | FormEvent<HTMLFormElement>) => {
        e.preventDefault();

        if (this.state.saving) {
            return;
        }

        this.setState({
            saving: true,
            clientError: null,
        });

        const triggerWords = [];
        if (this.state.triggerWords) {
            for (let triggerWord of this.state.triggerWords.split('\n')) {
                triggerWord = triggerWord.trim();

                if (triggerWord.length > 0) {
                    triggerWords.push(triggerWord);
                }
            }
        }

        const messageAction = this.isMessageAction();

        if (messageAction && !this.state.displayName.trim()) {
            this.setState({
                saving: false,
                clientError: (
                    <FormattedMessage
                        id='add_outgoing_webhook.messageAction.nameRequired'
                        defaultMessage="A title is required: it's the action's name in the message menu."
                    />
                ),
            });

            return;
        }

        if (!messageAction && !this.state.channelId && triggerWords.length === 0) {
            this.setState({
                saving: false,
                clientError: (
                    <FormattedMessage
                        id='add_outgoing_webhook.triggerWordsOrChannelRequired'
                        defaultMessage='A valid channel or a list of trigger words is required'
                    />
                ),
            });

            return;
        }

        const callbackUrls = [];
        for (let callbackUrl of this.state.callbackUrls.split('\n')) {
            callbackUrl = callbackUrl.trim();

            if (callbackUrl.length > 0) {
                callbackUrls.push(callbackUrl);
            }
        }

        if (callbackUrls.length === 0) {
            this.setState({
                saving: false,
                clientError: (
                    <FormattedMessage
                        id='add_outgoing_webhook.callbackUrlsRequired'
                        defaultMessage='One or more callback URLs are required'
                    />
                ),
            });

            return;
        }

        const hook = {
            team_id: this.props.team.id,
            channel_id: this.state.channelId,
            trigger_words: messageAction ? [] : triggerWords,
            trigger_when: this.state.triggerWhen,
            callback_urls: callbackUrls,
            display_name: this.state.displayName,
            content_type: this.state.contentType,
            description: this.state.description,
            username: this.state.username,
            icon_url: this.state.iconURL,
            id: this.props.initialHook?.id || '',
            create_at: this.props.initialHook?.create_at || 0,
            update_at: this.props.initialHook?.update_at || 0,
            delete_at: this.props.initialHook?.delete_at || 0,
            creator_id: this.props.initialHook?.creator_id || '',
            token: this.props.initialHook?.token || '',
        };

        this.props.action(hook, messageAction ? this.state.iconFile : null).then(() => this.setState({saving: false}));
    };

    updateDisplayName: ChangeEventHandler<HTMLInputElement> = (e) => {
        this.setState({
            displayName: e.target.value,
        });
    };

    updateDescription: ChangeEventHandler<HTMLInputElement> = (e) => {
        this.setState({
            description: e.target.value,
        });
    };

    updateContentType: ChangeEventHandler<HTMLSelectElement> = (e) => {
        this.setState({
            contentType: e.target.value,
        });
    };

    updateChannelId: ChangeEventHandler<HTMLSelectElement> = (e) => {
        this.setState({
            channelId: e.target.value,
        });
    };

    updateTriggerWords: ChangeEventHandler<HTMLTextAreaElement> = (e) => {
        this.setState({
            triggerWords: e.target.value,
        });
    };

    updateTriggerWhen: ChangeEventHandler<HTMLSelectElement> = (e) => {
        this.setState({
            triggerWhen: parseInt(e.target.value, 10),
        });
    };

    updateCallbackUrls: ChangeEventHandler<HTMLTextAreaElement> = (e) => {
        this.setState({
            callbackUrls: e.target.value,
        });
    };

    updateUsername: ChangeEventHandler<HTMLInputElement> = (e) => {
        this.setState({
            username: e.target.value,
        });
    };

    updateIconURL: ChangeEventHandler<HTMLInputElement> = (e) => {
        this.setState({
            iconURL: e.target.value,
        });
    };

    // A picked icon replaces the icon's URL; it is uploaded once the webhook is saved.
    updateIconFile: ChangeEventHandler<HTMLInputElement> = (e) => {
        const file = e.target.files?.[0] || null;
        this.revokeIconPreview();
        this.setState({
            iconFile: file,
            iconPreview: file && typeof URL.createObjectURL === 'function' ? URL.createObjectURL(file) : '',
            iconURL: file ? '' : this.state.iconURL,
        });
    };

    removeIcon = () => {
        this.revokeIconPreview();
        this.setState({
            iconFile: null,
            iconPreview: '',
            iconURL: '',
        });
    };

    renderMessageActionIcon = () => {
        const uploaded = this.state.iconURL.startsWith(UPLOADED_ICON_PATH);
        const preview = this.state.iconPreview || (uploaded ? Client4.getUrl() + this.state.iconURL : '');
        return (
            <div className='form-group'>
                <label
                    className='control-label col-sm-4'
                    htmlFor='messageActionIconURL'
                >
                    <FormattedMessage
                        id='add_outgoing_webhook.messageAction.icon'
                        defaultMessage='Icon'
                    />
                </label>
                <div className='col-md-5 col-sm-8'>
                    {preview ? (
                        <div className='d-flex align-items-center'>
                            <img
                                src={preview}
                                alt={localizeMessage({id: 'add_outgoing_webhook.messageAction.iconPreview', defaultMessage: 'The icon, in grey as in the message menu'})}
                                width={32}
                                height={32}
                                style={{borderRadius: 6, filter: 'grayscale(1)', marginRight: 12}}
                            />
                            <button
                                type='button'
                                className='btn btn-link'
                                onClick={this.removeIcon}
                            >
                                <FormattedMessage
                                    id='add_outgoing_webhook.messageAction.removeIcon'
                                    defaultMessage='Remove the icon'
                                />
                            </button>
                        </div>
                    ) : (
                        <input
                            id='messageActionIconURL'
                            type='text'
                            maxLength={1024}
                            className='form-control'
                            placeholder='https://'
                            value={this.state.iconURL}
                            onChange={this.updateIconURL}
                        />
                    )}
                    {this.props.canUploadIcon && (
                        <div className='form__help'>
                            <label htmlFor='messageActionIconFile'>
                                <FormattedMessage
                                    id='add_outgoing_webhook.messageAction.upload'
                                    defaultMessage='Or upload an image:'
                                />
                            </label>
                            {' '}
                            <input
                                id='messageActionIconFile'
                                type='file'
                                accept='image/png,image/jpeg,image/gif'
                                onChange={this.updateIconFile}
                            />
                        </div>
                    )}
                    <div className='form__help'>
                        <FormattedMessage
                            id='add_outgoing_webhook.messageAction.iconHelp'
                            defaultMessage="The action's icon in the message menu, shown in grey: the URL of a .png or .jpg image, or an uploaded one. Without an icon, the action gets a neutral one."
                        />
                    </div>
                </div>
            </div>
        );
    };

    render() {
        const contentTypeOption1 = 'application/x-www-form-urlencoded';
        const contentTypeOption2 = 'application/json';

        const headerToRender = this.props.header;
        const footerToRender = this.props.footer;
        const renderExtra = this.props.renderExtra;
        const messageAction = this.isMessageAction();

        return (
            <div className='backstage-content'>
                <BackstageHeader>
                    <Link to={`/${this.props.team.name}/integrations/outgoing_webhooks`}>
                        <FormattedMessage
                            id='add_outgoing_webhook.header'
                            defaultMessage='Outgoing Webhooks'
                        />
                    </Link>
                    <FormattedMessage
                        id={headerToRender.id}
                        defaultMessage={headerToRender.defaultMessage}
                    />
                </BackstageHeader>
                <div className='backstage-form'>
                    <form
                        className='form-horizontal'
                        onSubmit={this.handleSubmit}
                    >
                        <div className='form-group'>
                            <label
                                className='control-label col-sm-4'
                                htmlFor='displayName'
                            >
                                <FormattedMessage
                                    id='add_outgoing_webhook.displayName'
                                    defaultMessage='Title'
                                />
                            </label>
                            <div className='col-md-5 col-sm-8'>
                                <input
                                    id='displayName'
                                    type='text'
                                    maxLength={64}
                                    className='form-control'
                                    value={this.state.displayName}
                                    onChange={this.updateDisplayName}
                                />
                                <div className='form__help'>
                                    <FormattedMessage
                                        id='add_outgoing_webhook.displayName.help'
                                        defaultMessage='Specify a title, of up to 64 characters, for the webhook settings page.'
                                    />
                                </div>
                            </div>
                        </div>
                        <div className='form-group'>
                            <label
                                className='control-label col-sm-4'
                                htmlFor='description'
                            >
                                <FormattedMessage
                                    id='add_outgoing_webhook.description'
                                    defaultMessage='Description'
                                />
                            </label>
                            <div className='col-md-5 col-sm-8'>
                                <input
                                    id='description'
                                    type='text'
                                    maxLength={500}
                                    className='form-control'
                                    value={this.state.description}
                                    onChange={this.updateDescription}
                                />
                                <div className='form__help'>
                                    <FormattedMessage
                                        id='add_outgoing_webhook.description.help'
                                        defaultMessage='Describe your outgoing webhook.'
                                    />
                                </div>
                            </div>
                        </div>
                        <div className='form-group'>
                            <label
                                className='control-label col-sm-4'
                                htmlFor='contentType'
                            >
                                <FormattedMessage
                                    id='add_outgoing_webhook.content_Type'
                                    defaultMessage='Content Type'
                                />
                            </label>
                            <div className='col-md-5 col-sm-8'>
                                <select
                                    className='form-control'
                                    value={this.state.contentType}
                                    onChange={this.updateContentType}
                                >
                                    <option
                                        value={contentTypeOption1}
                                    >
                                        {contentTypeOption1}
                                    </option>
                                    <option
                                        value={contentTypeOption2}
                                    >
                                        {contentTypeOption2}
                                    </option>
                                </select>
                                <div className='form__help'>
                                    <FormattedMessage
                                        id='add_outgoing_webhook.contentType.help1'
                                        defaultMessage='Specify the content type by which to send the request.'
                                    />
                                </div>
                                <div className='form__help'>
                                    <FormattedMessage
                                        id='add_outgoing_webhook.contentType.help2'
                                        defaultMessage='For the server to encode the parameters in a URL format in the request body, select application/x-www-form-urlencoded.'
                                    />
                                </div>
                                <div className='form__help'>
                                    <FormattedMessage
                                        id='add_outgoing_webhook.contentType.help3'
                                        defaultMessage='For the server to format the request body as JSON, select application/json.'
                                    />
                                </div>
                            </div>
                        </div>
                        <div className='form-group'>
                            <label
                                className='control-label col-sm-4'
                                htmlFor='channelId'
                            >
                                <FormattedMessage
                                    id='add_outgoing_webhook.channel'
                                    defaultMessage='Channel'
                                />
                            </label>
                            <div className='col-md-5 col-sm-8'>
                                <ChannelSelect
                                    value={this.state.channelId}
                                    onChange={this.updateChannelId}
                                    selectOpen={true}
                                    selectPrivate={false}
                                    selectDm={false}
                                />
                                <div className='form__help'>
                                    {messageAction ? (
                                        <FormattedMessage
                                            id='add_outgoing_webhook.messageAction.channelHelp'
                                            defaultMessage='Optional. The public channel whose messages get this action in their menu; without one, every public channel of the community does.'
                                        />
                                    ) : (
                                        <FormattedMessage
                                            id='add_outgoing_webhook.channel.help'
                                            defaultMessage='This field is optional if you specify at least one trigger word. Specify the public channel that delivers the payload to the webhook.'
                                        />
                                    )}
                                </div>
                            </div>
                        </div>
                        {!messageAction && (
                            <div className='form-group'>
                                <label
                                    className='control-label col-sm-4'
                                    htmlFor='triggerWords'
                                >
                                    <FormattedMessage
                                        id='add_outgoing_webhook.triggerWords'
                                        defaultMessage='Trigger Words (One Per Line)'
                                    />
                                </label>
                                <div className='col-md-5 col-sm-8'>
                                    <textarea
                                        id='triggerWords'
                                        rows={3}
                                        maxLength={1000}
                                        className='form-control'
                                        value={this.state.triggerWords}
                                        onChange={this.updateTriggerWords}
                                    />
                                    <div className='form__help'>
                                        <FormattedMessage
                                            id='add_outgoing_webhook.triggerWords.help'
                                            defaultMessage='Specify the trigger words that send an HTTP POST request to your application. The trigger can be for the channel, the outgoing webhook, or both. If you select only Channel, trigger words are optional. If you select both, the message must match both values.'
                                        />
                                    </div>
                                </div>
                            </div>
                        )}
                        <div className='form-group'>
                            <label
                                className='control-label col-sm-4'
                                htmlFor='triggerWords'
                            >
                                <FormattedMessage
                                    id='add_outgoing_webhook.triggerWordsTriggerWhen'
                                    defaultMessage='Trigger When'
                                />
                            </label>
                            <div className='col-md-5 col-sm-8'>
                                <select
                                    id='triggerWhen'
                                    className='form-control'
                                    value={this.state.triggerWhen}
                                    onChange={this.updateTriggerWhen}
                                >
                                    <option
                                        value='0'
                                    >
                                        {localizeMessage({id: 'add_outgoing_webhook.triggerWordsTriggerWhenFullWord', defaultMessage: 'First word matches a trigger word exactly'})}
                                    </option>
                                    <option
                                        value='1'
                                    >
                                        {localizeMessage({id: 'add_outgoing_webhook.triggerWordsTriggerWhenStartsWith', defaultMessage: 'First word starts with a trigger word'})}
                                    </option>
                                    <option
                                        value={String(TRIGGER_MESSAGE_MENU)}
                                    >
                                        {localizeMessage({id: 'add_outgoing_webhook.triggerWordsTriggerWhenMessageMenu', defaultMessage: 'From the message menu'})}
                                    </option>
                                </select>
                                <div className='form__help'>
                                    {messageAction ? (
                                        <FormattedMessage
                                            id='add_outgoing_webhook.messageAction.help'
                                            defaultMessage="The webhook doesn't fire on posted messages: it's an action under More actions in the message menu, and sends the message someone picks it for."
                                        />
                                    ) : (
                                        <FormattedMessage
                                            id='add_outgoing_webhook.triggerWordsTriggerWhen.help'
                                            defaultMessage='Specify when to trigger the outgoing webhook.'
                                        />
                                    )}
                                </div>
                            </div>
                        </div>
                        <div className='form-group'>
                            <label
                                className='control-label col-sm-4'
                                htmlFor='callbackUrls'
                            >
                                <FormattedMessage
                                    id='add_outgoing_webhook.callbackUrls'
                                    defaultMessage='Callback URLs (One Per Line)'
                                />
                            </label>
                            <div className='col-md-5 col-sm-8'>
                                <textarea
                                    id='callbackUrls'
                                    rows={3}
                                    maxLength={1000}
                                    className='form-control'
                                    value={this.state.callbackUrls}
                                    onChange={this.updateCallbackUrls}
                                />
                                <div className='form__help'>
                                    <FormattedMessage
                                        id='add_outgoing_webhook.callbackUrls.help'
                                        defaultMessage='Specify the URL that the messages will be sent to. If the URL is private, add it as a {link}.'
                                        values={{
                                            link: (
                                                <ExternalLink
                                                    href={DocLinks.TRUSTED_CONNECTION}
                                                    location='abstract_outgoing_webhook'
                                                >
                                                    <FormattedMessage
                                                        id='add_outgoing_webhook.callbackUrls.helpLinkText'
                                                        defaultMessage='trusted internal connection'
                                                    />
                                                </ExternalLink>
                                            ),
                                        }}
                                    />
                                </div>
                            </div>
                        </div>
                        {this.props.enablePostUsernameOverride &&
                            <div className='form-group'>
                                <label
                                    className='control-label col-sm-4'
                                    htmlFor='username'
                                >
                                    <FormattedMessage
                                        id='add_outgoing_webhook.username'
                                        defaultMessage='Username'
                                    />
                                </label>
                                <div className='col-md-5 col-sm-8'>
                                    <input
                                        id='username'
                                        type='text'
                                        maxLength={22}
                                        className='form-control'
                                        value={this.state.username}
                                        onChange={this.updateUsername}
                                    />
                                    <div className='form__help'>
                                        <FormattedMessage
                                            id='add_outgoing_webhook.username.help'
                                            defaultMessage='Specify the username this integration will post as. Usernames can be up to 22 characters, and contain lowercase letters, numbers and the symbols "-", "_", and ".". If left blank, the name specified by the webhook creator is used.'
                                        />
                                    </div>
                                </div>
                            </div>
                        }
                        {messageAction && this.renderMessageActionIcon()}
                        {this.props.enablePostIconOverride && !messageAction &&
                            <div className='form-group'>
                                <label
                                    className='control-label col-sm-4'
                                    htmlFor='iconURL'
                                >
                                    <FormattedMessage
                                        id='add_outgoing_webhook.icon_url'
                                        defaultMessage='Profile Picture'
                                    />
                                </label>
                                <div className='col-md-5 col-sm-8'>
                                    <input
                                        id='iconURL'
                                        type='text'
                                        maxLength={1024}
                                        className='form-control'
                                        value={this.state.iconURL}
                                        onChange={this.updateIconURL}
                                    />
                                    <div className='form__help'>
                                        <FormattedMessage
                                            id='add_outgoing_webhook.icon_url.help'
                                            defaultMessage='Enter the URL of a .png or .jpg file for this integration to use as the profile picture when posting. The file should be at least 128 pixels by 128 pixels. If left blank, the profile picture specified by the webhook creator is used.'
                                        />
                                    </div>
                                </div>
                            </div>
                        }
                        <div className='backstage-form__footer'>
                            <FormError
                                type='backstage'
                                errors={[this.props.serverError, this.state.clientError]}
                            />
                            <Link
                                className={buttonClassNames({emphasis: 'tertiary'})}
                                to={`/${this.props.team.name}/integrations/outgoing_webhooks`}
                            >
                                <FormattedMessage
                                    id='add_outgoing_webhook.cancel'
                                    defaultMessage='Cancel'
                                />
                            </Link>
                            <SpinnerButton
                                type='submit'
                                spinning={this.state.saving}
                                spinningText={this.props.loading}
                                onClick={this.handleSubmit}
                                id='saveWebhook'
                            >
                                <FormattedMessage
                                    id={footerToRender.id}
                                    defaultMessage={footerToRender.defaultMessage}
                                />
                            </SpinnerButton>
                            {renderExtra}
                        </div>
                    </form>
                </div>
            </div>
        );
    }
}
