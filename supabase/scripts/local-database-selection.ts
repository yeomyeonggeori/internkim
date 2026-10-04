export type LocalDatabaseEnvironment = { SUPABASE_URL?: string; SUPABASE_DB_URL?: string };
export type LocalDatabasePorts = { api: number; database: number };

function localURL(value: string, protocols: string[], port: number): URL {
	let parsed: URL;
	try { parsed = new URL(value); } catch { throw new Error('the local test connection URL is invalid'); }
	if (!protocols.includes(parsed.protocol) || !['localhost', '127.0.0.1', '[::1]'].includes(parsed.hostname) || Number(parsed.port) !== port) {
		throw new Error('the local test connection does not match this checkout’s configured ports');
	}
	return parsed;
}

export function localDatabaseSelected(environment: LocalDatabaseEnvironment, ports: LocalDatabasePorts): string {
	if (environment.SUPABASE_URL) {
		localURL(environment.SUPABASE_URL, ['http:'], ports.api);
		if (!environment.SUPABASE_DB_URL) throw new Error('SUPABASE_DB_URL is required alongside the local API; read both from the same owned stack status');
	}
	if (environment.SUPABASE_DB_URL) {
		localURL(environment.SUPABASE_DB_URL, ['postgres:', 'postgresql:'], ports.database);
		return environment.SUPABASE_DB_URL;
	}
	return `postgresql://supabase_admin:postgres@127.0.0.1:${ports.database}/postgres`;
}
