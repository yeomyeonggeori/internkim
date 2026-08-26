import { describe, expect, test } from 'bun:test';
import { sendsHomeToTheApp } from '../../../src/lib/server/home-redirect';

const centralPlane = {
	projectURL: 'https://example.supabase.co',
	publishableKey: 'sb_publishable_test'
};

describe('sendsHomeToTheApp', () => {
	test('a company host sends someone arriving at the root into the app', () => {
		expect(sendsHomeToTheApp({ isBuilding: false, ...centralPlane, pathname: '/' })).toBe(true);
	});

	test('a device host leaves the root alone', () => {
		expect(
			sendsHomeToTheApp({ isBuilding: false, projectURL: '', publishableKey: '', pathname: '/' })
		).toBe(false);
	});

	test('half a central plane is not a central plane', () => {
		expect(
			sendsHomeToTheApp({ isBuilding: false, ...centralPlane, publishableKey: '', pathname: '/' })
		).toBe(false);
	});

	test('only the root redirects', () => {
		expect(sendsHomeToTheApp({ isBuilding: false, ...centralPlane, pathname: '/flow/' })).toBe(false);
		expect(sendsHomeToTheApp({ isBuilding: false, ...centralPlane, pathname: '/auth/' })).toBe(false);
	});

	test('a build never redirects, or the board UI loses its own root page', () => {
		expect(sendsHomeToTheApp({ isBuilding: true, ...centralPlane, pathname: '/' })).toBe(false);
	});
});
