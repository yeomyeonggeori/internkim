import { describe, expect, test } from 'bun:test';
import type { CompanyCalendarEntry } from '$lib/server/public-api/record/company-calendar';
import { isOnScheduleOf } from '$lib/server/public-api/record/event-tools';

const sample = { personID: 'person-sample', name: '이샘플' };
const other = { personID: 'person-other', name: '박예시' };
const whoseSample = { everyone: false, personIDs: [sample.personID] };

function entry(overrides: Partial<CompanyCalendarEntry>): CompanyCalendarEntry {
	return {
		id: 'entry',
		uid: 'entry',
		title: '일정',
		description: '',
		location: '',
		startISO: '2026-10-06T10:00:00+09:00',
		endISO: '2026-10-06T11:00:00+09:00',
		timeZone: 'Asia/Seoul',
		isAllDay: false,
		color: '',
		reminderMinutesBefore: null,
		participants: [],
		createdByEmail: '',
		createdByName: '',
		updatedAt: '',
		readOnly: false,
		source: 'event',
		...overrides
	};
}

describe('what is on a person schedule', () => {
	test('an event they attend', () => {
		expect(isOnScheduleOf(whoseSample, entry({ participants: [sample, other] }))).toBe(true);
	});

	test('an event open to the whole company, which names nobody', () => {
		expect(isOnScheduleOf(whoseSample, entry({ participants: [] }))).toBe(true);
	});

	test('not an event others attend', () => {
		expect(isOnScheduleOf(whoseSample, entry({ participants: [other] }))).toBe(false);
	});

	test('their own leave, but not the leave of somebody else', () => {
		expect(isOnScheduleOf(whoseSample, entry({ source: 'leave', participants: [sample] }))).toBe(true);
		expect(isOnScheduleOf(whoseSample, entry({ source: 'leave', participants: [other] }))).toBe(false);
	});
});
