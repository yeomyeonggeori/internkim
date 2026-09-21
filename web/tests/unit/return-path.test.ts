import { describe, expect, test } from 'bun:test';
import { ownPath, returnPathOf, withReturnPath } from '../../src/lib/return-path';

const thisApp = 'https://intern.kim';

describe('ownPath', () => {
	test('a path inside this app is kept', () => {
		expect(ownPath('/calendar/')).toBe('/calendar/');
		expect(ownPath('/flow/?taskID=7')).toBe('/flow/?taskID=7');
		expect(ownPath('/oauth/consent?authorization_id=abc')).toBe('/oauth/consent?authorization_id=abc');
	});

	test('anywhere off this host is refused', () => {
		expect(ownPath('https://example.test/steal')).toBe('');
		expect(ownPath('//example.test/steal')).toBe('');
		expect(ownPath('javascript:alert(1)')).toBe('');
	});

	test('a backslash that a browser reads as a slash is refused', () => {
		expect(ownPath('/\\example.test/steal')).toBe('');
		expect(ownPath('/\\\\example.test/steal')).toBe('');
	});

	test('what was never a path is refused', () => {
		expect(ownPath(undefined)).toBe('');
		expect(ownPath(7)).toBe('');
		expect(ownPath('calendar')).toBe('');
	});
});

describe('withReturnPath', () => {
	test('names the parameter once, whether or not the destination already asks something', () => {
		expect(withReturnPath('/auth/claim', '/oauth/consent?authorization_id=abc')).toBe(
			'/auth/claim?return=%2Foauth%2Fconsent%3Fauthorization_id%3Dabc'
		);
		expect(withReturnPath('/auth/claim?new-company=1', '/start')).toBe(
			'/auth/claim?new-company=1&return=%2Fstart'
		);
	});

	test('carries nothing when there is nowhere of ours to go back to', () => {
		expect(withReturnPath('/start', '')).toBe('/start');
		expect(withReturnPath('/start', '//example.test/steal')).toBe('/start');
	});
});

describe('returnPathOf', () => {
	test('reads back what withReturnPath wrote', () => {
		const asked = '/oauth/consent?authorization_id=abc&scope=openid';
		const link = withReturnPath('/auth/claim?new-company=1', asked);
		expect(returnPathOf(new URL(link, thisApp))).toBe(asked);
	});

	test('refuses a return parameter that points off this app', () => {
		expect(returnPathOf(new URL('/auth/claim?return=https://example.test/steal', thisApp))).toBe('');
		expect(returnPathOf(new URL('/auth/claim?return=%2F%5Cexample.test', thisApp))).toBe('');
	});

	test('is empty when nothing was asked', () => {
		expect(returnPathOf(new URL('/auth/claim', thisApp))).toBe('');
	});
});
