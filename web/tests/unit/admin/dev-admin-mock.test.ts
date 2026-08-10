import { describe, expect, test } from 'bun:test';
import {
	createDevAdminMockResponse,
	createDevAdminMockState,
	shouldHandleDevAdminMockRequest
} from '../../../dev-admin-mock';

describe('dev admin mock', () => {
	test('presents an enrolled Buzz identity for an authenticated mock user', () => {
		const pathname = '/agent/api/buzz-vault';
		const request = { method: 'GET', pathname, searchParams: new URLSearchParams() };

		expect(shouldHandleDevAdminMockRequest(request.method, pathname)).toBe(true);
		expect(createDevAdminMockResponse(createDevAdminMockState('crm@example.com'), request)).toEqual({
			status: 200,
			body: { found: true }
		});
	});
});
