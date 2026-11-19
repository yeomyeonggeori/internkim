import { describe, expect, test } from 'bun:test';
import { ToolRefused } from '../../../src/lib/tool-answer';
import { addTasksFromNote, isEveryTaskAdded } from '../../../src/routes/task/task-quick-add-run';

type Call = { name: string; input: Record<string, unknown> };

function invokerAnswering(drafts: Record<string, unknown>[], answerTaskAdd: (input: Record<string, unknown>) => unknown) {
	const calls: Call[] = [];
	const invoke = async (name: string, input: Record<string, unknown>): Promise<unknown> => {
		calls.push({ name, input });
		if (name === 'task_draft_get') return { tasks: drafts };
		return answerTaskAdd(input);
	};
	return { calls, invoke };
}

describe('adding tasks from a note', () => {
	test('sends each drafted input to task_add unchanged, in the order it was drafted', async () => {
		const drafts = [{ title: '견적서 보내기', endsAt: '2026-11-20' }, { title: '디자인 리뷰', participantPersonHints: ['박예시'] }];
		const { calls, invoke } = invokerAnswering(drafts, (input) => ({ taskID: String(input.title), content: input.title, ownerName: '이샘플', endDate: input.endsAt }));

		const outcomes = await addTasksFromNote('금요일까지 견적서, 박예시 님과 디자인 리뷰', invoke);

		expect(calls).toEqual([
			{ name: 'task_draft_get', input: { note: '금요일까지 견적서, 박예시 님과 디자인 리뷰' } },
			{ name: 'task_add', input: drafts[0] },
			{ name: 'task_add', input: drafts[1] }
		]);
		expect(outcomes).toEqual([
			{ kind: 'added', title: '견적서 보내기', ownerName: '이샘플', endDate: '2026-11-20', leftOut: [] },
			{ kind: 'added', title: '디자인 리뷰', ownerName: '이샘플', endDate: '', leftOut: [] }
		]);
		expect(isEveryTaskAdded(outcomes)).toBe(true);
	});

	test('leaves out each value the record could not resolve, and adds the task without it', async () => {
		const unresolved = new Set(['박예시', '마케팅']);
		const { calls, invoke } = invokerAnswering(
			[{ title: '디자인 리뷰', participantPersonHints: ['박예시', '이샘플'], business: '마케팅', endsAt: '2026-11-20' }],
			(input) => {
				const people = Array.isArray(input.participantPersonHints) ? input.participantPersonHints : [];
				const named = [...people, input.business].find((value) => typeof value === 'string' && unresolved.has(value));
				if (typeof named === 'string') throw new ToolRefused(`${named} is uncertain`, 'interaction_required', 409, named);
				return { taskID: 'added', content: input.title, ownerName: '이샘플' };
			}
		);

		const outcomes = await addTasksFromNote('박예시, 이샘플과 디자인 리뷰', invoke);

		expect(calls.at(-1)?.input).toEqual({ title: '디자인 리뷰', participantPersonHints: ['이샘플'], endsAt: '2026-11-20' });
		expect(outcomes).toEqual([{ kind: 'added', title: '디자인 리뷰', ownerName: '이샘플', endDate: '', leftOut: ['박예시', '마케팅'] }]);
	});

	test('keeps adding after task_add refuses one for a reason no left-out value answers', async () => {
		const { invoke } = invokerAnswering([{ title: '첫째' }, { title: '둘째' }, { title: '셋째', participantPersonHints: ['박'] }], (input) => {
			if (input.title === '둘째') throw new ToolRefused('the caller may not write tasks', 'task_write_forbidden', 403);
			if (input.title === '셋째') throw new ToolRefused('박 is uncertain', 'interaction_required', 409, '이샘플');
			return { taskID: String(input.title), content: input.title };
		});

		const outcomes = await addTasksFromNote('세 가지', invoke);

		expect(outcomes.map((outcome) => outcome.kind)).toEqual(['added', 'refused', 'refused']);
		expect(outcomes[1]).toEqual({ kind: 'refused', title: '둘째', reason: 'the caller may not write tasks' });
		expect(isEveryTaskAdded(outcomes)).toBe(false);
	});

	test('lets a failure that is not a refusal through', async () => {
		const { invoke } = invokerAnswering([{ title: '첫째' }], () => {
			throw new TypeError('Failed to fetch');
		});

		expect(addTasksFromNote('첫째', invoke)).rejects.toThrow('Failed to fetch');
	});

	test('adds nothing when the note describes no work', async () => {
		const { calls, invoke } = invokerAnswering([], () => ({ taskID: 'never' }));

		const outcomes = await addTasksFromNote('음', invoke);

		expect(outcomes).toEqual([]);
		expect(calls.map((call) => call.name)).toEqual(['task_draft_get']);
		expect(isEveryTaskAdded(outcomes)).toBe(false);
	});
});
