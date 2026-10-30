import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { emailInCompany, idInCompany, slugOfCompany, workerCompanyCount } from '../e2e/support/worker-company';

function seed(): string {
	return readFileSync(new URL('../../../supabase/seed.dev.sql', import.meta.url), 'utf8');
}

describe('supabase/seed.dev.sql seeds the companies the e2e workers sign in to', () => {
	const seeded = seed();

	test('it seeds one company per worker the e2e config may start', () => {
		const copies = seeded.match(/seed_worker_company\(company_number\) from generate_series\(1, (\d+)\)/);
		expect(copies).not.toBeNull();
		expect(Number(copies?.[1]) + 1).toBe(workerCompanyCount);
	});

	test('it names a worker company the way the specs look it up', () => {
		expect(idInCompany('000000ee-0000-0000-0000-000000000003', 5)).toBe('000000ee-0000-0000-0005-000000000003');
		expect(seeded).toContain("overlay(fixture_id::text placing lpad(company_number::text, 4, '0') from 20 for 4)");
		expect(emailInCompany('member1@example.com', 5)).toBe('member1+company5@example.com');
		expect(seeded).toContain("replace(email, '@', '+company' || company_number || '@')");
		expect(slugOfCompany(5)).toBe('example-co-5');
		expect(seeded).toContain("format('slug || %L', '-' || company_number)");
	});
});
