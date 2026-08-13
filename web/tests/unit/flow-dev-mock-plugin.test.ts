import { describe, expect, test } from 'bun:test';
import { createDevFlowMockResponse, createDevFlowMockState } from '../../dev-flow-mock-plugin';
import type { FlowState, FlowTask } from '../../src/routes/flow/flow-types';

describe('dev flow mock plugin', () => {
	test('provides completed attendance detail preview tasks for today', () => {
		const state = createDevFlowMockState('kim@example.com');
		const previewTasks = state.flowState.tasks.filter((task) => task.id.startsWith('attendance-preview-task-'));

		expect(previewTasks.map((task) => task.content)).toEqual([
			'근무 기록 카드 UI 정리',
			'날짜별 근태 편집 흐름 검증'
		]);
		expect(previewTasks.every((task) => task.status === '완료')).toBe(true);
	});

	test('creates a task with a generated id when the draft sends a blank id', async () => {
		const state = createDevFlowMockState('admin@example.com');
		const member = state.flowState.members[0];
		const parent = state.flowState.tasks[0];
		if (!member) throw new Error('expected mock member');
		if (!parent) throw new Error('expected mock task');

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
				parentTaskID: parent.id,
				content: '새 업무',
				status: '진행'
			})
		});
		const createdTask = state.flowState.tasks.at(-1);

		expect(response).toEqual({ status: 200, body: { ok: true } });
		expect(createdTask?.id.startsWith('dev-flow-task-')).toBe(true);
		expect(createdTask?.status).toBe('진행');
		expect(createdTask?.parentTaskID).toBe(parent.id);
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

	test('updates and clears only a development task parent relationship', async () => {
		const state = createDevFlowMockState('admin@example.com');
		const child = state.flowState.tasks[0] as FlowTask;
		const parent = state.flowState.tasks[1] as FlowTask;
		const originalChild = { ...child };

		const linkedResponse = await createDevFlowMockResponse(state, {
			method: 'PATCH',
			pathname: `/flow/api/tasks/${encodeURIComponent(child.id)}/parent`,
			searchParams: new URLSearchParams(),
			body: JSON.stringify({ parentTaskID: parent.id })
		});
		const linkedChild = state.flowState.tasks.find((task) => task.id === child.id);

		expect(linkedResponse).toEqual({ status: 200, body: { ok: true } });
		expect(linkedChild).toEqual({ ...originalChild, parentTaskID: parent.id });

		const unlinkedResponse = await createDevFlowMockResponse(state, {
			method: 'PATCH',
			pathname: `/flow/api/tasks/${encodeURIComponent(child.id)}/parent`,
			searchParams: new URLSearchParams(),
			body: JSON.stringify({ parentTaskID: null })
		});
		const unlinkedChild = state.flowState.tasks.find((task) => task.id === child.id);

		expect(unlinkedResponse).toEqual({ status: 200, body: { ok: true } });
		expect(unlinkedChild).toEqual({ ...originalChild, parentTaskID: undefined });
	});

	test('updates multiple development task parent relationships together', async () => {
		const state = createDevFlowMockState('admin@example.com');
		const [parent, firstChild, secondChild] = state.flowState.tasks;
		if (!parent || !firstChild || !secondChild) throw new Error('expected mock tasks');

		const response = await createDevFlowMockResponse(state, {
			method: 'PATCH',
			pathname: '/flow/api/tasks/parents',
			searchParams: new URLSearchParams(),
			body: JSON.stringify({ taskIDs: [firstChild.id, secondChild.id], parentTaskID: parent.id })
		});

		expect(response).toEqual({ status: 200, body: { ok: true } });
		expect(state.flowState.tasks
			.filter((task) => task.id === firstChild.id || task.id === secondChild.id)
			.every((task) => task.parentTaskID === parent.id)).toBe(true);
	});

	test('deletes a development task through the mock task endpoint', async () => {
		const state = createDevFlowMockState('admin@example.com');
		const currentState = (await createDevFlowMockResponse(state, {
			method: 'GET',
			pathname: '/flow/api/state',
			searchParams: new URLSearchParams()
		}))?.body as FlowState;
		const task = currentState.tasks[0] as FlowTask;

		const response = await createDevFlowMockResponse(state, {
			method: 'DELETE',
			pathname: `/flow/api/tasks/${task.id}`,
			searchParams: new URLSearchParams()
		});
		const updatedState = (await createDevFlowMockResponse(state, {
			method: 'GET',
			pathname: '/flow/api/state',
			searchParams: new URLSearchParams()
		}))?.body as FlowState;

		expect(response).toEqual({ status: 200, body: { ok: true } });
		expect(updatedState.tasks.some((value) => value.id === task.id)).toBe(false);
	});

	test('clears child relationships when deleting their development parent', async () => {
		const state = createDevFlowMockState('admin@example.com');
		const parent = state.flowState.tasks.find((task) =>
			state.flowState.tasks.some((candidate) => candidate.parentTaskID === task.id)
		);
		if (!parent) throw new Error('expected mock parent task');

		const response = await createDevFlowMockResponse(state, {
			method: 'DELETE',
			pathname: `/flow/api/tasks/${parent.id}`,
			searchParams: new URLSearchParams()
		});

		expect(response).toEqual({ status: 200, body: { ok: true } });
		expect(state.flowState.tasks.some((task) => task.parentTaskID === parent.id)).toBe(false);
	});

	test('moves a development task through the mock board move endpoint', async () => {
		const state = createDevFlowMockState('admin@example.com');
		const currentState = (await createDevFlowMockResponse(state, {
			method: 'GET',
			pathname: '/flow/api/state',
			searchParams: new URLSearchParams()
		}))?.body as FlowState;
		const movedTask = currentState.tasks.find((value) => value.status === '요청') as FlowTask;
		const beforeTask = currentState.tasks.find((value) => value.status === '진행') as FlowTask;

		const response = await createDevFlowMockResponse(state, {
			method: 'POST',
			pathname: '/flow/api/tasks/move',
			searchParams: new URLSearchParams(),
			body: JSON.stringify({
				taskID: movedTask.id,
				targetStatus: '진행',
				beforeTaskID: beforeTask.id
			})
		});
		const updatedState = (await createDevFlowMockResponse(state, {
			method: 'GET',
			pathname: '/flow/api/state',
			searchParams: new URLSearchParams()
		}))?.body as FlowState;
		const updatedTask = updatedState.tasks.find((value) => value.id === movedTask.id);
		const updatedBeforeTask = updatedState.tasks.find((value) => value.id === beforeTask.id);

		expect(response).toEqual({ status: 200, body: { ok: true } });
		expect(updatedTask).toMatchObject({
			status: '진행'
		});
		expect((updatedTask?.statusRank ?? 0) < (updatedBeforeTask?.statusRank ?? 0)).toBe(true);
	});

	test('resets mutated development flow state through the mock reset endpoint', async () => {
		const state = createDevFlowMockState('admin@example.com');
		const currentState = (await createDevFlowMockResponse(state, {
			method: 'GET',
			pathname: '/flow/api/state',
			searchParams: new URLSearchParams()
		}))?.body as FlowState;
		const movedTask = currentState.tasks.find((value) => value.status === '요청') as FlowTask;

		await createDevFlowMockResponse(state, {
			method: 'POST',
			pathname: '/flow/api/tasks/move',
			searchParams: new URLSearchParams(),
			body: JSON.stringify({
				taskID: movedTask.id,
				targetStatus: '진행',
				beforeTaskID: null
			})
		});

		const response = await createDevFlowMockResponse(state, {
			method: 'POST',
			pathname: '/flow/api/test/reset',
			searchParams: new URLSearchParams()
		});
		const resetState = (await createDevFlowMockResponse(state, {
			method: 'GET',
			pathname: '/flow/api/state',
			searchParams: new URLSearchParams()
		}))?.body as FlowState;

		expect(response).toEqual({ status: 200, body: { ok: true } });
		expect(resetState.tasks.find((value) => value.id === movedTask.id)).toMatchObject({
			status: '요청',
			statusRank: movedTask.statusRank
		});
	});

	test('accepts a no-op board move through the mock board move endpoint', async () => {
		const state = createDevFlowMockState('admin@example.com');
		const currentState = (await createDevFlowMockResponse(state, {
			method: 'GET',
			pathname: '/flow/api/state',
			searchParams: new URLSearchParams()
		}))?.body as FlowState;
		const plannedTasks = currentState.tasks.filter((value) => value.status === '예정');
		const lastTask = plannedTasks.at(-1) as FlowTask;

		const response = await createDevFlowMockResponse(state, {
			method: 'POST',
			pathname: '/flow/api/tasks/move',
			searchParams: new URLSearchParams(),
			body: JSON.stringify({
				taskID: lastTask.id,
				targetStatus: lastTask.status,
				beforeTaskID: null
			})
		});

		expect(response).toEqual({ status: 200, body: { ok: true } });
	});

	test('rejects invalid board move requests through the mock board move endpoint', async () => {
		const state = createDevFlowMockState('admin@example.com');
		const currentState = (await createDevFlowMockResponse(state, {
			method: 'GET',
			pathname: '/flow/api/state',
			searchParams: new URLSearchParams()
		}))?.body as FlowState;
		const requestedTask = currentState.tasks.find((value) => value.status === '요청') as FlowTask;
		const plannedTask = currentState.tasks.find((value) => value.status === '예정') as FlowTask;
		const invalidMoveRequests = [
			{
				body: { taskID: ' ', targetStatus: '진행', beforeTaskID: null },
				status: 400
			},
			{
				body: { taskID: 'missing-task', targetStatus: '진행', beforeTaskID: null },
				status: 404
			},
			{
				body: { taskID: requestedTask.id, targetStatus: '기각', beforeTaskID: null },
				status: 400
			},
			{
				body: { taskID: requestedTask.id, targetStatus: '요청', beforeTaskID: requestedTask.id },
				status: 400
			},
			{
				body: { taskID: requestedTask.id, targetStatus: '진행', beforeTaskID: plannedTask.id },
				status: 400
			}
		];

		for (const invalidMoveRequest of invalidMoveRequests) {
			const response = await createDevFlowMockResponse(state, {
				method: 'POST',
				pathname: '/flow/api/tasks/move',
				searchParams: new URLSearchParams(),
				body: JSON.stringify(invalidMoveRequest.body)
			});

			expect(response?.status).toBe(invalidMoveRequest.status);
		}
	});
});
