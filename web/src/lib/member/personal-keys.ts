import { supabase } from '$lib/supabase';
import type { PublicAPIPermission } from '$lib/public-api-permission';

export type PersonalKey = {
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
	if (!response.ok) throw new Error((await response.text()).trim() || `the key store returned ${response.status}`);
	return response.json();
}

export async function personalKeys(): Promise<PersonalKey[]> {
	const answer = (await answerOf(
		await fetch('/api/member/key', { headers: await signedInHeaders() })
	)) as { keys?: PersonalKey[] };
	return answer.keys ?? [];
}

export async function issuePersonalKey(name: string, permission: PublicAPIPermission): Promise<string> {
	const answer = (await answerOf(
		await fetch('/api/member/key', {
			method: 'POST',
			headers: await signedInHeaders(),
			body: JSON.stringify({ name, permission })
		})
	)) as { apiKey: string };
	return answer.apiKey;
}

export async function forgetPersonalKey(name: string): Promise<void> {
	await answerOf(
		await fetch(`/api/member/key?name=${encodeURIComponent(name)}`, {
			method: 'DELETE',
			headers: await signedInHeaders()
		})
	);
}
