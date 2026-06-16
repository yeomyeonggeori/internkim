import { describe, expect, test } from 'bun:test';
import { BOARD_STATUS_VALUES, buildFlowTaskBoard } from '../../src/routes/flow/flow-task-board-model';
import type { FlowTask } from '../../src/routes/flow/flow-types';

describe('flow task board model', () => {
	test('groups only board statuses and excludes rejected or stopped tasks', () => {
		const board = buildFlowTaskBoard([
			flowTask({ id: 'requested-1', status: '요청', statusRank: 100 }),
			flowTask({ id: 'rejected-1', status: '기각', statusRank: 100 }),
			flowTask({ id: 'stopped-1', status: '중단', statusRank: 100 }),
			flowTask({ id: 'paused-1', status: '일시정지', statusRank: 100 })
		]);

		expect(board.map((column) => column.status)).toEqual(BOARD_STATUS_VALUES);
		expect(board.flatMap((column) => column.tasks.map((task) => task.id))).toEqual(['requested-1', 'paused-1']);
	});

	test('sorts tasks from top to bottom by status rank inside each column', () => {
		const board = buildFlowTaskBoard([
			flowTask({ id: 'bottom', status: '진행', statusRank: 300 }),
			flowTask({ id: 'top', status: '진행', statusRank: 100 }),
			flowTask({ id: 'middle', status: '진행', statusRank: 200 })
		]);

		const progressColumn = board.find((column) => column.status === '진행');

		expect(progressColumn?.tasks.map((task) => task.id)).toEqual(['top', 'middle', 'bottom']);
	});

	test('uses task id as a stable tie-break when status ranks match', () => {
		const board = buildFlowTaskBoard([
			flowTask({ id: 'task-c', status: '진행', statusRank: 100 }),
			flowTask({ id: 'task-a', status: '진행', statusRank: 100 }),
			flowTask({ id: 'task-b', status: '진행', statusRank: 100 })
		]);

		const progressColumn = board.find((column) => column.status === '진행');

		expect(progressColumn?.tasks.map((task) => task.id)).toEqual(['task-a', 'task-b', 'task-c']);
	});

	test('assigns a visible accent theme to every board column status', () => {
		const board = buildFlowTaskBoard([]);

		expect(board.map((column) => [column.status, column.theme.dotClass])).toEqual([
			['요청', 'bg-[#7c3aed]'],
			['예정', 'bg-[#d97706]'],
			['진행', 'bg-[#0284c7]'],
			['완료', 'bg-[#16a34a]'],
			['일시정지', 'bg-[#e11d48]']
		]);
	});
});

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
		goal: '완료',
		size: 'M',
		status: '예정',
		statusRank: 0,
		weekCode: '26W23',
		flag: 0,
		...overrides
	};
}
