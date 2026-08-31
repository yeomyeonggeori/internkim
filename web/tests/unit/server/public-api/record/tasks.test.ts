import { describe, expect, test } from 'bun:test';
import {
	NothingMatchesTheHint,
	participantsOfHints,
	ownerOfScope,
	taskOfHint,
	taskWriteArguments,
	type TaskRow
} from '$lib/server/public-api/record/tasks';

function row(overrides: Partial<TaskRow> = {}): TaskRow {
	return {
		id: 't1',
		title: '분기 보고서 초안',
		status: 'in_progress',
		note: '작년 것을 참고',
		location: null,
		business: '영업',
		type: '문서',
		size: 'M',
		is_event: false,
		is_whole_day: false,
		notify_minutes_before: null,
		starts_at: '2026-08-01T00:00:00.000Z',
		ends_at: '2026-08-31T23:59:59.999Z',
		updated_at: '2026-08-20T10:00:00.000Z',
		task_participant: [{ member_id: 'm1' }, { member_id: 'm2' }],
		...overrides
	};
}

describe('naming a task', () => {
	const tasks = [row(), row({ id: 't2', title: '분기 보고서 초안 검토' }), row({ id: 't3', title: '휴가 신청' })];

	test('takes an id, then a whole title, then a unique part', () => {
		expect(taskOfHint(tasks, 't2').id).toBe('t2');
		expect(taskOfHint(tasks, '분기 보고서 초안').id).toBe('t1');
		expect(taskOfHint(tasks, '휴가').id).toBe('t3');
	});

	test('refuses a part two tasks answer to, and says which', () => {
		const refusal = (() => {
			try {
				taskOfHint(tasks, '보고서');
			} catch (thrown) {
				return thrown as NothingMatchesTheHint;
			}
			throw new Error('the hint was expected to be refused');
		})();
		expect(refusal.candidates).toEqual(['분기 보고서 초안', '분기 보고서 초안 검토']);
	});
});

describe('writing a task that already exists', () => {
	test('carries every field the caller did not name', () => {
		const written = taskWriteArguments(row(), { status: 'completed' });
		expect(written).toMatchObject({
			target_task_id: 't1',
			target_title: '분기 보고서 초안',
			target_status: 'completed',
			target_note: '작년 것을 참고',
			target_business: '영업',
			target_type: '문서',
			target_size: 'M',
			target_starts_at: '2026-08-01T00:00:00.000Z',
			target_ends_at: '2026-08-31T23:59:59.999Z',
			target_participant_ids: ['m1', 'm2'],
			target_expected_updated_at: '2026-08-20T10:00:00.000Z'
		});
	});

	test('leaves the dates alone unless one was named', () => {
		expect(taskWriteArguments(row(), { title: '새 제목' }).target_write_dates).toBe(false);
		expect(taskWriteArguments(row(), { endsAt: '2026-09-30T00:00:00.000Z' }).target_write_dates).toBe(true);
	});

	test('writes a field the caller emptied on purpose', () => {
		expect(taskWriteArguments(row(), { size: null }).target_size).toBeNull();
	});
});

describe('writing a task that is new', () => {
	test('names no expected version and writes its dates', () => {
		const written = taskWriteArguments(null, { title: '새 업무', participantIDs: ['m1'] });
		expect(written.target_task_id).toBeNull();
		expect(written.target_expected_updated_at).toBeUndefined();
		expect(written.target_write_dates).toBe(true);
		expect(written.target_status).toBe('planned');
	});
});

describe('who a task belongs to', () => {
	const people = [
		{ personID: 'm1', name: '이샘플', email: 'sample@example.com' },
		{ personID: 'm2', name: '박예시', email: 'yesi@example.com' }
	];

	test('is the requester when the caller named nobody', () => {
		expect(participantsOfHints(people, [], 'm1')).toEqual(['m1']);
	});

	test('is left alone when the caller said nothing about it', () => {
		expect(participantsOfHints(people, undefined, 'm1')).toBeUndefined();
	});

	test('is who the caller named', () => {
		expect(participantsOfHints(people, ['박예시'], 'm1')).toEqual(['m2']);
	});
});

describe('whose tasks a list answers with', () => {
	const people = [
		{ personID: 'm1', name: '이샘플', email: 'sample@example.com' },
		{ personID: 'm2', name: '박예시', email: 'yesi@example.com' }
	];

	test('is the caller by default, everyone on request, and the person named over both', () => {
		expect(ownerOfScope(people, undefined, undefined, 'm1')).toBe('m1');
		expect(ownerOfScope(people, 'all', undefined, 'm1')).toBeNull();
		expect(ownerOfScope(people, 'all', '박예시', 'm1')).toBe('m2');
	});
});
