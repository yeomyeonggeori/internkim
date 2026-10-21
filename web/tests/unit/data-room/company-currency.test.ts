import { describe, expect, test } from 'bun:test';
import { companyIn, pairOf, recordedCurrencies, type ExchangeRates } from '../../../src/lib/data-room/company-currency';
import type { DataRoomCompany } from '../../../src/lib/data-room/company';

const company: DataRoomCompany = {
	profile: { capital: { amount: 150_000_000, currency: 'KRW' } },
	metrics: [{ metric: 'annualRevenue', period: '2025', amount: { amount: 1_000_000_000, currency: 'KRW' } }, { metric: 'mau', period: '2026', value: 15_200 }],
	records: [
		{
			category: 'funding',
			title: 'Series A',
			status: 'in_progress',
			amount: { amount: 3_000_000_000, currency: 'KRW' },
			committed: { amount: 1_800_000_000, currency: 'KRW' },
			attributes: { preMoney: { amount: 30_000_000_000, currency: 'KRW' }, instrument: 'RCPS' }
		},
		{ category: 'grant', title: 'Grant', amount: { amount: 100, currency: 'EUR' }, attributes: {} }
	]
};

const rates: ExchangeRates = new Map([[pairOf('KRW', 'USD'), { rate: 0.00075, asOf: '2026-10-18' }]]);

describe('company in a display currency', () => {
	test('names every currency the company recorded money in, attributes included', () => {
		expect(recordedCurrencies(company).toSorted()).toEqual(['EUR', 'KRW']);
	});

	test('converts every amount, commitment, capital and money attribute it has a rate for', () => {
		const converted = companyIn(company, 'USD', rates);
		expect(converted.profile.capital).toEqual({ amount: 112_500, currency: 'USD' });
		expect(converted.metrics[0].amount).toEqual({ amount: 750_000, currency: 'USD' });
		expect(converted.records[0].amount).toEqual({ amount: 2_250_000, currency: 'USD' });
		expect(converted.records[0].committed).toEqual({ amount: 1_350_000, currency: 'USD' });
		expect(converted.records[0].attributes).toEqual({ preMoney: { amount: 22_500_000, currency: 'USD' }, instrument: 'RCPS' });
	});

	test('leaves an amount in its own currency when no rate came back for it', () => {
		expect(companyIn(company, 'USD', rates).records[1].amount).toEqual({ amount: 100, currency: 'EUR' });
	});

	test('leaves values that are not money alone', () => {
		expect(companyIn(company, 'USD', rates).metrics[1]).toEqual({ metric: 'mau', period: '2026', value: 15_200 });
	});
});
