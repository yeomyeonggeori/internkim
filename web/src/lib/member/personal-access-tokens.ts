import { supabase } from '$lib/supabase';
import type { PublicAPIPermission } from '$lib/public-api-permission';

export type PersonalAccessToken = {
	name: string;
	permission: PublicAPIPermission;
};

async function signedInHeaders(): Promise<Record<string, string>> {
	const { data } = await supabase().auth.getSession();
	const accessToken = data.session?.access_token;
	if (!accessToken) throw new Error('sign in first');
	return { Authorization: `Bearer ${accessToken}`, 'Content-Type': 'application/json' };
}

async function answerOf(response: Response): Promise<unknown> {
	const written = await response.text();
	if (!response.ok) {
		let spoken = '';
		try {
			const parsed: unknown = JSON.parse(written);
			if (typeof parsed === 'object' && parsed !== null) {
				const { error, message } = parsed as { error?: unknown; message?: unknown };
				spoken = typeof error === 'string' ? error : typeof message === 'string' ? message : '';
			}
		} catch {
			spoken = written.trim();
		}
		throw new Error(spoken || `the token store returned ${response.status}`);
	}
	return JSON.parse(written);
}

export async function personalAccessTokens(): Promise<PersonalAccessToken[]> {
	const answer = (await answerOf(
		await fetch('/api/v1/tokens', { headers: await signedInHeaders() })
	)) as { tokens?: PersonalAccessToken[] };
	return answer.tokens ?? [];
}

export async function issuePersonalAccessToken(name: string, permission: PublicAPIPermission): Promise<string> {
	const answer = (await answerOf(
		await fetch('/api/v1/token', {
			method: 'POST',
			headers: await signedInHeaders(),
			body: JSON.stringify({ name, permission })
		})
	)) as { token: string };
	return answer.token;
}

export async function forgetPersonalAccessToken(name: string): Promise<void> {
	await answerOf(
		await fetch(`/api/v1/token?name=${encodeURIComponent(name)}`, {
			method: 'DELETE',
			headers: await signedInHeaders()
		})
	);
}
