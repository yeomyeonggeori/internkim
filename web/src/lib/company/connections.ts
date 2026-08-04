import { supabase } from '$lib/supabase';

export type CompanyConnectionKind = 'smtp' | 'imap' | 'caldav' | 'mattermost';

export type CompanyConnection = {
	kind: string;
	host: string;
	settings: Record<string, unknown>;
	hasSecret: boolean;
};

export type CompanyConnectionInput = {
	kind: CompanyConnectionKind;
	host: string;
	settings: Record<string, unknown>;
	secret?: string;
};

async function withToken(path: string, init: RequestInit = {}): Promise<Response> {
	const { data } = await supabase().auth.getSession();
	const accessToken = data.session?.access_token;
	if (!accessToken) throw new Error('sign in first');
	return fetch(path, {
		...init,
		headers: { ...(init.headers ?? {}), Authorization: `Bearer ${accessToken}`, 'Content-Type': 'application/json' }
	});
}

async function orThrow(response: Response): Promise<Response> {
	if (response.ok) return response;
	throw new Error((await response.text()).trim() || `the request returned ${response.status}`);
}

export async function fetchCompanyConnections(): Promise<CompanyConnection[]> {
	const response = await orThrow(await withToken('/api/company/connections'));
	const document = (await response.json()) as { connections?: CompanyConnection[] };
	return document.connections ?? [];
}

export async function saveCompanyConnection(connection: CompanyConnectionInput): Promise<void> {
	await orThrow(await withToken('/api/company/connections', { method: 'PUT', body: JSON.stringify(connection) }));
}

export async function forgetCompanyConnection(kind: CompanyConnectionKind): Promise<void> {
	await orThrow(await withToken(`/api/company/connections?kind=${encodeURIComponent(kind)}`, { method: 'DELETE' }));
}
