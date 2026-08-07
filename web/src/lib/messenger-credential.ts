import { supabase } from '$lib/supabase';

export type ActorCredential = {
	kind: string;
	secret: string;
};

export class NoMessengerCredentialError extends Error {
	constructor() {
		super('this account has no messenger credential yet');
		this.name = 'NoMessengerCredentialError';
	}
}

let held: Promise<ActorCredential> | undefined;

export async function messengerCredential(): Promise<ActorCredential> {
	if (held) return held;
	const attempt = fetchCredential();
	held = attempt;
	attempt.catch(() => forgetCredential(attempt));
	return attempt;
}

export function forgetMessengerCredential(): void {
	held = undefined;
}

function forgetCredential(attempt: Promise<ActorCredential>): void {
	if (held !== attempt) return;
	held = undefined;
}

async function fetchCredential(): Promise<ActorCredential> {
	const client = supabase();
	const { data } = await client.auth.getSession();
	const accessToken = data.session?.access_token;
	if (!accessToken) throw new Error('sign in first');

	const response = await fetch('/api/member/messenger-credential?kind=mattermost', {
		headers: { Authorization: `Bearer ${accessToken}` }
	});
	if (response.status === 404) throw new NoMessengerCredentialError();
	if (!response.ok) throw new Error(`the messenger credential answered ${response.status}`);

	const credential = (await response.json()) as { kind?: string; secret?: string };
	if (!credential.secret) throw new NoMessengerCredentialError();
	return { kind: `${credential.kind ?? 'mattermost'}-token`, secret: credential.secret };
}
