import { describe, expect, test } from 'bun:test';
import {
	companyActivityIntensity,
	companyActivityPulsePoints,
	companyLocalCurrency,
	companyMetricBarWidth,
	companyMetricChangeAssessment,
	companyMetricChangePercentage,
	companyMetricDisplayValue,
	companyMetricLabel,
	companyMetricPeriodLabel,
	companyMetricTrendPoints,
	formatCompanyMetricValue,
	companyWorkStatusPercentage,
	latestCompanyMetrics,
	type CompanyShareMetric
} from '../../src/routes/company/company-page-model';

describe('company page model', () => {
	test('selects the latest period for each metric', () => {
		const latest = latestCompanyMetrics([
			{ metric: 'revenue', year: 2025, quarter: 4, value: 10 },
			{ metric: 'revenue', year: 2026, quarter: 1, value: 12 },
			{ metric: 'users', year: 2026, month: 1, value: 100 }
		]);
		expect(latest).toEqual([
			{ metric: 'revenue', year: 2026, quarter: 1, value: 12 },
			{ metric: 'users', year: 2026, month: 1, value: 100 }
		]);
	});

	test('formats metric periods', () => {
		expect(companyMetricPeriodLabel({ metric: 'users', year: 2026, month: 2, value: 1 })).toBe('2026.02');
		expect(companyMetricPeriodLabel({ metric: 'revenue', year: 2026, quarter: 3, value: 1 })).toBe('2026 Q3');
		expect(companyMetricPeriodLabel({ metric: 'revenue', year: 2026, quarter: 3, value: 1 }, 'ko')).toBe('2026년 3분기');
	});

	test('scales bars against the largest absolute value', () => {
		expect(companyMetricBarWidth(25, [25, 100])).toBe(25);
		expect(companyMetricBarWidth(0, [0, 0])).toBe(0);
	});

	test('derives metric change and trend geometry from published periods', () => {
		const series: CompanyShareMetric[] = [
			{ metric: 'annualRevenue', year: 2025, quarter: 4, value: 100 },
			{ metric: 'annualRevenue', year: 2026, quarter: 1, value: 125 }
		];
		expect(companyMetricChangePercentage(series, 'USD')).toBe(25);
		expect(companyMetricTrendPoints(series, 'USD', 100, 40)).toBe('12.0,28.0 88.0,12.0');
		expect(companyMetricLabel('annualRevenue')).toBe('Annual Revenue');
		expect(companyMetricChangeAssessment(25, 'increase')).toBe('favorable');
		expect(companyMetricChangeAssessment(25, 'decrease')).toBe('unfavorable');
		expect(companyMetricChangeAssessment(-10, 'decrease')).toBe('favorable');
		expect(companyMetricChangeAssessment(-10, 'neutral')).toBe('neutral');
	});

	test('defaults convertible money to USD and supports its local currency', () => {
		const metric: CompanyShareMetric = { metric: 'arr', year: 2025, value: 420000000, currency: 'KRW', valueUSD: 304000 };
		expect(companyMetricDisplayValue(metric, 'USD')).toBe(304000);
		expect(companyMetricDisplayValue(metric, 'local')).toBe(420000000);
		expect(formatCompanyMetricValue(metric, 'USD', 'ko')).toBe('$304K');
		expect(formatCompanyMetricValue(metric, 'local', 'ko').includes('₩')).toBe(true);
	});

	test('shows a local toggle only for one convertible non-USD currency', () => {
		expect(companyLocalCurrency([{ metric: 'arr', year: 2025, value: 1, currency: 'KRW', valueUSD: 1 }])).toBe('KRW');
		expect(companyLocalCurrency([{ metric: 'arr', year: 2025, value: 1, currency: 'USD', valueUSD: 1 }])).toBe(undefined);
		expect(companyLocalCurrency([
			{ metric: 'arr', year: 2025, value: 1, currency: 'KRW', valueUSD: 1 },
			{ metric: 'gmv', year: 2025, value: 1, currency: 'JPY', valueUSD: 1 }
		])).toBe(undefined);
	});

	test('leaves non-monetary and legacy values unchanged', () => {
		const metric = { metric: 'sites', year: 2025, value: 63, unit: '곳' };
		expect(companyMetricDisplayValue(metric, 'USD')).toBe(63);
		expect(formatCompanyMetricValue(metric, 'USD', 'ko')).toBe('63 곳');
	});

	test('derives activity intensity and pulse geometry from daily counts', () => {
		const days = [
			{ date: '2026-07-12', attendanceCount: 0, workCount: 0 },
			{ date: '2026-07-13', attendanceCount: 2, workCount: 1 },
			{ date: '2026-07-14', attendanceCount: 4, workCount: 2 }
		];
		expect(companyActivityIntensity(days[0], days)).toBe(0);
		expect(companyActivityIntensity(days[1], days)).toBe(2);
		expect(companyActivityIntensity(days[2], days)).toBe(4);
		expect(companyActivityPulsePoints(days, 100, 40)).toBe('12.0,28.0 50.0,20.0 88.0,12.0');
	});

	test('calculates work distribution from actual status counts', () => {
		const statuses = [
			{ status: 'inProgress' as const, count: 3 },
			{ status: 'completed' as const, count: 1 }
		];
		expect(companyWorkStatusPercentage(3, statuses)).toBe(75);
	});
});
