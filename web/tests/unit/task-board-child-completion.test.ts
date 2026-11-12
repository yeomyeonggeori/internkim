import { afterAll, describe, expect, mock, test } from 'bun:test';
import type { TaskBoardMoveRequest } from '../../src/routes/task/task-board-drag';
import type { ChildCompletionChoice, TaskBoardController } from '../../src/routes/task/task-board-controller.svelte';
import type { Task, TaskSummary } from '../../src/routes/task/task-types';
import { taskText } from '../../src/routes/task/text';

const savedRequests: TaskBoardMoveRequest[] = [];
const taskBoardSave = { ...(await import('../../src/routes/task/task-board-save')) };

mock.module('../../src/routes/task/task-board-save', () => ({
	saveTaskBoardMove: async (input: { request: TaskBoardMoveRequest }) => {
		savedRequests.push(input.request);
		return 'saved';
	}
}));

afterAll(() => {
	mock.module('../../src/routes/task/task-board-save', () => taskBoardSave);
});

describe('completing a parent on the board', () => {
	test('asks about unfinished children and completes them with the parent when told to', async () => {
		expect(await completeParentAnswering('with_children', { completed: 1, total: 3 })).toEqual({
			asked: { taskTitle: '부모 업무', unfinishedChildren: 2 },
			saved: [{ taskID: 'parent', targetStatus: 'completed', completeChildren: true }]
		});
	});

	test('completes only the parent when told to leave the children as they are', async () => {
		expect((await completeParentAnswering('alone', { completed: 0, total: 1 })).saved).toEqual([
			{ taskID: 'parent', targetStatus: 'completed', completeChildren: false }
		]);
	});

	test('moves nothing when the question is dismissed', async () => {
		expect((await completeParentAnswering('cancelled', { completed: 0, total: 1 })).saved).toEqual([]);
	});

	test('asks nothing when every child is already done', async () => {
		expect(await completeParentAnswering('with_children', { completed: 2, total: 2 })).toEqual({
			asked: null,
			saved: [{ taskID: 'parent', targetStatus: 'completed', completeChildren: false }]
		});
	});
});

async function completeParentAnswering(
	choice: ChildCompletionChoice,
	progress: { completed: number; total: number }
): Promise<{ asked: { taskTitle: string; unfinishedChildren: number } | null; saved: TaskBoardMoveRequest[] }> {
	savedRequests.length = 0;
	const board = await boardShowing(boardSummary(progress));
	const moving = board.moveTaskOnBoard({ taskID: 'parent', targetStatus: 'completed' });
	await Promise.resolve();
	const question = board.childCompletionQuestion;
	question?.answer(choice);
	await moving;
	return {
		asked: question ? { taskTitle: question.taskTitle, unfinishedChildren: question.unfinishedChildren } : null,
		saved: [...savedRequests]
	};
}

async function boardShowing(summary: TaskSummary): Promise<TaskBoardController> {
	const originalState = Reflect.get(globalThis, '$state');
	Reflect.set(globalThis, '$state', <Value>(value: Value): Value => value);
	try {
		const { TaskBoardController } = await import('../../src/routes/task/task-board-controller.svelte');
		const board = new TaskBoardController();
		board.sync({
			text: taskText.ko,
			loadTask: async () => true,
			currentWeek: () => summary.week.code,
			getSummary: () => summary,
			setSummary: () => {},
			setPageErrorMessage: () => {}
		});
		return board;
	} finally {
		if (originalState === undefined) Reflect.deleteProperty(globalThis, '$state');
		else Reflect.set(globalThis, '$state', originalState);
	}
}

function boardSummary(progress: { completed: number; total: number }): TaskSummary {
	return {
		completeness: 'board',
		peopleReady: true,
		childProgressByParent: { parent: { ...progress, percent: Math.round((100 * progress.completed) / progress.total) } },
		week: { code: '26W45', startISO: '2026-11-02', endISO: '2026-11-08', previous: '26W44', next: '26W46', isCurrent: true },
		members: [],
		tasks: [taskOf({ id: 'parent', content: '부모 업무', status: 'in_progress', canUpdate: true })],
		metrics: {
			totalTasks: 1,
			completedTasks: 0,
			requestedTasks: 0,
			pausedTasks: 0,
			stoppedTasks: 0,
			statusCounts: {},
			businessCounts: {},
			typeCounts: {}
		},
		definitions: { categories: [], types: [], sizes: [] },
		statusOptions: [],
		currentUserEmail: 'owner@example.com',
		currentUserName: '이샘플',
		isAdmin: false
	};
}

function taskOf(overrides: Partial<Task>): Task {
	return {
		id: 'task-1',
		ownerID: 'owner',
		ownerName: '이샘플',
		participantIDs: ['owner'],
		participantNames: ['이샘플'],
		business: '',
		type: '기능',
		content: '업무',
		size: 'M',
		status: 'planned',
		weekCode: '26W45',
		...overrides
	};
}
