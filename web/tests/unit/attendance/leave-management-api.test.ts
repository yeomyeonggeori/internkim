import { describe, expect, test } from 'bun:test';
import {
	adjustManagedLeave,
	applyLegacyAbsenceMigration,
	fetchLegacyAbsenceMigrationPreview
} from '../../../src/routes/attendance/management/leave-management-api';
import { createMockFetch } from '../test-fetch';

describe('leave management API', () => {
	test('loads and applies the legacy leave migration with its preview fingerprint', async () => {
		const originalFetch = globalThis.fetch;
		const requests: Array<{ url: string; init?: RequestInit }> = [];
		try {
			globalThis.fetch = createMockFetch(async (input, init) => {
				requests.push({ url: String(input), init });
				if (String(input).endsWith('/legacy-migration')) {
					return Response.json({
						candidateLeaveCount: 3,
						candidateOccurrenceCount: 4,
						alreadyMigratedCount: 0,
						conflictCount: 0,
						preservedOtherCount: 1,
						items: [],
						fingerprint: 'sha256:preview'
					});
				}
				return Response.json({
					batch: {
						id: 'batch-1',
						status: 'applied',
						leaveCount: 3,
						preservedOtherCount: 1,
						createdAt: '2026-07-31T00:00:00Z'
					}
				});
			});

			const preview = await fetchLegacyAbsenceMigrationPreview();
			const batch = await applyLegacyAbsenceMigration(preview.fingerprint);

			expect(preview.candidateLeaveCount).toBe(3);
			expect(batch.leaveCount).toBe(3);
			expect(requests).toHaveLength(2);
			expect(requests[0]).toMatchObject({
				url: '/attendance/api/leave-management/legacy-migration',
				init: { credentials: 'include', cache: 'no-store' }
			});
			expect(requests[1]).toMatchObject({
				url: '/attendance/api/leave-management/legacy-migration/apply',
				init: {
					method: 'POST',
					credentials: 'include',
					headers: { 'Content-Type': 'application/json' },
					body: JSON.stringify({ fingerprint: 'sha256:preview' })
				}
			});
		} finally {
			globalThis.fetch = originalFetch;
		}
	});

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
