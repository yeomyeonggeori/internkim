import { describe, expect, test } from 'bun:test';
import {
	CallLedger,
	WaitingCalls,
	decideCall,
	parseClientCall,
	parseOneShotCall,
	parseServerMessage,
	serverOfflineStatus
} from './routing';

const call = { kind: 'call', requestID: 'r1', capability: 'person.messenger.channels', body: { limit: 20 } };

describe('parseClientCall', () => {
	test('takes a call naming a request and a capability', () => {
		expect(parseClientCall(call)).toEqual(call as never);
	});

	test('a call with no body carries an empty one', () => {
		expect(parseClientCall({ kind: 'call', requestID: 'r1', capability: 'asset.link' })).toEqual({
			kind: 'call',
			requestID: 'r1',
			capability: 'asset.link',
			body: {}
		} as never);
	});

	test('refuses anything else', () => {
		expect(parseClientCall({ ...call, kind: 'publish' })).toBeNull();
		expect(parseClientCall({ ...call, requestID: '  ' })).toBeNull();
		expect(parseClientCall({ ...call, capability: '' })).toBeNull();
		expect(parseClientCall({ ...call, body: 'a body' })).toBeNull();
		expect(parseClientCall(null)).toBeNull();
	});
});

describe('parseServerMessage', () => {
	test('reads a result and a delivery', () => {
		expect(parseServerMessage({ kind: 'result', requestID: 'r1', status: 200, body: { ok: true } })).toEqual({
			kind: 'result',
			requestID: 'r1',
			status: 200,
			body: { ok: true }
		});
		expect(parseServerMessage({ kind: 'deliver', event: { id: 'e1' }, audienceMemberIDs: ['m1', ''] })).toEqual({
			kind: 'deliver',
			event: { id: 'e1' },
			audienceMemberIDs: ['m1']
		});
	});

	test('an empty audience means everyone rather than nobody', () => {
		const delivery = parseServerMessage({ kind: 'deliver', event: { id: 'e1' }, audienceMemberIDs: [] });
		expect(delivery).toEqual({ kind: 'deliver', event: { id: 'e1' }, audienceMemberIDs: undefined });
	});
});

describe('decideCall', () => {
	test('forwards a call the server can take', () => {
		const decision = decideCall(call as never, 'm1', new CallLedger(), true);
		expect(decision).toEqual({
			action: 'forward',
			routed: { kind: 'call', requestID: 'r1', memberID: 'm1', capability: call.capability, body: call.body }
		});
	});

	test('answers server_offline rather than queueing', () => {
		const decision = decideCall(call as never, 'm1', new CallLedger(), false);
		expect(decision.action).toBe('answer');
		expect(decision.action === 'answer' && decision.answer.status).toBe(serverOfflineStatus);
	});

	test('a reconnect replays the answer instead of the call', () => {
		const ledger = new CallLedger();
		ledger.markPending('r1');
		ledger.recordAnswer({ kind: 'result', requestID: 'r1', status: 200, body: { eventID: 'e1' } });

		const decision = decideCall(call as never, 'm1', ledger, true);
		expect(decision).toEqual({
			action: 'answer',
			answer: { kind: 'result', requestID: 'r1', status: 200, body: { eventID: 'e1' } }
		});
	});

	test('a duplicate arriving before the answer is dropped, not run twice', () => {
		const ledger = new CallLedger();
		ledger.markPending('r1');
		expect(decideCall(call as never, 'm1', ledger, true).action).toBe('ignore');
	});
});

describe('CallLedger', () => {
	test('forgets the oldest answers once it is full', () => {
		const ledger = new CallLedger(2);
		for (const requestID of ['r1', 'r2', 'r3']) {
			ledger.recordAnswer({ kind: 'result', requestID, status: 200, body: null });
		}
		expect(ledger.answerFor('r1')).toBeUndefined();
		expect(ledger.answerFor('r3')).toBeDefined();
	});
});

describe('parseOneShotCall', () => {
	test('takes the body the api worker posts, which names no kind', () => {
		expect(parseOneShotCall({ requestID: 'r1', capability: 'person.api.request', body: { path: '/tools' } })).toEqual({
			kind: 'call',
			requestID: 'r1',
			capability: 'person.api.request',
			body: { path: '/tools' }
		});
	});

	test('refuses a body naming no request or no capability', () => {
		expect(parseOneShotCall({ capability: 'person.api.request' })).toBeNull();
		expect(parseOneShotCall({ requestID: 'r1' })).toBeNull();
		expect(parseOneShotCall(null)).toBeNull();
	});
});

describe('WaitingCalls', () => {
	test('a waiting call is settled by the answer naming it', async () => {
		const calls = new WaitingCalls(2, 1_000);
		const answered = calls.waitFor('r1');
		expect(calls.waitingCount).toBe(1);
		expect(calls.settle({ kind: 'result', requestID: 'r1', status: 200, body: { ok: true } })).toBe(true);
		expect(await answered).toEqual({ kind: 'result', requestID: 'r1', status: 200, body: { ok: true } });
		expect(calls.waitingCount).toBe(0);
	});

	test('an answer nobody waits for settles nothing', () => {
		const calls = new WaitingCalls(2, 1_000);
		expect(calls.settle({ kind: 'result', requestID: 'r1', status: 200, body: null })).toBe(false);
	});

	test('a call outliving its bound answers 504 and stops waiting', async () => {
		const calls = new WaitingCalls(2, 1);
		expect(await calls.waitFor('r1')).toEqual({
			kind: 'result',
			requestID: 'r1',
			status: 504,
			body: { error: 'call_timed_out' }
		});
		expect(calls.waitingCount).toBe(0);
	});

	test('fills at its limit and empties as calls are answered', async () => {
		const calls = new WaitingCalls(2, 1_000);
		const first = calls.waitFor('r1');
		calls.waitFor('r2');
		expect(calls.isFull).toBe(true);
		calls.settle({ kind: 'result', requestID: 'r1', status: 200, body: null });
		await first;
		expect(calls.isFull).toBe(false);
	});
});
