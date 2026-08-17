import { describe, expect, test } from 'bun:test';
import { readdirSync } from 'node:fs';
import { join } from 'node:path';
import {
	companyPathOf,
	companySlugOf,
	reservedFirstSegments,
	routePathOf,
	wantsCompanyPrefix
} from '../../src/lib/company-path';

describe('the company slug in the path', () => {
	test('reads the slug a company address leads with', () => {
		expect(companySlugOf('/samplecompany/flow')).toBe('samplecompany');
		expect(companySlugOf('/samplecompany')).toBe('samplecompany');
	});

	test('does not read a route name as a slug', () => {
		expect(companySlugOf('/flow')).toBe('');
		expect(companySlugOf('/files/api/roots')).toBe('');
		expect(companySlugOf('/auth/claim')).toBe('');
	});

	test('does not read something the wrong shape as a slug', () => {
		expect(companySlugOf('/A')).toBe('');
		expect(companySlugOf('/-leading')).toBe('');
		expect(companySlugOf('/')).toBe('');
	});
});

describe('routing a company path to the route that serves it', () => {
	test('takes the slug off the front', () => {
		expect(routePathOf('/samplecompany/flow')).toBe('/flow');
		expect(routePathOf('/samplecompany/files/api/roots')).toBe('/files/api/roots');
	});

	test('sends a bare company address to the root route', () => {
		expect(routePathOf('/samplecompany')).toBe('/');
	});

	test('leaves a path that names no company alone', () => {
		expect(routePathOf('/flow')).toBe('/flow');
		expect(routePathOf('/api/company')).toBe('/api/company');
		expect(routePathOf('/')).toBe('/');
	});
});

describe('building a company path', () => {
	test('puts the slug in front', () => {
		expect(companyPathOf('samplecompany', '/flow')).toBe('/samplecompany/flow');
		expect(companyPathOf('samplecompany', '/')).toBe('/samplecompany');
	});

	test('leaves the path alone when no company is known yet', () => {
		expect(companyPathOf('', '/flow')).toBe('/flow');
	});

	test('round-trips with the routing it is the inverse of', () => {
		for (const path of ['/flow', '/files/api/roots', '/attendance']) {
			expect(routePathOf(companyPathOf('samplecompany', path))).toBe(path);
		}
	});
});

describe('the reserved segments', () => {
	test('name every top-level route, so none is mistaken for a company', () => {
		const routeDirectories = readdirSync(join(import.meta.dir, '../../src/routes'), {
			withFileTypes: true
		})
			.filter((entry) => entry.isDirectory() && !entry.name.startsWith('.'))
			.map((entry) => entry.name)
			.sort();

		expect(reservedFirstSegments.slice().sort()).toEqual(routeDirectories);
	});
});

describe('paths that still need a company in front of them', () => {
	test('asks for a prefix on an app path that has none', () => {
		expect(wantsCompanyPrefix('/flow')).toBe(true);
		expect(wantsCompanyPrefix('/files/api/roots')).toBe(true);
	});

	test('leaves a path that already names a company alone', () => {
		expect(wantsCompanyPrefix('/samplecompany/flow')).toBe(false);
	});

	test('leaves signing in and founding a company outside any company', () => {
		expect(wantsCompanyPrefix('/auth/claim')).toBe(false);
		expect(wantsCompanyPrefix('/start')).toBe(false);
		expect(wantsCompanyPrefix('/api/company')).toBe(false);
	});

	test('leaves the root alone, because it names no app', () => {
		expect(wantsCompanyPrefix('/')).toBe(false);
	});
});
