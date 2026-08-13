import {
	isFlowStatusCompleted,
	isFlowStatusStopped
} from './flow-status';
import type { FlowTask } from './flow-types';

export type FlowTaskChildProgress = {
	completed: number;
	total: number;
	percent: number;
};

export type FlowTaskRelationships = {
	parent: FlowTask | undefined;
	children: FlowTask[];
};

export function buildFlowTaskRelationships(task: FlowTask, tasks: FlowTask[]): FlowTaskRelationships {
	return {
		parent: tasks.find((candidate) => candidate.id === task.parentTaskID),
		children: directFlowTaskChildren(task.id, tasks)
	};
}

export function directFlowTaskChildren(parentTaskID: string, tasks: FlowTask[]): FlowTask[] {
	return tasks.filter((task) => task.parentTaskID === parentTaskID);
}

export function buildFlowTaskChildProgress(parentTaskID: string, tasks: FlowTask[]): FlowTaskChildProgress | undefined {
	const activeChildren = directFlowTaskChildren(parentTaskID, tasks).filter((task) => !isFlowStatusStopped(task.status));
	if (activeChildren.length === 0) return undefined;
	const completed = activeChildren.filter((task) => isFlowStatusCompleted(task.status)).length;
	return {
		completed,
		total: activeChildren.length,
		percent: Math.round((completed / activeChildren.length) * 100)
	};
}

export function buildFlowTaskChildProgressByParent(tasks: FlowTask[]): Map<string, FlowTaskChildProgress> {
	const parentIDs = new Set(tasks.flatMap((task) => task.parentTaskID ? [task.parentTaskID] : []));
	return new Map(
		[...parentIDs].flatMap((parentTaskID) => {
			const progress = buildFlowTaskChildProgress(parentTaskID, tasks);
			return progress ? [[parentTaskID, progress]] : [];
		})
	);
}

export function flowTaskParentCandidates(task: FlowTask, tasks: FlowTask[], currentMemberID: string): FlowTask[] {
	const excludedIDs = new Set([task.id, ...flowTaskDescendantIDs(task.id, tasks)]);
	return tasks.filter((candidate) => isOwnedFlowTask(candidate, currentMemberID) && !excludedIDs.has(candidate.id));
}

export function flowTaskChildCandidates(task: FlowTask, tasks: FlowTask[], currentMemberID: string): FlowTask[] {
	const excludedIDs = new Set([task.id, ...flowTaskAncestorIDs(task, tasks)]);
	return tasks.filter((candidate) =>
		!candidate.parentTaskID && isOwnedFlowTask(candidate, currentMemberID) && !excludedIDs.has(candidate.id)
	);
}

function isOwnedFlowTask(task: FlowTask, currentMemberID: string): boolean {
	return Boolean(currentMemberID) && (
		task.ownerID === currentMemberID || task.participantIDs.includes(currentMemberID)
	);
}

function flowTaskDescendantIDs(taskID: string, tasks: FlowTask[]): string[] {
	const descendantIDs: string[] = [];
	const pendingIDs = [taskID];
	while (pendingIDs.length > 0) {
		const parentTaskID = pendingIDs.shift();
		if (!parentTaskID) continue;
		for (const child of directFlowTaskChildren(parentTaskID, tasks)) {
			if (descendantIDs.includes(child.id)) continue;
			descendantIDs.push(child.id);
			pendingIDs.push(child.id);
		}
	}
	return descendantIDs;
}

function flowTaskAncestorIDs(task: FlowTask, tasks: FlowTask[]): string[] {
	const taskByID = new Map(tasks.map((candidate) => [candidate.id, candidate]));
	const ancestorIDs: string[] = [];
	let parentTaskID = task.parentTaskID;
	while (parentTaskID && !ancestorIDs.includes(parentTaskID)) {
		ancestorIDs.push(parentTaskID);
		parentTaskID = taskByID.get(parentTaskID)?.parentTaskID;
	}
	return ancestorIDs;
}
