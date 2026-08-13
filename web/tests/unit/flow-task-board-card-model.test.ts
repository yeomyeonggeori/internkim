import { describe, expect, test } from 'bun:test';
import { buildFlowTaskBoardCardDisplay } from '../../src/routes/flow/flow-task-board-card-model';
import type { FlowTask } from '../../src/routes/flow/flow-types';

describe('flow task board card model', () => {
	test('keeps every participant in one uniform list without a separate owner', () => {
		const display = buildFlowTaskBoardCardDisplay(flowTask({
			ownerName: '김철수',
			participantIDs: ['member-1', 'member-2', 'member-3'],
			participantNames: ['김철수', '박민준', '최서연'],
			business: '여명거리',
			type: '기능',
			startDate: '2026-06-01',
			endDate: '2026-06-03',
			flag: 2
		}));

		expect(display).toEqual({
			participantNames: ['김철수', '박민준', '최서연'],
			participantIDs: ['member-1', 'member-2', 'member-3'],
			businessLabel: '여명거리',
			metadataLabels: ['기능']
		});
	});

	test('keeps distinct participants who have the same display name', () => {
		const display = buildFlowTaskBoardCardDisplay(flowTask({
			ownerID: 'member-1',
			ownerName: '김철수',
			participantIDs: ['member-1', 'member-2'],
			participantNames: ['김철수', '김철수']
		}));

		expect(display.participantNames).toEqual(['김철수', '김철수']);
		expect(display.participantIDs).toEqual(['member-1', 'member-2']);
	});

	test('labels empty business as 기타', () => {
		const display = buildFlowTaskBoardCardDisplay(flowTask({
			business: '',
			type: '운영'
		}));

		expect(display.businessLabel).toBe('기타');
		expect(display.metadataLabels).toEqual(['운영']);
	});

	test('uses the provided empty business fallback', () => {
		const display = buildFlowTaskBoardCardDisplay(flowTask({
			business: '',
			type: 'Operations'
		}), 'Other');

		expect(display.businessLabel).toBe('Other');
		expect(display.metadataLabels).toEqual(['Operations']);
	});
});

function flowTask(overrides: Partial<FlowTask>): FlowTask {
	return {
		id: 'task-1',
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
