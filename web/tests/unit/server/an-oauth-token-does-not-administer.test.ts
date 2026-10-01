import { describe, expect, test } from 'bun:test';
import { signedInAccessTokenOf } from '../../../src/lib/server/member-request';

const credentials = {
	projectURL: 'https://project.example.test',
	serviceRoleKey: 'service-role',
	signingKey: '{}'
};

function encodedPart(value: unknown): string {
	return Buffer.from(JSON.stringify(value)).toString('base64url');
}

function bearing(claims: Record<string, unknown>): Request {
	const token = `${encodedPart({ alg: 'ES256', typ: 'JWT' })}.${encodedPart(claims)}.signature`;
	return new Request('https://intern.example.test/api/member/invite', {
		method: 'POST',
		headers: { Authorization: `Bearer ${token}` }
	});
}

async function statusOf(answer: () => Promise<unknown>): Promise<number> {
	try {
		await answer();
		return 200;
	} catch (thrown) {
		const refusal = thrown as { status?: number };
		if (typeof refusal.status !== 'number') throw thrown;
		return refusal.status;
	}
}

describe('administering the company', () => {
	test('is refused to a token the member granted to another application', async () => {
		const request = bearing({ sub: 'member-account', role: 'authenticated', client_id: 'a-registered-client' });
		expect(await statusOf(() => signedInAccessTokenOf(request, credentials))).toBe(403);
	});

	test("is open to the member's own signed-in session", async () => {
		const request = bearing({ sub: 'member-account', role: 'authenticated', session_id: 'a-session' });
		expect(await statusOf(() => signedInAccessTokenOf(request, credentials))).toBe(200);
	});
});
