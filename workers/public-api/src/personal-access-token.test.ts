import { describe, expect, test } from 'bun:test';
import {
	PersonalTokenCache,
	RecordRefused,
	callerOfPersonalAccessToken,
	isPersonalAccessToken,
	keyCacheSeconds,
	permissionNamed,
	type Caller,
	type FetchDocument
} from './personal-access-token';

const credentials = { projectURL: 'https://plane.supabase.co', serviceRoleKey: 'service-role' };

function recordAnswering(rows: unknown, status = 200): { fetchDocument: FetchDocument; asked: string[] } {
	const asked: string[] = [];
	const fetchDocument: FetchDocument = async (url) => {
		asked.push(url);
		return { ok: status < 400, status, json: async () => rows };
	};
	return { fetchDocument, asked };
}

const oneTokenRow = [
	{ name: 'laptop', permission: 'write', member: { id: 'm1', email: 'Someone@Example.com', company_id: 'c1' } }
];

describe('telling a personal access token from anything else', () => {
	test('is the prefix the web app issues', () => {
		expect(isPersonalAccessToken('ik_0123')).toBe(true);
		expect(isPersonalAccessToken('eyJhbGciOi')).toBe(false);
	});
});

describe('the permission a key carries', () => {
	test('is one of the three the ladder has', () => {
		expect(permissionNamed('read')).toBe('read');
		expect(permissionNamed('write')).toBe('write');
		expect(permissionNamed('delete')).toBe('delete');
	});

	test('is read when the record says something else, so an unknown value widens nothing', () => {
		expect(permissionNamed(null)).toBe('read');
		expect(permissionNamed('admin')).toBe('read');
		expect(permissionNamed(3)).toBe('read');
	});
});

describe('resolving a key to the member who holds it', () => {
	test('asks the credential table for the hash of the key, never the key', async () => {
		const { fetchDocument, asked } = recordAnswering(oneTokenRow);
		await callerOfPersonalAccessToken(credentials, 'ik_secret', fetchDocument);

		const url = new URL(asked[0]);
		expect(url.pathname).toBe('/rest/v1/credential');
		expect(url.searchParams.get('kind')).toBe('eq.api_key');
		expect(url.searchParams.get('select')).toBe('name,permission,member(id,email,company_id)');
		expect(url.searchParams.get('external_id')).toStartWith('eq.');
		expect(asked[0]).not.toContain('ik_secret');
	});

	test('answers the address and company the call runs as', async () => {
		const { fetchDocument } = recordAnswering(oneTokenRow);
		expect(await callerOfPersonalAccessToken(credentials, 'ik_secret', fetchDocument)).toEqual({
			email: 'someone@example.com',
			companyID: 'c1',
			memberID: 'm1',
			tokenName: 'laptop',
			permission: 'write'
		});
	});

	test('answers nobody for a key the record does not hold', async () => {
		const { fetchDocument } = recordAnswering([]);
		expect(await callerOfPersonalAccessToken(credentials, 'ik_unknown', fetchDocument)).toBeNull();
	});

	test('answers nobody for a bearer that is not a personal access token, without asking the record', async () => {
		const { fetchDocument, asked } = recordAnswering(oneTokenRow);
		expect(await callerOfPersonalAccessToken(credentials, 'not-a-key', fetchDocument)).toBeNull();
		expect(asked).toHaveLength(0);
	});

	test('answers nobody when the row names no member', async () => {
		const { fetchDocument } = recordAnswering([{ permission: 'delete', member: null }]);
		expect(await callerOfPersonalAccessToken(credentials, 'ik_orphan', fetchDocument)).toBeNull();
	});

	test('refuses rather than answering nobody when the record itself is unreachable', async () => {
		const { fetchDocument } = recordAnswering(null, 500);
		expect(callerOfPersonalAccessToken(credentials, 'ik_secret', fetchDocument)).rejects.toThrow(RecordRefused);
	});
});

describe('the key cache each colo keeps', () => {
	function cacheOver(callers: Caller[]): { cache: PersonalTokenCache; reads: number[] } {
		const reads = [0];
		const cache = new PersonalTokenCache(async () => {
			reads[0] += 1;
			return callers[Math.min(reads[0] - 1, callers.length - 1)];
		});
		return { cache, reads };
	}

	const caller: Caller = { email: 'someone@example.com', companyID: 'c1', permission: 'delete' };

	test('reads the record once for a key presented again inside its life', async () => {
		const { cache, reads } = cacheOver([caller]);
		expect(await cache.callerOf('ik_secret', 1_000)).toEqual(caller);
		expect(await cache.callerOf('ik_secret', 1_000 + keyCacheSeconds * 1000 - 1)).toEqual(caller);
		expect(reads[0]).toBe(1);
	});

	test('reads the record again once the key has been held that long', async () => {
		const { cache, reads } = cacheOver([caller]);
		await cache.callerOf('ik_secret', 1_000);
		await cache.callerOf('ik_secret', 1_000 + keyCacheSeconds * 1000);
		expect(reads[0]).toBe(2);
	});

	test('keeps nothing for a key the record does not hold', async () => {
		const reads = [0];
		const cache = new PersonalTokenCache(async () => {
			reads[0] += 1;
			return null;
		});
		expect(await cache.callerOf('ik_unknown', 1_000)).toBeNull();
		expect(await cache.callerOf('ik_unknown', 1_001)).toBeNull();
		expect(reads[0]).toBe(2);
	});
});
