// Flow 업무 컨트롤러의 권한 방어를 검증한다.
import { describe, expect, test } from 'bun:test';
import { taskText } from '../../src/routes/task/text';
import type { TaskMember, TaskSummary, Task } from '../../src/routes/task/task-types';

describe('flow tasks controller', () => {
	test('does not save a task when the current member cannot update it', async () => {
		const originalState = Reflect.get(globalThis, '$state');
		Reflect.set(globalThis, '$state', <Value>(value: Value): Value => value);
		const { createTasksController } = await import('../../src/routes/task/tasks-controller.svelte');
		const controller = createTasksController();
		const task = taskOf({
			ownerID: 'owner',
			participantIDs: ['participant']
		});
		let saveRequestCount = 0;
		const originalFetch = globalThis.fetch;
		const fetchMock: typeof fetch = Object.assign(async () => {
			saveRequestCount += 1;
			return new Response(null, { status: 200 });
		}, { preconnect: originalFetch.preconnect });
		globalThis.fetch = fetchMock;

		try {
			controller.sync({
				summary: taskSummary({
					currentUserEmail: 'viewer@example.com',
					members: [
						taskMember({ id: 'owner', email: 'owner@example.com' }),
						taskMember({ id: 'participant', email: 'participant@example.com' }),
						taskMember({ id: 'viewer', email: 'viewer@example.com' })
					],
					tasks: [task]
				}),
				text: taskText.ko,
				loadTask: async () => true,
				setPageErrorMessage: () => {}
			});
			controller.openTask(task);

			await controller.saveTask();

			expect(saveRequestCount).toBe(0);
			expect(controller.taskDraft).not.toBe(null);
			expect(controller.isSavingTask).toBe(false);
		} finally {
			globalThis.fetch = originalFetch;
			if (originalState === undefined) {
				Reflect.deleteProperty(globalThis, '$state');
			} else {
				Reflect.set(globalThis, '$state', originalState);
			}
		}
	});

	test('recomputes compatibility owner from every participant edit', async () => {
		const originalState = Reflect.get(globalThis, '$state');
		Reflect.set(globalThis, '$state', <Value>(value: Value): Value => value);
		const { createTasksController } = await import('../../src/routes/task/tasks-controller.svelte');
		const controller = createTasksController();
		const task = taskOf({ ownerID: '', ownerName: '', participantIDs: [], participantNames: [] });

		try {
			controller.sync({
				summary: taskSummary({
					currentUserEmail: 'admin@example.com',
					isAdmin: true,
					source: 'supabase',
					members: [
						taskMember({ id: 'admin', name: '관리자', email: 'admin@example.com' }),
						taskMember({ id: 'owner', name: '담당자', email: 'owner@example.com' }),
						taskMember({ id: 'participant', name: '참여자', email: 'participant@example.com' })
					],
					tasks: [task]
				}),
				text: taskText.ko,
				loadTask: async () => true,
				setPageErrorMessage: () => {}
			});
			controller.openTask(task);

			controller.setParticipantIDs(['owner']);
			expect(controller.taskDraft?.ownerID).toBe('owner');
			expect(controller.taskDraft?.ownerName).toBe('담당자');

			controller.setParticipantIDs(['owner', 'participant']);
			expect(controller.taskDraft?.ownerID).toBe('');
			expect(controller.taskDraft?.ownerName).toBe('');

			controller.setParticipantIDs([]);
			expect(controller.taskDraft?.ownerID).toBe('');
			expect(controller.taskDraft?.ownerName).toBe('');
			expect(controller.taskDraft?.participantIDs).toEqual([]);
		} finally {
			if (originalState === undefined) {
				Reflect.deleteProperty(globalThis, '$state');
			} else {
				Reflect.set(globalThis, '$state', originalState);
			}
		}
	});

	test('preserves canonical IDs when edited participants share a display name', async () => {
		const controller = await syncedController([
			taskMember({ id: 'requester', name: '요청자', email: 'requester@example.com' }),
			taskMember({ id: 'target-left', name: '동명이인', email: 'left@example.com' }),
			taskMember({ id: 'target-right', name: '동명이인', email: 'right@example.com' })
		]);
		const task = taskOf({ ownerID: '', ownerName: '', participantIDs: [], participantNames: [] });
		controller.openTask(task);

		controller.setParticipantIDs(['target-left', 'target-right']);

		expect(controller.taskDraft?.participantIDs).toEqual(['target-left', 'target-right']);
		expect(controller.taskDraft?.participantNames).toEqual(['동명이인', '동명이인']);
		expect(controller.taskDraft?.ownerID).toBe('');
	});

	test('keeps the legacy device owner in participant edits without exposing an owner mutation', async () => {
		const controller = await syncedController([
			taskMember({ id: 'owner', name: '담당자', email: 'owner@example.com' }),
			taskMember({ id: 'participant', name: '참여자', email: 'participant@example.com' })
		], 'sqlite', 'owner@example.com');
		const task = taskOf({
			ownerID: 'owner',
			ownerName: '담당자',
			participantIDs: ['owner', 'participant'],
			participantNames: ['담당자', '참여자']
		});
		controller.openTask(task);

		controller.setParticipantIDs(['participant']);

		expect(controller.taskDraft?.ownerID).toBe('owner');
		expect(controller.taskDraft?.ownerName).toBe('담당자');
		expect(controller.taskDraft?.participantIDs).toEqual(['owner', 'participant']);
	});

	test('creates normal work for the current member without requester provenance', async () => {
		const controller = await syncedController();
		controller.setParticipantFilterIDs(['target']);

		controller.createTask('예정');

		expect(controller.taskDraft?.participantIDs).toEqual(['requester']);
		expect(controller.taskDraft?.ownerID).toBe('requester');
		expect(controller.taskDraft?.requesterID).toBe('');
		expect(controller.taskDraft?.requesterName).toBe('');
	});

	test('creates a request for selected targets without adding the requester', async () => {
		const controller = await syncedController();
		controller.setParticipantFilterIDs(['target']);

		controller.createTask('요청');

		expect(controller.taskDraft?.requesterID).toBe('requester');
		expect(controller.taskDraft?.requesterName).toBe('요청자');
		expect(controller.taskDraft?.participantIDs).toEqual(['target']);
		expect(controller.taskDraft?.participantNames).toEqual(['대상자']);
		expect(controller.taskDraft?.ownerID).toBe('target');
		expect(controller.taskDraft?.ownerName).toBe('대상자');
	});

	test('preserves selected member IDs when request targets share a display name', async () => {
		const controller = await syncedController([
			taskMember({ id: 'requester', name: '요청자', email: 'requester@example.com' }),
			taskMember({ id: 'target-left', name: '동명이인', email: 'left@example.com' }),
			taskMember({ id: 'target-right', name: '동명이인', email: 'right@example.com' })
		]);
		controller.setParticipantFilterIDs(['target-left']);

		controller.createTask('요청');

		expect(controller.taskDraft?.participantIDs).toEqual(['target-left']);
		expect(controller.taskDraft?.ownerID).toBe('target-left');
	});

	test('treats an empty everyone viewing filter as no assignment consent and falls back to requester', async () => {
		const controller = await syncedController();
		controller.setParticipantFilterIDs([]);

		controller.createTask('요청');

		expect(controller.taskDraft?.requesterID).toBe('requester');
		expect(controller.taskDraft?.participantIDs).toEqual(['requester']);
		expect(controller.taskDraft?.ownerID).toBe('requester');
		expect(controller.taskDraft?.participantIDs).not.toContain('target');
	});

	test('does not expose an explicit compatibility owner mutation', async () => {
		const controller = await syncedController();

		expect('setTaskOwnerID' in controller).toBe(false);
	});

	test('creates a child draft with the selected parent relationship', async () => {
		const originalState = Reflect.get(globalThis, '$state');
		Reflect.set(globalThis, '$state', <Value>(value: Value): Value => value);
		const { createTasksController } = await import('../../src/routes/task/tasks-controller.svelte');
		const controller = createTasksController();

		try {
			controller.sync({
				summary: taskSummary({
					currentUserEmail: 'owner@example.com',
					members: [taskMember({ id: 'owner', email: 'owner@example.com' })]
				}),
				text: taskText.ko,
				loadTask: async () => true,
				setPageErrorMessage: () => {}
			});

			controller.createChildTask('parent-task');

			expect(controller.taskDraft?.parentTaskID).toBe('parent-task');
			expect(controller.editor.isEditingTask).toBe(true);
		} finally {
			if (originalState === undefined) {
				Reflect.deleteProperty(globalThis, '$state');
			} else {
				Reflect.set(globalThis, '$state', originalState);
			}
		}
	});
});

async function syncedController(members: TaskMember[] = [
	taskMember({ id: 'requester', name: '요청자', email: 'requester@example.com' }),
	taskMember({ id: 'target', name: '대상자', email: 'target@example.com' })
], source = 'supabase', currentUserEmail = 'requester@example.com') {
	const originalState = Reflect.get(globalThis, '$state');
	Reflect.set(globalThis, '$state', <Value>(value: Value): Value => value);
	const { createTasksController } = await import('../../src/routes/task/tasks-controller.svelte');
	const controller = createTasksController();
	controller.sync({
		summary: taskSummary({
			currentUserEmail,
			source,
			members
		}),
		text: taskText.ko,
		loadTask: async () => true,
		setPageErrorMessage: () => {}
	});
	if (originalState === undefined) Reflect.deleteProperty(globalThis, '$state');
	else Reflect.set(globalThis, '$state', originalState);
	return controller;
}

function taskSummary(overrides: Partial<TaskSummary>): TaskSummary {
	return {
		week: {
			code: '26W23',
			startISO: '2026-06-01',
			endISO: '2026-06-07',
			previous: '26W22',
			next: '26W24',
			isCurrent: true
		},
		members: [],
		tasks: [],
		metrics: {
			totalTasks: 0,
			completedTasks: 0,
			requestedTasks: 0,
			pausedTasks: 0,
			stoppedTasks: 0,
			statusCounts: {},
			businessCounts: {},
			typeCounts: {}
		},
		definitions: {
			categories: [],
			types: [],
			sizes: []
		},
		statusOptions: [],
		currentUserEmail: '',
		currentUserName: '',
		isAdmin: false,
		source: 'test',
		...overrides
	};
}

function taskMember(overrides: Partial<TaskMember>): TaskMember {
	return {
		id: 'member-1',
		name: '최견본',
		email: 'member1@example.com',
		role: 'member',
		mattermostStatus: '',
		activeTaskCount: 0,
		completeTaskCount: 0,
		...overrides
	};
}

function taskOf(overrides: Partial<Task>): Task {
	return {
		id: 'task-1',
		ownerID: 'owner',
		ownerName: '담당자',
		participantIDs: ['owner'],
		participantNames: ['담당자'],
		business: '',
		type: '기능',
		content: '업무',
		size: 'M',
		status: '예정',
		statusRank: 0,
		weekCode: '26W23',
		...overrides
	};
}
