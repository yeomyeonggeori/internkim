import { describe, expect, test } from 'bun:test';
import {
	emailByPersonIDOf,
	eventAsTask,
	matchParticipant,
	timeRunsBackwards,
	titleOf,
	type DeviceCalendarEvent,
	type DevicePerson
} from './calendar-event-as-task';

const people: DevicePerson[] = [
	{
		userID: '22168a62-6d47-47a8-8da6-1663547a76b9',
		name: '김표본',
		handle: 'iam',
		email: 'iam@dawn.kim',
		image: '/calendar/api/participants/9b2a1effd496/image'
	},
	{
		userID: '71657dd3-7c7d-4cfe-8d50-2abfe1bc46d2',
		name: '이모형',
		handle: 'mohyeong',
		email: 'mohyeong@example.com',
		image: '/calendar/api/participants/abea48c06299/image'
	},
	{ userID: 'a1000000-0000-0000-0000-000000000001', name: '이가명', handle: 'lee', email: 'lee@dawn.kim' }
];

const directory = emailByPersonIDOf(people);

function eventWith(fields: Partial<DeviceCalendarEvent> = {}): DeviceCalendarEvent {
	return {
		id: 'event-1',
		uid: 'event-1@internkim',
		title: '마켓컬리 CMO 미팅',
		startISO: '2026-08-20T10:00:00Z',
		endISO: '2026-08-20T11:00:00Z',
		...fields
	};
}

describe('the person a participant names', () => {
	test('reads both shapes of an id, because the calendar uses the shorter one', () => {
		expect(directory.get('22168a62-6d47-47a8-8da6-1663547a76b9')).toBe('iam@dawn.kim');
		expect(directory.get('9b2a1effd496')).toBe('iam@dawn.kim');
	});

	test('lets the id decide, whatever name was written beside it', () => {
		const match = matchParticipant({ personID: '9b2a1effd496', name: '누가 적었든' }, people, directory);
		expect(match).toEqual({ email: 'iam@dawn.kim', by: 'personID' });
	});

	test('refuses an id nobody holds rather than falling back to the name', () => {
		expect(matchParticipant({ personID: 'ffffffffffff', name: '김표본' }, people, directory)).toBeUndefined();
	});

	test('matches a full name or a handle outright', () => {
		expect(matchParticipant({ name: '이모형' }, people, directory)?.by).toBe('nameOrHandle');
		expect(matchParticipant({ name: 'iam' }, people, directory)?.email).toBe('iam@dawn.kim');
	});

	test('matches a given name only when one person bears it', () => {
		expect(matchParticipant({ name: '모형' }, people, directory)).toEqual({
			email: 'mohyeong@example.com',
			by: 'givenName'
		});
		const twoBearIt: DevicePerson[] = [
			{ name: '김표본', email: 'one@example.com' },
			{ name: '이여명', email: 'two@example.com' }
		];
		expect(matchParticipant({ name: '여명' }, twoBearIt, emailByPersonIDOf(twoBearIt))).toBeUndefined();
	});

	test('is nobody when the box holds something that is not a person', () => {
		for (const written of ['사유: 개인 휴가', '참조: 표본 님', '나', '']) {
			expect(matchParticipant({ name: written }, people, directory)).toBeUndefined();
		}
	});
});

describe('the task an event becomes', () => {
	test('keeps the instants the device recorded, so an all-day event stays put', () => {
		const wholeDay = eventAsTask(
			eventWith({ startISO: '2026-06-06T15:00:00Z', endISO: '2026-06-07T15:00:00Z', isAllDay: true, timeZone: 'Asia/Seoul' })
		);
		expect(wholeDay.startsAt).toBe('2026-06-06T15:00:00Z');
		expect(wholeDay.endsAt).toBe('2026-06-07T15:00:00Z');
		expect(wholeDay.isWholeDay).toBe(true);
		expect(wholeDay.calendar.timeZone).toBe('Asia/Seoul');
	});

	test('counts a reminder in minutes and leaves an absent one alone', () => {
		expect(eventAsTask(eventWith({ reminderLeadHours: 24 })).notifyMinutesBefore).toBe(1440);
		expect(eventAsTask(eventWith({ reminderLeadHours: 0 })).notifyMinutesBefore).toBeNull();
		expect(eventAsTask(eventWith()).notifyMinutesBefore).toBeNull();
	});

	test('carries the device identity, and a provider identity beside it', () => {
		const mirrored = eventAsTask(
			eventWith({ remoteSource: 'google', remoteHref: 'https://example.test/e.ics' })
		).calendar.mirrors;
		expect(mirrored).toEqual([
			{ source: 'internkim-device', externalID: 'event-1@internkim' },
			{ source: 'google', externalID: 'event-1@internkim', href: 'https://example.test/e.ics' }
		]);
	});

	test('puts a written location in the bag the task keeps, and nothing when it is blank', () => {
		expect(eventAsTask(eventWith({ location: ' 코엑스 ' })).location).toEqual({ name: '코엑스' });
		expect(eventAsTask(eventWith({ location: '   ' })).location).toBeNull();
	});

	test('names an event nobody titled', () => {
		expect(titleOf(eventWith({ title: '  ' }))).toBe('(제목 없음)');
	});

	test('spots a range the record would refuse', () => {
		expect(timeRunsBackwards(eventWith())).toBe(false);
		expect(timeRunsBackwards(eventWith({ endISO: '2026-08-20T09:00:00Z' }))).toBe(true);
	});
});
