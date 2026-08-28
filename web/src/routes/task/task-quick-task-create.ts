import { createQuickTask, type TaskQuickTaskRequest } from './task-api';
import type { TaskQuickTaskCreateResult } from './task-types';

export async function requestQuickTaskCreation(request: TaskQuickTaskRequest, fallbackMessage: string): Promise<TaskQuickTaskCreateResult> {
	const result = await createQuickTask(request, fallbackMessage);
	if (result.status === 'skipped_duplicate') return 'duplicate';
	return 'created';
}
