// Flow 개발 mock 플러그인의 쓰기 요청 처리를 검증합니다.
import { describe, expect, test } from 'bun:test';
import { createDevFlowMockResponse, createDevFlowMockState } from '../../dev-flow-mock-plugin';
import type { FlowState, FlowTask } from '../../src/routes/flow/flow-types';

describe('dev flow mock plugin', () => {
	test('creates a task with a generated id when the draft sends a blank id', async () => {
		const state = createDevFlowMockState('admin@example.com');
		const member = state.flowState.members[0];
		if (!member) throw new Error('expected mock member');

		const response = await createDevFlowMockResponse(state, {
			method: 'POST',
			pathname: '/flow/api/tasks',
			searchParams: new URLSearchParams(),
			body: JSON.stringify({
				id: '',
				ownerID: member.id,
				ownerName: member.name,
				participantIDs: [member.id],
				participantNames: [member.name],
				content: '새 업무',
				status: '진행'
			})
		});
		const createdTask = state.flowState.tasks.at(-1);

		expect(response).toEqual({ status: 200, body: { ok: true } });
		expect(createdTask?.id.startsWith('dev-flow-task-')).toBe(true);
		expect(createdTask?.status).toBe('진행');
	});

	test('updates a development task through the mock task endpoint', async () => {
		const state = createDevFlowMockState('admin@example.com');
		const currentState = (await createDevFlowMockResponse(state, {
			method: 'GET',
			pathname: '/flow/api/state',
			searchParams: new URLSearchParams()
		}))?.body as FlowState;
		const task = currentState.tasks.find((value) => value.status === '요청') as FlowTask;

		const response = await createDevFlowMockResponse(state, {
			method: 'PUT',
			pathname: `/flow/api/tasks/${task.id}`,
			searchParams: new URLSearchParams(),
			body: JSON.stringify({ ...task, status: '예정', statusRank: 4096 })
		});
		const updatedState = (await createDevFlowMockResponse(state, {
			method: 'GET',
			pathname: '/flow/api/state',
			searchParams: new URLSearchParams()
		}))?.body as FlowState;

		expect(response).toEqual({ status: 200, body: { ok: true } });
		expect(updatedState.tasks.find((value) => value.id === task.id)).toMatchObject({
			status: '예정',
			statusRank: 4096
		});
	});
});
