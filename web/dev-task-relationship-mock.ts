import type { Task } from './src/routes/task/task-types';

type DevFlowTaskRelationshipMockResult = {
	tasks: Task[];
	response: {
		status: number;
		body: { ok: true } | { error: string };
	};
};

export function updateDevFlowTaskParent(
	tasks: Task[],
	taskID: string,
	parsed: Record<string, unknown>
): DevFlowTaskRelationshipMockResult {
	if (!Object.hasOwn(parsed, 'parentTaskID') || !isNullableString(parsed.parentTaskID)) {
		return failedUpdate(tasks, 400, 'parent task id must be a string or null');
	}
	if (!tasks.some((task) => task.id === taskID)) {
		return failedUpdate(tasks, 404, 'task not found');
	}
	const parentTaskID = normalizedParentTaskID(parsed.parentTaskID);
	if (parentTaskID === taskID) {
		return failedUpdate(tasks, 400, 'task cannot be its own parent');
	}
	if (parentTaskID && !tasks.some((task) => task.id === parentTaskID)) {
		return failedUpdate(tasks, 404, 'parent task not found');
	}
	return {
		tasks: tasks.map((task) =>
			task.id === taskID ? { ...task, parentTaskID: parentTaskID ?? undefined } : task
		),
		response: { status: 200, body: { ok: true } }
	};
}

export function updateDevFlowTaskParents(
	tasks: Task[],
	parsed: Record<string, unknown>
): DevFlowTaskRelationshipMockResult {
	if (!Array.isArray(parsed.taskIDs) || parsed.taskIDs.some((taskID) => typeof taskID !== 'string')) {
		return failedUpdate(tasks, 400, 'task ids must be strings');
	}
	if (typeof parsed.parentTaskID !== 'string' || !parsed.parentTaskID.trim()) {
		return failedUpdate(tasks, 400, 'parent task id must be a string');
	}
	const taskIDs = [...new Set(parsed.taskIDs.map((taskID) => taskID.trim()))];
	const parentTaskID = parsed.parentTaskID.trim();
	if (taskIDs.length !== parsed.taskIDs.length || taskIDs.includes('')) {
		return failedUpdate(tasks, 400, 'task ids must be unique and non-empty');
	}
	if (!tasks.some((task) => task.id === parentTaskID)) {
		return failedUpdate(tasks, 404, 'parent task not found');
	}
	if (taskIDs.some((taskID) => taskID === parentTaskID)) {
		return failedUpdate(tasks, 400, 'task cannot be its own parent');
	}
	if (taskIDs.some((taskID) => !tasks.some((task) => task.id === taskID))) {
		return failedUpdate(tasks, 404, 'task not found');
	}
	return {
		tasks: tasks.map((task) => taskIDs.includes(task.id) ? { ...task, parentTaskID } : task),
		response: { status: 200, body: { ok: true } }
	};
}

function isNullableString(value: unknown): value is string | null {
	return value === null || typeof value === 'string';
}

function normalizedParentTaskID(value: string | null): string | null {
	if (value === null) return null;
	const trimmedValue = value.trim();
	return trimmedValue ? trimmedValue : null;
}

function failedUpdate(tasks: Task[], status: number, error: string): DevFlowTaskRelationshipMockResult {
	return { tasks, response: { status, body: { error } } };
}
