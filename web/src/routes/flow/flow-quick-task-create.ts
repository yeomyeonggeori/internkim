import { createQuickFlowTask, type FlowQuickTaskRequest } from './flow-api';
import type { FlowQuickTaskCreateResult } from './flow-types';

export async function requestQuickTaskCreation(request: FlowQuickTaskRequest, fallbackMessage: string): Promise<FlowQuickTaskCreateResult> {
	const result = await createQuickFlowTask(request, fallbackMessage);
	if (result.status === 'skipped_duplicate') return 'duplicate';
	return 'created';
}
