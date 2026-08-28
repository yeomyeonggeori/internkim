import { describe, expect, test } from 'bun:test';
import { emailByPersonIDOf, matchParticipant, type DevicePerson } from './calendar-event-as-task';
import { deviceTaskAsTask, peopleOnDeviceTask, titleOfDeviceTask, type DeviceTask } from './task-as-task';

const seoul = 'Asia/Seoul';

const people: DevicePerson[] = [
	{ userID: 'a1000000-0000-0000-0000-000000000001', name: '박예시', handle: 'parkyesi', email: 'parkyesi@example.com', image: '/calendar/api/participants/8820b5025006/image' },
	{ userID: 'a1000000-0000-0000-0000-000000000002', name: '김예시', handle: 'kimyesi', email: 'kimyesi@example.com', image: '/calendar/api/participants/9b2a1effd496/image' }
];
const directory = emailByPersonIDOf(people);

function taskWith(fields: Partial<DeviceTask> = {}): DeviceTask {
	return { id: '79e580be536d', content: '새 일정', status: '예정', ...fields };
}

describe('who is on a flow task', () => {
	test('puts the owner first and keeps every participant', () => {
		const written = peopleOnDeviceTask(
			taskWith({
				ownerID: '8820b5025006',
				ownerName: '박예시',
				participantIDs: ['9b2a1effd496'],
				participantNames: ['김예시']
			})
		);
		expect(written).toEqual([
			{ personID: '8820b5025006', name: '박예시' },
			{ personID: '9b2a1effd496', name: '김예시' }
		]);
	});

	test('keeps a name that has no id beside it, so an older row still resolves', () => {
		const written = peopleOnDeviceTask(taskWith({ participantIDs: [], participantNames: ['김예시'] }));
		expect(written).toEqual([{ name: '김예시' }]);
		expect(matchParticipant(written[0], people, directory)?.email).toBe('kimyesi@example.com');
	});

	test('reads the same ids the calendar does', () => {
		const written = peopleOnDeviceTask(taskWith({ ownerID: '8820b5025006', ownerName: '박예시' }));
		expect(matchParticipant(written[0], people, directory)).toEqual({ email: 'parkyesi@example.com', by: 'personID' });
	});
});

describe('the task a flow task becomes', () => {
	test('gives every status the device has one of its own', () => {
		const statuses = ['요청', '예정', '진행', '완료', '일시정지', '기각', '중단'].map(
			(status) => deviceTaskAsTask(taskWith({ status }), seoul).status
		);
		expect(statuses).toEqual([
			'requested',
			'planned',
			'in_progress',
			'completed',
			'paused',
			'rejected',
			'cancelled'
		]);
		expect(new Set(statuses).size).toBe(statuses.length);
	});

	test('treats a status it has never seen as work not started', () => {
		expect(deviceTaskAsTask(taskWith({ status: '누가 새로 만든 상태' }), seoul).status).toBe('planned');
	});

	test('spans the day in the company that owns it, not in UTC', () => {
		const spanning = deviceTaskAsTask(taskWith({ startDate: '2026-09-23', endDate: '2026-09-24' }), seoul);
		expect(spanning.startsAt).toBe('2026-09-23T00:00:00+09:00');
		expect(spanning.endsAt).toBe('2026-09-24T23:59:00+09:00');
		expect(spanning.isWholeDay).toBe(true);
	});

	test('starts on the day it ends when only an end is written', () => {
		const ending = deviceTaskAsTask(taskWith({ endDate: '2026-09-23' }), seoul);
		expect(ending.startsAt).toBe('2026-09-23T00:00:00+09:00');
	});

	test('reads the offset that a zone was on for that very day', () => {
		const winter = deviceTaskAsTask(taskWith({ endDate: '2026-01-15' }), 'America/New_York');
		const summer = deviceTaskAsTask(taskWith({ endDate: '2026-07-15' }), 'America/New_York');
		expect(winter.endsAt).toBe('2026-01-15T23:59:00-05:00');
		expect(summer.endsAt).toBe('2026-07-15T23:59:00-04:00');
	});

	test('carries no dates when the board gave none', () => {
		const undated = deviceTaskAsTask(taskWith(), seoul);
		expect(undated.startsAt).toBeNull();
		expect(undated.endsAt).toBeNull();
		expect(undated.isWholeDay).toBe(false);
	});

	test('keeps the goal and the reason somebody asked, and nothing when neither exists', () => {
		expect(deviceTaskAsTask(taskWith({ goal: '출시', requestReason: '대표 요청' }), seoul).note).toBe('목표: 출시\n대표 요청');
		expect(deviceTaskAsTask(taskWith({ goal: '  ' }), seoul).note).toBeNull();
	});

	test('rides the device id so a second run finds what the first wrote', () => {
		expect(deviceTaskAsTask(taskWith(), seoul).calendar.mirrors).toEqual([
			{ source: 'internkim-device', externalID: '79e580be536d' }
		]);
		expect(deviceTaskAsTask(taskWith({ id: '' }), seoul).calendar.mirrors).toEqual([]);
	});

	test('names a task nobody titled', () => {
		expect(titleOfDeviceTask(taskWith({ content: '   ' }))).toBe('(제목 없음)');
	});
});
