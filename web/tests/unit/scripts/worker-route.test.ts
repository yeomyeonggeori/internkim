import { existsSync, readFileSync, readdirSync } from 'node:fs';
import { describe, expect, test } from 'bun:test';
import { routePatternOfSubdomain, zoneOfSettings } from '../../../scripts/worker-route';
import { defaultZone } from '../../../src/lib/server/fleet-domain';

const workersDirectory = new URL('../../../../workers/', import.meta.url);

describe('the zone a worker route is rendered under', () => {
	test('falls back to the fleet domain', () => {
		expect(zoneOfSettings({})).toBe(defaultZone);
		expect(zoneOfSettings({ CLOUDFLARE_DOMAIN: '  ' })).toBe(defaultZone);
	});

	test('takes configuration over the default, as every other consumer does', () => {
		expect(zoneOfSettings({ CLOUDFLARE_DOMAIN: 'example.test' })).toBe('example.test');
		expect(zoneOfSettings({ CLOUDFLARE_DOMAIN: 'Self.Hosted.Test' })).toBe('self.hosted.test');
	});
});

describe('rendering a route from a subdomain', () => {
	test('puts the label under the zone and answers for every path', () => {
		expect(routePatternOfSubdomain('updates', defaultZone)).toBe(`updates.${defaultZone}/*`);
		expect(routePatternOfSubdomain('Updates', 'example.test')).toBe('updates.example.test/*');
	});

	test('refuses a hostname, because the zone is not the caller to choose', () => {
		expect(() => routePatternOfSubdomain('updates.intern.kim', defaultZone)).toThrow('not a subdomain');
		expect(() => routePatternOfSubdomain('  ', defaultZone)).toThrow('name the subdomain');
	});
});

describe('no worker configuration writes the fleet domain down again', () => {
	const configurations = readdirSync(workersDirectory, { withFileTypes: true })
		.filter((entry) => entry.isDirectory() && existsSync(new URL(`${entry.name}/wrangler.jsonc`, workersDirectory)))
		.map((entry) => ({ worker: entry.name, written: readFileSync(new URL(`${entry.name}/wrangler.jsonc`, workersDirectory), 'utf8') }));

	test('there is a worker to check', () => {
		expect(configurations.length).toBeGreaterThan(0);
	});

	for (const { worker, written } of configurations) {
		test(`${worker} declares no routes, so its hostname comes from companyzone at deploy time`, () => {
			expect(written).not.toMatch(/"routes"/);
			expect(written).not.toMatch(/"zone_name"/);
		});
	}
});
