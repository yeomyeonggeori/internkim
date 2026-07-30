import { afterAll, beforeAll, describe, expect, test } from 'bun:test';
import { buildEmployeeLeaveFixture } from '../../../dev-attendance-leave-fixture';
import {
	buildLeaveHistory,
	filterLeaveHistory,
	milliDaysValue
} from '../../../src/routes/attendance/leave/leave-history-model';

let originalState: unknown;

beforeAll(() => {
	originalState = Reflect.get(globalThis, '$state');
	Reflect.set(globalThis, '$state', <Value>(value: Value): Value => value);
});

afterAll(() => {
	if (originalState === undefined) {
		Reflect.deleteProperty(globalThis, '$state');
	} else {
		Reflect.set(globalThis, '$state', originalState);
	}
});

describe('employee leave history model', () => {
	test('unifies requests and ledger entries in latest-first order', () => {
		const fixture = buildEmployeeLeaveFixture();
		const history = buildLeaveHistory(fixture.requests, fixture.ledgerEntries, fixture.leaveTypes);

		expect(history[0].occurredAt).toBe('2026-07-27T13:20:00+09:00');
		expect(history.some((item) => item.kind === 'request')).toBe(true);
		expect(history.some((item) => item.kind === 'ledger')).toBe(true);
		expect(filterLeaveHistory(history, 'requests').every((item) => item.kind === 'request')).toBe(
			true
		);
		expect(filterLeaveHistory(history, 'balance').every((item) => item.kind === 'ledger')).toBe(
			true
		);
	});

	test('projects linked balance and untracked leave without estimating it', () => {
		const fixture = buildEmployeeLeaveFixture();
		const history = buildLeaveHistory(fixture.requests, fixture.ledgerEntries, fixture.leaveTypes);
		const pendingRequest = history.find((item) => item.id === 'request:leave-request-pending');
		const sickRequest = history.find((item) => item.id === 'request:leave-request-rejected');

		expect(pendingRequest).toMatchObject({
			kind: 'request',
			balanceAfterMilliDays: 13500,
			isUntracked: false
		});
		expect(sickRequest).toMatchObject({ kind: 'request', isUntracked: true });
		expect(milliDaysValue(13500)).toBe('13.5');
	});
});

describe('leave request draft', () => {
	test('keeps server-backed date and partial-time fields separate', async () => {
		const { LeaveRequestDraft } =
			await import('../../../src/routes/attendance/leave/leave-request-draft.svelte');
		const fixture = buildEmployeeLeaveFixture();
		const draft = new LeaveRequestDraft();

		draft.synchronize(fixture.leaveTypes, '2026-08-03');
		expect(draft.previewRequest()).toEqual({
			leaveTypeID: 'annual',
			unit: 'fullDay',
			startDate: '2026-08-03',
			endDate: '2026-08-03'
		});

		draft.setUnit('quarterDay');
		expect(draft.partialPeriod).toBe('custom');
		expect(draft.previewRequest()).toBe(null);
		draft.startTime = '15:00';

		expect(draft.submission()).toEqual({
			mode: 'create',
			request: {
				leaveTypeID: 'annual',
				unit: 'quarterDay',
				startDate: '2026-08-03',
				partialPeriod: 'custom',
				startTime: '15:00',
				reason: ''
			}
		});
	});

	test('loads a pending request for editing and tracks removed evidence', async () => {
		const { LeaveRequestDraft } =
			await import('../../../src/routes/attendance/leave/leave-request-draft.svelte');
		const fixture = buildEmployeeLeaveFixture();
		const request = fixture.requests[0];
		const draft = new LeaveRequestDraft();

		draft.loadRequest(request);
		draft.reason = '수정한 개인 일정';
		draft.removeExistingAttachment('leave-attachment-pending');

		expect(draft.submission()).toEqual({
			mode: 'edit',
			request: {
				leaveTypeID: 'annual',
				unit: 'quarterDay',
				startDate: '2026-08-03',
				partialPeriod: 'custom',
				startTime: '14:00',
				reason: '수정한 개인 일정',
				revision: 1,
				removedAttachmentIDs: ['leave-attachment-pending']
			}
		});
		expect(draft.existingAttachments).toEqual([]);
	});

	test('keeps an inactive leave type while resubmitting an existing request', async () => {
		const { LeaveRequestDraft } =
			await import('../../../src/routes/attendance/leave/leave-request-draft.svelte');
		const fixture = buildEmployeeLeaveFixture();
		const request = {
			...fixture.requests[1],
			leaveTypeID: 'inactive-type',
			leaveTypeName: '이전 특별 휴가'
		};
		const draft = new LeaveRequestDraft();

		draft.loadRequest(request);
		draft.synchronize(
			[
				...fixture.leaveTypes,
				{
					id: 'inactive-type',
					name: '이전 특별 휴가',
					balanceMode: 'none' as const,
					allowedUnits: ['quarterDay' as const],
					includeInSummary: false,
					isActive: false,
					requiresHireDate: false
				}
			],
			'2026-08-03'
		);
		draft.response = '증빙을 보완했습니다.';

		expect(draft.leaveTypeID).toBe('inactive-type');
		expect(draft.submission()).toMatchObject({
			mode: 'resubmit',
			request: {
				response: '증빙을 보완했습니다.'
			}
		});
	});

	test('fully resets a resubmission draft for the next request', async () => {
		const { LeaveRequestDraft } =
			await import('../../../src/routes/attendance/leave/leave-request-draft.svelte');
		const fixture = buildEmployeeLeaveFixture();
		const draft = new LeaveRequestDraft();

		draft.loadRequest(fixture.requests[1]);
		draft.response = '증빙을 보완했습니다.';
		draft.setAttachments([
			new File(['updated evidence'], 'updated.pdf', {
				type: 'application/pdf'
			})
		]);
		draft.reset(fixture.leaveTypes, '2026-08-03');

		expect({
			requestID: draft.requestID,
			leaveTypeID: draft.leaveTypeID,
			unit: draft.unit,
			startDate: draft.startDate,
			endDate: draft.endDate,
			partialPeriod: draft.partialPeriod,
			startTime: draft.startTime,
			reason: draft.reason,
			response: draft.response,
			attachments: draft.attachments,
			existingAttachments: draft.existingAttachments,
			removedAttachmentIDs: draft.removedAttachmentIDs
		}).toEqual({
			requestID: '',
			leaveTypeID: 'annual',
			unit: 'fullDay',
			startDate: '2026-08-03',
			endDate: '2026-08-03',
			partialPeriod: 'morning',
			startTime: '',
			reason: '',
			response: '',
			attachments: [],
			existingAttachments: [],
			removedAttachmentIDs: []
		});
	});
});
