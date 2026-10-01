import { spawnSync } from 'node:child_process';
import { readdirSync, readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

const migrationVersionPattern = /^(\d{14})_(.+)\.sql$/;
const repositoryRoot = fileURLToPath(new URL('../../', import.meta.url));

export type AppliedMigrationLoader = () => Promise<ReadonlyArray<string>>;

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

export function appliedVersionsOfMigrationList(output: string): string[] {
	const listing: unknown = JSON.parse(output);
	if (!isMigrationListing(listing)) throw new Error('supabase migration list returned an unrecognized document');
	return listing.migrations.map((migration) => migration.remote).filter((version) => version !== '');
}

export async function ensureProductionSchemaIsCurrent(
	loadAppliedVersions: AppliedMigrationLoader = linkedAppliedVersions,
	directory = fileURLToPath(new URL('../../supabase/migrations/', import.meta.url))
): Promise<void> {
	compareMigrationVersions(localMigrationFiles(directory), await loadAppliedVersions());
}

async function linkedAppliedVersions(): Promise<string[]> {
	const listed = spawnSync('supabase', ['migration', 'list', '--linked', '--output-format', 'json'], {
		cwd: repositoryRoot,
		encoding: 'utf8'
	});
	if (listed.status !== 0) {
		throw new Error(
			`supabase migration list --linked failed; sign in with \`supabase login\` and link the project with \`supabase link --project-ref ${configuredProjectReference()}\`: ${listed.stderr}`
		);
	}
	return appliedVersionsOfMigrationList(listed.stdout);
}

function configuredProjectReference(): string {
	const configuration = readFileSync(fileURLToPath(new URL('../../supabase/config.toml', import.meta.url)), 'utf8');
	return /^project_id = "([^"]*)"/m.exec(configuration)?.[1] ?? '<project ref>';
}

function isMigrationListing(value: unknown): value is { migrations: Array<{ remote: string }> } {
	if (typeof value !== 'object' || value === null || !('migrations' in value)) return false;
	return Array.isArray(value.migrations) && value.migrations.every(isMigrationRow);
}

function isMigrationRow(value: unknown): value is { remote: string } {
	return typeof value === 'object' && value !== null && 'remote' in value && typeof value.remote === 'string';
}

function migrationVersion(fileName: string): string {
	const match = migrationVersionPattern.exec(fileName);
	if (!match) throw new Error(`malformed migration filename: ${fileName}`);
	return match[1];
}
