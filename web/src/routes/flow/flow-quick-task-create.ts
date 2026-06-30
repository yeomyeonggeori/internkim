// AI 업무 빠른 추가 API 응답을 화면 제어용 결과 타입으로 변환한다.
import { createQuickFlowTask, type FlowQuickTaskRequest } from './flow-api';
import type { FlowQuickTaskCreateResult } from './flow-types';

export async function requestQuickTaskCreation(request: FlowQuickTaskRequest, fallbackMessage: string): Promise<FlowQuickTaskCreateResult> {
	const result = await createQuickFlowTask(request, fallbackMessage);
	if (result.status === 'skipped_duplicate') return 'duplicate';
	return 'created';
}
