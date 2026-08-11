import { describe, expect, test } from 'bun:test';
import {
	apiHostnameOf,
	appHostnameOf,
	movesToTheOneAddress
} from '../../../src/lib/server/company-host-redirect';

const zone = 'intern.kim';

function asks(hostname: string): boolean {
	return movesToTheOneAddress({ hostname, zone });
}

describe('the one address every company signs in at', () => {
	test('moves a company subdomain to the app host', () => {
		expect(asks('dawnstreet.intern.kim')).toBe(true);
	});

	test('leaves the app host itself alone', () => {
		expect(asks(appHostnameOf(zone))).toBe(false);
	});

	test('leaves the api host alone, because the same project serves it', () => {
		expect(asks(apiHostnameOf(zone))).toBe(false);
	});

	test('leaves the zone alone, because another site answers there', () => {
		expect(asks('intern.kim')).toBe(false);
	});

	test('leaves a preview deployment alone', () => {
		expect(asks('feat-something.quick-claw.pages.dev')).toBe(false);
	});

	test('leaves local development alone', () => {
		expect(asks('localhost')).toBe(false);
		expect(asks('127.0.0.1')).toBe(false);
	});

	test('is not fooled by a host that merely ends in the zone text', () => {
		expect(asks('notintern.kim')).toBe(false);
		expect(asks('evil-intern.kim')).toBe(false);
	});

	test('reads a host the browser sent in capitals', () => {
		expect(asks('DawnStreet.Intern.Kim')).toBe(true);
	});

	test('stays put when no zone is configured, rather than moving somewhere wrong', () => {
		expect(movesToTheOneAddress({ hostname: 'dawnstreet.intern.kim', zone: '' })).toBe(false);
	});
});

describe('the hosts derived from the zone', () => {
	test('names them from the zone rather than spelling them out', () => {
		expect(appHostnameOf('example.test')).toBe('app.example.test');
		expect(apiHostnameOf('example.test')).toBe('api.example.test');
	});
});
