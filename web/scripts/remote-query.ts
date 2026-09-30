import { requiredSetting } from './repository-setting';

export function projectReference(): string {
	return requiredSetting('SUPABASE_PROJECT_REF');
}

export function accessToken(): string {
	return requiredSetting('SUPABASE_ACCESS_TOKEN').trim();
}

export async function remoteQuery<Row>(sql: string, writable = false): Promise<Row[]> {
	const response = await fetch(
		`https://api.supabase.com/v1/projects/${projectReference()}/database/query`,
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
