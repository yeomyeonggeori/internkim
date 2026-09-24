import { invokeTool } from '$lib/public-api-call';

export type RecordLabel = { name: string; color?: string };

export type RecordTaskLabels = {
	businesses: RecordLabel[];
	types: RecordLabel[];
	sizes: string[];
	statuses: string[];
	etcBusinessColor?: string;
	etcTypeColor?: string;
};

export type RecordTask = {
	taskID: string;
	parentTaskID?: string;
	requesterID?: string;
	requesterName?: string;
	createdAt?: string;
	ownerID?: string;
	ownerName?: string;
	participantIDs?: string[];
	participantNames?: string[];
	business?: string;
	type?: string;
	content?: string;
	size?: string;
	status?: string;
	startDate?: string;
	endDate?: string;
	weekCode?: string;
};

export type RecordTaskList = {
	scope: string;
	count: number;
	tasks: RecordTask[];
	registeredLabels: RecordTaskLabels;
};

export type WrittenTask = {
	title: string;
	status: string;
	business?: string;
	type?: string;
	startsAt: string;
	endsAt: string;
	participantPersonHints: string[];
	size?: string;
};

export function everyTaskOfTheCompany(): Promise<RecordTaskList> {
	return invokeTool('task_list', { scope: 'all', everyWeek: true });
}

export function addTask(written: WrittenTask & { parentTaskHint?: string }): Promise<RecordTask> {
	return invokeTool('task_add', written);
}

export function updateTask(taskID: string, written: Partial<WrittenTask>): Promise<RecordTask> {
	return invokeTool('task_update', { taskHint: taskID, ...written });
}

export function deleteTask(taskID: string): Promise<{ taskID: string; deleted: true }> {
	return invokeTool('task_delete', { taskHint: taskID });
}

export function setTaskParent(taskID: string, parentTaskID: string | null): Promise<RecordTask> {
	return invokeTool('task_update', { taskHint: taskID, parentTaskHint: parentTaskID ?? '' });
}

export function linkTaskChildren(parentTaskID: string, childTaskIDs: string[]): Promise<RecordTask> {
	return invokeTool('task_update', { taskHint: parentTaskID, childTaskHints: childTaskIDs });
}

export function setTaskLabels(written: {
	businesses: RecordLabel[];
	types: RecordLabel[];
	etcBusinessColor?: string;
	etcTypeColor?: string;
}): Promise<RecordTaskLabels> {
	return invokeTool('task_vocabulary_set', written);
}
