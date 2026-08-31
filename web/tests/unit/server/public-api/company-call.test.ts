import { describe, expect, test } from 'bun:test';
import { callCompany, gatewayHTTPAddress } from '$lib/server/public-api/company-call';

const environment = {
	GATEWAY_URL: 'wss://gateway.example/',
	GATEWAY_ADMIN_TOKEN: 'the-token'
};

async function refusalOf(call: Promise<unknown>): Promise<{ status: number; message: string }> {
	try {
		await call;
	} catch (thrown) {
		const refusal = thrown as { status?: number; body?: { message?: string } };
		return { status: refusal.status ?? 0, message: refusal.body?.message ?? '' };
	}
	throw new Error('the call was expected to be refused');
}

describe('the gateway address', () => {
	test('is the socket address spoken over http', () => {
		expect(gatewayHTTPAddress('wss://gateway.example/')).toBe('https://gateway.example');
		expect(gatewayHTTPAddress('ws://127.0.0.1:8787')).toBe('http://127.0.0.1:8787');
		expect(gatewayHTTPAddress('https://gateway.example')).toBe('https://gateway.example');
	});
});

describe('a call carried to a company', () => {
	test('names the company, holds the gateway token, and answers what the company said', async () => {
		let seen: { url: string; headers: Record<string, string>; body: unknown } | undefined;
		const answer = await callCompany(
			environment,
			'company one',
			'person.api.request',
			{ path: '/tools' },
			(url, options) => {
				seen = { url, headers: options.headers, body: JSON.parse(options.body) };
				return Promise.resolve({
					status: 200,
					json: async () => ({ requestID: 'r1', status: 201, body: { made: true } })
				});
			}
		);

		expect(answer).toEqual({ status: 201, body: { made: true } });
		expect(seen?.url).toBe('https://gateway.example/company/company%20one/call');
		expect(seen?.headers.Authorization).toBe('Bearer the-token');
		expect(seen?.body).toMatchObject({ capability: 'person.api.request', body: { path: '/tools' } });
	});

	test('is refused before it leaves when no gateway is configured', async () => {
		let asked = false;
		const refusal = await refusalOf(
			callCompany({ GATEWAY_URL: '', GATEWAY_ADMIN_TOKEN: '' }, 'c1', 'person.api.request', {}, () => {
				asked = true;
				return Promise.resolve({ status: 200, json: async () => null });
			})
		);
		expect(refusal.status).toBe(503);
		expect(asked).toBe(false);
	});

	test('turns an answer carrying no usable status into a bad gateway', async () => {
		const refusal = await refusalOf(
			callCompany(environment, 'c1', 'person.api.request', {}, () =>
				Promise.resolve({ status: 500, json: async () => ({ error: 'the durable object fell over' }) })
			)
		);
		expect(refusal.status).toBe(502);
		expect(refusal.message).toContain('500');
	});
});
