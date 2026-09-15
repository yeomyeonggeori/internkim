import { describe, expect, test } from 'bun:test';
import { compareMigrationVersions, ensureProductionSchemaIsCurrent } from '../../scripts/production-schema';

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

	test('propagates remote query failures', async () => {
		const queryError = new Error('remote schema query failed');
		await expect(ensureProductionSchemaIsCurrent(() => Promise.reject(queryError))).rejects.toBe(queryError);
	});
});
