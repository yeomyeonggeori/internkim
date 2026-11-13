import { afterAll, describe, expect, test } from 'bun:test';
import { desktopWorkForwarder, desktopWorkPath, gatewayUnreachedStatus, noGatewayStatus } from './desktop-work';

type Asked = { path: string; authorization: string | null; body: string };

const asked: Asked[] = [];
let gatewayAnswer = () => Response.json({ requestID: 'desk-1', status: 200, body: { output: '{}' } });

const gateway = Bun.serve({
	hostname: '127.0.0.1',
	port: 0,
	async fetch(request) {
		asked.push({ path: new URL(request.url).pathname, authorization: request.headers.get('Authorization'), body: await request.text() });
		return gatewayAnswer();
	}
});
afterAll(() => gateway.stop(true));

const work = JSON.stringify({ requestID: 'desk-1', desktopOwner: 'sample@example.com', work: { arguments: ['snapshot'] } });

function handedOver(): Request {
	return new Request(`http://127.0.0.1:18091${desktopWorkPath}`, { method: 'POST', body: work });
}

function forwarderTo(gatewayURL: string | undefined) {
	return desktopWorkForwarder({ gatewayURL, companyID: 'company 1', hostAccessToken: async () => 'fresh.host.session' });
}

describe('desktop work handed to the relay', () => {
	test('goes to the gateway over http as this company host, and its answer comes back unchanged', async () => {
		asked.length = 0;
		const answered = await forwarderTo(`ws://127.0.0.1:${gateway.port}/`)(handedOver());

		expect(asked).toEqual([{ path: '/company/company%201/desktop-work', authorization: 'Bearer fresh.host.session', body: work }]);
		expect(answered.status).toBe(200);
		expect(await answered.json()).toEqual({ requestID: 'desk-1', status: 200, body: { output: '{}' } });
	});

	test('carries a refusal from the gateway back with its own status', async () => {
		gatewayAnswer = () => Response.json({ error: 'this computer stands by for the company and does not serve it' }, { status: 409 });
		const answered = await forwarderTo(`http://127.0.0.1:${gateway.port}`)(handedOver());

		expect(answered.status).toBe(409);
		expect(await answered.json()).toEqual({ error: 'this computer stands by for the company and does not serve it' });
	});

	test('is answered by the relay itself when it has no gateway', async () => {
		const answered = await forwarderTo(undefined)(handedOver());

		expect(answered.status).toBe(noGatewayStatus);
		expect(await answered.text()).toContain('GATEWAY_URL');
	});

	test('names the gateway it could not reach', async () => {
		const closed = Bun.serve({ hostname: '127.0.0.1', port: 0, fetch: () => new Response() });
		const address = `http://127.0.0.1:${closed.port}`;
		closed.stop(true);

		const answered = await forwarderTo(address)(handedOver());

		expect(answered.status).toBe(gatewayUnreachedStatus);
		expect(await answered.text()).toContain(`${address}/company/company%201/desktop-work`);
	});
});
