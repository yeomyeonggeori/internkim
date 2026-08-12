import { describe, expect, test } from 'bun:test';
import {
	movesToTheOneAddress,
	spaceHostnameOf
} from '../../../src/lib/server/company-host-redirect';

const zone = 'intern.kim';

function asks(hostname: string, pathname = '/flow/'): boolean {
	return movesToTheOneAddress({ hostname, zone, pathname });
}

describe('the one address every company signs in at', () => {
	test('moves a company subdomain to the space host', () => {
		expect(asks('dawnstreet.intern.kim')).toBe(true);
	});

	test('leaves the space host itself alone', () => {
		expect(asks(spaceHostnameOf(zone))).toBe(false);
	});

	test('moves the address companies used before, so old links still arrive', () => {
		expect(asks('app.intern.kim')).toBe(true);
	});

	test('moves the api host too, because it served the same pages under a second name', () => {
		expect(asks('api.intern.kim')).toBe(true);
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
		expect(movesToTheOneAddress({ hostname: 'dawnstreet.intern.kim', zone: '', pathname: '/flow/' })).toBe(false);
	});
});

describe('a caller that carries its own credential', () => {
	test('is answered where it landed, because a redirect would drop its bearer token', () => {
		expect(asks('dawnstreet.intern.kim', '/api/agent/connection')).toBe(false);
		expect(asks('app.intern.kim', '/api/agent/host-session')).toBe(false);
		expect(asks('api.intern.kim', '/api/agent/host-session')).toBe(false);
	});

	test('still moves a page request on the same host', () => {
		expect(asks('dawnstreet.intern.kim', '/apiary')).toBe(true);
	});
});

describe('the host derived from the zone', () => {
	test('names it from the zone rather than spelling it out', () => {
		expect(spaceHostnameOf('example.test')).toBe('space.example.test');
	});
});
