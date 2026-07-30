import { describe, expect, test } from 'bun:test';
import {
	canCreateFlowTaskInColumn,
	flowBoardParticipantScope,
	shouldHideEmptyRequestColumn
} from '../../src/routes/flow/flow-board-participant-scope';

describe('flow board participant scope', () => {
	test('reads the filter as self, other, or everyone', () => {
		expect(flowBoardParticipantScope([], 'kim')).toBe('everyone');
		expect(flowBoardParticipantScope(['kim'], 'kim')).toBe('self');
		expect(flowBoardParticipantScope(['kim', 'lee'], 'kim')).toBe('self');
		expect(flowBoardParticipantScope(['lee'], 'kim')).toBe('other');
		expect(flowBoardParticipantScope(['lee'], undefined)).toBe('other');
	});

	test('allows adding only requests while another person is filtered', () => {
		expect(canCreateFlowTaskInColumn('요청', 'other')).toBe(true);
		expect(canCreateFlowTaskInColumn('예정', 'other')).toBe(false);
		expect(canCreateFlowTaskInColumn('예정', 'self')).toBe(true);
		expect(canCreateFlowTaskInColumn('예정', 'everyone')).toBe(true);
	});

	test('hides an empty request column only for the viewer own filter', () => {
		expect(shouldHideEmptyRequestColumn('self')).toBe(true);
		expect(shouldHideEmptyRequestColumn('other')).toBe(false);
		expect(shouldHideEmptyRequestColumn('everyone')).toBe(false);
	});
});
