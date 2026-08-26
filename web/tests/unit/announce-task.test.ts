import { describe, expect, test } from 'bun:test';
import { whoTaskMoveConcerns } from '../../src/lib/server/announce-task';

const task = (requesterID: string | null, participantIDs: string[]) => ({
	id: 'task-1',
	title: '월간 보고서',
	status: 'done',
	requester_id: requesterID,
	task_participant: participantIDs.map((member_id) => ({ member_id }))
});

describe('whoTaskMoveConcerns', () => {
	test('the one who asked and the ones doing it are told', () => {
		expect(whoTaskMoveConcerns(task('asker', ['doer']), 'onlooker').sort()).toEqual(['asker', 'doer']);
	});

	test('whoever moved it is not told about their own move', () => {
		expect(whoTaskMoveConcerns(task('asker', ['doer']), 'doer')).toEqual(['asker']);
		expect(whoTaskMoveConcerns(task('asker', ['doer']), 'asker')).toEqual(['doer']);
	});

	test('someone who both asked and does it is told once', () => {
		expect(whoTaskMoveConcerns(task('both', ['both']), 'onlooker')).toEqual(['both']);
	});

	test('a task nobody asked for still reaches the ones doing it', () => {
		expect(whoTaskMoveConcerns(task(null, ['doer']), 'onlooker')).toEqual(['doer']);
	});

	test('a task whose only concern moved it tells nobody', () => {
		expect(whoTaskMoveConcerns(task('asker', []), 'asker')).toEqual([]);
	});
});
