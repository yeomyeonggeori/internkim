import { readdirSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { remoteQuery } from './remote-query';

const migrationVersionPattern = /^(\d{14})_(.+)\.sql$/;
const migrationQuery = 'select version from supabase_migrations.schema_migrations order by version';

export type MigrationQuery = (sql: string) => Promise<ReadonlyArray<{ version: string }>>;

export function localMigrationFiles(directory = fileURLToPath(new URL('../../supabase/migrations/', import.meta.url))): string[] {
	return readdirSync(directory).filter((name) => name.endsWith('.sql')).sort();
}

export function compareMigrationVersions(localFiles: string[], appliedVersions: ReadonlyArray<string>): void {
	const localVersions = localFiles.map((name) => migrationVersion(name));
	const duplicateVersion = localVersions.find((version, index) => localVersions.indexOf(version) !== index);
	if (duplicateVersion) throw new Error(`duplicate migration version: ${duplicateVersion}`);
	const applied = new Set(appliedVersions);
	const missing = localFiles.filter((_, index) => !applied.has(localVersions[index]));
	if (missing.length > 0) {
		throw new Error(
			`refusing production deploy: migrations are not applied: ${missing.join(', ')}; apply and verify these migrations before deploying`
		);
	}
}

export async function ensureProductionSchemaIsCurrent(
	query: MigrationQuery = (sql) => remoteQuery<{ version: string }>(sql),
	directory = fileURLToPath(new URL('../../supabase/migrations/', import.meta.url))
): Promise<void> {
	const applied = await query(migrationQuery);
	compareMigrationVersions(localMigrationFiles(directory), applied.map((row) => row.version));
}

function migrationVersion(fileName: string): string {
	const match = migrationVersionPattern.exec(fileName);
	if (!match) throw new Error(`malformed migration filename: ${fileName}`);
	return match[1];
}
