import { describe, expect, test } from 'bun:test';
import {
	movesToTheOneAddress,
	theOneAddressOf
} from '../../../src/lib/server/company-host-redirect';

const zone = 'example.test';

function asks(hostname: string, pathname = '/flow/'): boolean {
	return movesToTheOneAddress({ hostname, zone, pathname });
}

describe('the one address every company signs in at', () => {
	test('moves a company subdomain to the one address', () => {
		expect(asks('samplecompany.example.test')).toBe(true);
	});

	test('leaves the one address itself alone', () => {
		expect(asks(theOneAddressOf(zone))).toBe(false);
	});

	test('moves a page request on the api host, which is not a place to read', () => {
		expect(asks('api.example.test')).toBe(true);
	});

	test('leaves a preview deployment alone', () => {
		expect(asks('feat-something.quick-claw.pages.dev')).toBe(false);
	});

	test('leaves local development alone', () => {
		expect(asks('localhost')).toBe(false);
		expect(asks('127.0.0.1')).toBe(false);
	});

	test('is not fooled by a host that merely ends in the zone text', () => {
		expect(asks('notexample.test')).toBe(false);
		expect(asks('evil-example.test')).toBe(false);
	});

	test('reads a host the browser sent in capitals', () => {
		expect(asks('SampleCompany.Example.Test')).toBe(true);
	});

	test('stays put when no zone is configured, rather than moving somewhere wrong', () => {
		expect(movesToTheOneAddress({ hostname: 'samplecompany.example.test', zone: '', pathname: '/flow/' })).toBe(false);
	});
});

describe('a caller that carries its own credential', () => {
	test('is answered where it landed, because a redirect would drop its bearer token', () => {
		expect(asks('samplecompany.example.test', '/api/agent/connection')).toBe(false);
		expect(asks('api.example.test', '/api/agent/host-session')).toBe(false);
		expect(asks('api.example.test', '/v1/tools/message_send/invoke')).toBe(false);
	});

	test('answers the tool server\'s resource metadata on the host it describes', () => {
		expect(asks('api.example.test', '/.well-known/oauth-protected-resource/v1/mcp')).toBe(false);
	});

	test('still moves a page request on the same host', () => {
		expect(asks('samplecompany.example.test', '/apiary')).toBe(true);
	});
});

describe('the host derived from the zone', () => {
	test('is the zone itself, rather than a name under it', () => {
		expect(theOneAddressOf('example.test')).toBe('example.test');
	});
});
