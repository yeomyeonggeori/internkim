import { describe, expect, test } from 'bun:test';
import {
	changeFromPrevious,
	companyFactGroups,
	dataRoomCompanySchema,
	fiscalYearEndLabel,
	headcount,
	isEmptyCompany,
	localized,
	metricBars,
	ownershipShares,
	roundBeingRaised,
	metricSeries,
	periodLabel,
	proofGroups,
	recordAttributes,
	milestoneRecords,
	totalRaised,
	yearsOperating,
	type CompanyMetric,
	type CompanyRecord
, type CompanyFactGroup } from '../../../src/lib/data-room/company';

const metric = (year: number, value: number, quarter?: number): CompanyMetric => ({
	metric: 'annualRevenue', year, amount: { amount: value, currency: 'KRW' }, ...(quarter ? { quarter } : {})
});

describe('the company a data room shows', () => {
	test('reads what the database answers, nothing shown included', () => {
		const company = dataRoomCompanySchema.parse({ profile: {}, metrics: [], records: [] });
		expect(isEmptyCompany(company)).toBe(true);
	});

	test('speaks the reader language and falls back to whichever one was written', () => {
		expect(localized({ ko: '샘플', en: 'Sample' }, 'ko')).toBe('샘플');
		expect(localized({ ko: '샘플' }, 'en')).toBe('샘플');
		expect(localized(undefined, 'ko')).toBe('');
	});

	test('lists only the facts it was given, legal attributes under their own labels', () => {
		const groups = companyFactGroups({
			capital: { amount: 500000000, currency: 'KRW' },
			representative: '샘플 이',
			representativeTitle: '대표이사',
			phone: '02-0000-0000',
			legalAttributes: { ko: { '사업자등록번호': '123-45-67890' } }
		}, 'ko');
		expect(groups).toEqual([
			{ group: 'company', facts: [{ field: 'representative', value: '대표이사 이샘플' }] },
			{ group: 'entity', facts: [{ field: 'capital', value: '₩500,000,000' }] },
			{ group: 'registration', facts: [{ field: '사업자등록번호', value: '123-45-67890' }] }
		]);
	});

	test('prints the legal labels written for the reader language, falling back to English', () => {
		const english = { 'Business number': '123-45-67890' };
		const registration: CompanyFactGroup[] = [{ group: 'registration', facts: [{ field: 'Business number', value: '123-45-67890' }] }];
		expect(companyFactGroups({ legalAttributes: { ko: { '사업자등록번호': '123-45-67890' }, en: english } }, 'en')).toEqual(registration);
		expect(companyFactGroups({ legalAttributes: { en: english } }, 'ko')).toEqual(registration);
	});

	test('orders a metric by period and compares its latest value with the one before at the same granularity', () => {
		const [series] = metricSeries([metric(2025, 150), metric(2024, 100), metric(2025, 120, 1)]);
		expect(series.points.map((point) => periodLabel(point, 'en'))).toEqual(['2024', '2025']);
		expect(changeFromPrevious(series)).toEqual({ kind: 'percent', percent: 50 });
	});

	test('names a loss turning into a profit instead of a percentage across zero', () => {
		expect(changeFromPrevious(metricSeries([metric(2024, -180), metric(2025, 240)])[0])).toEqual({ kind: 'turnedPositive' });
		expect(changeFromPrevious(metricSeries([metric(2024, 50), metric(2025, -10)])[0])).toEqual({ kind: 'turnedNegative' });
	});

	test('has no change to show for a single period or a zero before it', () => {
		expect(changeFromPrevious(metricSeries([metric(2025, 150)])[0])).toBeUndefined();
		expect(changeFromPrevious(metricSeries([metric(2024, 0), metric(2025, 10)])[0])).toBeUndefined();
	});

	test('keeps record attributes under their own keys for the reader language to name, dropping the empty ones', () => {
		expect(recordAttributes({ category: 'funding', title: 'Seed', attributes: { leadInvestor: 'ABC Ventures', note: '', amount: 2000000 } }, 'en'))
			.toEqual([['leadInvestor', 'ABC Ventures'], ['amount', '2000000']]);
	});

	test('shows a money attribute as an amount in its currency', () => {
		expect(recordAttributes({ category: 'funding', title: 'A', attributes: { preMoney: { amount: 30_000_000_000, currency: 'KRW' } } }, 'ko'))
			.toEqual([['preMoney', '₩300억']]);
	});

	const record = (category: string, title: string, date?: string, amount?: number, currency = 'KRW'): CompanyRecord => ({
		category, title, attributes: {}, ...(date ? { date } : {}), ...(amount ? { amount: { amount, currency } } : {})
	});

	test('adds up equity rounds only when they share one currency', () => {
		expect(totalRaised([record('funding', 'Seed', '2021-01-01', 5), record('funding', 'A', '2023-01-01', 20), record('grant', 'Grant', '2022-01-01', 7)]))
			.toEqual({ amount: 25, currency: 'KRW' });
		expect(totalRaised([record('funding', 'Seed', '2021-01-01', 5), record('funding', 'A', '2023-01-01', 20, 'USD')])).toBeUndefined();
	});

	test('groups proof by kind and leaves everything else to the timeline, newest first and undated last', () => {
		const records = [
			record('award', 'Prize', '2024-10-01'),
			record('certification', 'ISO 27001', '2024-11-01'),
			record('history', 'Founded', '2021-03-15'),
			record('expansion', 'Tokyo office', '2026-04-10'),
			record('history', 'Undated note'),
			record('funding', 'Seed', '2021-09-01', 5)
		];
		expect(proofGroups(records).map((group) => group.category)).toEqual(['certification', 'award']);
		expect(milestoneRecords(records).map((entry) => entry.title)).toEqual(['Tokyo office', 'Founded', 'Undated note']);
	});

	test('counts people from the profile first, then from the latest headcount metric', () => {
		const employees = (year: number, value: number): CompanyMetric => ({ metric: 'employees', year, value });
		expect(headcount({ profile: { employeeCount: 9 }, metrics: [employees(2026, 30)], records: [] })).toBe(9);
		expect(headcount({ profile: {}, metrics: [employees(2025, 20), employees(2026, 30)], records: [] })).toBe(30);
		expect(headcount({ profile: {}, metrics: [], records: [] })).toBeUndefined();
	});

	test('says how long the company has been running and names its fiscal year end month', () => {
		expect(yearsOperating('2021-03-15', new Date('2026-10-16'))).toBe(6);
		expect(yearsOperating(undefined, new Date('2026-10-16'))).toBeUndefined();
		expect(fiscalYearEndLabel('12', 'ko')).toBe('12월');
		expect(fiscalYearEndLabel('3', 'en')).toBe('March');
		expect(fiscalYearEndLabel('12-31', 'en')).toBe('12-31');
	});

	test('labels every period of a metric and draws a loss below the zero line', () => {
		const chart = metricBars(metricSeries([metric(2022, -90), metric(2023, -60), metric(2024, 0), metric(2025, 30)])[0], 5, 'ko');
		expect(chart.bars.map((bar) => [bar.period, bar.value, bar.share, bar.isLatest])).toEqual([
			['2022', -90, 1, false],
			['2023', -60, 60 / 90, false],
			['2024', 0, 0, false],
			['2025', 30, 1, true]
		]);
		expect([chart.positiveShare, chart.negativeShare]).toEqual([0.25, 0.75]);
	});

	test('keeps the latest periods when there are more than fit', () => {
		const points = [2019, 2020, 2021, 2022, 2023, 2024].map((year) => metric(year, year));
		expect(metricBars(metricSeries(points)[0], 5, 'en').bars.map((bar) => bar.period)).toEqual(['2020', '2021', '2022', '2023', '2024']);
	});

	test('keeps a round being raised out of what was raised, and names it as the round in progress', () => {
		const raising = { ...record('funding', 'Series B', '2026-12-31', 100), status: 'in_progress' as const };
		const records = [raising, record('funding', 'Seed', '2021-09-01', 5), record('funding', 'A', '2023-01-01', 20)];
		expect(totalRaised(records)).toEqual({ amount: 25, currency: 'KRW' });
		expect(roundBeingRaised(records)?.title).toBe('Series B');
		expect(roundBeingRaised(records.slice(1))).toBeUndefined();
	});

	test('shares out ownership by kind of holder in a fixed order, leaving out kinds that hold nothing', () => {
		expect(ownershipShares([
			{ kind: 'investor', shares: 200 },
			{ kind: 'founder', shares: 700 },
			{ kind: 'option_pool', shares: 100 },
			{ kind: 'other', shares: 0 }
		])).toEqual([
			{ kind: 'founder', shares: 700, percent: 70 },
			{ kind: 'investor', shares: 200, percent: 20 },
			{ kind: 'option_pool', shares: 100, percent: 10 }
		]);
	});
});
