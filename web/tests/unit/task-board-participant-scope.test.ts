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
		expect(canCreateTaskInColumn('요청', 'other')).toBe(true);
		expect(canCreateTaskInColumn('예정', 'other')).toBe(false);
		expect(canCreateTaskInColumn('예정', 'self')).toBe(true);
		expect(canCreateTaskInColumn('예정', 'everyone')).toBe(true);
	});

	test('hides an empty request column only for the viewer own filter', () => {
		expect(shouldHideEmptyRequestColumn('self')).toBe(true);
		expect(shouldHideEmptyRequestColumn('other')).toBe(false);
		expect(shouldHideEmptyRequestColumn('everyone')).toBe(false);
	});
});
