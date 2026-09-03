import { execFileSync } from 'node:child_process';

export const projectReference = process.env.SUPABASE_PROJECT_REF ?? 'mutvimjbvmoludotyehk';

export function accessToken(): string {
	const fromEnvironment = process.env.SUPABASE_ACCESS_TOKEN?.trim();
	if (fromEnvironment) return fromEnvironment;
	const stored = execFileSync(
		'security',
		['find-generic-password', '-s', 'Supabase CLI', '-a', 'access-token', '-w'],
		{ encoding: 'utf8' }
	).trim();
	// The CLI stores the token through go-keyring, which base64s it behind a prefix.
	const encodedPrefix = 'go-keyring-base64:';
	if (!stored.startsWith(encodedPrefix)) return stored;
	return Buffer.from(stored.slice(encodedPrefix.length), 'base64').toString('utf8');
}

export async function remoteQuery<Row>(sql: string, writable = false): Promise<Row[]> {
	const response = await fetch(
		`https://api.supabase.com/v1/projects/${projectReference}/database/query`,
		{
			method: 'POST',
			headers: {
				Authorization: `Bearer ${accessToken()}`,
				'Content-Type': 'application/json'
			},
			body: JSON.stringify({ query: sql, read_only: !writable })
		}
	);
	const body = await response.text();
	if (!response.ok) throw new Error(`${response.status} ${body}`);
	return JSON.parse(body) as Row[];
}
