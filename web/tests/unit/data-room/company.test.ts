import { describe, expect, test } from 'bun:test';
import {
	changeFromPrevious,
	companyFacts,
	dataRoomCompanySchema,
	isEmptyCompany,
	localized,
	metricSeries,
	periodLabel,
	recordAttributes,
	type CompanyMetric
} from '../../../src/lib/data-room/company';

const metric = (year: number, value: number, quarter?: number): CompanyMetric => ({
	metric: 'annualRevenue', year, value, currency: 'KRW', ...(quarter ? { quarter } : {})
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
		const facts = companyFacts({
			capital: { amount: 500000000, currency: 'KRW' },
			representative: { ko: '이샘플' },
			representativeTitle: { ko: '대표이사' },
			legalAttributes: { ko: { '사업자등록번호': '123-45-67890' } }
		}, 'ko');
		expect(facts).toEqual([
			{ field: 'representative', value: '대표이사 이샘플' },
			{ field: 'capital', value: '₩500,000,000' },
			{ field: '사업자등록번호', value: '123-45-67890' }
		]);
	});

	test('prints the legal labels written for the reader language, falling back to English', () => {
		const english = { 'Business number': '123-45-67890' };
		expect(companyFacts({ legalAttributes: { ko: { '사업자등록번호': '123-45-67890' }, en: english } }, 'en'))
			.toEqual([{ field: 'Business number', value: '123-45-67890' }]);
		expect(companyFacts({ legalAttributes: { en: english } }, 'ko'))
			.toEqual([{ field: 'Business number', value: '123-45-67890' }]);
	});

	test('orders a metric by period and compares its latest value with the one before', () => {
		const [series] = metricSeries([metric(2025, 150), metric(2024, 100), metric(2025, 120, 1)]);
		expect(series.points.map((point) => periodLabel(point, 'en'))).toEqual(['2024', '2025 Q1', '2025']);
		expect(changeFromPrevious(series)).toBe(25);
	});

	test('has no change to show for a single period or a zero before it', () => {
		expect(changeFromPrevious(metricSeries([metric(2025, 150)])[0])).toBeUndefined();
		expect(changeFromPrevious(metricSeries([metric(2024, 0), metric(2025, 10)])[0])).toBeUndefined();
	});

	test('names record attributes for people and drops the empty ones', () => {
		expect(recordAttributes({ category: 'funding', title: 'Seed', attributes: { leadInvestor: 'ABC Ventures', note: '', amount: 2000000 } }))
			.toEqual([['Lead Investor', 'ABC Ventures'], ['Amount', '2000000']]);
	});
});
