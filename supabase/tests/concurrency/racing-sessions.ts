import { SQL } from 'bun';
import { localDatabaseURL } from '../../scripts/local-database-url';

export type LoginRole = {
	name: string;
	password: string;
	grants: string[];
};

export function administratorSession(): SQL {
	return new SQL(localDatabaseURL, { max: 1 });
}

export function sessionAs(role: LoginRole): SQL {
	const url = new URL(localDatabaseURL);
	url.username = role.name;
	url.password = role.password;
	return new SQL(url.toString(), { max: 1 });
}

export async function createLoginRole(administrator: SQL, role: LoginRole): Promise<void> {
	await administrator.unsafe(`create role ${role.name} login password '${role.password}'`);
	for (const grant of role.grants) {
		await administrator.unsafe(`grant ${grant} to ${role.name}`);
	}
}

export async function dropLoginRole(administrator: SQL, roleName: string): Promise<void> {
	const rows: { exists: boolean }[] = await administrator`
		select exists (select 1 from pg_catalog.pg_roles where rolname = ${roleName}) as exists
	`;
	if (!rows[0]?.exists) return;
	await administrator.unsafe(`drop owned by ${roleName}`);
	await administrator.unsafe(`drop role ${roleName}`);
}

export async function actAs(session: SQL, userID: string): Promise<void> {
	await session`select set_config('request.jwt.claim.sub', ${userID}, false)`;
}

export async function backendProcessID(session: SQL): Promise<number> {
	const rows: { pid: number }[] = await session`select pg_backend_pid() as pid`;
	const [row] = rows;
	if (!row) throw new Error('pg_backend_pid() returned no row');
	return row.pid;
}

async function isWaitingOnLock(observer: SQL, processID: number): Promise<boolean> {
	const rows: { waiting: boolean }[] = await observer`
		select exists (
			select 1
			from pg_catalog.pg_stat_activity
			where pid = ${processID} and wait_event_type = 'Lock'
		) as waiting
	`;
	return rows[0]?.waiting === true;
}

export async function waitUntilWaitingOnLock(observer: SQL, processID: number): Promise<boolean> {
	for (let attempt = 0; attempt < 100; attempt++) {
		if (await isWaitingOnLock(observer, processID)) return true;
		await Bun.sleep(10);
	}
	return false;
}

export function refusalOf(query: Promise<unknown>): Promise<string | undefined> {
	return query.then(
		() => undefined,
		(error: unknown) => {
			if (error instanceof SQL.PostgresError && error.code === 'ERR_POSTGRES_SERVER_ERROR') return error.message;
			throw error;
		}
	);
}
