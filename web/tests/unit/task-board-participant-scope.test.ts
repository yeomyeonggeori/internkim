import { describe, expect, test } from 'bun:test';
import {
	canCreateTaskInColumn,
	taskBoardParticipantScope,
	shouldHideEmptyRequestColumn
} from '../../src/routes/task/task-board-participant-scope';

describe('flow board participant scope', () => {
	test('reads the filter as self, other, or everyone', () => {
		expect(taskBoardParticipantScope([], 'kim')).toBe('everyone');
		expect(taskBoardParticipantScope(['kim'], 'kim')).toBe('self');
		expect(taskBoardParticipantScope(['kim', 'leesample'], 'kim')).toBe('self');
		expect(taskBoardParticipantScope(['leesample'], 'kim')).toBe('other');
		expect(taskBoardParticipantScope(['leesample'], undefined)).toBe('other');
	});

	test('allows adding only requests while another person is filtered', () => {
		expect(canCreateTaskInColumn('requested', 'other')).toBe(true);
		expect(canCreateTaskInColumn('planned', 'other')).toBe(false);
		expect(canCreateTaskInColumn('planned', 'self')).toBe(true);
		expect(canCreateTaskInColumn('planned', 'everyone')).toBe(true);
	});

	test('hides an empty request column only for the viewer own filter', () => {
		expect(shouldHideEmptyRequestColumn('self')).toBe(true);
		expect(shouldHideEmptyRequestColumn('other')).toBe(false);
		expect(shouldHideEmptyRequestColumn('everyone')).toBe(false);
	});
});
