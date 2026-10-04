import { describe, expect, test } from 'bun:test';
import { pathAfterFounding } from '../../src/routes/start/path-after-founding';

describe('where a founder goes once the company exists', () => {
	test('to setup when they were only on their way to a page of the app', () => {
		expect(pathAfterFounding('/attendance')).toBe('/settings/setup');
		expect(pathAfterFounding('/attendance/')).toBe('/settings/setup');
		expect(pathAfterFounding('/crm?view=deals')).toBe('/settings/setup');
		expect(pathAfterFounding('')).toBe('/settings/setup');
	});

	test('back to a flow outside the app they were in the middle of', () => {
		expect(pathAfterFounding('/oauth/consent?authorization_id=abc')).toBe('/oauth/consent?authorization_id=abc');
	});
});
