import { describe, expect, test } from 'bun:test';
import { deliveryOf, reasonOf, retryDelayMilliseconds, serverSocketURL } from './gateway-socket';

describe('deliveryOf', () => {
	test('is the frame the gateway fans out, naming an audience only when there is one', () => {
		expect(deliveryOf({ kind: 'message.arrived', conversationID: 'channel-1' })).toEqual({
			kind: 'deliver',
			event: { kind: 'message.arrived', conversationID: 'channel-1' }
		});
		expect(deliveryOf({ kind: 'message.arrived' }, [])).toEqual({ kind: 'deliver', event: { kind: 'message.arrived' } });
		expect(deliveryOf({ kind: 'message.arrived' }, ['m1'])).toEqual({
			kind: 'deliver',
			event: { kind: 'message.arrived' },
			audienceMemberIDs: ['m1']
		});
	});
});


describe('retryDelayMilliseconds', () => {
	test('backs off and then stops growing', () => {
		expect(retryDelayMilliseconds(1)).toBe(500);
		expect(retryDelayMilliseconds(3)).toBe(2000);
		expect(retryDelayMilliseconds(20)).toBe(30_000);
	});
});

describe('reasonOf', () => {
	test('carries what the answer said', () => {
		expect(reasonOf({ error: 'this member has no messenger account' })).toBe('this member has no messenger account');
	});

	test('says so rather than printing an object nobody can read', () => {
		expect(reasonOf({ conversations: [] })).toBe('no reason given');
		expect(reasonOf('')).toBe('no reason given');
		expect(reasonOf(null)).toBe('no reason given');
	});
});

describe('serverSocketURL', () => {
	test('names the company whichever way the gateway url ends', () => {
		expect(serverSocketURL('wss://gateway.test/', 'company-1')).toBe('wss://gateway.test/company/company-1/server');
		expect(serverSocketURL('wss://gateway.test', 'company-1')).toBe('wss://gateway.test/company/company-1/server');
	});
});
