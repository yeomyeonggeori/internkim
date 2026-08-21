import { describe, expect, test } from 'bun:test';
import { daysLabel, formatCRMDate, opportunityStageLabel } from '../../../src/routes/crm/crm-view-model';
import { crmText, type CRMText } from '../../../src/routes/crm/text';
import type { CRMPipelineStage } from '../../../src/routes/crm/crm-types';

const text = crmText.ko;

describe('opportunityStageLabel', () => {
	const stages: CRMPipelineStage[] = [
		{ pipeline: 'partnership', stage: 'review', label: '검토', position: 1, outcome: 'open' },
		{ pipeline: 'partnership', stage: 'stage-9x2', label: '계약 검토', position: 2, outcome: 'open' },
		{ pipeline: 'sales', stage: 'lead', label: 'lead', position: 1, outcome: 'open' }
	];

	test('prefers the vocabulary name over the static table', () => {
		expect(opportunityStageLabel(stages, 'review', text)).toBe('검토');
	});

	test('shows the vocabulary name for a generated stage id', () => {
		expect(opportunityStageLabel(stages, 'stage-9x2', text)).toBe('계약 검토');
	});

	test('falls back to the static table when the label is just the id', () => {
		expect(opportunityStageLabel(stages, 'lead', text)).toBe('리드');
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
