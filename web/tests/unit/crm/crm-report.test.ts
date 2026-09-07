import { describe, expect, test } from 'bun:test';
import {
	crmReportRange,
	daysBetween,
	monthBuckets,
	monthsBetween,
	opportunitiesInRange,
	ownerRows,
	periodOutcome,
	pipelineRows,
	quietAccounts,
	stageRows,
	winRate,
	type MoneyConverter
} from '../../../src/routes/crm/crm-report-model';
import type {
	CRMOpportunity,
	CRMOrganization,
	CRMPipeline,
	CRMPipelineStage
} from '../../../src/routes/crm/crm-types';

const stages: CRMPipelineStage[] = [
	{ stage: 'waiting', label: 'waiting', position: 1, outcome: 'open' },
	{ stage: 'in_progress', label: 'in_progress', position: 2, outcome: 'open' },
	{ stage: 'done', label: 'done', position: 4, outcome: 'won' },
	{ stage: 'lost', label: 'lost', position: 6, outcome: 'lost' }
];

// A dollar is worth a thousand won here, so a mixed period has one comparable total.
const convert: MoneyConverter = (value, currency) =>
	value === undefined ? 0 : currency === 'USD' ? value * 1000 : value;

function deal(overrides: Partial<CRMOpportunity> & { id: string }): CRMOpportunity {
	return {
		organizationID: 'organization-one',
		business: '사업하나',
		name: overrides.id,
		stage: 'waiting',
		ownerName: '이샘플',
		ownerEmail: 'sample@example.com',
		currency: 'KRW',
		importance: 'medium',
		targetDate: '2026-08-10',
		staleDays: 1,
		...overrides
	};
}

function account(overrides: Partial<CRMOrganization> & { id: string }): CRMOrganization {
	return {
		name: overrides.id,
		types: ['customer'],
		status: 'active',
		importance: 'medium',
		ownerName: '이샘플',
		ownerEmail: 'sample@example.com',
		team: '',
		tags: [],
		description: '',
		lastContactDate: '2026-08-01',
		nextActionDate: '',
		openOpportunityCount: 0,
		expectedValues: {},
		...overrides
	};
}

describe('the report period', () => {
	const bounds = {
		today: '2026-09-07',
		next90DaysEnd: '2026-12-06',
		quarterStart: '2026-07-01',
		quarterEnd: '2026-09-30'
	};

	test('reads 전체 기간 as no range at all', () => {
		expect(crmReportRange('all', bounds)).toBe(null);
		const deals = [deal({ id: 'one', targetDate: '2020-01-01' })];
		expect(opportunitiesInRange(deals, null)).toEqual(deals);
	});

	test('keeps only the deals closing inside the chosen window', () => {
		const deals = [
			deal({ id: 'before', targetDate: '2026-06-30' }),
			deal({ id: 'inside', targetDate: '2026-08-10' }),
			deal({ id: 'after', targetDate: '2026-10-01' })
		];

		const quarter = opportunitiesInRange(deals, crmReportRange('quarter', bounds));
		expect(quarter.map((entry) => entry.id)).toEqual(['inside']);

		const next90 = opportunitiesInRange(deals, crmReportRange('next_90_days', bounds));
		expect(next90.map((entry) => entry.id)).toEqual(['after']);
	});
});

describe('the period outcome', () => {
	const deals = [
		deal({ id: 'open-krw', stage: 'in_progress', expectedValue: 3_000_000 }),
		deal({ id: 'open-usd', stage: 'waiting', expectedValue: 2_000, currency: 'USD' }),
		deal({ id: 'won', stage: 'done', expectedValue: 5_000_000 }),
		deal({ id: 'lost', stage: 'lost', expectedValue: 9_000_000 })
	];

	test('totals open and won separately, keeping each currency for the breakdown', () => {
		const outcome = periodOutcome(deals, stages, convert);

		expect(outcome.openTotals).toEqual({ KRW: 3_000_000, USD: 2_000 });
		expect(outcome.openAmount).toBe(5_000_000);
		expect(outcome.wonTotals).toEqual({ KRW: 5_000_000 });
		expect(outcome.wonAmount).toBe(5_000_000);
	});

	test('reads the win rate as won over decided, ignoring the deals still open', () => {
		expect(periodOutcome(deals, stages, convert).winRate).toBe(0.5);
		expect(winRate(3, 1)).toBe(0.75);
	});

	test('has no win rate to report until something is decided', () => {
		const stillOpen = [deal({ id: 'open', stage: 'waiting', expectedValue: 1 })];

		expect(periodOutcome(stillOpen, stages, convert).winRate).toBe(null);
		expect(winRate(0, 0)).toBe(null);
	});
});

describe('the monthly buckets', () => {
	test('names every month between the ends, including the empty ones', () => {
		expect(monthsBetween('2026-11', '2027-02')).toEqual(['2026-11', '2026-12', '2027-01', '2027-02']);
		expect(monthsBetween('2026-03', '2026-03')).toEqual(['2026-03']);
		expect(monthsBetween('2026-05', '2026-04')).toEqual([]);
	});

	test('splits each month into what is open and what is won', () => {
		const deals = [
			deal({ id: 'august-open', stage: 'waiting', targetDate: '2026-08-04', expectedValue: 1_000_000 }),
			deal({ id: 'august-won', stage: 'done', targetDate: '2026-08-20', expectedValue: 4_000_000 }),
			deal({ id: 'october-open', stage: 'in_progress', targetDate: '2026-10-02', expectedValue: 2_000_000 })
		];

		const buckets = monthBuckets(deals, stages, null, convert);

		// September holds no deal but keeps its place, so the bars do not close a gap that exists.
		expect(buckets.map((bucket) => bucket.month)).toEqual(['2026-08', '2026-09', '2026-10']);
		expect(buckets[0]).toMatchObject({ openAmount: 1_000_000, wonAmount: 4_000_000 });
		expect(buckets[1]).toMatchObject({ openAmount: 0, wonAmount: 0 });
		expect(buckets[2]).toMatchObject({ openAmount: 2_000_000, wonAmount: 0 });
	});

	test('spans the chosen window rather than the deals when a period is set', () => {
		const deals = [deal({ id: 'one', targetDate: '2026-08-10', expectedValue: 1 })];

		const buckets = monthBuckets(deals, stages, { start: '2026-07-01', end: '2026-09-30' }, convert);

		expect(buckets.map((bucket) => bucket.month)).toEqual(['2026-07', '2026-08', '2026-09']);
	});
});

describe('the stage and pipeline rows', () => {
	test('orders stages by position, never by how many deals sit in them', () => {
		const deals = [
			deal({ id: 'won-one', stage: 'done' }),
			deal({ id: 'won-two', stage: 'done' }),
			deal({ id: 'waiting-one', stage: 'waiting' })
		];

		const rows = stageRows(deals, stages, (stage) => stage, convert);

		expect(rows.map((row) => row.stage)).toEqual(['waiting', 'in_progress', 'done', 'lost']);
		expect(rows.map((row) => row.count)).toEqual([1, 0, 2, 0]);
		expect(rows.find((row) => row.stage === 'done')?.outcome).toBe('won');
	});

	test('leaves out a pipeline with no deals and carries the definition colour', () => {
		const pipelines: CRMPipeline[] = [
			{ pipeline: 'sales', label: '영업', direction: 'outbound', isActive: true, color: '#2563eb' },
			{ pipeline: 'empty', label: '빈 것', direction: 'outbound', isActive: true, color: '#000000' }
		];
		const deals = [deal({ id: 'one', pipeline: 'sales', expectedValue: 1_000 })];

		const rows = pipelineRows(deals, [], pipelines, (kind) => kind, convert);

		expect(rows).toHaveLength(1);
		expect(rows[0]).toMatchObject({ pipeline: 'sales', label: '영업', color: '#2563eb', count: 1 });
	});
});

describe('the quiet accounts', () => {
	test('puts the longest silence first and counts the days since', () => {
		const accounts = [
			account({ id: 'recent', lastContactDate: '2026-09-01' }),
			account({ id: 'oldest', lastContactDate: '2026-06-01' }),
			account({ id: 'middle', lastContactDate: '2026-08-01' })
		];

		const quiet = quietAccounts(accounts, '2026-09-07', 8);

		expect(quiet.map((entry) => entry.id)).toEqual(['oldest', 'middle', 'recent']);
		expect(quiet[0].daysSinceContact).toBe(98);
		expect(quiet[2].daysSinceContact).toBe(6);
	});

	test('takes only as many as asked for and skips an account never contacted', () => {
		const accounts = [
			account({ id: 'one', lastContactDate: '2026-01-01' }),
			account({ id: 'two', lastContactDate: '2026-02-01' }),
			account({ id: 'never', lastContactDate: '' })
		];

		expect(quietAccounts(accounts, '2026-09-07', 1).map((entry) => entry.id)).toEqual(['one']);
		expect(quietAccounts(accounts, '2026-09-07', 8).map((entry) => entry.id)).toEqual(['one', 'two']);
	});

	test('counts no negative days when the last contact is in the future', () => {
		expect(daysBetween('2026-09-20', '2026-09-07')).toBe(0);
	});
});

describe('the owner rows', () => {
	test('counts each owner’s accounts, open deals and won amount', () => {
		const accounts = [
			account({ id: 'a', ownerName: '이샘플', ownerPersonID: 'person-sample' }),
			account({ id: 'b', ownerName: '박예시', ownerEmail: 'yesi@example.com' })
		];
		const deals = [
			deal({ id: 'open', stage: 'waiting', ownerName: '이샘플', expectedValue: 1_000_000, nextActionID: 'action-one' }),
			deal({ id: 'won', stage: 'done', ownerName: '이샘플', expectedValue: 7_000_000 }),
			deal({ id: 'other', stage: 'in_progress', ownerName: '박예시', expectedValue: 500_000 })
		];

		const rows = ownerRows(deals, accounts, [], stages, convert);
		const sample = rows.find((row) => row.name === '이샘플');

		expect(sample).toMatchObject({ organizationCount: 1, openCount: 1, openAmount: 1_000_000, wonAmount: 7_000_000 });
		expect(sample?.seed).toBe('person-sample');
		// The linked action is missing from nextActions, so it counts as no next activity.
		expect(sample?.missingActionCount).toBe(1);
		expect(rows.find((row) => row.name === '박예시')?.openCount).toBe(1);
	});
});
