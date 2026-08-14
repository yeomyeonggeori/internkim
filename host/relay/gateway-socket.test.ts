import { describe, expect, test } from 'bun:test';
import { retryDelayMilliseconds, serverSocketURL } from './gateway-socket';

describe('retryDelayMilliseconds', () => {
	test('backs off and then stops growing', () => {
		expect(retryDelayMilliseconds(1)).toBe(500);
		expect(retryDelayMilliseconds(3)).toBe(2000);
		expect(retryDelayMilliseconds(20)).toBe(30_000);
	});
});

describe('serverSocketURL', () => {
	test('names the company whichever way the gateway url ends', () => {
		expect(serverSocketURL('wss://gateway.test/', 'company-1')).toBe('wss://gateway.test/company/company-1/server');
		expect(serverSocketURL('wss://gateway.test', 'company-1')).toBe('wss://gateway.test/company/company-1/server');
	});
});
