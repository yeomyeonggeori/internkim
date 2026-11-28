import type { TaskDefinitions, TaskState, TaskSummary, Task, TaskWeeklySummary } from './task-types';
import type { TaskBoardMoveRequest } from './task-board-drag';
import {
	forgetTaskStateRead,
	moveTask,
	removeTask,
	saveTask as saveTaskOnRecord,
	saveTaskVocabulary,
	taskNote,
	taskState,
	taskWeeklySummaryOf,
	updateTaskParent as setTaskParentOnRecord,
	updateTaskParents as setTaskParentsOnRecord
} from '$lib/task/task-state';

export const taskStateDependency = 'app:task-state';

export function fetchTaskState(scope?: string, boardWeek?: string): Promise<TaskState> {
	return taskState(scope, boardWeek);
}

export { taskWeeklySummaryOf, forgetTaskStateRead, taskNote };

export function mergeTaskSummary(state: TaskState, weeklySummary: TaskWeeklySummary): TaskSummary {
	return {
		completeness: state.completeness,
		peopleReady: state.peopleReady,
		boardWeek: state.boardWeek,
		childProgressByParent: state.childProgressByParent,
		week: weeklySummary.week,
		currentWeek: state.currentWeek ?? weeklySummary.currentWeek,
		members: state.members,
		tasks: state.tasks,
		weeklyTasks: weeklySummary.weeklyTasks,
		metrics: {
			...weeklySummary.metrics,
			...(state.completeness === 'full' ? {
				memberScores: state.metrics.memberScores ?? {},
				memberScoreDetails: state.metrics.memberScoreDetails ?? {},
				totalScore: state.metrics.totalScore ?? 0
			} : {})
		},
		definitions: state.definitions,
		report: weeklySummary.report,
		statusOptions: state.statusOptions,
		currentUserEmail: state.currentUserEmail,
		currentUserName: state.currentUserName,
		isAdmin: state.isAdmin
	};
}

export function saveTask(
	task: Task,
	originalTask?: Task,
	childTaskIDs: string[] = []
): Promise<void> {
	return saveTaskOnRecord(task, originalTask, childTaskIDs);
}

export function moveTaskOnBoard(request: TaskBoardMoveRequest): Promise<void> {
	return moveTask(request.taskID, request.targetStatus, request.completeChildren);
}

export function deleteTask(taskID: string): Promise<void> {
	return removeTask(taskID);
}

export function updateTaskParent(taskID: string, parentTaskID: string | undefined): Promise<void> {
	return setTaskParentOnRecord(taskID, parentTaskID ?? null);
}

export async function updateTaskParents(taskIDs: string[], parentTaskID: string): Promise<void> {
	if (taskIDs.length === 0) return;
	return setTaskParentsOnRecord(taskIDs, parentTaskID);
}

export function saveTaskDefinitions(
	definitions: Omit<TaskDefinitions, 'sizes'>,
	fallbackMessage: string,
	inUseMessage: string
): Promise<void> {
	return saveTaskVocabulary(definitions, { failure: fallbackMessage, inUse: inUseMessage });
}
