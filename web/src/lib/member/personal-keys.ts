import { supabase } from '$lib/supabase';

export type PersonalKey = {
	keyID: string;
	name: string;
	createdAt: string;
	lastSeenAt: string | null;
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

export async function listPersonalKeys(): Promise<PersonalKey[]> {
	const answer = (await answerOf(
		await fetch('/api/member/key', { headers: await signedInHeaders() })
	)) as { keys?: PersonalKey[] };
	return answer.keys ?? [];
}

// The key comes back once. It is kept as a hash, so nothing can read it out
// again for whoever loses it.
export async function issuePersonalKey(name: string): Promise<{ key: PersonalKey; apiKey: string }> {
	return (await answerOf(
		await fetch('/api/member/key', {
			method: 'POST',
			headers: await signedInHeaders(),
			body: JSON.stringify({ name })
		})
	)) as { key: PersonalKey; apiKey: string };
}

export async function revokePersonalKey(keyID: string): Promise<void> {
	await answerOf(
		await fetch(`/api/member/key?keyID=${encodeURIComponent(keyID)}`, {
			method: 'DELETE',
			headers: await signedInHeaders()
		})
	);
}
