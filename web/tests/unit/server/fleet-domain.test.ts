import { describe, expect, test } from 'bun:test';
import { defaultZone, docsHost } from '../../../src/lib/server/fleet-domain';
import { declaredZone } from '../fleet-domain-declaration';

describe('the web zone declaration matches companyzone', () => {
	test('defaultZone is the same string internal/companyzone/companyzone.go declares', () => {
		expect(defaultZone).toBe(declaredZone());
	});
});

describe('docsHost', () => {
	test('names the docs site under the given zone', () => {
		expect(docsHost('example.test')).toBe('docs.example.test');
	});

	test('falls back to the default zone', () => {
		expect(docsHost()).toBe(`docs.${defaultZone}`);
	});
});
