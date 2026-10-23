// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useState} from 'react';
import {useIntl} from 'react-intl';

import {Section} from 'fusion/modals/settings_controls';
import {am} from 'fusion/utils/class_names';

import CommunityRulesSettings from './community_rules_settings';
import OnboardingSettings from './onboarding_editor';

type Props = {
    setUnsaved?: (unsaved: boolean) => void;

    // Whether the community has roles (custom groups), which its onboarding's answers give.
    onboarding: boolean;
};

// OnboardingRulesSettings is the Onboarding and rules tab of the community settings: what newcomers meet, its rules to
// agree to, then its onboarding, each with its own Save. Either one unsaved keeps the tab from being left.
export default function OnboardingRulesSettings({setUnsaved, onboarding}: Props) {
    const {formatMessage} = useIntl();
    const [rulesUnsaved, setRulesUnsaved] = useState(false);
    const [onboardingUnsaved, setOnboardingUnsaved] = useState(false);
    const unsaved = rulesUnsaved || onboardingUnsaved;
    useEffect(() => {
        setUnsaved?.(unsaved);
    }, [unsaved, setUnsaved]);
    useEffect(() => () => setUnsaved?.(false), [setUnsaved]);

    return (
        <div className={am('onboarding-rules')}>
            <Section title={formatMessage({id: 'fusion.communityRules.tab', defaultMessage: 'Rules'})}/>
            <CommunityRulesSettings setUnsaved={setRulesUnsaved}/>
            {onboarding && (
                <>
                    <Section title={formatMessage({id: 'fusion.onboarding.editTitle', defaultMessage: 'Onboarding'})}/>
                    <OnboardingSettings setUnsaved={setOnboardingUnsaved}/>
                </>
            )}
        </div>
    );
}
