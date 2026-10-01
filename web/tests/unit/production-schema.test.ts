import { describe, expect, test } from 'bun:test';
import {
	appliedVersionsOfMigrationList,
	compareMigrationVersions,
	ensureProductionSchemaIsCurrent
} from '../../scripts/production-schema';

describe('production schema preflight', () => {
	test('refuses when a local migration is missing remotely', () => {
		expect(() => compareMigrationVersions(['20260914000005_a_write_is_remembered_by_its_key.sql'], [])).toThrow(
			'20260914000005_a_write_is_remembered_by_its_key.sql'
		);
	});

	test('accepts when every local migration is applied', () => {
		expect(() => compareMigrationVersions(['20260914000005_a_write_is_remembered_by_its_key.sql'], ['20260914000005'])).not.toThrow();
	});

	test('allows newer remote migrations', () => {
		expect(() =>
			compareMigrationVersions(
				['20260914000005_a_write_is_remembered_by_its_key.sql'],
				['20260914000005', '20260915000001']
			)
		).not.toThrow();
	});

	test('rejects malformed and duplicate local migration versions', () => {
		expect(() => compareMigrationVersions(['migration.sql'], [])).toThrow('malformed migration filename');
		expect(() =>
			compareMigrationVersions(
				['20260914000005_first.sql', '20260914000005_second.sql'],
				['20260914000005']
			)
		).toThrow('duplicate migration version');
	});

	test('propagates failures to list the applied migrations', async () => {
		const listingError = new Error('supabase migration list failed');
		await expect(ensureProductionSchemaIsCurrent(() => Promise.reject(listingError))).rejects.toBe(listingError);
	});

	test('reads applied versions from the CLI listing and skips local-only rows', () => {
		const listing = JSON.stringify({
			migrations: [
				{ local: '20260914000005', remote: '20260914000005', time: '2026-09-14 00:00:05' },
				{ local: '20260915000001', remote: '', time: '2026-09-15 00:00:01' },
				{ local: '', remote: '20260916000001', time: '2026-09-16 00:00:01' }
			]
		});
		expect(appliedVersionsOfMigrationList(listing)).toEqual(['20260914000005', '20260916000001']);
	});

	test('refuses a listing that is not the CLI document', () => {
		expect(() => appliedVersionsOfMigrationList('{"message":"ok"}')).toThrow('unrecognized document');
	});
});
