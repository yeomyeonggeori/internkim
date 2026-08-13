import { describe, expect, test } from 'bun:test';
import { publishOutcomeOf, retryDelayMilliseconds, serverSocketURL } from './gateway-socket';

describe('retryDelayMilliseconds', () => {
	test('backs off and then stops growing', () => {
		expect(retryDelayMilliseconds(1)).toBe(500);
		expect(retryDelayMilliseconds(3)).toBe(2000);
		expect(retryDelayMilliseconds(20)).toBe(30_000);
	});
});

describe('publishOutcomeOf', () => {
	test('reads an accepted event', () => {
		expect(publishOutcomeOf(['OK', 'e1', true, ''])).toEqual({ eventID: 'e1', isStored: true, refusal: '' });
	});

	test('carries the reason the relay gave for a refusal', () => {
		expect(publishOutcomeOf(['OK', 'e1', false, 'blocked: not a member'])).toEqual({
			eventID: 'e1',
			isStored: false,
			refusal: 'blocked: not a member'
		});
	});

	test('ignores every other frame the relay sends', () => {
		expect(publishOutcomeOf(['EVENT', 'subscription', {}])).toBeNull();
		expect(publishOutcomeOf(['NOTICE', 'anything'])).toBeNull();
		expect(publishOutcomeOf(['OK', 'e1'])).toBeNull();
		expect(publishOutcomeOf('OK')).toBeNull();
	});
});

describe('serverSocketURL', () => {
	test('names the company whichever way the gateway url ends', () => {
		expect(serverSocketURL('wss://gateway.test/', 'company-1')).toBe('wss://gateway.test/company/company-1/server');
		expect(serverSocketURL('wss://gateway.test', 'company-1')).toBe('wss://gateway.test/company/company-1/server');
	});
});
