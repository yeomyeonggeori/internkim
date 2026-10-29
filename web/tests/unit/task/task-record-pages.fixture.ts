import { describe, expect, mock, test } from 'bun:test';

const calls: { name: string; input: Record<string, unknown> }[] = [];
const labels = { businesses: [], types: [], sizes: [], statuses: [] };
const pages = [
	{
		scope: 'all', count: 1, totalCount: 2, unfinishedCount: 2, registeredLabels: labels, boardWeek: '2026-09-28',
		tasks: [{ taskID: 'first' }], childProgress: [{ parentTaskID: 'first', completed: 1, total: 2, percent: 50 }],
		nextCursor: 'after-first'
	},
	{
		scope: 'all', count: 1, totalCount: 2, unfinishedCount: 2, registeredLabels: labels, boardWeek: '2026-09-28',
		tasks: [{ taskID: 'second' }], childProgress: [{ parentTaskID: 'second', completed: 0, total: 1, percent: 0 }]
	}
];

mock.module('../../../src/lib/public-api-call', () => ({
	memberAccessToken: async () => 'fixture-token',
	isRefusalCode: () => false,
	invokeTool: async (name: string, input: Record<string, unknown>) => {
		calls.push({ name, input });
		return input.cursor === 'after-first' ? pages[1] : pages[0];
	}
}));

const { everyTaskOfTheCompany } = await import('../../../src/lib/task/task-record');

describe('every task of the company', () => {
	test('follows nextCursor until the last page and joins the pages into one list', async () => {
		const listed = await everyTaskOfTheCompany('2026-09-28');

		expect(calls).toEqual([
			{ name: 'task_board_get', input: { scope: 'all', boardWeek: '2026-09-28', limit: 100 } },
			{ name: 'task_board_get', input: { scope: 'all', boardWeek: '2026-09-28', limit: 100, cursor: 'after-first' } }
		]);
		expect(listed.tasks.map((task) => task.taskID)).toEqual(['first', 'second']);
		expect(listed.count).toBe(2);
		expect(listed.childProgress?.map((progress) => progress.parentTaskID)).toEqual(['first', 'second']);
		expect('nextCursor' in listed).toBe(false);
	});
});
