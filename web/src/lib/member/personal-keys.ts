import { supabase } from '$lib/supabase';

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

export async function personalKeyNames(): Promise<string[]> {
	const answer = (await answerOf(
		await fetch('/api/member/key', { headers: await signedInHeaders() })
	)) as { names?: string[] };
	return answer.names ?? [];
}

// The key comes back once, and making another by the same name replaces it.
export async function issuePersonalKey(name: string): Promise<string> {
	const answer = (await answerOf(
		await fetch('/api/member/key', {
			method: 'POST',
			headers: await signedInHeaders(),
			body: JSON.stringify({ name })
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
