import { describe, expect, test } from 'bun:test';
import { createMockFetch } from '../test-fetch';
import {
	countryCodeOf,
	nagerDateProvider,
	yearsBetween
} from '../../../src/lib/server/national-holidays';

const oneHourInMilliseconds = 60 * 60 * 1000;
const twelveHoursInMilliseconds = 12 * oneHourInMilliseconds;
const epochInMilliseconds = Date.UTC(2026, 0, 1);

function holiday(date: string, types: string[] | null | undefined = ['Public']) {
	const entry: Record<string, unknown> = { date, localName: '기념일', name: 'Holiday', countryCode: 'KR' };
	if (types !== undefined) entry.types = types;
	return entry;
}

describe('countryCodeOf', () => {
	test('accepts a two-letter code in any case', () => {
		expect(countryCodeOf('kr')).toBe('KR');
		expect(countryCodeOf(' US ')).toBe('US');
	});

	test('refuses anything that is not a two-letter code', () => {
		expect(() => countryCodeOf('KOR')).toThrow('two-letter country code');
		expect(() => countryCodeOf('')).toThrow('two-letter country code');
	});
});

describe('yearsBetween', () => {
	test('covers every year the range touches', () => {
		expect(yearsBetween('2026-12-28', '2027-01-04')).toEqual([2026, 2027]);
		expect(yearsBetween('2026-08-01', '2026-08-31')).toEqual([2026]);
	});

	test('refuses a malformed or reversed range', () => {
		expect(() => yearsBetween('2026-8-1', '2026-08-31')).toThrow('YYYY-MM-DD');
		expect(() => yearsBetween('2026-09-01', '2026-08-31')).toThrow('must not end before it begins');
	});
});

describe('nagerDateProvider().holidayDates', () => {
	test('keeps public holidays and drops the rest', async () => {
		const provider = nagerDateProvider({
			fetch: createMockFetch(async () =>
				Response.json([
					holiday('2026-01-01'),
					holiday('2026-03-01', ['Public', 'Bank']),
					holiday('2026-04-05', ['Observance']),
					holiday('2026-05-05', []),
					holiday('2026-06-06', undefined)
				])
			),
			now: () => epochInMilliseconds
		});

		expect(await provider.holidayDates('KR', 2026)).toEqual([
			'2026-01-01',
			'2026-03-01',
			'2026-05-05',
			'2026-06-06'
		]);
	});

	test('refuses holidays for a country nobody asked about', async () => {
		const provider = nagerDateProvider({
			fetch: createMockFetch(async () =>
				Response.json([{ date: '2026-01-01', countryCode: 'JP', types: ['Public'] }])
			),
			now: () => epochInMilliseconds
		});

		await expect(provider.holidayDates('KR', 2026)).rejects.toThrow('not KR');
	});

	test('refuses a payload that is not a list of dated entries', async () => {
		const notAList = nagerDateProvider({
			fetch: createMockFetch(async () => Response.json({ holidays: [] })),
			now: () => epochInMilliseconds
		});
		await expect(notAList.holidayDates('KR', 2026)).rejects.toThrow('expected a JSON array');

		const undated = nagerDateProvider({
			fetch: createMockFetch(async () => Response.json([{ countryCode: 'KR' }])),
			now: () => epochInMilliseconds
		});
		await expect(undated.holidayDates('KR', 2026)).rejects.toThrow('no calendar date');
	});

	test('walks the cache through a cold failure, population, a hit, expiry, and a fallback', async () => {
		let fetchCallCount = 0;
		let fetchShouldFail = true;
		let currentTimeInMilliseconds = epochInMilliseconds;
		const provider = nagerDateProvider({
			fetch: createMockFetch(async () => {
				fetchCallCount += 1;
				if (fetchShouldFail) return new Response('service unavailable', { status: 503 });
				return Response.json([holiday('2026-01-01'), holiday('2026-03-01')]);
			}),
			now: () => currentTimeInMilliseconds
		});

		try {
			await provider.holidayDates('KR', 2026);
			throw new Error('expected holidayDates to reject on a cold cache');
		} catch (error) {
			if (!(error instanceof Error)) throw error;
			expect(error.message).toContain('Nager.Date');
			expect(error.message).toContain('/v3/PublicHolidays/2026/KR');
			expect(error.message).toContain('503');
		}
		expect(fetchCallCount).toBe(1);

		fetchShouldFail = false;
		expect(await provider.holidayDates('KR', 2026)).toEqual(['2026-01-01', '2026-03-01']);
		expect(fetchCallCount).toBe(2);

		expect(await provider.holidayDates('KR', 2026)).toEqual(['2026-01-01', '2026-03-01']);
		expect(fetchCallCount).toBe(2);

		currentTimeInMilliseconds += twelveHoursInMilliseconds + oneHourInMilliseconds;
		fetchShouldFail = true;
		expect(await provider.holidayDates('KR', 2026)).toEqual(['2026-01-01', '2026-03-01']);
		expect(fetchCallCount).toBe(3);
	});

	test('caches each year and country separately', async () => {
		const askedPaths: string[] = [];
		const provider = nagerDateProvider({
			fetch: createMockFetch(async (input) => {
				askedPaths.push(new URL(String(input)).pathname);
				return Response.json([]);
			}),
			now: () => epochInMilliseconds
		});

		await provider.holidayDates('KR', 2026);
		await provider.holidayDates('KR', 2027);
		await provider.holidayDates('KR', 2026);

		expect(askedPaths).toEqual(['/api/v3/PublicHolidays/2026/KR', '/api/v3/PublicHolidays/2027/KR']);
	});

	test('tries once more when the request itself fails', async () => {
		let fetchCallCount = 0;
		const provider = nagerDateProvider({
			fetch: createMockFetch(async () => {
				fetchCallCount += 1;
				if (fetchCallCount === 1) throw new Error('connection reset');
				return Response.json([holiday('2026-01-01')]);
			}),
			now: () => epochInMilliseconds
		});

		expect(await provider.holidayDates('KR', 2026)).toEqual(['2026-01-01']);
		expect(fetchCallCount).toBe(2);
	});
});

describe('nagerDateProvider().holidays', () => {
	test('carries both names, so a calendar can title the day', async () => {
		const provider = nagerDateProvider({
			fetch: createMockFetch(async () =>
				Response.json([
					{ date: '2026-01-01', localName: '새해', name: "New Year's Day", countryCode: 'KR' }
				])
			),
			now: () => epochInMilliseconds
		});

		expect(await provider.holidays('KR', 2026)).toEqual([
			{ date: '2026-01-01', localName: '새해', name: "New Year's Day" }
		]);
	});

	test('keeps two holidays that fall on one day', async () => {
		const provider = nagerDateProvider({
			fetch: createMockFetch(async () =>
				Response.json([
					{ date: '2026-03-01', localName: '삼일절', name: 'Independence Movement Day', countryCode: 'KR' },
					{ date: '2026-03-01', localName: '대체공휴일', name: 'Substitute Holiday', countryCode: 'KR' }
				])
			),
			now: () => epochInMilliseconds
		});

		const holidays = await provider.holidays('KR', 2026);
		expect(holidays.map((one) => one.name)).toEqual(['Independence Movement Day', 'Substitute Holiday']);
		expect(await provider.holidayDates('KR', 2026)).toEqual(['2026-03-01']);
	});

	test('refuses an entry with no name at all', async () => {
		const provider = nagerDateProvider({
			fetch: createMockFetch(async () =>
				Response.json([{ date: '2026-01-01', localName: '  ', name: '', countryCode: 'KR' }])
			),
			now: () => epochInMilliseconds
		});

		expect(provider.holidays('KR', 2026)).rejects.toThrow('has no name');
	});
});
