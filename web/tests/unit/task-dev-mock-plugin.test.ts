import { describe, expect, test } from 'bun:test';
import { createDevTaskMockResponse, createDevTaskMockState } from '../../dev-task-mock-plugin';
import type { TaskState, Task } from '../../src/routes/task/task-types';

describe('dev flow mock plugin', () => {
	test('provides completed attendance detail preview tasks for today', () => {
		const state = createDevTaskMockState('kim@example.com');
		const previewTasks = state.taskState.tasks.filter((task) => task.id.startsWith('attendance-preview-task-'));

		expect(previewTasks.map((task) => task.content)).toEqual([
			'근무 기록 카드 UI 정리',
			'날짜별 근태 편집 흐름 검증'
		]);
		expect(previewTasks.every((task) => task.status === 'completed')).toBe(true);
	});

	test('creates a task with a generated id when the draft sends a blank id', async () => {
		const state = createDevTaskMockState('admin@example.com');
		const member = state.taskState.members[0];
		const parent = state.taskState.tasks[0];
		if (!member) throw new Error('expected mock member');
		if (!parent) throw new Error('expected mock task');

		const response = await createDevTaskMockResponse(state, {
			method: 'POST',
			pathname: '/task/api/tasks',
			searchParams: new URLSearchParams(),
			body: JSON.stringify({
				id: '',
				ownerID: member.id,
				ownerName: member.name,
				participantIDs: [member.id],
				participantNames: [member.name],
				parentTaskID: parent.id,
				content: '새 업무',
				status: 'in_progress'
			})
		});
		const createdTask = state.taskState.tasks.at(-1);

		expect(response).toEqual({ status: 200, body: { ok: true } });
		expect(createdTask?.id.startsWith('dev-task-task-')).toBe(true);
		expect(createdTask?.status).toBe('in_progress');
		expect(createdTask?.parentTaskID).toBe(parent.id);
	});

	test('updates a development task through the mock task endpoint', async () => {
		const state = createDevTaskMockState('admin@example.com');
		const currentState = (await createDevTaskMockResponse(state, {
			method: 'GET',
			pathname: '/task/api/state',
			searchParams: new URLSearchParams()
		}))?.body as TaskState;
		const task = currentState.tasks.find((value) => value.status === 'requested') as Task;

		const response = await createDevTaskMockResponse(state, {
			method: 'PUT',
			pathname: `/task/api/tasks/${task.id}`,
			searchParams: new URLSearchParams(),
			body: JSON.stringify({ ...task, status: 'planned', statusRank: 4096 })
		});
		const updatedState = (await createDevTaskMockResponse(state, {
			method: 'GET',
			pathname: '/task/api/state',
			searchParams: new URLSearchParams()
		}))?.body as TaskState;

		expect(response).toEqual({ status: 200, body: { ok: true } });
		expect(updatedState.tasks.find((value) => value.id === task.id)).toMatchObject({
			status: 'planned',
			statusRank: 4096
		});
	});

	test('updates and clears only a development task parent relationship', async () => {
		const state = createDevTaskMockState('admin@example.com');
		const child = state.taskState.tasks[0] as Task;
		const parent = state.taskState.tasks[1] as Task;
		const originalChild = { ...child };

		const linkedResponse = await createDevTaskMockResponse(state, {
			method: 'PATCH',
			pathname: `/task/api/tasks/${encodeURIComponent(child.id)}/parent`,
			searchParams: new URLSearchParams(),
			body: JSON.stringify({ parentTaskID: parent.id })
		});
		const linkedChild = state.taskState.tasks.find((task) => task.id === child.id);

		expect(linkedResponse).toEqual({ status: 200, body: { ok: true } });
		expect(linkedChild).toEqual({ ...originalChild, parentTaskID: parent.id });

		const unlinkedResponse = await createDevTaskMockResponse(state, {
			method: 'PATCH',
			pathname: `/task/api/tasks/${encodeURIComponent(child.id)}/parent`,
			searchParams: new URLSearchParams(),
			body: JSON.stringify({ parentTaskID: null })
		});
		const unlinkedChild = state.taskState.tasks.find((task) => task.id === child.id);

		expect(unlinkedResponse).toEqual({ status: 200, body: { ok: true } });
		expect(unlinkedChild).toEqual({ ...originalChild, parentTaskID: undefined });
	});

	test('updates multiple development task parent relationships together', async () => {
		const state = createDevTaskMockState('admin@example.com');
		const [parent, firstChild, secondChild] = state.taskState.tasks;
		if (!parent || !firstChild || !secondChild) throw new Error('expected mock tasks');

		const response = await createDevTaskMockResponse(state, {
			method: 'PATCH',
			pathname: '/task/api/tasks/parents',
			searchParams: new URLSearchParams(),
			body: JSON.stringify({ taskIDs: [firstChild.id, secondChild.id], parentTaskID: parent.id })
		});

		expect(response).toEqual({ status: 200, body: { ok: true } });
		expect(state.taskState.tasks
			.filter((task) => task.id === firstChild.id || task.id === secondChild.id)
			.every((task) => task.parentTaskID === parent.id)).toBe(true);
	});

	test('deletes a development task through the mock task endpoint', async () => {
		const state = createDevTaskMockState('admin@example.com');
		const currentState = (await createDevTaskMockResponse(state, {
			method: 'GET',
			pathname: '/task/api/state',
			searchParams: new URLSearchParams()
		}))?.body as TaskState;
		const task = currentState.tasks[0] as Task;

		const response = await createDevTaskMockResponse(state, {
			method: 'DELETE',
			pathname: `/task/api/tasks/${task.id}`,
			searchParams: new URLSearchParams()
		});
		const updatedState = (await createDevTaskMockResponse(state, {
			method: 'GET',
			pathname: '/task/api/state',
			searchParams: new URLSearchParams()
		}))?.body as TaskState;

		expect(response).toEqual({ status: 200, body: { ok: true } });
		expect(updatedState.tasks.some((value) => value.id === task.id)).toBe(false);
	});

	test('clears child relationships when deleting their development parent', async () => {
		const state = createDevTaskMockState('admin@example.com');
		const parent = state.taskState.tasks.find((task) =>
			state.taskState.tasks.some((candidate) => candidate.parentTaskID === task.id)
		);
		if (!parent) throw new Error('expected mock parent task');

		const response = await createDevTaskMockResponse(state, {
			method: 'DELETE',
			pathname: `/task/api/tasks/${parent.id}`,
			searchParams: new URLSearchParams()
		});

		expect(response).toEqual({ status: 200, body: { ok: true } });
		expect(state.taskState.tasks.some((task) => task.parentTaskID === parent.id)).toBe(false);
	});

	test('moves a development task through the mock board move endpoint', async () => {
		const state = createDevTaskMockState('admin@example.com');
		const currentState = (await createDevTaskMockResponse(state, {
			method: 'GET',
			pathname: '/task/api/state',
			searchParams: new URLSearchParams()
		}))?.body as TaskState;
		const movedTask = currentState.tasks.find((value) => value.status === 'requested') as Task;
		const beforeTask = currentState.tasks.find((value) => value.status === 'in_progress') as Task;

		const response = await createDevTaskMockResponse(state, {
			method: 'POST',
			pathname: '/task/api/tasks/move',
			searchParams: new URLSearchParams(),
			body: JSON.stringify({
				taskID: movedTask.id,
				targetStatus: 'in_progress',
				beforeTaskID: beforeTask.id
			})
		});
		const updatedState = (await createDevTaskMockResponse(state, {
			method: 'GET',
			pathname: '/task/api/state',
			searchParams: new URLSearchParams()
		}))?.body as TaskState;
		const updatedTask = updatedState.tasks.find((value) => value.id === movedTask.id);
		const updatedBeforeTask = updatedState.tasks.find((value) => value.id === beforeTask.id);

		expect(response).toEqual({ status: 200, body: { ok: true } });
		expect(updatedTask).toMatchObject({
			status: 'in_progress'
		});
		expect((updatedTask?.statusRank ?? 0) < (updatedBeforeTask?.statusRank ?? 0)).toBe(true);
	});

	test('resets mutated development flow state through the mock reset endpoint', async () => {
		const state = createDevTaskMockState('admin@example.com');
		const currentState = (await createDevTaskMockResponse(state, {
			method: 'GET',
			pathname: '/task/api/state',
			searchParams: new URLSearchParams()
		}))?.body as TaskState;
		const movedTask = currentState.tasks.find((value) => value.status === 'requested') as Task;

		await createDevTaskMockResponse(state, {
			method: 'POST',
			pathname: '/task/api/tasks/move',
			searchParams: new URLSearchParams(),
			body: JSON.stringify({
				taskID: movedTask.id,
				targetStatus: 'in_progress',
				beforeTaskID: null
			})
		});

		const response = await createDevTaskMockResponse(state, {
			method: 'POST',
			pathname: '/task/api/test/reset',
			searchParams: new URLSearchParams()
		});
		const resetState = (await createDevTaskMockResponse(state, {
			method: 'GET',
			pathname: '/task/api/state',
			searchParams: new URLSearchParams()
		}))?.body as TaskState;

		expect(response).toEqual({ status: 200, body: { ok: true } });
		expect(resetState.tasks.find((value) => value.id === movedTask.id)).toMatchObject({
			status: 'requested',
			statusRank: movedTask.statusRank
		});
	});

	test('accepts a no-op board move through the mock board move endpoint', async () => {
		const state = createDevTaskMockState('admin@example.com');
		const currentState = (await createDevTaskMockResponse(state, {
			method: 'GET',
			pathname: '/task/api/state',
			searchParams: new URLSearchParams()
		}))?.body as TaskState;
		const plannedTasks = currentState.tasks.filter((value) => value.status === 'planned');
		const lastTask = plannedTasks.at(-1) as Task;

		const response = await createDevTaskMockResponse(state, {
			method: 'POST',
			pathname: '/task/api/tasks/move',
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
		const state = createDevTaskMockState('admin@example.com');
		const currentState = (await createDevTaskMockResponse(state, {
			method: 'GET',
			pathname: '/task/api/state',
			searchParams: new URLSearchParams()
		}))?.body as TaskState;
		const requestedTask = currentState.tasks.find((value) => value.status === 'requested') as Task;
		const plannedTask = currentState.tasks.find((value) => value.status === 'planned') as Task;
		const invalidMoveRequests = [
			{
				body: { taskID: ' ', targetStatus: 'in_progress', beforeTaskID: null },
				status: 400
			},
			{
				body: { taskID: 'missing-task', targetStatus: 'in_progress', beforeTaskID: null },
				status: 404
			},
			{
				body: { taskID: requestedTask.id, targetStatus: 'rejected', beforeTaskID: null },
				status: 400
			},
			{
				body: { taskID: requestedTask.id, targetStatus: 'requested', beforeTaskID: requestedTask.id },
				status: 400
			},
			{
				body: { taskID: requestedTask.id, targetStatus: 'in_progress', beforeTaskID: plannedTask.id },
				status: 400
			}
		];

		for (const invalidMoveRequest of invalidMoveRequests) {
			const response = await createDevTaskMockResponse(state, {
				method: 'POST',
				pathname: '/task/api/tasks/move',
				searchParams: new URLSearchParams(),
				body: JSON.stringify(invalidMoveRequest.body)
			});

			expect(response?.status).toBe(invalidMoveRequest.status);
		}
	});
});
