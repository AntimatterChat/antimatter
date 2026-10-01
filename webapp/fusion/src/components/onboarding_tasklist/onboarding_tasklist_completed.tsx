// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useRef, type JSX} from 'react';
import {FormattedMessage} from 'react-intl';
import {CSSTransition} from 'react-transition-group';
import styled from 'styled-components';

import {isDesktopApp} from '@mattermost/shared/utils/user_agent';

import ExternalLink from 'components/external_link';

import completedImg from 'images/completed.svg';

const CompletedWrapper = styled.div`
    display: flex;
    flex-direction: column;
    align-items: center;
    padding: 26px 24px 0 24px;
    margin: auto;
    text-align: center;
    word-break: break-word;
    width: 100%;
    height: 500px;

    &.fade-enter {
        transform: scale(0);
    }
    &.fade-enter-active {
        transform: scale(1);
    }
    &.fade-enter-done {
        transform: scale(1);
    }
    &.fade-exit {
        transform: scale(1);
    }
    &.fade-exit-active {
        transform: scale(1);
    }
    &.fade-exit-done {
        transform: scale(1);
    }
    .got-it-button {
        padding: 13px 20px;
        background: var(--button-bg);
        border-radius: 4px;
        color: var(--sidebar-text);
        border: none;
        font-weight: bold;
        margin-top: 15px;
        min-height: 40px;
        &:hover {
            background: var(--button-bg) !important;
            color: var(--sidebar-text) !important;
        }
    }

    h2 {
        font-size: 20px;
        margin: 0 0 10px;
        font-weight: 600;
    }

    .completed-subtitle {
        font-size: 14px !important;
        color: rgba(var(--center-channel-color-rgb), 0.75);
        line-height: 20px;
    }

    .completed-subtitle {
        margin-top: 5px;
    }

    .download-apps {
        width: 90%;
        margin-top: 15px;
        color: rgba(var(--center-channel-color-rgb), 0.75);
        font-family: "Open Sans";
        font-style: normal;
        font-weight: normal;
        line-height: 16px;
    }

    .download-apps {
        margin-top: 24px;
        width: 200px;
        font-size: 12px;
    }
`;

interface Props {
    dismissAction: () => void;
}

const Completed = (props: Props): JSX.Element => {
    const {dismissAction} = props;

    const wrapperRef = useRef<HTMLDivElement>(null);

    return (
        <>
            <CSSTransition
                in={true}
                nodeRef={wrapperRef}
                timeout={150}
                classNames='fade'
            >
                <CompletedWrapper ref={wrapperRef}>
                    <img
                        src={completedImg}
                        alt={'completed tasks image'}
                    />
                    <h2>
                        <FormattedMessage
                            id={'onboardingTask.checklist.completed_title'}
                            defaultMessage='Well done. You’ve completed all of the tasks!'
                        />
                    </h2>
                    <span className='completed-subtitle'>
                        <FormattedMessage
                            id={'onboardingTask.checklist.completed_subtitle'}
                            defaultMessage='We hope Antimatter is more familiar now.'
                        />
                    </span>

                    <button
                        onClick={dismissAction}
                        className='got-it-button'
                    >
                        <FormattedMessage
                            id={'collapsed_reply_threads_modal.confirm'}
                            defaultMessage='Got it'
                        />
                    </button>
                    {!isDesktopApp() && (
                        <div className='download-apps'>
                            <span>
                                <FormattedMessage
                                    id='onboardingTask.checklist.downloads'
                                    defaultMessage='Now that you’re all set up, <link>download our apps.</link>'
                                    values={{
                                        link: (msg: React.ReactNode) => (
                                            <ExternalLink
                                                location='onboarding_tasklist_completed'
                                                href='https://mattermost.com/download#desktop'
                                            >
                                                {msg}
                                            </ExternalLink>
                                        ),
                                    }}
                                />
                            </span>
                        </div>
                    )}
                </CompletedWrapper>
            </CSSTransition>
        </>
    );
};

export default Completed;
