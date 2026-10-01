import { projectReference, remoteQuery as query } from './remote-query';
import { localMigrationFiles } from './production-schema';

const recorded = (await query<{ version: string }>(
	'select version from supabase_migrations.schema_migrations order by version'
)).map((row) => row.version);

const localFiles = localMigrationFiles();
const local = localFiles.map((name) => name.split('_')[0]);
const unrecorded = local.filter((version) => !recorded.includes(version));
const unknownLocally = recorded.filter((version) => !local.includes(version));

console.log(`project        ${projectReference()}`);
console.log(`local files    ${local.length}, newest ${local[local.length - 1]}`);
console.log(`recorded       ${recorded.length}, newest ${recorded[recorded.length - 1]}`);

if (unrecorded.length === 0 && unknownLocally.length === 0) {
	console.log('\nthe ledger matches the migrations directory');
} else {
	for (const version of unrecorded) console.log(`\n! not recorded remotely   ${localFiles.find((name) => name.startsWith(version + '_')) ?? version}`);
	for (const version of unknownLocally) console.log(`\n! recorded but no file    ${version}`);
}

const functions = await query<{ proname: string }>(
	`select distinct proname from pg_proc
	 join pg_namespace on pg_namespace.oid = pg_proc.pronamespace
	 where pg_namespace.nspname = 'public'
	   and proname in ('save_flow_task','link_task_children','set_task_parent',
	                   'can_manage_task_relationship','lock_task_requester',
	                   'correct_attendance_events','validate_attendance_correction')
	 order by proname`
);
console.log('\nfunctions present: ' + (functions.map((row) => row.proname).join(', ') || 'none'));

const columns = await query<{ table_name: string; column_name: string }>(
	`select table_name, column_name from information_schema.columns
	 where table_schema = 'public'
	   and (table_name, column_name) in (
	     ('task','requester_id'), ('task','requester_name'), ('task','parent_task_id'),
	     ('task','calendar'), ('task','created_at'), ('member','messenger'),
	     ('attendance','original_occurred_at'), ('attendance','edit_reason'))
	 order by table_name, column_name`
);
console.log('columns present:   ' + (columns.map((row) => `${row.table_name}.${row.column_name}`).join(', ') || 'none'));

// Two pending migrations write link_task_children, and only the later one counts
// the rows it updated, so the body is the only thing that tells them apart.
const [bodies] = await query<{ link_task_children_counts_its_update: boolean }>(
	`select coalesce((select pg_get_functiondef(pg_proc.oid) like '%get diagnostics%'
		from pg_proc join pg_namespace on pg_namespace.oid = pg_proc.pronamespace
		where pg_namespace.nspname = 'public' and pg_proc.proname = 'link_task_children' limit 1), false)
		as link_task_children_counts_its_update`
);
console.log(
	`\n20260813080000 child overwrite guard: ${bodies.link_task_children_counts_its_update ? 'applied' : 'NOT applied'}`
);
