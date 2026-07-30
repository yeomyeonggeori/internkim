import { describe, expect, test } from 'bun:test';
import { adjustManagedLeave } from '../../../src/routes/attendance/management/leave-management-api';
import { createMockFetch } from '../test-fetch';

describe('leave management API', () => {
	test('preserves the typed error code without exposing server prose', async () => {
		const originalFetch = globalThis.fetch;
		try {
			globalThis.fetch = createMockFetch(async () =>
				Response.json(
					{
						code: 'workConflict',
						error: 'leave request conflicts with confirmed work'
					},
					{ status: 409 }
				)
			);

			await expect(
				adjustManagedLeave({
					employeeEmail: 'staff@example.com',
					leaveTypeID: 'annual',
					amountMilliDays: -500,
					kind: 'adjustment',
					reason: '',
					effectiveOn: '2026-07-29',
					expiresOn: ''
				})
			).rejects.toMatchObject({
				name: 'EmployeeLeaveAPIError',
				code: 'workConflict',
				status: 409,
				message: 'Employee leave API request failed'
			});
		} finally {
			globalThis.fetch = originalFetch;
		}
	});
});
