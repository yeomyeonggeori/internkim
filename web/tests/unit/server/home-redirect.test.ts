import { describe, expect, test } from 'bun:test';
import { sendsHomeToFlow } from '../../../src/lib/server/home-redirect';

const centralPlane = {
	projectURL: 'https://example.supabase.co',
	publishableKey: 'sb_publishable_test'
};

describe('sendsHomeToFlow', () => {
	test('a company host sends someone arriving at the root to their work', () => {
		expect(sendsHomeToFlow({ isBuilding: false, ...centralPlane, pathname: '/' })).toBe(true);
	});

	test('a device host leaves the root alone', () => {
		expect(
			sendsHomeToFlow({ isBuilding: false, projectURL: '', publishableKey: '', pathname: '/' })
		).toBe(false);
	});

	test('half a central plane is not a central plane', () => {
		expect(
			sendsHomeToFlow({ isBuilding: false, ...centralPlane, publishableKey: '', pathname: '/' })
		).toBe(false);
	});

	test('only the root redirects', () => {
		expect(sendsHomeToFlow({ isBuilding: false, ...centralPlane, pathname: '/flow/' })).toBe(false);
		expect(sendsHomeToFlow({ isBuilding: false, ...centralPlane, pathname: '/auth/' })).toBe(false);
	});

	test('a build never redirects, or the board UI loses its own root page', () => {
		expect(sendsHomeToFlow({ isBuilding: true, ...centralPlane, pathname: '/' })).toBe(false);
	});
});
