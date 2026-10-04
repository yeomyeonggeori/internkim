import { readFileSync } from 'node:fs';
import { localDatabaseSelected } from './local-database-selection';

const configuration = Bun.TOML.parse(readFileSync(new URL('../config.toml', import.meta.url), 'utf8'));
function configuredPort(section: string): number {
	if (!(section in configuration)) throw new Error('the local database configuration is incomplete');
	const settings: unknown = Reflect.get(configuration, section);
	if (!settings || typeof settings !== 'object' || !('port' in settings) || typeof settings.port !== 'number' || !Number.isInteger(settings.port) || settings.port < 1 || settings.port > 65535) {
		throw new Error('the local database configuration has an invalid port');
	}
	return settings.port;
}
const ports = { api: configuredPort('api'), database: configuredPort('db') };
export const localDatabaseURL = localDatabaseSelected({
	SUPABASE_URL: process.env.SUPABASE_URL,
	SUPABASE_DB_URL: process.env.SUPABASE_DB_URL
}, ports);
