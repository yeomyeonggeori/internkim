import { readFileSync, readdirSync } from 'node:fs';
import { join } from 'node:path';
import { remoteQuery } from './remote-query';

// supabase db push needs the session pooler, which this network cannot reach, so
// a migration is sent over the management API instead. Its ledger row goes in the
// same request as the migration itself: a schema change the ledger does not know
// about is what left the project sixteen migrations behind in the first place.

const requested = process.argv.slice(2);
if (requested.length === 0) {
	throw new Error('name the migration versions to apply, oldest first');
}

const directory = join(import.meta.dirname, '..', '..', 'supabase', 'migrations');
const files = readdirSync(directory).filter((name) => name.endsWith('.sql'));

const recorded = new Set(
	(await remoteQuery<{ version: string }>(
		'select version from supabase_migrations.schema_migrations'
	)).map((row) => row.version)
);

for (const version of requested) {
	const fileName = files.find((name) => name.startsWith(version + '_'));
	if (!fileName) throw new Error(`no migration file for ${version}`);
	if (recorded.has(version)) {
		console.log(`${fileName}: already recorded, skipped`);
		continue;
	}

	const statements = readFileSync(join(directory, fileName), 'utf8');
	const name = fileName.slice(version.length + 1).replace(/\.sql$/, '');
	const ledgerRow = `insert into supabase_migrations.schema_migrations (version, name, statements)
		values (${quote(version)}, ${quote(name)}, array[${quote(statements)}]);`;

	await remoteQuery(`${statements}\n\n${ledgerRow}`, true);
	console.log(`${fileName}: applied`);
}

function quote(value: string): string {
	return `'${value.replaceAll("'", "''")}'`;
}
