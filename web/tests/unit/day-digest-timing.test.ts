import { describe, expect, test } from 'bun:test';
import { dayIn, timeOfDayIn, whoseHourItIs } from '../../src/lib/notifications/day-digest-timing';

const seoulMorning = new Date('2026-08-25T23:00:00Z');

describe('timeOfDayIn', () => {
	test('the same moment is a different hour in each place', () => {
		expect(timeOfDayIn('Asia/Seoul', seoulMorning)).toBe('08:00');
		expect(timeOfDayIn('UTC', seoulMorning)).toBe('23:00');
	});
});

describe('dayIn', () => {
	test('a moment before midnight UTC is already tomorrow in Seoul', () => {
		expect(dayIn('Asia/Seoul', seoulMorning)).toBe('2026-08-26');
		expect(dayIn('UTC', seoulMorning)).toBe('2026-08-25');
	});
});

describe('whoseHourItIs', () => {
	const listener = (memberID: string, timeZone: string, settings: unknown) => ({
		memberID,
		timeZone,
		notificationSettings: settings
	});

	test('only the one whose chosen time has come is told', () => {
		const chosen = whoseHourItIs(
			[
				listener('early', 'Asia/Seoul', { calendar: true, calendarAt: '08:00' }),
				listener('later', 'Asia/Seoul', { calendar: true, calendarAt: '09:00' })
			],
			seoulMorning
		);
		expect(chosen.map((one) => one.memberID)).toEqual(['early']);
	});

	test('the same chosen time means different moments in different places', () => {
		const chosen = whoseHourItIs(
			[
				listener('seoul', 'Asia/Seoul', { calendar: true, calendarAt: '08:00' }),
				listener('london', 'Europe/London', { calendar: true, calendarAt: '08:00' })
			],
			seoulMorning
		);
		expect(chosen.map((one) => one.memberID)).toEqual(['seoul']);
	});

	test('someone who turned the day off is never told', () => {
		const chosen = whoseHourItIs(
			[listener('quiet', 'Asia/Seoul', { calendar: false, calendarAt: '08:00' })],
			seoulMorning
		);
		expect(chosen).toEqual([]);
	});

	test('someone who chose nothing keeps the morning the default names', () => {
		const chosen = whoseHourItIs([listener('unset', 'Asia/Seoul', null)], seoulMorning);
		expect(chosen.map((one) => one.memberID)).toEqual(['unset']);
	});
});
