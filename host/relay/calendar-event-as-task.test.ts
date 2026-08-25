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
		name: '김여명',
		handle: 'iam',
		email: 'iam@dawn.kim',
		image: '/calendar/api/participants/9b2a1effd496/image'
	},
	{
		userID: '71657dd3-7c7d-4cfe-8d50-2abfe1bc46d2',
		name: '이찬희',
		handle: 'chanhee',
		email: 'chanhee@example.com',
		image: '/calendar/api/participants/abea48c06299/image'
	},
	{ userID: 'a1000000-0000-0000-0000-000000000001', name: '이동하', handle: 'lee', email: 'lee@dawn.kim' }
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

	test('an id nobody holds falls through to the name, which only a member bears', () => {
		expect(matchParticipant({ personID: 'ffffffffffff', name: '김여명' }, people, directory)).toEqual({
			email: 'iam@dawn.kim',
			by: 'nameOrHandle'
		});
	});

	test('an id nobody holds beside a name nobody bears still matches nobody', () => {
		expect(matchParticipant({ personID: 'ffffffffffff', name: '남의 회사 사람' }, people, directory)).toBeUndefined();
	});

	test('matches a full name or a handle outright', () => {
		expect(matchParticipant({ name: '이찬희' }, people, directory)?.by).toBe('nameOrHandle');
		expect(matchParticipant({ name: 'iam' }, people, directory)?.email).toBe('iam@dawn.kim');
	});

	test('matches a given name only when one person bears it', () => {
		expect(matchParticipant({ name: '찬희' }, people, directory)).toEqual({
			email: 'chanhee@example.com',
			by: 'givenName'
		});
		const twoBearIt: DevicePerson[] = [
			{ name: '김여명', email: 'one@example.com' },
			{ name: '이여명', email: 'two@example.com' }
		];
		expect(matchParticipant({ name: '여명' }, twoBearIt, emailByPersonIDOf(twoBearIt))).toBeUndefined();
	});

	test('is nobody when the box holds something that is not a person', () => {
		for (const written of ['사유: 개인 휴가', '참조: 여명 님', '나', '']) {
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

describe('a participant is matched by whichever identity resolves', () => {
	const people: DevicePerson[] = [
		{ name: '박세은', handle: 'seeun', email: 'seeun@dawn.kim' },
		{ name: '김여명', handle: 'iam', email: 'iam@dawn.kim' }
	];
	const emailByPersonID = emailByPersonIDOf(people);

	test('a personID nothing holds does not veto the name written beside it', () => {
		const match = matchParticipant(
			{ personID: 'f84346c5-fe32-4a48-a934-5835cdffafba', name: '박세은' },
			people,
			emailByPersonID
		);
		expect(match?.email).toBe('seeun@dawn.kim');
	});

	test('an address the company holds is taken before any name', () => {
		const match = matchParticipant(
			{ personID: 'gone', name: '이름이 다르게 적힘', email: 'Seeun@Dawn.kim' },
			people,
			emailByPersonID
		);
		expect(match).toEqual({ email: 'seeun@dawn.kim', by: 'email' });
	});

	test('an address nobody holds is not adopted', () => {
		const match = matchParticipant({ email: 'stranger@example.com' }, people, emailByPersonID);
		expect(match).toBeUndefined();
	});

	test('a given name only one member bears still resolves', () => {
		const match = matchParticipant({ name: '여명' }, people, emailByPersonID);
		expect(match).toEqual({ email: 'iam@dawn.kim', by: 'givenName' });
	});
});
