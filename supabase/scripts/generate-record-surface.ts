import { SQL } from 'bun';
import { readdir, readFile, writeFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import { localDatabaseURL } from './local-database-url';

const supabaseDirectory = fileURLToPath(new URL('..', import.meta.url));
const surfacePath = `${supabaseDirectory}generated/record-surface.json`;

type RecordFunction = {
	name: string;
	arguments: string;
	returns: string;
	security: 'definer' | 'invoker';
	callableBy: string[];
};

type EdgeFunction = { name: string; verifyJWT: boolean | null };

type ScheduledJob = { schedule: string; command: string };

type RecordSurface = {
	rpc: RecordFunction[];
	triggerFunctions: string[];
	edgeFunctions: EdgeFunction[];
	scheduled: ScheduledJob[];
};

type FunctionRow = {
	name: string;
	arguments: string;
	returns: string;
	is_definer: boolean;
	callable_by_anon: boolean;
	callable_by_authenticated: boolean;
	callable_by_service_role: boolean;
};

async function readFunctions(sql: SQL): Promise<FunctionRow[]> {
	return sql`
		select p.proname as name,
		       pg_get_function_arguments(p.oid) as arguments,
		       pg_get_function_result(p.oid) as returns,
		       p.prosecdef as is_definer,
		       has_function_privilege('anon', p.oid, 'execute') as callable_by_anon,
		       has_function_privilege('authenticated', p.oid, 'execute') as callable_by_authenticated,
		       has_function_privilege('service_role', p.oid, 'execute') as callable_by_service_role
		from pg_proc p
		join pg_namespace n on n.oid = p.pronamespace
		where n.nspname = 'public' and p.prokind = 'f'
		order by p.proname
	`;
}

async function readScheduledJobs(sql: SQL): Promise<ScheduledJob[]> {
	try {
		return await sql`select schedule, command from cron.job order by jobname`;
	} catch {
		return [];
	}
}

function callersOf(row: FunctionRow): string[] {
	const callers = [];
	if (row.callable_by_anon) callers.push('anon');
	if (row.callable_by_authenticated) callers.push('authenticated');
	if (row.callable_by_service_role) callers.push('service_role');
	return callers;
}

// config.toml gives a function its own [functions.<name>] table; a function the
// file does not name runs on the CLI's defaults.
function verifyJWTOf(configuration: string, name: string): boolean | null {
	const section = configuration.split(`[functions.${name}]`)[1];
	if (section === undefined) return null;
	const setting = section.split('[')[0].match(/verify_jwt\s*=\s*(true|false)/);
	return setting ? setting[1] === 'true' : null;
}

async function readEdgeFunctions(): Promise<EdgeFunction[]> {
	const configuration = await readFile(`${supabaseDirectory}config.toml`, 'utf8');
	const entries = await readdir(`${supabaseDirectory}functions`, { withFileTypes: true });
	return entries
		.filter((entry) => entry.isDirectory() && !entry.name.startsWith('_'))
		.map((entry) => ({ name: entry.name, verifyJWT: verifyJWTOf(configuration, entry.name) }))
		.sort((one, other) => one.name.localeCompare(other.name));
}

async function readRecordSurface(): Promise<RecordSurface> {
	const sql = new SQL(localDatabaseURL);
	try {
		const functions = await readFunctions(sql);
		return {
			rpc: functions
				.filter((row) => row.returns !== 'trigger')
				.map((row) => ({
					name: row.name,
					arguments: row.arguments,
					returns: row.returns,
					security: row.is_definer ? 'definer' : 'invoker',
					callableBy: callersOf(row)
				})),
			triggerFunctions: functions.filter((row) => row.returns === 'trigger').map((row) => row.name),
			edgeFunctions: await readEdgeFunctions(),
			scheduled: await readScheduledJobs(sql)
		};
	} finally {
		await sql.end();
	}
}

const surface = await readRecordSurface();
const document = `${JSON.stringify(surface, null, 2)}\n`;

if (process.argv.includes('--check')) {
	const committed = await readFile(surfacePath, 'utf8').catch(() => '');
	if (committed !== document) {
		console.error(`${surfacePath} is stale; run bun run supabase/scripts/generate-record-surface.ts`);
		process.exit(1);
	}
	console.log('record surface is current');
} else {
	await writeFile(surfacePath, document);
	console.log(
		`wrote ${surface.rpc.length} functions, ${surface.triggerFunctions.length} trigger functions, ${surface.edgeFunctions.length} edge functions, ${surface.scheduled.length} scheduled jobs`
	);
}
