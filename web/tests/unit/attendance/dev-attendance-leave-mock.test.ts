import { describe, expect, test } from 'bun:test';
import {
	buildEmployeeLeavePreview,
	createDevEmployeeLeaveMockResponse,
	createDevEmployeeLeaveMockState
} from '../../../dev-attendance-leave-mock';
import type { EmployeeLeavePayload } from '../../../src/routes/attendance/leave/employee-leave-types';

describe('development employee leave mock', () => {
	test('provides a complete manual leave test scenario', () => {
		const state = createDevEmployeeLeaveMockState();

		expect(state.payload.summary).toEqual({
			usedMilliDays: 1000,
			reservedMilliDays: 500,
			availableMilliDays: 15500
		});
		expect(state.payload.requests.map((request) => request.status)).toEqual([
			'pending',
			'needsChanges',
			'approved',
			'approved',
			'cancelled',
			'rejected'
		]);
		expect(
			state.payload.requests.find((request) => request.id === 'leave-request-approved-morning')
		).toMatchObject({
			startDate: '2026-08-10',
			status: 'approved',
			canCancel: true
		});
		expect(
			state.payload.requests.find((request) => request.id === 'leave-request-cancelled')
		).toMatchObject({
			status: 'cancelled',
			canCancel: false
		});
	});

	test('previews server-owned work time and excluded dates', () => {
		const preview = buildEmployeeLeavePreview({
			leaveTypeID: 'annual',
			unit: 'fullDay',
			startDate: '2026-08-14',
			endDate: '2026-08-17'
		});
		const customQuarterDay = buildEmployeeLeavePreview({
			leaveTypeID: 'annual',
			unit: 'quarterDay',
			startDate: '2026-08-18',
			partialPeriod: 'custom',
			startTime: '11:00'
		});

		expect(preview).toEqual({
			occurrences: [
				{
					date: '2026-08-14',
					startTime: '09:00',
					endTime: '18:00',
					deductionMilliDays: 1000
				}
			],
			excludedDates: [
				{ date: '2026-08-15', reason: 'nonWorkingDay' },
				{ date: '2026-08-16', reason: 'nonWorkingDay' },
				{ date: '2026-08-17', reason: 'holiday' }
			],
			totalDeductionMilliDays: 1000
		});
		expect(customQuarterDay.occurrences[0]).toEqual({
			date: '2026-08-18',
			startTime: '11:00',
			endTime: '14:00',
			deductionMilliDays: 250
		});
		expect(
			buildEmployeeLeavePreview({
				leaveTypeID: 'annual',
				unit: 'halfDay',
				startDate: '2026-08-18',
				partialPeriod: 'morning'
			}).occurrences[0]
		).toEqual({
			date: '2026-08-18',
			startTime: '09:00',
			endTime: '14:00',
			deductionMilliDays: 500
		});
	});

	test('creates and cancels requests while reloading balance state', () => {
		const state = createDevEmployeeLeaveMockState();
		const initialUsedMilliDays = state.payload.summary.usedMilliDays;
		const initialReservedMilliDays = state.payload.summary.reservedMilliDays;
		const initialAvailableMilliDays = state.payload.summary.availableMilliDays;
		const multipart = multipartRequest({
			request: {
				leaveTypeID: 'annual',
				unit: 'fullDay',
				startDate: '2026-08-18',
				endDate: '2026-08-18',
				reason: 'local request'
			},
			fileName: 'evidence.pdf',
			fileContent: 'evidence'
		});

		const createResponse = createDevEmployeeLeaveMockResponse(state, {
			method: 'POST',
			pathname: '/attendance/api/leave-requests',
			body: multipart.body,
			contentType: multipart.contentType
		});
		const createdRequest = state.payload.requests[0];

		expect(createResponse?.status).toBe(200);
		expect(createdRequest).toMatchObject({
			status: 'pending',
			deductionMilliDays: 1000,
			reason: 'local request',
			canCancel: true
		});
		expect(createdRequest.attachments[0]).toMatchObject({
			fileName: 'evidence.pdf',
			contentType: 'application/pdf'
		});
		expect(state.payload.summary).toEqual({
			usedMilliDays: initialUsedMilliDays,
			reservedMilliDays: initialReservedMilliDays + 1000,
			availableMilliDays: initialAvailableMilliDays - 1000
		});

		const cancelResponse = createDevEmployeeLeaveMockResponse(state, {
			method: 'POST',
			pathname: `/attendance/api/leave-requests/${createdRequest.id}/cancel`
		});
		const getResponse = createDevEmployeeLeaveMockResponse(state, {
			method: 'GET',
			pathname: '/attendance/api/leave'
		});
		const payload = getResponse?.body as EmployeeLeavePayload;

		expect(cancelResponse?.status).toBe(200);
		expect(payload.requests.some((request) => request.id === createdRequest.id)).toBe(false);
		expect(payload.summary.reservedMilliDays).toBe(initialReservedMilliDays);
		expect(payload.summary.availableMilliDays).toBe(initialAvailableMilliDays);
		expect(payload.ledgerEntries[0]).toMatchObject({
			operationType: 'release'
		});
		expect(payload.ledgerEntries[0].requestID).toBe(undefined);
		expect(
			payload.ledgerEntries.filter((entry) => entry.operationKey.includes(createdRequest.id)).length
		).toBe(2);
		expect(
			payload.ledgerEntries
				.filter((entry) => entry.operationKey.includes(createdRequest.id))
				.every((entry) => entry.requestID === undefined)
		).toBe(true);
	});

	test('updates each leave balance without adding excluded accounts to the summary', () => {
		const state = createDevEmployeeLeaveMockState();
		const initialSummary = structuredClone(state.payload.summary);
		const rewardBalance = state.payload.leaveTypes.find(
			(leaveType) => leaveType.id === 'reward'
		)?.balance;
		const familyEventBalance = state.payload.leaveTypes.find(
			(leaveType) => leaveType.id === 'family-event'
		)?.balance;

		for (const [leaveTypeID, startDate] of [
			['reward', '2026-08-18'],
			['family-event', '2026-08-19']
		]) {
			const multipart = multipartRequest({
				request: {
					leaveTypeID,
					unit: 'fullDay',
					startDate,
					endDate: startDate,
					reason: ''
				},
				fileName: `${leaveTypeID}.pdf`,
				fileContent: leaveTypeID
			});
			const response = createDevEmployeeLeaveMockResponse(state, {
				method: 'POST',
				pathname: '/attendance/api/leave-requests',
				body: multipart.body,
				contentType: multipart.contentType
			});
			expect(response?.status).toBe(200);
		}

		expect(rewardBalance).toEqual({
			usedMilliDays: 0,
			reservedMilliDays: 1000,
			availableMilliDays: 1000
		});
		expect(familyEventBalance).toEqual({
			usedMilliDays: 0,
			reservedMilliDays: 1000,
			availableMilliDays: 2000
		});
		expect(state.payload.summary).toEqual({
			usedMilliDays: initialSummary.usedMilliDays,
			reservedMilliDays: initialSummary.reservedMilliDays + 1000,
			availableMilliDays: initialSummary.availableMilliDays - 1000
		});
	});

	test('resubmits only a request in the changes-requested state', () => {
		const state = createDevEmployeeLeaveMockState();
		const requestID = 'leave-request-needs-changes';
		const multipart = multipartRequest({
			request: {
				leaveTypeID: 'annual',
				unit: 'quarterDay',
				startDate: '2026-08-07',
				partialPeriod: 'afternoon',
				reason: 'updated reason',
				response: 'added evidence'
			},
			fileName: 'updated.pdf',
			fileContent: 'updated'
		});

		const response = createDevEmployeeLeaveMockResponse(state, {
			method: 'POST',
			pathname: `/attendance/api/leave-requests/${requestID}/resubmit`,
			body: multipart.body,
			contentType: multipart.contentType
		});
		const request = state.payload.requests.find((candidate) => candidate.id === requestID);

		expect(response?.status).toBe(200);
		expect(request).toMatchObject({
			status: 'pending',
			reason: 'updated reason',
			adminResponse: '방문 일정을 확인할 수 있는 자료를 보완해 주세요.',
			canResubmit: false
		});
		expect(request?.attachments.length).toBe(2);
	});

	test('updates a pending request with revision and attachment replacement', () => {
		const state = createDevEmployeeLeaveMockState();
		const requestID = 'leave-request-pending';
		const initialReservedMilliDays = state.payload.summary.reservedMilliDays;
		const initialRequestDeductionMilliDays =
			state.payload.requests.find((candidate) => candidate.id === requestID)?.deductionMilliDays ??
			0;
		const multipart = multipartRequest({
			request: {
				leaveTypeID: 'annual',
				unit: 'halfDay',
				startDate: '2026-08-04',
				partialPeriod: 'afternoon',
				reason: 'updated schedule',
				revision: 1,
				removedAttachmentIDs: ['leave-attachment-pending']
			},
			fileName: 'replacement.pdf',
			fileContent: 'replacement'
		});

		const response = createDevEmployeeLeaveMockResponse(state, {
			method: 'POST',
			pathname: `/attendance/api/leave-requests/${requestID}/update`,
			body: multipart.body,
			contentType: multipart.contentType
		});
		const request = state.payload.requests.find((candidate) => candidate.id === requestID);

		expect(response?.status).toBe(200);
		expect(request).toMatchObject({
			status: 'pending',
			unit: 'halfDay',
			reason: 'updated schedule',
			revision: 2,
			canEdit: true,
			deductionMilliDays: 500
		});
		expect(request?.attachments.map((attachment) => attachment.fileName)).toEqual([
			'replacement.pdf'
		]);
		expect(state.payload.summary.reservedMilliDays).toBe(
			initialReservedMilliDays - initialRequestDeductionMilliDays + 500
		);

		const staleResponse = createDevEmployeeLeaveMockResponse(state, {
			method: 'POST',
			pathname: `/attendance/api/leave-requests/${requestID}/update`,
			body: multipart.body,
			contentType: multipart.contentType
		});
		expect(staleResponse).toMatchObject({
			status: 409,
			body: { code: 'invalidStatus' }
		});
	});

	test('preserves an approved future request as cancelled and restores used balance', () => {
		const state = createDevEmployeeLeaveMockState();
		const approvedRequest = state.payload.requests.find(
			(request) => request.id === 'leave-request-approved'
		);
		if (!approvedRequest) throw new Error('approved fixture request is required');
		approvedRequest.startDate = '2027-08-03';
		approvedRequest.canCancel = true;
		const initialUsedMilliDays = state.payload.summary.usedMilliDays;
		const initialAvailableMilliDays = state.payload.summary.availableMilliDays;

		const response = createDevEmployeeLeaveMockResponse(state, {
			method: 'POST',
			pathname: `/attendance/api/leave-requests/${approvedRequest.id}/cancel`
		});
		const cancelledRequest = state.payload.requests.find(
			(request) => request.id === approvedRequest.id
		);

		expect(response?.status).toBe(200);
		expect(cancelledRequest).toMatchObject({
			status: 'cancelled',
			canCancel: false,
			canResubmit: false
		});
		expect(state.payload.summary.usedMilliDays).toBe(
			initialUsedMilliDays - approvedRequest.deductionMilliDays
		);
		expect(state.payload.summary.availableMilliDays).toBe(
			initialAvailableMilliDays + approvedRequest.deductionMilliDays
		);
		expect(state.payload.ledgerEntries[0]).toMatchObject({
			operationType: 'restore',
			requestID: approvedRequest.id
		});
	});

	test('returns structured error codes for invalid request actions', () => {
		const state = createDevEmployeeLeaveMockState();
		const response = createDevEmployeeLeaveMockResponse(state, {
			method: 'POST',
			pathname: '/attendance/api/leave-requests/leave-request-approved/resubmit'
		});

		expect(response).toEqual({
			status: 409,
			body: {
				code: 'invalidStatus',
				error: 'leave request cannot be resubmitted'
			}
		});
	});
});

function multipartRequest(input: {
	request: Record<string, unknown>;
	fileName: string;
	fileContent: string;
}): { body: string; contentType: string } {
	const boundary = 'attendance-leave-boundary';
	const body = [
		`--${boundary}`,
		'Content-Disposition: form-data; name="request"',
		'Content-Type: application/json',
		'',
		JSON.stringify(input.request),
		`--${boundary}`,
		`Content-Disposition: form-data; name="attachments"; filename="${input.fileName}"`,
		'Content-Type: application/pdf',
		'',
		input.fileContent,
		`--${boundary}--`,
		''
	].join('\r\n');
	return {
		body,
		contentType: `multipart/form-data; boundary=${boundary}`
	};
}
