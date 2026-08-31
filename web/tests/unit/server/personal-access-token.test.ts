import { describe, expect, test } from 'bun:test';
import { isPersonalAccessToken } from '../../../src/lib/server/control-plane';
import {
	fullPublicAPIPermission,
	publicAPIPermissionOf
} from '../../../src/lib/public-api-permission';
import { memberAccessTokenOf } from '../../../src/lib/server/member-request';

const credentials = { projectURL: 'https://example.supabase.co', serviceRoleKey: 'service-role' };

function asking(authorization?: string): Request {
	return new Request('https://intern.kim/api/member/me', {
		headers: authorization ? { authorization } : {}
	});
}

async function refusalOf(call: Promise<unknown>): Promise<{ status: number; message: string }> {
	try {
		await call;
	} catch (thrown) {
		const refusal = thrown as { status?: number; body?: { message?: string } };
		return { status: refusal.status ?? 0, message: refusal.body?.message ?? '' };
	}
	throw new Error('expected a refusal');
}

describe('telling a personal access token from the session a browser carries', () => {
	test('a personal access token is known by its prefix', () => {
		expect(isPersonalAccessToken('ik_0f1e2d3c')).toBe(true);
	});

	test('a supabase access token is not', () => {
		expect(isPersonalAccessToken('eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.body.signature')).toBe(false);
	});
});

describe('what a call may present as the member it acts for', () => {
	test('a session token is passed through untouched, and nothing is exchanged', async () => {
		expect(await memberAccessTokenOf(asking('Bearer eyJhbGciOi.body.sig'), credentials)).toEqual({
			accessToken: 'eyJhbGciOi.body.sig',
			permission: 'delete'
		});
	});

	test('presenting nothing is refused before anything is looked up', async () => {
		expect(await refusalOf(memberAccessTokenOf(asking(), credentials))).toEqual({
			status: 401,
			message: 'sign in first'
		});
	});

	test('an authorization that is not a bearer is refused the same way', async () => {
		expect(await refusalOf(memberAccessTokenOf(asking('Basic bGVlOnNlY3JldA=='), credentials))).toEqual({
			status: 401,
			message: 'sign in first'
		});
	});
});

describe('the rung a key is issued on', () => {
	test('a key that says nothing reaches as far as its holder', () => {
		expect(fullPublicAPIPermission).toBe('delete');
	});

	test('the ladder has three rungs and answers with the one asked for', () => {
		expect(publicAPIPermissionOf('read')).toBe('read');
		expect(publicAPIPermissionOf('write')).toBe('write');
		expect(publicAPIPermissionOf('delete')).toBe('delete');
	});

	test('anything else is no rung at all', () => {
		expect(publicAPIPermissionOf('admin')).toBe(null);
		expect(publicAPIPermissionOf('')).toBe(null);
		expect(publicAPIPermissionOf(3)).toBe(null);
	});
});
