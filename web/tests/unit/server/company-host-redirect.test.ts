import { describe, expect, test } from 'bun:test';
import { movesToTheOneAddress } from '../../../src/lib/server/company-host-redirect';

const zone = 'intern.kim';
const apiHostname = 'api.intern.kim';

function asks(hostname: string): boolean {
	return movesToTheOneAddress({ hostname, zone, apiHostname });
}

describe('the one address every company signs in at', () => {
	test('moves a company subdomain to the zone', () => {
		expect(asks('dawnstreet.intern.kim')).toBe(true);
	});

	test('leaves the zone itself alone', () => {
		expect(asks('intern.kim')).toBe(false);
	});

	test('leaves the api host alone, because the same project serves it', () => {
		expect(asks('api.intern.kim')).toBe(false);
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
		expect(movesToTheOneAddress({ hostname: 'dawnstreet.intern.kim', zone: '', apiHostname: '' })).toBe(false);
	});
});
