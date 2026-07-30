import { describe, expect, test } from 'bun:test';
import {
	cancelEmployeeLeaveRequest,
	createEmployeeLeaveRequest,
	EmployeeLeaveAPIError,
	employeeLeaveAttachmentURL,
	fetchEmployeeLeave,
	previewEmployeeLeave,
	resubmitEmployeeLeaveRequest,
	updateEmployeeLeaveRequest
} from '../../../src/routes/attendance/leave/employee-leave-api';
import { buildEmployeeLeaveFixture } from '../../../dev-attendance-leave-fixture';
import { createMockFetch } from '../test-fetch';

describe('employee leave API', () => {
	test('loads leave state without browser caching and normalizes optional fields', async () => {
		const originalFetch = globalThis.fetch;
		const fixture = buildEmployeeLeaveFixture();
		fixture.requests[0] = {
			...fixture.requests[0],
			endDate: '',
			startTime: '',
			endTime: '',
			adminResponse: '',
			updatedAt: '',
			attachments: []
		};
		let requestedURL = '';
		let requestOptions: RequestInit | undefined;
		try {
			globalThis.fetch = createMockFetch(async (input, init) => {
				requestedURL = String(input);
				requestOptions = init;
				return Response.json(fixture);
			});

			const payload = await fetchEmployeeLeave();

			expect(requestedURL).toBe('/attendance/api/leave');
			expect(requestOptions).toEqual({ credentials: 'include', cache: 'no-store' });
			expect(payload.requests[0]).toMatchObject({
				endDate: undefined,
				startTime: undefined,
				endTime: undefined,
				adminResponse: undefined,
				updatedAt: undefined,
				attachments: []
			});
		} finally {
			globalThis.fetch = originalFetch;
		}
	});

	test('requests the server preview without empty optional fields', async () => {
		const originalFetch = globalThis.fetch;
		let requestedBody: Record<string, unknown> = {};
		try {
			globalThis.fetch = createMockFetch(async (_input, init) => {
				requestedBody = JSON.parse(String(init?.body)) as Record<string, unknown>;
				return Response.json({
					occurrences: [
						{
							date: '2026-08-03',
							startTime: '09:00',
							endTime: '18:00',
							deductionMilliDays: 1000
						}
					],
					excludedDates: [],
					totalDeductionMilliDays: 1000
				});
			});

			const preview = await previewEmployeeLeave({
				leaveTypeID: ' annual ',
				unit: 'fullDay',
				startDate: '2026-08-03',
				endDate: ' ',
				startTime: ''
			});

			expect(requestedBody).toEqual({
				leaveTypeID: 'annual',
				unit: 'fullDay',
				startDate: '2026-08-03'
			});
			expect(preview.totalDeductionMilliDays).toBe(1000);
		} finally {
			globalThis.fetch = originalFetch;
		}
	});

	test('submits create, update, and resubmit requests as JSON metadata with repeated attachments', async () => {
		const originalFetch = globalThis.fetch;
		const capturedRequests: Array<{ url: string; formData: FormData }> = [];
		try {
			globalThis.fetch = createMockFetch(async (input, init) => {
				capturedRequests.push({
					url: String(input),
					formData: init?.body as FormData
				});
				return new Response(null, { status: 204 });
			});
			const attachment = new File(['evidence'], 'evidence.pdf', {
				type: 'application/pdf'
			});

			await createEmployeeLeaveRequest(
				{
					leaveTypeID: 'annual',
					unit: 'halfDay',
					startDate: '2026-08-04',
					partialPeriod: 'custom',
					startTime: ' 10:00 ',
					reason: ' personal schedule '
				},
				[attachment]
			);
			await resubmitEmployeeLeaveRequest(
				'request 1',
				{
					leaveTypeID: 'annual',
					unit: 'quarterDay',
					startDate: '2026-08-07',
					partialPeriod: 'afternoon',
					reason: ' updated reason ',
					response: ' added evidence '
				},
				[attachment]
			);
			await updateEmployeeLeaveRequest(
				'request 2',
				{
					leaveTypeID: 'annual',
					unit: 'fullDay',
					startDate: '2026-08-10',
					endDate: '2026-08-10',
					reason: ' revised schedule ',
					revision: 3,
					removedAttachmentIDs: ['attachment-1']
				},
				[attachment]
			);

			expect(capturedRequests[0].url).toBe('/attendance/api/leave-requests');
			expect(JSON.parse(String(capturedRequests[0].formData.get('request')))).toEqual({
				leaveTypeID: 'annual',
				unit: 'halfDay',
				startDate: '2026-08-04',
				partialPeriod: 'custom',
				startTime: '10:00',
				reason: 'personal schedule'
			});
			expect(capturedRequests[0].formData.getAll('attachments')).toEqual([attachment]);
			expect(capturedRequests[1].url).toBe(
				'/attendance/api/leave-requests/request%201/resubmit'
			);
			expect(JSON.parse(String(capturedRequests[1].formData.get('request')))).toEqual({
				leaveTypeID: 'annual',
				unit: 'quarterDay',
				startDate: '2026-08-07',
				partialPeriod: 'afternoon',
				reason: 'updated reason',
				response: 'added evidence'
			});
			expect(capturedRequests[2].url).toBe(
				'/attendance/api/leave-requests/request%202/update'
			);
			expect(JSON.parse(String(capturedRequests[2].formData.get('request')))).toEqual({
				leaveTypeID: 'annual',
				unit: 'fullDay',
				startDate: '2026-08-10',
				endDate: '2026-08-10',
				reason: 'revised schedule',
				revision: 3,
				removedAttachmentIDs: ['attachment-1']
			});
		} finally {
			globalThis.fetch = originalFetch;
		}
	});

	test('cancels by encoded request ID and exposes only a recognized error code', async () => {
		const originalFetch = globalThis.fetch;
		let requestedURL = '';
		let caughtError: unknown;
		try {
			globalThis.fetch = createMockFetch(async (input) => {
				requestedURL = String(input);
				return Response.json(
					{ code: 'invalidStatus', error: 'already cancelled with private details' },
					{ status: 409 }
				);
			});

			try {
				await cancelEmployeeLeaveRequest('request 1');
			} catch (error) {
				caughtError = error;
			}

			expect(caughtError instanceof EmployeeLeaveAPIError).toBe(true);
			if (!(caughtError instanceof EmployeeLeaveAPIError)) {
				throw new Error('expected EmployeeLeaveAPIError');
			}
			expect(caughtError.code).toBe('invalidStatus');
			expect(caughtError.status).toBe(409);
			expect(caughtError.message.includes('private details')).toBe(false);
			expect(requestedURL).toBe('/attendance/api/leave-requests/request%201/cancel');
			expect(employeeLeaveAttachmentURL('request 1', 'file 1')).toBe(
				'/attendance/api/leave-requests/request%201/attachments/file%201'
			);
		} finally {
			globalThis.fetch = originalFetch;
		}
	});

	test('falls back without exposing an unknown or non-JSON error response', async () => {
		const originalFetch = globalThis.fetch;
		const caughtErrors: unknown[] = [];
		try {
			globalThis.fetch = createMockFetch(async () =>
				Response.json(
					{ code: 'unexpectedCode', error: 'internal database details' },
					{ status: 500 }
				)
			);
			try {
				await previewEmployeeLeave({
					leaveTypeID: 'annual',
					unit: 'fullDay',
					startDate: '2026-08-03'
				});
			} catch (error) {
				caughtErrors.push(error);
			}
			globalThis.fetch = createMockFetch(async () =>
				new Response('<html>proxy failure</html>', {
					status: 502,
					headers: { 'Content-Type': 'text/html' }
				})
			);
			try {
				await previewEmployeeLeave({
					leaveTypeID: 'annual',
					unit: 'fullDay',
					startDate: '2026-08-03'
				});
			} catch (error) {
				caughtErrors.push(error);
			}

			expect(caughtErrors.length).toBe(2);
			expect(
				caughtErrors.every(
					(error) => error instanceof EmployeeLeaveAPIError && error.code === null
				)
			).toBe(true);
			expect(
				caughtErrors.every(
					(error) => error instanceof Error && !error.message.includes('database')
				)
			).toBe(true);
		} finally {
			globalThis.fetch = originalFetch;
		}
	});
});
