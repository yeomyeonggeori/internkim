import { describe, expect, test } from 'bun:test';
import { accountStatusRank, activityStatusRank, daysLabel, formatCRMDate, getActivityStatusVariant, getStageVariant, opportunityStageLabel } from '../../../src/routes/crm/crm-view-model';
import { crmText, type CRMText } from '../../../src/routes/crm/text';
import type { CRMPipelineStage } from '../../../src/routes/crm/crm-types';

const text = crmText.ko;

describe('opportunityStageLabel', () => {
	const stages: CRMPipelineStage[] = [
		{ stage: 'review', label: '검토', position: 1, outcome: 'open' },
		{ stage: 'in_progress', label: '계약 검토', position: 2, outcome: 'open' },
		{ stage: 'waiting', label: 'waiting', position: 3, outcome: 'open' }
	];

	test('prefers the vocabulary name over the static table', () => {
		expect(opportunityStageLabel(stages, 'review', text)).toBe('검토');
	});

	test('shows the vocabulary name a company gave one of the stages', () => {
		expect(opportunityStageLabel(stages, 'in_progress', text)).toBe('계약 검토');
	});

	test('falls back to the static table when the label is just the id', () => {
		expect(opportunityStageLabel(stages, 'waiting', text)).toBe('신규');
	});

	test('shows the raw id when nothing knows it', () => {
		expect(opportunityStageLabel(stages, 'mystery_stage', text)).toBe('mystery stage');
	});
});

describe('formatCRMDate', () => {
	test('defaults to the ko locale', () => {
		expect(formatCRMDate('2026-07-22')).toBe('7월 22일');
	});

	test('renders the en locale without Hangul', () => {
		expect(formatCRMDate('2026-07-22', 'en')).toBe('Jul 22');
	});
});

describe('daysLabel', () => {
	const minimalText = { daysAgo: '{days}일', daysAgoOne: '1일' } as CRMText;

	test('uses the singular text for exactly one day', () => {
		expect(daysLabel(1, minimalText)).toBe('1일');
	});

	test('uses the plural template for more than one day', () => {
		expect(daysLabel(3, minimalText)).toBe('3일');
	});
});

describe('activity status badge variants', () => {
	test('fills a finished status, outlines the ones still waiting, and marks a failure destructive', () => {
		expect(getActivityStatusVariant('in_progress')).toBe('default');
		expect(getActivityStatusVariant('completed')).toBe('secondary');
		expect(getActivityStatusVariant('planned')).toBe('outline');
		expect(getActivityStatusVariant('requested')).toBe('outline');
		expect(getActivityStatusVariant('paused')).toBe('outline');
		expect(getActivityStatusVariant('rejected')).toBe('destructive');
		expect(getActivityStatusVariant('stopped')).toBe('destructive');
	});

	test('marks a failed activity the way a lost deal is marked', () => {
		expect(getActivityStatusVariant('rejected')).toBe(getStageVariant('lost'));
		expect(getActivityStatusVariant('stopped')).toBe(getStageVariant('lost'));
	});
});

describe('status ranks', () => {
	test('ranks what still needs doing above what is finished', () => {
		const ranked = ['planned', 'in_progress', 'paused', 'completed', 'rejected']
			.sort((left, right) => activityStatusRank(right) - activityStatusRank(left));

		expect(ranked).toEqual(['planned', 'in_progress', 'paused', 'completed', 'rejected']);
		expect(activityStatusRank('requested')).toBe(activityStatusRank('planned'));
		expect(activityStatusRank('stopped')).toBe(activityStatusRank('rejected'));
	});

	test('ranks a live account above a lead, and a lead above a dormant one', () => {
		const ranked = ['paused', 'active', 'prospect']
			.sort((left, right) => accountStatusRank(right as never) - accountStatusRank(left as never));

		expect(ranked).toEqual(['active', 'prospect', 'paused']);
	});
});
