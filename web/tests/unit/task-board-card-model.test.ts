import { describe, expect, test } from 'bun:test';
import { buildTaskBoardCardDisplay } from '../../src/routes/task/task-board-card-model';
import type { Task } from '../../src/routes/task/task-types';

describe('flow task board card model', () => {
	test('keeps every participant in one uniform list without a separate owner', () => {
		const display = buildTaskBoardCardDisplay(task({
			ownerName: '이샘플',
			participantIDs: ['member-1', 'member-2', 'member-3'],
			participantNames: ['이샘플', '박예시', '최견본'],
			business: '샘플거리',
			type: '기능',
			startDate: '2026-06-01',
			endDate: '2026-06-03'
		}));

		expect(display).toEqual({
			participantNames: ['이샘플', '박예시', '최견본'],
			participantIDs: ['member-1', 'member-2', 'member-3'],
			businessLabel: '샘플거리',
			metadataLabels: ['기능']
		});
	});

	test('keeps distinct participants who have the same display name', () => {
		const display = buildTaskBoardCardDisplay(task({
			ownerID: 'member-1',
			ownerName: '이샘플',
			participantIDs: ['member-1', 'member-2'],
			participantNames: ['이샘플', '이샘플']
		}));

		expect(display.participantNames).toEqual(['이샘플', '이샘플']);
		expect(display.participantIDs).toEqual(['member-1', 'member-2']);
	});

	test('labels null business as 기타', () => {
		const display = buildTaskBoardCardDisplay(task({
			business: null,
			type: '운영'
		}));

		expect(display.businessLabel).toBe('기타');
		expect(display.metadataLabels).toEqual(['운영']);
	});

	test('labels a null type with the etc label', () => {
		const display = buildTaskBoardCardDisplay(task({
			business: null,
			type: null
		}), 'Etc.');

		expect(display.businessLabel).toBe('Etc.');
		expect(display.metadataLabels).toEqual(['Etc.']);
	});
});

function task(overrides: Partial<Task>): Task {
	return {
		id: 'task-1',
		ownerID: 'member-1',
		ownerName: '김철수',
		participantIDs: ['member-1'],
		participantNames: ['김철수'],
		business: '샘플거리',
		type: '기능',
		content: '업무',
		size: 'M',
		status: 'planned',
		statusRank: 0,
		weekCode: '26W23',
		...overrides
	};
}
