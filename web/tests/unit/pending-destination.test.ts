import { describe, expect, test } from 'bun:test';
import { ownPath } from '../../src/lib/notifications/pending-destination';

describe('ownPath', () => {
	test('a path inside this app is kept', () => {
		expect(ownPath('/calendar/')).toBe('/calendar/');
		expect(ownPath('/flow/?taskID=7')).toBe('/flow/?taskID=7');
	});

	test('anywhere off this host is refused', () => {
		expect(ownPath('https://example.test/steal')).toBe('');
		expect(ownPath('//example.test/steal')).toBe('');
		expect(ownPath('javascript:alert(1)')).toBe('');
	});

	test('what was never a path is refused', () => {
		expect(ownPath(undefined)).toBe('');
		expect(ownPath(7)).toBe('');
		expect(ownPath('calendar')).toBe('');
	});
});
