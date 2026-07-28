import { describe, expect, test } from 'bun:test';
import { organizationTenure } from '../../../src/routes/organization/organization-tenure';

describe('organization tenure', () => {
	const today = new Date('2026-07-28T09:00:00');

	test('counts whole years and remaining months', () => {
		expect(organizationTenure('2024-04-10', today)).toEqual({ years: 2, months: 3 });
	});

	test('counts only completed months', () => {
		expect(organizationTenure('2026-06-29', today)).toEqual({ years: 0, months: 0 });
		expect(organizationTenure('2026-06-28', today)).toEqual({ years: 0, months: 1 });
	});

	test('ignores missing, malformed, and future hire dates', () => {
		expect(organizationTenure(undefined, today)).toBe(undefined);
		expect(organizationTenure('2026', today)).toBe(undefined);
		expect(organizationTenure('2026-09-01', today)).toBe(undefined);
	});
});
