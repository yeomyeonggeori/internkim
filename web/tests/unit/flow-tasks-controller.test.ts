// Flow 업무 컨트롤러의 권한 방어를 검증한다.
import { describe, expect, test } from 'bun:test';
import { flowText } from '../../src/routes/flow/text';
import type { FlowMember, FlowSummary, FlowTask } from '../../src/routes/flow/flow-types';

describe('flow tasks controller', () => {
	test('does not save a task when the current member cannot update it', async () => {
		const originalState = Reflect.get(globalThis, '$state');
		Reflect.set(globalThis, '$state', <Value>(value: Value): Value => value);
		const { createFlowTasksController } = await import('../../src/routes/flow/flow-tasks-controller.svelte');
		const controller = createFlowTasksController();
		const task = flowTask({
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
				summary: flowSummary({
					currentUserEmail: 'viewer@example.com',
					members: [
						flowMember({ id: 'owner', email: 'owner@example.com' }),
						flowMember({ id: 'participant', email: 'participant@example.com' }),
						flowMember({ id: 'viewer', email: 'viewer@example.com' })
					],
					tasks: [task]
				}),
				text: flowText.ko,
				loadFlow: async () => true,
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
		const { createFlowTasksController } = await import('../../src/routes/flow/flow-tasks-controller.svelte');
		const controller = createFlowTasksController();
		const task = flowTask({ ownerID: '', ownerName: '', participantIDs: [], participantNames: [] });

		try {
			controller.sync({
				summary: flowSummary({
					currentUserEmail: 'admin@example.com',
					isAdmin: true,
					members: [
						flowMember({ id: 'admin', name: '관리자', email: 'admin@example.com' }),
						flowMember({ id: 'owner', name: '담당자', email: 'owner@example.com' }),
						flowMember({ id: 'participant', name: '참여자', email: 'participant@example.com' })
					],
					tasks: [task]
				}),
				text: flowText.ko,
				loadFlow: async () => true,
				setPageErrorMessage: () => {}
			});
			controller.openTask(task);

			controller.setParticipantNames(['담당자']);
			expect(controller.taskDraft?.ownerID).toBe('owner');
			expect(controller.taskDraft?.ownerName).toBe('담당자');

			controller.setParticipantNames(['담당자', '참여자']);
			expect(controller.taskDraft?.ownerID).toBe('');
			expect(controller.taskDraft?.ownerName).toBe('');

			controller.setParticipantNames([]);
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

	test('falls back to the current member when a request has no selected target', async () => {
		const controller = await syncedController();
		controller.setParticipantFilterIDs([]);

		controller.createTask('요청');

		expect(controller.taskDraft?.requesterID).toBe('requester');
		expect(controller.taskDraft?.participantIDs).toEqual(['requester']);
		expect(controller.taskDraft?.ownerID).toBe('requester');
	});
});

async function syncedController() {
	const originalState = Reflect.get(globalThis, '$state');
	Reflect.set(globalThis, '$state', <Value>(value: Value): Value => value);
	const { createFlowTasksController } = await import('../../src/routes/flow/flow-tasks-controller.svelte');
	const controller = createFlowTasksController();
	controller.sync({
		summary: flowSummary({
			currentUserEmail: 'requester@example.com',
			source: 'supabase',
			members: [
				flowMember({ id: 'requester', name: '요청자', email: 'requester@example.com' }),
				flowMember({ id: 'target', name: '대상자', email: 'target@example.com' })
			]
		}),
		text: flowText.ko,
		loadFlow: async () => true,
		setPageErrorMessage: () => {}
	});
	if (originalState === undefined) Reflect.deleteProperty(globalThis, '$state');
	else Reflect.set(globalThis, '$state', originalState);
	return controller;
}

function flowSummary(overrides: Partial<FlowSummary>): FlowSummary {
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

function flowMember(overrides: Partial<FlowMember>): FlowMember {
	return {
		id: 'member-1',
		name: '최견본',
		email: 'lee@example.com',
		role: 'member',
		mattermostStatus: '',
		activeTaskCount: 0,
		completeTaskCount: 0,
		...overrides
	};
}

function flowTask(overrides: Partial<FlowTask>): FlowTask {
	return {
		id: 'task-1',
		ownerID: 'owner',
		ownerName: '담당자',
		participantIDs: ['owner'],
		participantNames: ['담당자'],
		business: '',
		type: '기능',
		content: '업무',
		goal: '완료',
		size: 'M',
		status: '예정',
		statusRank: 0,
		weekCode: '26W23',
		flag: 0,
		...overrides
	};
}
