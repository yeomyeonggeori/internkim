import { describe, expect, test } from 'bun:test';
import {
	buildTaskChildProgress,
	buildTaskRelationships,
	taskChildCandidates,
	taskParentCandidates
} from '../../src/routes/task/task-relationships';
import type { Task } from '../../src/routes/task/task-types';

describe('flow task relationships', () => {
	test('finds only the direct parent and children', () => {
		const parent = task({ id: 'parent' });
		const child = task({ id: 'child', parentTaskID: 'parent' });
		const sibling = task({ id: 'sibling', parentTaskID: 'parent' });
		const grandchild = task({ id: 'grandchild', parentTaskID: 'child' });

		expect(buildTaskRelationships(child, [parent, child, sibling, grandchild])).toEqual({
			parent,
			children: [grandchild]
		});
		expect(buildTaskRelationships(parent, [parent, child, sibling, grandchild]).children.map((task) => task.id)).toEqual([
			'child',
			'sibling'
		]);
	});

	test('counts completed children and excludes only stopped children', () => {
		const progress = buildTaskChildProgress('parent', [
			task({ id: 'done-1', parentTaskID: 'parent', status: '완료' }),
			task({ id: 'done-2', parentTaskID: 'parent', status: '완료' }),
			task({ id: 'progress', parentTaskID: 'parent', status: '진행' }),
			task({ id: 'planned', parentTaskID: 'parent', status: '예정' }),
			task({ id: 'requested', parentTaskID: 'parent', status: '요청' }),
			task({ id: 'paused', parentTaskID: 'parent', status: '일시정지' }),
			task({ id: 'rejected', parentTaskID: 'parent', status: '기각' }),
			task({ id: 'stopped', parentTaskID: 'parent', status: '중단' })
		]);

		expect(progress).toEqual({ completed: 2, total: 7, percent: 29 });
	});

	test('shows every direct child with incomplete work before completed work', () => {
		const relationships = buildTaskRelationships(task({ id: 'parent' }), [
			task({ id: 'done', parentTaskID: 'parent', status: '완료' }),
			task({ id: 'progress', parentTaskID: 'parent', status: '진행' }),
			task({ id: 'rejected', parentTaskID: 'parent', status: '기각' }),
			task({ id: 'stopped', parentTaskID: 'parent', status: '중단' })
		]);

		expect(relationships.children.map((task) => task.id)).toEqual(['progress', 'rejected', 'stopped', 'done']);
	});

	test('keeps rejected children in progress and hides progress when only stopped children remain', () => {
		expect(buildTaskChildProgress('parent', [
			task({ id: 'rejected', parentTaskID: 'parent', status: '기각' })
		])).toEqual({ completed: 0, total: 1, percent: 0 });
		expect(buildTaskChildProgress('parent', [
			task({ id: 'stopped', parentTaskID: 'parent', status: '중단' })
		])).toBeUndefined();
	});

	test('limits parent candidates to owned work and excludes descendants', () => {
		const current = task({ id: 'current', parentTaskID: 'ancestor' });
		const descendant = task({ id: 'descendant', parentTaskID: 'current' });
		const deepDescendant = task({ id: 'deep-descendant', parentTaskID: 'descendant' });
		const ownedCandidate = task({ id: 'owned', ownerID: 'me', participantIDs: ['other'] });
		const participatedCandidate = task({ id: 'participated', participantIDs: ['me'] });
		const requestedCandidate = task({ id: 'requested', requesterID: 'me', ownerID: 'other', participantIDs: ['other'] });
		const unrelatedCandidate = task({ id: 'unrelated', requesterID: 'other', participantIDs: ['other'] });

		expect(taskParentCandidates(current, [
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
		const ancestor = task({ id: 'ancestor', participantIDs: ['me'] });
		const current = task({ id: 'current', parentTaskID: 'ancestor', participantIDs: ['me'] });
		const available = task({ id: 'available', ownerID: 'me', participantIDs: ['other'] });
		const alreadyParented = task({ id: 'already-parented', parentTaskID: 'other-parent', participantIDs: ['me'] });
		const unrelated = task({ id: 'unrelated', participantIDs: ['other'] });

		expect(taskChildCandidates(current, [ancestor, current, available, alreadyParented, unrelated], 'me').map((task) => task.id)).toEqual([
			'available'
		]);
	});
});

function task(overrides: Partial<Task>): Task {
	return {
		id: 'task',
		ownerID: 'owner',
		ownerName: '담당자',
		participantIDs: ['owner'],
		participantNames: ['담당자'],
		business: '김인턴',
		type: '기능',
		content: '업무',
		size: 'M',
		status: '예정',
		statusRank: 0,
		weekCode: '2026-08-10',
		...overrides
	};
}
