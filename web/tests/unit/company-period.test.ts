import { describe, expect, test } from 'bun:test';
import { comparePeriodsNewestFirst, periodLabel, periodOf } from '../../src/lib/company-period';

describe('a company period', () => {
	test('spans the days its precision names', () => {
		expect(periodOf('2025')).toMatchObject({ precision: 'year', start: '2025-01-01', end: '2025-12-31' });
		expect(periodOf('2025-H2')).toMatchObject({ precision: 'half', start: '2025-07-01', end: '2025-12-31' });
		expect(periodOf('2024-Q1')).toMatchObject({ precision: 'quarter', start: '2024-01-01', end: '2024-03-31' });
		expect(periodOf('2024-02')).toMatchObject({ precision: 'month', start: '2024-02-01', end: '2024-02-29' });
		expect(periodOf('2025-07-31')).toMatchObject({ precision: 'day', start: '2025-07-31', end: '2025-07-31' });
	});

	test('is nothing when the text names no period', () => {
		for (const text of ['2025-13', '2025-Q5', '2025-H3', '2025-02-30', 'July 2025', '', undefined]) {
			expect(periodOf(text)).toBeUndefined();
		}
	});

	test('sorts by when it began, the longer first when two begin together, and undated last', () => {
		const sorted = ['2025-07', undefined, '2025-Q3', '2024', '2025-H2', '2025-07-31'].toSorted(comparePeriodsNewestFirst);
		expect(sorted).toEqual(['2025-07-31', '2025-H2', '2025-Q3', '2025-07', '2024', undefined]);
	});

	test('reads the way each language writes a period', () => {
		expect(periodLabel('2025-H2', 'ko')).toBe('2025 하반기');
		expect(periodLabel('2025-H1', 'en')).toBe('H1 2025');
		expect(periodLabel('2025-Q3', 'ko')).toBe('2025 3분기');
		expect(periodLabel('2025-Q3', 'en')).toBe('Q3 2025');
		expect(periodLabel('2025-07', 'en')).toBe('2025.07');
		expect(periodLabel('2025-07-31', 'ko')).toBe('2025.07.31');
		expect(periodLabel('2025', 'ko')).toBe('2025');
	});
});
