import { storedTaskSnapshot } from './task-snapshot-storage';
import type { Task } from './task-types';

export function storedRelationshipTasks(scope: string): Task[] | null {
	return storedTaskSnapshot(scope)?.state.tasks ?? null;
}

export function relationshipTasksWithSnapshot(shownTasks: Task[], snapshotTasks: Task[]): Task[] {
	const shownTaskIDs = new Set(shownTasks.map((task) => task.id));
	return [...shownTasks, ...snapshotTasks.filter((task) => !shownTaskIDs.has(task.id))];
}
