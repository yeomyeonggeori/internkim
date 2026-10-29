import { invokeTool } from '$lib/public-api-call';
import { everyPageOf } from '$lib/record/every-page';
import { clearStoredTaskSnapshot } from '../../routes/task/task-snapshot-storage';
import type { TaskChildProgress } from '../../routes/task/task-relationships';

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
	canDelete?: boolean;
	canUpdate?: boolean;
};

export type RecordTaskList = {
	scope: string;
	count: number;
	tasks: RecordTask[];
	registeredLabels: RecordTaskLabels;
	childProgress?: (TaskChildProgress & { parentTaskID: string })[];
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

type RecordTaskListPage = RecordTaskList & { nextCursor?: string };

export async function everyTaskOfTheCompany(boardWeek?: string): Promise<RecordTaskList> {
	const pages = boardWeek
		? await everyPageOf<RecordTaskListPage>('task_board_get', { scope: 'all', boardWeek })
		: await everyPageOf<RecordTaskListPage>('task_list', { scope: 'all', everyWeek: true });
	return taskListOfPages(pages);
}

function taskListOfPages(pages: RecordTaskListPage[]): RecordTaskList {
	const { nextCursor: _lastCursor, ...first } = pages[0];
	const tasks = pages.flatMap((page) => page.tasks);
	const childProgress = pages.flatMap((page) => page.childProgress ?? []);
	return { ...first, count: tasks.length, tasks, ...(first.childProgress ? { childProgress } : {}) };
}

export function addTask(written: WrittenTask & { parentTaskHint?: string }): Promise<RecordTask> {
	clearStoredTaskSnapshot();
	return invokeTool<RecordTask>('task_add', written).then(afterTaskWrite);
}

export function updateTask(taskID: string, written: Partial<WrittenTask>): Promise<RecordTask> {
	clearStoredTaskSnapshot();
	return invokeTool<RecordTask>('task_update', { taskHint: taskID, ...written }).then(afterTaskWrite);
}

export function deleteTask(taskID: string): Promise<{ taskID: string; deleted: true }> {
	clearStoredTaskSnapshot();
	return invokeTool<{ taskID: string; deleted: true }>('task_delete', { taskHint: taskID }).then(afterTaskWrite);
}

export function setTaskParent(taskID: string, parentTaskID: string | null): Promise<RecordTask> {
	clearStoredTaskSnapshot();
	return invokeTool<RecordTask>('task_update', { taskHint: taskID, parentTaskHint: parentTaskID ?? '' }).then(afterTaskWrite);
}

export function linkTaskChildren(parentTaskID: string, childTaskIDs: string[]): Promise<RecordTask> {
	clearStoredTaskSnapshot();
	return invokeTool<RecordTask>('task_update', { taskHint: parentTaskID, childTaskHints: childTaskIDs }).then(afterTaskWrite);
}

export function setTaskLabels(written: {
	businesses: RecordLabel[];
	types: RecordLabel[];
	etcBusinessColor?: string;
	etcTypeColor?: string;
}): Promise<RecordTaskLabels> {
	clearStoredTaskSnapshot();
	return invokeTool<RecordTaskLabels>('task_vocabulary_set', written).then(afterTaskWrite);
}

function afterTaskWrite<T>(value: T): T {
	clearStoredTaskSnapshot();
	return value;
}
