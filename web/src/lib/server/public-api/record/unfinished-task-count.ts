import { isTaskStatusFinished } from '../../../../routes/task/task-status';

export function countUnfinishedTasks(rows: { status: string }[]): number {
	return rows.filter((row) => !isTaskStatusFinished(row.status)).length;
}
