import { describe, expect, test } from 'bun:test';
import {
	companyCalendarHolidays,
	holidayTitleOf,
	mergedCalendarHolidays,
	nationalCalendarHolidays,
	type CompanyHolidayRecord
} from '../../../src/lib/server/calendar/holidays';
import type {
	NationalHoliday,
	NationalHolidayProvider
} from '../../../src/lib/server/national-holidays';

function providerOf(byYear: Record<number, NationalHoliday[]>): NationalHolidayProvider {
	const holidays = async (_countryCode: string, year: number) => byYear[year] ?? [];
	return {
		holidays,
		holidayDates: async (countryCode, year) => [
			...new Set((await holidays(countryCode, year)).map((holiday) => holiday.date))
		]
	};
}

const newYear: NationalHoliday = { date: '2026-01-01', localName: '새해', name: "New Year's Day" };
const nextNewYear: NationalHoliday = {
	date: '2027-01-01',
	localName: '새해',
	name: "New Year's Day"
};

describe('holidayTitleOf', () => {
	test('names a Korean holiday in Korean and everything else in English', () => {
		expect(holidayTitleOf('KR', 'ko', newYear)).toBe('새해');
		expect(holidayTitleOf('KR', 'en', newYear)).toBe("New Year's Day");
		expect(holidayTitleOf('US', 'ko', newYear)).toBe("New Year's Day");
	});

	test('falls back to whichever name the source gave', () => {
		expect(holidayTitleOf('KR', 'ko', { date: '2026-01-01', localName: '', name: 'Only English' })).toBe(
			'Only English'
		);
		expect(holidayTitleOf('US', 'en', { date: '2026-01-01', localName: 'Only Local', name: '' })).toBe(
			'Only Local'
		);
	});
});

describe('companyCalendarHolidays', () => {
	const founding: CompanyHolidayRecord = {
		id: 'company-holiday-1',
		title: '창립기념일',
		date: '2020-05-04',
		recursAnnually: true
	};
	const workshop: CompanyHolidayRecord = {
		id: 'company-holiday-2',
		title: '워크숍',
		date: '2026-09-18',
		recursAnnually: false
	};

	test('repeats an annual holiday in every year the range touches', () => {
		const holidays = companyCalendarHolidays([founding], '2025-12-01', '2027-06-01');
		expect(holidays.map((holiday) => holiday.date)).toEqual(['2026-05-04', '2027-05-04']);
		expect(holidays[0].id).toBe('holiday:company:company-holiday-1:2026-05-04');
		expect(holidays[0].source).toBe('company');
		expect(holidays[0].readOnly).toBe(true);
	});

	test('leaves a one-off holiday outside the range alone', () => {
		expect(companyCalendarHolidays([workshop], '2026-01-01', '2026-09-01')).toEqual([]);
		expect(companyCalendarHolidays([workshop], '2026-09-01', '2026-10-01')).toHaveLength(1);
	});

	test('holds the end of the range open', () => {
		expect(companyCalendarHolidays([workshop], '2026-09-18', '2026-09-19')).toHaveLength(1);
		expect(companyCalendarHolidays([workshop], '2026-09-17', '2026-09-18')).toEqual([]);
	});

	test('gives a leap day no occurrence in a year that has none', () => {
		const leapDay: CompanyHolidayRecord = {
			id: 'company-holiday-3',
			title: '윤일',
			date: '2024-02-29',
			recursAnnually: true
		};
		expect(
			companyCalendarHolidays([leapDay], '2025-01-01', '2029-01-01').map((holiday) => holiday.date)
		).toEqual(['2028-02-29']);
	});

	test('ignores a record whose date is not a date', () => {
		const malformed: CompanyHolidayRecord = {
			id: 'company-holiday-4',
			title: '언젠가',
			date: 'someday',
			recursAnnually: false
		};
		expect(companyCalendarHolidays([malformed], '2026-01-01', '2027-01-01')).toEqual([]);
	});
});

describe('nationalCalendarHolidays', () => {
	test('asks every year the range touches and keeps only what falls inside it', async () => {
		const holidays = await nationalCalendarHolidays(
			providerOf({ 2026: [newYear], 2027: [nextNewYear] }),
			'kr',
			'ko',
			'2026-06-01',
			'2027-06-01'
		);
		expect(holidays.map((holiday) => holiday.date)).toEqual(['2027-01-01']);
		expect(holidays[0].title).toBe('새해');
		expect(holidays[0].countryCode).toBe('KR');
		expect(holidays[0].id).toBe('holiday:holiday_api:KR:ko:2027-01-01:새해');
	});

	test('titles the same day differently for each locale, so the ids differ too', async () => {
		const [inKorean] = await nationalCalendarHolidays(
			providerOf({ 2026: [newYear] }),
			'KR',
			'ko',
			'2026-01-01',
			'2026-02-01'
		);
		const [inEnglish] = await nationalCalendarHolidays(
			providerOf({ 2026: [newYear] }),
			'KR',
			'en',
			'2026-01-01',
			'2026-02-01'
		);
		expect(inKorean.title).toBe('새해');
		expect(inEnglish.title).toBe("New Year's Day");
		expect(inKorean.id).not.toBe(inEnglish.id);
	});
});

describe('mergedCalendarHolidays', () => {
	test('reads as one list in date then title order', async () => {
		const national = await nationalCalendarHolidays(
			providerOf({ 2026: [newYear] }),
			'KR',
			'ko',
			'2026-01-01',
			'2027-01-01'
		);
		const company = companyCalendarHolidays(
			[
				{ id: 'a', title: '창립기념일', date: '2026-05-04', recursAnnually: false },
				{ id: 'b', title: '가족의 날', date: '2026-05-04', recursAnnually: false }
			],
			'2026-01-01',
			'2027-01-01'
		);

		expect(mergedCalendarHolidays(national, company).map((holiday) => holiday.title)).toEqual([
			'새해',
			'가족의 날',
			'창립기념일'
		]);
	});
});
