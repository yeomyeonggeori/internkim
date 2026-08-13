import { describe, expect, test } from 'bun:test';
import {
	CallLedger,
	decideCall,
	parseClientCall,
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
