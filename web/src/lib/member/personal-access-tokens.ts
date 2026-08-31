import { apiURL, supabase } from '$lib/supabase';
import type { PublicAPIPermission } from '$lib/public-api-permission';

export type PersonalAccessToken = {
	name: string;
	permission: PublicAPIPermission;
};

function tokenAddress(path: string): string {
	const address = apiURL();
	if (!address) throw new Error('the public api address is not configured');
	return `${address.replace(/\/+$/, '')}/v1${path}`;
}

async function signedInHeaders(): Promise<Record<string, string>> {
	const { data } = await supabase().auth.getSession();
	const accessToken = data.session?.access_token;
	if (!accessToken) throw new Error('sign in first');
	return { Authorization: `Bearer ${accessToken}`, 'Content-Type': 'application/json' };
}

async function answerOf(response: Response): Promise<unknown> {
	const answer: unknown = await response.json().catch(() => null);
	if (response.ok) return answer;
	const spoken =
		typeof answer === 'object' && answer !== null && 'error' in answer ? String(answer.error) : '';
	throw new Error(spoken || `the token store returned ${response.status}`);
}

export async function personalAccessTokens(): Promise<PersonalAccessToken[]> {
	const answer = (await answerOf(
		await fetch(tokenAddress('/tokens'), { headers: await signedInHeaders() })
	)) as { tokens?: PersonalAccessToken[] };
	return answer.tokens ?? [];
}

export async function issuePersonalAccessToken(name: string, permission: PublicAPIPermission): Promise<string> {
	const answer = (await answerOf(
		await fetch(tokenAddress('/token'), {
			method: 'POST',
			headers: await signedInHeaders(),
			body: JSON.stringify({ name, permission })
		})
	)) as { token: string };
	return answer.token;
}

export async function forgetPersonalAccessToken(name: string): Promise<void> {
	await answerOf(
		await fetch(`${tokenAddress('/token')}?name=${encodeURIComponent(name)}`, {
			method: 'DELETE',
			headers: await signedInHeaders()
		})
	);
}
