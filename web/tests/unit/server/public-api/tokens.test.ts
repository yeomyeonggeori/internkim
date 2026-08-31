import { describe, expect, test } from 'bun:test';
import { issueRefusal, nextUnusedName, revocationRefusal } from '$lib/server/public-api/tokens';
import { reachesPermission } from '$lib/public-api-permission';

describe('a token nobody named', () => {
	test('takes the first unused name in the ladder', () => {
		expect(nextUnusedName([])).toBe('pat-1');
		expect(nextUnusedName([{ name: 'pat-1' }, { name: 'pat-3' }])).toBe('pat-2');
		expect(nextUnusedName([{ name: 'laptop' }])).toBe('pat-1');
	});
});

describe('the rungs of the ladder', () => {
	test('reach downward and never up', () => {
		expect(reachesPermission('delete', 'write')).toBe(true);
		expect(reachesPermission('write', 'write')).toBe(true);
		expect(reachesPermission('write', 'delete')).toBe(false);
		expect(reachesPermission('read', 'write')).toBe(false);
	});
});

describe('making a token', () => {
	const caller = { permission: 'write' as const, tokenName: 'laptop' };

	test('is allowed at or below the rung the caller holds', () => {
		expect(issueRefusal(caller, 'phone', 'write')).toBeNull();
		expect(issueRefusal(caller, 'phone', 'read')).toBeNull();
	});

	test('is refused above the caller’s rung', () => {
		expect(issueRefusal(caller, 'phone', 'delete')?.status).toBe(403);
	});

	test('is refused when it would replace the token making the call', () => {
		expect(issueRefusal(caller, 'laptop', 'write')?.status).toBe(409);
	});

	test('is refused when the name is longer than the store keeps', () => {
		expect(issueRefusal(caller, 'x'.repeat(65), 'write')?.status).toBe(400);
	});

	test('lets a session, which holds no token name, use any name', () => {
		expect(issueRefusal({ permission: 'delete', tokenName: '' }, '', 'delete')).toBeNull();
	});
});

describe('revoking a token', () => {
	test('names the token, and never the one making the call', () => {
		expect(revocationRefusal({ tokenName: 'laptop' }, '')?.status).toBe(400);
		expect(revocationRefusal({ tokenName: 'laptop' }, 'laptop')?.status).toBe(409);
		expect(revocationRefusal({ tokenName: 'laptop' }, 'phone')).toBeNull();
	});
});
