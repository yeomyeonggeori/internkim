//   bun run host/messenger/bridge.ts

import { createClient, type SupabaseClient } from '@supabase/supabase-js';
import {
	addReaction,
	editPost,
	erasePost,
	openDirectChannel,
	readChannels,
	readPeople,
	readPosts,
	removeReaction,
	signIn,
	writePost,
	type MattermostSession,
	type MattermostSettings
} from './mattermost';

type Call = { callID?: string; method?: string; path?: string; body?: unknown };
type Answer = { callID: string; status: number; body: unknown };

const projectURL = required('SUPABASE_URL');
const publishableKey = required('SUPABASE_PUBLISHABLE_KEY');
const memberEmail = required('HOST_MEMBER_EMAIL');
const memberPassword = required('HOST_MEMBER_PASSWORD');
const mattermost: MattermostSettings = {
	baseURL: required('MATTERMOST_URL'),
	email: required('MATTERMOST_EMAIL'),
	password: required('MATTERMOST_PASSWORD')
};

function required(name: string): string {
	const value = process.env[name];
	if (!value) throw new Error(`set ${name}`);
	return value;
}

const client = createClient(projectURL, publishableKey);
const signedIn = await client.auth.signInWithPassword({ email: memberEmail, password: memberPassword });
if (signedIn.error) throw new Error(signedIn.error.message);

const member = await client
	.from('member')
	.select('id, company_id')
	.eq('user_id', signedIn.data.user.id)
	.single<{ id: string; company_id: string }>();
if (member.error) throw new Error(member.error.message);

const session = await signIn(mattermost);
console.log(`mattermost ready as ${mattermost.email}`);
await refreshContacts(client, member.data.company_id, session);

await client.realtime.setAuth();
const channel = client.channel(`company:${member.data.company_id}`, { config: { private: true } });

channel.on('broadcast', { event: 'call' }, ({ payload }) => {
	void answer(payload as Call);
});

await new Promise<void>((resolve, reject) => {
	channel.subscribe((status, error) => {
		if (status === 'SUBSCRIBED') resolve();
		if (error) reject(error);
	});
});
await channel.track({ startedAt: new Date().toISOString() });
console.log(`listening on company:${member.data.company_id}`);

async function answer(call: Call): Promise<void> {
	if (typeof call.callID !== 'string') return;
	try {
		const body = await route(call.method ?? 'GET', call.path ?? '/', call.body);
		await reply({ callID: call.callID, status: 200, body });
	} catch (error) {
		const message = error instanceof Error ? error.message : 'the app could not do that';
		await reply({ callID: call.callID, status: 500, body: { error: message } });
	}
}

async function reply(answer: Answer): Promise<void> {
	await channel.send({ type: 'broadcast', event: 'answer', payload: answer });
}

async function route(method: string, path: string, body: unknown): Promise<unknown> {
	const [route, query] = path.split('?');
	const parameters = new URLSearchParams(query ?? '');
	const parts = route.split('/').filter(Boolean);
	const asked = body as { body?: string; parentID?: string; emoji?: string } | null;

	if (method === 'GET' && parts[0] === 'person') return readPeople(mattermost, session);
	if (method === 'GET' && parts.length === 1 && parts[0] === 'channel') return readChannels(mattermost, session);
	if (method === 'POST' && parts[0] === 'channel' && parts[1] === 'direct') {
		const asked = body as { memberIDs?: string[] } | null;
		return openDirectChannel(mattermost, session, await externalIDsOf(asked?.memberIDs ?? []));
	}
	if (method === 'GET' && parts[0] === 'channel' && parts[2] === 'post') return readPosts(mattermost, session, parts[1]);
	if (method === 'POST' && parts[0] === 'channel' && parts[2] === 'post') {
		return writePost(mattermost, session, parts[1], asked?.body ?? '', asked?.parentID);
	}
	if (method === 'PUT' && parts[0] === 'post') return editPost(mattermost, session, parts[1], asked?.body ?? '');
	if (method === 'DELETE' && parts[0] === 'post' && parts.length === 2) {
		await erasePost(mattermost, session, parts[1]);
		return null;
	}
	if (method === 'POST' && parts[0] === 'post' && parts[2] === 'reaction') {
		await addReaction(mattermost, session, parts[1], asked?.emoji ?? '');
		return null;
	}
	if (method === 'DELETE' && parts[0] === 'post' && parts[2] === 'reaction') {
		await removeReaction(mattermost, session, parts[1], parameters.get('emoji') ?? '');
		return null;
	}
	throw new Error(`the app has nothing at ${method} ${route}`);
}

async function externalIDsOf(memberIDs: string[]): Promise<string[]> {
	if (memberIDs.length === 0) return [];
	const contacts = await client
		.from('contact')
		.select('external_id, member_id')
		.eq('platform', 'mattermost')
		.in('member_id', memberIDs)
		.returns<{ external_id: string; member_id: string }[]>();
	if (contacts.error) throw new Error(contacts.error.message);
	return contacts.data.map((contact) => contact.external_id);
}

async function refreshContacts(client: SupabaseClient, companyID: string, session: MattermostSession): Promise<void> {
	const people = await readPeople(mattermost, session);
	const members = await client.from('member').select('id, email').returns<{ id: string; email: string | null }[]>();
	if (members.error) throw new Error(members.error.message);
	const memberByEmail = new Map(members.data.map((entry) => [entry.email ?? '', entry.id]));

	const { error } = await client.from('contact').upsert(
		people.map((person) => ({
			company_id: companyID,
			platform: 'mattermost',
			external_id: person.externalID,
			name: person.name,
			member_id: memberByEmail.get(person.email) ?? null
		})),
		{ onConflict: 'company_id,platform,external_id' }
	);
	if (error) throw new Error(error.message);
	console.log(`${people.length} contacts recorded`);
}
