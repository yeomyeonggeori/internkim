import type { TaskBoardWeekPosition } from './task-board-model';
import type { TaskSummary } from './task-types';

export function taskBoardWeekPosition(summary: TaskSummary | null): TaskBoardWeekPosition {
	if (!summary || summary.week.isCurrent) return 'current';
	const currentWeekStartISO = summary.currentWeek?.startISO ?? '';
	if (!currentWeekStartISO) return 'current';
	return summary.week.startISO < currentWeekStartISO ? 'past' : 'future';
}
