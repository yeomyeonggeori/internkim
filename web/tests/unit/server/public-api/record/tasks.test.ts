import { describe, expect, test } from 'bun:test';
import {
	participantsOfHints,
	ownerOfScope,
	taskOfHint,
	taskWriteArguments,
	type TaskRow
} from '$lib/server/public-api/record/tasks';
import { HintRefused } from '$lib/server/public-api/record/hint-resolution';
import { personInTheDirectory } from './directory-fixture';

function row(overrides: Partial<TaskRow> = {}): TaskRow {
	return {
		id: 't1',
		parent_task_id: null,
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
		created_at: '2026-08-01T09:00:00.000Z',
		updated_at: '2026-08-20T10:00:00.000Z',
		organization_id: null,
		opportunity_id: null,
		contact_id: null,
		due_at: null,
		requester_id: null,
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

	test('takes a part whose spacing differs from the title it holds', () => {
		expect(taskOfHint(tasks, '  분기   보고서 초안 검토  ').id).toBe('t2');
	});

	test('refuses a part two tasks answer to, and says which', () => {
		const refusal = refusalOf('보고서');
		expect(refusal.outcome).toBe('ambiguous');
		expect(refusal.errorCode).toBe('interaction_required');
		expect(refusal.candidates).toEqual([
			{ id: 't1', label: '분기 보고서 초안' },
			{ id: 't2', label: '분기 보고서 초안 검토' }
		]);
	});

	test('lets the requester’s own task break a tie neither title answers exactly', () => {
		const mine = row({ id: 't4', title: '주간 보고 정리', task_participant: [{ member_id: 'm9' }] });
		const theirs = row({ id: 't5', title: '주간 보고 정리 검토', task_participant: [{ member_id: 'm2' }] });
		expect(taskOfHint([mine, theirs], '보고 정리', 'task', 'm9').id).toBe('t4');
		expect(() => taskOfHint([mine, theirs], '보고 정리')).toThrow(HintRefused);
	});

	test('breaks a tie on ownership alone, never on taking part in somebody else’s', () => {
		const theirs = row({ id: 't6', title: '월간 회의', task_participant: [{ member_id: 'm2' }, { member_id: 'm9' }] });
		const alsoTheirs = row({ id: 't7', title: '월간 회의 준비', task_participant: [{ member_id: 'm3' }] });
		expect(() => taskOfHint([theirs, alsoTheirs], '회의', 'task', 'm9')).toThrow(HintRefused);
	});

	test('offers the nearest titles when nothing holds the hint, and resolves nothing', () => {
		const refusal = refusalOf('분기 보고서 초안을');
		expect(refusal.outcome).toBe('approximate');
		expect(refusal.candidates.map((one) => one.id)).toContain('t1');
	});

	test('caps the nearest at eight', () => {
		const many = Array.from({ length: 20 }, (_, index) =>
			row({ id: `n${index}`, title: `분기 보고 ${index}` })
		);
		expect(refusalOf('분기 보고서 초안을', many).candidates.length).toBe(8);
	});

	test('refuses an id that is a character off rather than approximating it', () => {
		expect(refusalOf('t9').outcome).toBe('not_found');
		expect(refusalOf('t9').errorCode).toBe('task_not_found');
	});

	function refusalOf(hint: string, over: TaskRow[] = tasks): HintRefused {
		try {
			taskOfHint(over, hint);
		} catch (thrown) {
			if (thrown instanceof HintRefused) return thrown;
			throw thrown;
		}
		throw new Error(`${hint} was expected to be refused`);
	}
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

	test('keeps what no task caller names: where it is, whether it takes a whole day, when it warns', () => {
		const written = taskWriteArguments(
			row({ location: { name: '사무실' }, is_whole_day: true, notify_minutes_before: 30 }),
			{ status: 'completed' }
		);
		expect(written).toMatchObject({
			target_location: { name: '사무실' },
			target_is_whole_day: true,
			target_notify_minutes_before: 30
		});
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

	test('goes under the task it names, and under nothing when it names none', () => {
		expect(
			taskWriteArguments(null, { title: '딸린 업무', parentTaskID: 'parent-1' }).target_parent_task_id
		).toBe('parent-1');
		expect(taskWriteArguments(null, { title: '홀로 선 업무' }).target_parent_task_id).toBeNull();
	});
});

describe('who a task belongs to', () => {
	const people = [
		{ personID: 'm1', name: '이샘플', email: 'sample@example.com' },
		{ personID: 'm2', name: '박예시', email: 'yesi@example.com' }
	].map(personInTheDirectory);

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
	].map(personInTheDirectory);

	test('is the caller by default, everyone on request, and the person named over both', () => {
		expect(ownerOfScope(people, undefined, undefined, 'm1')).toBe('m1');
		expect(ownerOfScope(people, 'all', undefined, 'm1')).toBeNull();
		expect(ownerOfScope(people, 'all', '박예시', 'm1')).toBe('m2');
	});
});
