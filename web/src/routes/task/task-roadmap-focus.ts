import type { Task } from './task-types';

export function relatedTaskIDs(taskID: string, tasks: Task[]): ReadonlySet<string> {
	const task = tasks.find((candidate) => candidate.id === taskID);
	if (!task) return new Set();
	const parentID = task.parentTaskID;
	return new Set([
		taskID,
		...(parentID ? [parentID] : []),
		...tasks.flatMap((candidate) => {
			const isChild = candidate.parentTaskID === taskID;
			const isSibling = Boolean(parentID) && candidate.parentTaskID === parentID;
			return isChild || isSibling ? [candidate.id] : [];
		})
	]);
}

export function hasRelatedTasks(taskID: string, tasks: Task[]): boolean {
	return relatedTaskIDs(taskID, tasks).size > 1;
}
