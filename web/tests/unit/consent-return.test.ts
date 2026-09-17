import { describe, expect, test } from 'bun:test';
import { hostOf, returnsToThisComputer } from '../../src/lib/consent-return';

describe('where an app takes the access a person allows', () => {
	test('stays on this computer for a loopback callback or an app of its own', () => {
		expect(returnsToThisComputer('http://localhost:33418/callback')).toBe(true);
		expect(returnsToThisComputer('http://127.0.0.1:33418/callback')).toBe(true);
		expect(returnsToThisComputer('http://[::1]:33418/callback')).toBe(true);
		expect(returnsToThisComputer('cursor://anysphere.cursor-mcp/oauth/callback')).toBe(true);
	});

	test('leaves this computer for any site, whatever the app calls itself', () => {
		expect(returnsToThisComputer('https://claude.ai/api/mcp/auth_callback')).toBe(false);
		expect(returnsToThisComputer('http://localhost.example.test/callback')).toBe(false);
		expect(returnsToThisComputer('not an address')).toBe(false);
	});

	test('is named by its host', () => {
		expect(hostOf('https://claude.ai/api/mcp/auth_callback')).toBe('claude.ai');
		expect(hostOf('http://127.0.0.1:33418/callback')).toBe('127.0.0.1:33418');
		expect(hostOf('not an address')).toBe('not an address');
	});
});
