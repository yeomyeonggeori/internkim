import { describe, expect, test } from 'bun:test';
import {
	buildFlowTaskChildProgress,
	buildFlowTaskRelationships,
	flowTaskChildCandidates,
	flowTaskParentCandidates
} from '../../src/routes/flow/flow-task-relationships';
import type { FlowTask } from '../../src/routes/flow/flow-types';

describe('flow task relationships', () => {
	test('finds only the direct parent and children', () => {
		const parent = flowTask({ id: 'parent' });
		const child = flowTask({ id: 'child', parentTaskID: 'parent' });
		const sibling = flowTask({ id: 'sibling', parentTaskID: 'parent' });
		const grandchild = flowTask({ id: 'grandchild', parentTaskID: 'child' });

		expect(buildFlowTaskRelationships(child, [parent, child, sibling, grandchild])).toEqual({
			parent,
			children: [grandchild]
		});
		expect(buildFlowTaskRelationships(parent, [parent, child, sibling, grandchild]).children.map((task) => task.id)).toEqual([
			'child',
			'sibling'
		]);
	});

	test('counts completed children and excludes only stopped children', () => {
		const progress = buildFlowTaskChildProgress('parent', [
			flowTask({ id: 'done-1', parentTaskID: 'parent', status: '완료' }),
			flowTask({ id: 'done-2', parentTaskID: 'parent', status: '완료' }),
			flowTask({ id: 'progress', parentTaskID: 'parent', status: '진행' }),
			flowTask({ id: 'planned', parentTaskID: 'parent', status: '예정' }),
			flowTask({ id: 'requested', parentTaskID: 'parent', status: '요청' }),
			flowTask({ id: 'paused', parentTaskID: 'parent', status: '일시정지' }),
			flowTask({ id: 'rejected', parentTaskID: 'parent', status: '기각' }),
			flowTask({ id: 'stopped', parentTaskID: 'parent', status: '중단' })
		]);

		expect(progress).toEqual({ completed: 2, total: 7, percent: 29 });
	});

	test('shows every direct child regardless of progress eligibility', () => {
		const relationships = buildFlowTaskRelationships(flowTask({ id: 'parent' }), [
			flowTask({ id: 'done', parentTaskID: 'parent', status: '완료' }),
			flowTask({ id: 'progress', parentTaskID: 'parent', status: '진행' }),
			flowTask({ id: 'rejected', parentTaskID: 'parent', status: '기각' }),
			flowTask({ id: 'stopped', parentTaskID: 'parent', status: '중단' })
		]);

		expect(relationships.children.map((task) => task.id)).toEqual(['done', 'progress', 'rejected', 'stopped']);
	});

	test('keeps rejected children in progress and hides progress when only stopped children remain', () => {
		expect(buildFlowTaskChildProgress('parent', [
			flowTask({ id: 'rejected', parentTaskID: 'parent', status: '기각' })
		])).toEqual({ completed: 0, total: 1, percent: 0 });
		expect(buildFlowTaskChildProgress('parent', [
			flowTask({ id: 'stopped', parentTaskID: 'parent', status: '중단' })
		])).toBeUndefined();
	});

	test('limits parent candidates to owned work and excludes descendants', () => {
		const current = flowTask({ id: 'current', parentTaskID: 'ancestor' });
		const descendant = flowTask({ id: 'descendant', parentTaskID: 'current' });
		const deepDescendant = flowTask({ id: 'deep-descendant', parentTaskID: 'descendant' });
		const ownedCandidate = flowTask({ id: 'owned', ownerID: 'me', participantIDs: ['other'] });
		const participatedCandidate = flowTask({ id: 'participated', participantIDs: ['me'] });
		const requestedCandidate = flowTask({ id: 'requested', requesterID: 'me', ownerID: 'other', participantIDs: ['other'] });
		const unrelatedCandidate = flowTask({ id: 'unrelated', requesterID: 'other', participantIDs: ['other'] });

		expect(flowTaskParentCandidates(current, [
			current,
			descendant,
			deepDescendant,
			ownedCandidate,
			participatedCandidate,
			requestedCandidate,
			unrelatedCandidate
		], 'me').map((task) => task.id)).toEqual(['owned', 'participated']);
	});

	test('limits new children to unparented owned work and excludes ancestors', () => {
		const ancestor = flowTask({ id: 'ancestor', participantIDs: ['me'] });
		const current = flowTask({ id: 'current', parentTaskID: 'ancestor', participantIDs: ['me'] });
		const available = flowTask({ id: 'available', ownerID: 'me', participantIDs: ['other'] });
		const alreadyParented = flowTask({ id: 'already-parented', parentTaskID: 'other-parent', participantIDs: ['me'] });
		const unrelated = flowTask({ id: 'unrelated', participantIDs: ['other'] });

		expect(flowTaskChildCandidates(current, [ancestor, current, available, alreadyParented, unrelated], 'me').map((task) => task.id)).toEqual([
			'available'
		]);
	});
});

function flowTask(overrides: Partial<FlowTask>): FlowTask {
	return {
		id: 'task',
		ownerID: 'owner',
		ownerName: '담당자',
		participantIDs: ['owner'],
		participantNames: ['담당자'],
		business: '김인턴',
		type: '기능',
		content: '업무',
		goal: '완료 기준',
		size: 'M',
		status: '예정',
		statusRank: 0,
		weekCode: '2026-08-10',
		flag: 0,
		...overrides
	};
}
