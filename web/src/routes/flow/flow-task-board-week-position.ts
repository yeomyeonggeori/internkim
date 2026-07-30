import type { FlowTaskBoardWeekPosition } from './flow-task-board-model';
import type { FlowSummary } from './flow-types';

export function flowTaskBoardWeekPosition(summary: FlowSummary | null): FlowTaskBoardWeekPosition {
	if (!summary || summary.week.isCurrent) return 'current';
	const currentWeekStartISO = summary.currentWeek?.startISO ?? '';
	if (!currentWeekStartISO) return 'current';
	return summary.week.startISO < currentWeekStartISO ? 'past' : 'future';
}
