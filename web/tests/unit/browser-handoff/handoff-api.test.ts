import { describe, expect, mock, test } from 'bun:test';

type HostAnswer = { status: number; body: unknown };

const asked: { capability: string; body?: Record<string, unknown> }[] = [];
let answer: () => Promise<HostAnswer> = async () => ({ status: 200, body: {} });

mock.module('../../../src/lib/host-bridge', () => ({
	callCompanyApp: async (call: { capability: string; body?: Record<string, unknown> }) => {
		asked.push(call);
		return answer();
	}
}));

const { finishHandoff, sendHandoffInputs, watchHandoff } = await import('../../../src/lib/browser-handoff/handoff-api');

describe('watching a browser handoff', () => {
	test('a waiting handoff is watched with its message and viewport', async () => {
		answer = async () => ({
			status: 200,
			body: { handoffID: 'handoff-1', message: '로그인해 주세요', expiresAt: '2026-09-17T00:15:00.000Z', viewport: { width: 1280, height: 800 } }
		});

		expect(await watchHandoff('handoff-1', { width: 390, height: 700 })).toEqual({
			state: 'watching',
			watch: { handoffID: 'handoff-1', message: '로그인해 주세요', expiresAt: '2026-09-17T00:15:00.000Z', viewport: { width: 1280, height: 800 } }
		});
		expect(asked.at(-1)).toEqual({
			capability: 'person.browser.handoff.watch',
			body: { handoffID: 'handoff-1', viewport: { width: 390, height: 700 } }
		});
	});

	test('a finished handoff and someone else’s handoff are told apart', async () => {
		answer = async () => ({ status: 404, body: {} });
		expect(await watchHandoff('handoff-1', null)).toEqual({ state: 'missing' });

		answer = async () => ({ status: 403, body: {} });
		expect(await watchHandoff('handoff-1', null)).toEqual({ state: 'refused' });
	});

	test('a device that could not open the browser says why', async () => {
		answer = async () => ({ status: 500, body: { error: 'the device browser at http://127.0.0.1:9230 has no open page' } });
		expect(await watchHandoff('handoff-1', null)).toEqual({
			state: 'failed',
			reason: 'the device browser at http://127.0.0.1:9230 has no open page'
		});

		answer = async () => ({ status: 502, body: 'bad gateway' });
		expect(await watchHandoff('handoff-1', null)).toEqual({ state: 'failed', reason: 'the device answered 502' });
	});

	test('an answer that is not a handoff is an error', async () => {
		answer = async () => ({ status: 200, body: { handoffID: 'handoff-1' } });

		await expect(watchHandoff('handoff-1', null)).rejects.toThrow('returned 200');
	});
});

describe('controlling and finishing a browser handoff', () => {
	test('inputs are sent to the handoff and a refusal is an error', async () => {
		answer = async () => ({ status: 200, body: { accepted: 1 } });
		await sendHandoffInputs('handoff-1', [{ type: 'reload' }]);
		expect(asked.at(-1)).toEqual({ capability: 'person.browser.handoff.input', body: { handoffID: 'handoff-1', inputs: [{ type: 'reload' }] } });

		answer = async () => ({ status: 400, body: {} });
		await expect(sendHandoffInputs('handoff-1', [{ type: 'reload' }])).rejects.toThrow('400');

		answer = async () => ({ status: 500, body: { error: 'the device browser page is closed' } });
		await expect(sendHandoffInputs('handoff-1', [{ type: 'reload' }])).rejects.toThrow('the device browser page is closed');
	});

	test('finishing a handoff that already ended is not an error', async () => {
		answer = async () => ({ status: 404, body: {} });

		await finishHandoff('handoff-1', 'completed');

		expect(asked.at(-1)).toEqual({ capability: 'person.browser.handoff.finish', body: { handoffID: 'handoff-1', outcome: 'completed' } });
	});
});
