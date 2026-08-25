import { describe, expect, test } from 'bun:test';
import {
	createFlowTaskBoardMove,
	flowTaskBoardRankStep
} from '../../src/routes/flow/flow-task-board-drag';
import { buildFlowTaskBoard } from '../../src/routes/flow/flow-task-board-model';
import type { FlowTask } from '../../src/routes/flow/flow-types';

describe('flow task board drag', () => {
	test('moves a task to the bottom of another status column', () => {
		const result = createFlowTaskBoardMove([
			flowTask({ id: 'requested', status: '요청', statusRank: flowTaskBoardRankStep }),
			flowTask({ id: 'progress-a', status: '진행', statusRank: flowTaskBoardRankStep }),
			flowTask({ id: 'progress-b', status: '진행', statusRank: flowTaskBoardRankStep * 2 })
		], {
			taskID: 'requested',
			targetStatus: '진행',
			beforeTaskID: null
		});

		expect(result?.updates.map((task) => [task.id, task.status, task.statusRank])).toEqual([
			['requested', '진행', flowTaskBoardRankStep * 3]
		]);
		expect(boardTaskIDs(result?.tasks ?? [], '진행')).toEqual(['progress-a', 'progress-b', 'requested']);
	});

	test('moves a task between neighbors with one persisted update when rank space exists', () => {
		const result = createFlowTaskBoardMove([
			flowTask({ id: 'requested', status: '요청', statusRank: flowTaskBoardRankStep }),
			flowTask({ id: 'progress-a', status: '진행', statusRank: flowTaskBoardRankStep }),
			flowTask({ id: 'progress-b', status: '진행', statusRank: flowTaskBoardRankStep * 3 })
		], {
			taskID: 'requested',
			targetStatus: '진행',
			beforeTaskID: 'progress-b'
		});

		expect(result?.updates.map((task) => [task.id, task.status, task.statusRank])).toEqual([
			['requested', '진행', flowTaskBoardRankStep * 2]
		]);
		expect(boardTaskIDs(result?.tasks ?? [], '진행')).toEqual(['progress-a', 'requested', 'progress-b']);
	});

	test('reorders inside the same status with one persisted update when rank space exists', () => {
		const result = createFlowTaskBoardMove([
			flowTask({ id: 'task-a', status: '진행', statusRank: flowTaskBoardRankStep }),
			flowTask({ id: 'task-b', status: '진행', statusRank: flowTaskBoardRankStep * 2 }),
			flowTask({ id: 'task-c', status: '진행', statusRank: flowTaskBoardRankStep * 3 })
		], {
			taskID: 'task-c',
			targetStatus: '진행',
			beforeTaskID: 'task-a'
		});

		expect(result?.updates.map((task) => [task.id, task.statusRank])).toEqual([
			['task-c', flowTaskBoardRankStep / 2]
		]);
		expect(boardTaskIDs(result?.tasks ?? [], '진행')).toEqual(['task-c', 'task-a', 'task-b']);
	});

	test('returns the moved task first when rank space is exhausted and the target column needs normalization', () => {
		const result = createFlowTaskBoardMove([
			flowTask({ id: 'requested', status: '요청', statusRank: flowTaskBoardRankStep }),
			flowTask({ id: 'progress-a', status: '진행', statusRank: 1 }),
			flowTask({ id: 'progress-b', status: '진행', statusRank: 2 })
		], {
			taskID: 'requested',
			targetStatus: '진행',
			beforeTaskID: 'progress-b'
		});

		expect(result?.updates.map((task) => task.id)).toEqual(['requested', 'progress-a', 'progress-b']);
	});

	test('reorders tasks inside the same status column with stable ranks', () => {
		const result = createFlowTaskBoardMove([
			flowTask({ id: 'task-a', status: '진행', statusRank: 0 }),
			flowTask({ id: 'task-b', status: '진행', statusRank: 0 }),
			flowTask({ id: 'task-c', status: '진행', statusRank: 0 })
		], {
			taskID: 'task-c',
			targetStatus: '진행',
			beforeTaskID: 'task-a'
		});

		expect(result?.updates.map((task) => [task.id, task.statusRank])).toEqual([
			['task-c', flowTaskBoardRankStep],
			['task-a', flowTaskBoardRankStep * 2],
			['task-b', flowTaskBoardRankStep * 3]
		]);
		expect(boardTaskIDs(result?.tasks ?? [], '진행')).toEqual(['task-c', 'task-a', 'task-b']);
	});

	test('ignores drops that keep the same task order', () => {
		const result = createFlowTaskBoardMove([
			flowTask({ id: 'task-a', status: '진행', statusRank: flowTaskBoardRankStep }),
			flowTask({ id: 'task-b', status: '진행', statusRank: flowTaskBoardRankStep * 2 }),
			flowTask({ id: 'task-c', status: '진행', statusRank: flowTaskBoardRankStep * 3 })
		], {
			taskID: 'task-b',
			targetStatus: '진행',
			beforeTaskID: 'task-c'
		});

		expect(result).toBe(null);
	});

	test('ignores adjacent drops that target the moved task as the insertion boundary', () => {
		const result = createFlowTaskBoardMove([
			flowTask({ id: 'task-a', status: '진행', statusRank: flowTaskBoardRankStep }),
			flowTask({ id: 'task-b', status: '진행', statusRank: flowTaskBoardRankStep * 2 }),
			flowTask({ id: 'task-c', status: '진행', statusRank: flowTaskBoardRankStep * 3 })
		], {
			taskID: 'task-b',
			targetStatus: '진행',
			beforeTaskID: 'task-b'
		});

		expect(result).toBe(null);
	});

	test('ignores unknown tasks or non-board statuses', () => {
		expect(createFlowTaskBoardMove([], {
			taskID: 'missing',
			targetStatus: '진행',
			beforeTaskID: null
		})).toBe(null);
		expect(createFlowTaskBoardMove([
			flowTask({ id: 'task-a', status: '진행', statusRank: flowTaskBoardRankStep })
		], {
			taskID: 'task-a',
			targetStatus: '기각',
			beforeTaskID: null
		})).toBe(null);
	});
});

function boardTaskIDs(tasks: FlowTask[], status: string): string[] {
	const column = buildFlowTaskBoard(tasks).find((value) => value.status === status);
	return column?.tasks.map((task) => task.id) ?? [];
}

function flowTask(overrides: Partial<FlowTask>): FlowTask {
	return {
		id: '',
		ownerID: 'member-1',
		ownerName: '김철수',
		participantIDs: ['member-1'],
		participantNames: ['김철수'],
		business: '여명거리',
		type: '기능',
		content: '업무',
		size: 'M',
		status: '예정',
		statusRank: 0,
		weekCode: '26W23',
		...overrides
	};
}
