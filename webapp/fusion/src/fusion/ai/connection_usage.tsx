// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useIntl} from 'react-intl';

import type {AIConnectionUsage} from '@mattermost/types/ai_connections';

import {am} from 'fusion/utils/class_names';

import {dayDate, sortedPurposes, totalTokens, usageByDay} from './connections';

// ConnectionUsage shows a connection's usage this month: its totals against its budget, per purpose (people through
// apps, each built-in feature, each app's own) and per day.
export default function ConnectionUsage({usage}: {usage: AIConnectionUsage}) {
    const {formatMessage, formatNumber, formatDate} = useIntl();
    const tokens = totalTokens(usage.totals);
    const budget = usage.monthly_token_budget;
    const days = usageByDay(usage);
    const busiest = Math.max(1, ...days.map((d) => d.tokens));
    const compact = (n: number) => formatNumber(n, {notation: 'compact', maximumFractionDigits: 1});
    const month = dayDate((usage.month * 100) + 1);

    const purposeLabel = (purpose: string) => {
        switch (purpose) {
        case 'user':
            return formatMessage({id: 'fusion.aiConnections.purpose.user', defaultMessage: 'People, through apps'});
        case 'rewrite':
            return formatMessage({id: 'fusion.aiConnections.purpose.rewrite', defaultMessage: 'Rewrite'});
        case 'recaps':
            return formatMessage({id: 'fusion.aiConnections.purpose.recaps', defaultMessage: 'Recaps'});
        case 'auto_translation':
            return formatMessage({id: 'fusion.aiConnections.purpose.autoTranslation', defaultMessage: 'Auto-translation'});
        case 'text_to_speech':
            return formatMessage({id: 'fusion.aiConnections.purpose.textToSpeech', defaultMessage: 'Text to speech'});
        default:
            if (purpose.startsWith('plugin:')) {
                return formatMessage({id: 'fusion.aiConnections.purpose.app', defaultMessage: 'App: {id}'}, {id: purpose.slice('plugin:'.length)});
            }
            return purpose;
        }
    };

    return (
        <section
            className={am('ai-usage')}
            aria-label={formatMessage({id: 'fusion.aiConnections.usageLabel', defaultMessage: 'Usage in {month}'}, {month: formatDate(month, {month: 'long', year: 'numeric', timeZone: 'UTC'})})}
        >
            <div className={am('ai-usage-totals')}>
                <div>
                    <b>{formatNumber(usage.totals.requests)}</b>
                    <span>{formatMessage({id: 'fusion.aiConnections.usage.requests', defaultMessage: 'requests'})}</span>
                </div>
                <div>
                    <b>{compact(usage.totals.input_tokens)}</b>
                    <span>{formatMessage({id: 'fusion.aiConnections.usage.input', defaultMessage: 'tokens in'})}</span>
                </div>
                <div>
                    <b>{compact(usage.totals.output_tokens)}</b>
                    <span>{formatMessage({id: 'fusion.aiConnections.usage.output', defaultMessage: 'tokens out'})}</span>
                </div>
            </div>
            {budget > 0 && (
                <div className={am('ai-budget')}>
                    <div
                        className={am('ai-meter')}
                        role='meter'
                        aria-valuemin={0}
                        aria-valuemax={budget}
                        aria-valuenow={Math.min(tokens, budget)}
                        aria-label={formatMessage({id: 'fusion.aiConnections.usage.budgetLabel', defaultMessage: 'Monthly token budget'})}
                    >
                        <i style={{width: `${Math.min(100, (tokens / budget) * 100)}%`}}/>
                    </div>
                    <span>{formatMessage({id: 'fusion.aiConnections.usage.budget', defaultMessage: '{used} of {budget} tokens this month'}, {used: compact(tokens), budget: compact(budget)})}</span>
                </div>
            )}

            <h4>{formatMessage({id: 'fusion.aiConnections.usage.byPurpose', defaultMessage: 'By use'})}</h4>
            {sortedPurposes(usage).length === 0 && <p className={am('field-note')}>{formatMessage({id: 'fusion.aiConnections.usage.none', defaultMessage: 'Not used this month.'})}</p>}
            <table className={am('ai-usage-table')}>
                <tbody>
                    {sortedPurposes(usage).map(([purpose, t]) => (
                        <tr key={purpose}>
                            <th scope='row'>{purposeLabel(purpose)}</th>
                            <td>{formatMessage({id: 'fusion.aiConnections.usage.requestsCount', defaultMessage: '{count, plural, one {# request} other {# requests}}'}, {count: t.requests})}</td>
                            <td>{formatMessage({id: 'fusion.aiConnections.usage.tokensCount', defaultMessage: '{tokens} tokens'}, {tokens: compact(totalTokens(t))})}</td>
                        </tr>
                    ))}
                </tbody>
            </table>

            {days.length > 0 && (
                <>
                    <h4>{formatMessage({id: 'fusion.aiConnections.usage.byDay', defaultMessage: 'By day (UTC)'})}</h4>
                    <ol className={am('ai-days')}>
                        {days.map((d) => (
                            <li key={d.day}>
                                <span className={am('ai-day')}>{formatDate(dayDate(d.day), {day: 'numeric', month: 'short', timeZone: 'UTC'})}</span>
                                <span className={am('ai-bar')}>
                                    <i style={{width: `${(d.tokens / busiest) * 100}%`}}/>
                                </span>
                                <span className={am('ai-day-n')}>{formatMessage({id: 'fusion.aiConnections.usage.dayLine', defaultMessage: '{tokens} tokens · {requests, plural, one {# request} other {# requests}}'}, {tokens: compact(d.tokens), requests: d.requests})}</span>
                            </li>
                        ))}
                    </ol>
                </>
            )}
        </section>
    );
}
