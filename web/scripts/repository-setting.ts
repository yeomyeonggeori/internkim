// Bun reads settings from the directory it runs in, and these scripts run in
// web/ while the repository keeps its own settings in one file at the root. So
// a value that is plainly there reads as missing, and the failure tells the
// reader to set something they already set.

import { appendFileSync, readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

export const repositorySettingsPath = fileURLToPath(new URL('../../.env', import.meta.url));

function settingsAtRepositoryRoot(): Record<string, string> {
	let written = '';
	try {
		written = readFileSync(repositorySettingsPath, 'utf8');
	} catch {
		return {};
	}
	const settings: Record<string, string> = {};
	for (const line of written.split('\n')) {
		const assignment = /^\s*(?:export\s+)?([A-Z0-9_]+)\s*=\s*(.*)$/.exec(line);
		if (!assignment) continue;
		const [, name, value] = assignment;
		const unquoted = (value ?? '').trim().replace(/^['"]|['"]$/g, '');
		if (name && unquoted) settings[name] ??= unquoted;
	}
	return settings;
}

const atRepositoryRoot = settingsAtRepositoryRoot();

export function setting(name: string): string {
	return process.env[name] ?? atRepositoryRoot[name] ?? '';
}

export function requiredSetting(name: string): string {
	const value = setting(name);
	if (value) return value;
	throw new Error(`${name} is set neither in this shell nor at ${repositorySettingsPath}`);
}

export function keepSetting(name: string, value: string): void {
	if (setting(name)) throw new Error(`${name} is already set; remove it before issuing another`);
	appendFileSync(repositorySettingsPath, `${name}='${value}'\n`);
}
