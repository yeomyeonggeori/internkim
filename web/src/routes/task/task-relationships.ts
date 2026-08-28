import {
	isTaskStatusCompleted,
	isTaskStatusStopped
} from './task-status';
import type { Task } from './task-types';

export type TaskChildProgress = {
	completed: number;
	total: number;
	percent: number;
};

export type TaskRelationships = {
	parent: Task | undefined;
	children: Task[];
};

export function buildTaskRelationships(task: Task, tasks: Task[]): TaskRelationships {
	const children = directTaskChildren(task.id, tasks);
	return {
		parent: tasks.find((candidate) => candidate.id === task.parentTaskID),
		children: [
			...children.filter((child) => !isTaskStatusCompleted(child.status)),
			...children.filter((child) => isTaskStatusCompleted(child.status))
		]
	};
}

export function directTaskChildren(parentTaskID: string, tasks: Task[]): Task[] {
	return tasks.filter((task) => task.parentTaskID === parentTaskID);
}

export function buildTaskChildProgress(parentTaskID: string, tasks: Task[]): TaskChildProgress | undefined {
	const activeChildren = directTaskChildren(parentTaskID, tasks).filter((task) => !isTaskStatusStopped(task.status));
	if (activeChildren.length === 0) return undefined;
	const completed = activeChildren.filter((task) => isTaskStatusCompleted(task.status)).length;
	return {
		completed,
		total: activeChildren.length,
		percent: Math.round((completed / activeChildren.length) * 100)
	};
}

export function buildTaskChildProgressByParent(tasks: Task[]): Map<string, TaskChildProgress> {
	const parentIDs = new Set(tasks.flatMap((task) => task.parentTaskID ? [task.parentTaskID] : []));
	return new Map(
		[...parentIDs].flatMap((parentTaskID) => {
			const progress = buildTaskChildProgress(parentTaskID, tasks);
			return progress ? [[parentTaskID, progress]] : [];
		})
	);
}

export function taskParentCandidates(task: Task, tasks: Task[], currentMemberID: string): Task[] {
	const excludedIDs = new Set([task.id, ...taskDescendantIDs(task.id, tasks)]);
	return tasks.filter((candidate) => isOwnedTask(candidate, currentMemberID) && !excludedIDs.has(candidate.id));
}

export function taskChildCandidates(task: Task, tasks: Task[], currentMemberID: string): Task[] {
	const excludedIDs = new Set([task.id, ...taskAncestorIDs(task, tasks)]);
	return tasks.filter((candidate) =>
		!candidate.parentTaskID && isOwnedTask(candidate, currentMemberID) && !excludedIDs.has(candidate.id)
	);
}

function isOwnedTask(task: Task, currentMemberID: string): boolean {
	return Boolean(currentMemberID) && (
		task.ownerID === currentMemberID || task.participantIDs.includes(currentMemberID)
	);
}

function taskDescendantIDs(taskID: string, tasks: Task[]): string[] {
	const descendantIDs: string[] = [];
	const pendingIDs = [taskID];
	while (pendingIDs.length > 0) {
		const parentTaskID = pendingIDs.shift();
		if (!parentTaskID) continue;
		for (const child of directTaskChildren(parentTaskID, tasks)) {
			if (descendantIDs.includes(child.id)) continue;
			descendantIDs.push(child.id);
			pendingIDs.push(child.id);
		}
	}
	return descendantIDs;
}

function taskAncestorIDs(task: Task, tasks: Task[]): string[] {
	const taskByID = new Map(tasks.map((candidate) => [candidate.id, candidate]));
	const ancestorIDs: string[] = [];
	let parentTaskID = task.parentTaskID;
	while (parentTaskID && !ancestorIDs.includes(parentTaskID)) {
		ancestorIDs.push(parentTaskID);
		parentTaskID = taskByID.get(parentTaskID)?.parentTaskID;
	}
	return ancestorIDs;
}
