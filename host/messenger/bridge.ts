//   bun run host/messenger/bridge.ts

import { createClient, type SupabaseClient } from '@supabase/supabase-js';
import { readLinkPreview, type LinkPreview } from './link-preview';
import {
	addReaction,
	editPost,
	erasePost,
	openDirectChannel,
	readChannels,
	readPeople,
	readPosts,
	readCustomEmoji,
	readProfilePicture,
	removeReaction,
	signIn,
	writePost,
	type MattermostSession,
	type MattermostSettings
} from './mattermost';

type Call = { callID?: string; method?: string; path?: string; body?: unknown };
type Answer = { callID: string; status: number; body: unknown };

let knownContacts = -1;

const projectURL = required('SUPABASE_URL');
const publishableKey = required('SUPABASE_PUBLISHABLE_KEY');
const agentKey = await agentKeyFromEnvironmentOrFile();
const appURL = required('INTERNKIM_APP_URL');

function required(name: string): string {
	const value = process.env[name];
	if (!value) throw new Error(`set ${name}`);
	return value;
}

async function agentKeyFromEnvironmentOrFile(): Promise<string> {
	const given = process.env.AGENT_API_KEY?.trim();
	if (given) return given;
	const keptAt = process.env.AGENT_API_KEY_PATH?.trim();
	if (!keptAt) throw new Error('set AGENT_API_KEY or AGENT_API_KEY_PATH');
	const kept = (await Bun.file(keptAt).text()).trim();
	if (!kept) throw new Error(`${keptAt} holds no agent key`);
	return kept;
}

const mattermost = await askForConnection('mattermost');
const session = await signIn(mattermost);
console.log(`mattermost ready as ${mattermost.email}`);

let memberSession = await askForSession(session.userID);
const client = createClient(projectURL, publishableKey, {
	auth: { autoRefreshToken: false, persistSession: false },
	global: { headers: { Authorization: `Bearer ${memberSession.accessToken}` } }
});

const member = await client
	.from('member')
	.select('id, company_id')
	.eq('id', memberSession.memberID)
	.single<{ id: string; company_id: string }>();
if (member.error) throw new Error(member.error.message);
console.log(`acting as member ${memberSession.memberID}`);

await refreshContacts(client, member.data.company_id, session);
setInterval(() => void refreshContacts(client, member.data.company_id, session), 600_000);

client.realtime.setAuth(memberSession.accessToken);
const channel = client.channel(`company:${member.data.company_id}`, { config: { private: true } });

channel.on('broadcast', { event: 'call' }, ({ payload }) => {
	void answer(payload as Call);
});

setInterval(() => void keepSessionFresh(), 60_000);

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

const pictures = new Map<string, { dataURL: string } | null>();
const linkPreviews = new Map<string, LinkPreview | null>();
let customEmoji: { name: string; url: string }[] | null = null;

async function emojiSet(): Promise<{ name: string; url: string }[]> {
	customEmoji ??= await readCustomEmoji(mattermost, session);
	return customEmoji;
}

async function pictureOf(externalID: string): Promise<{ dataURL: string } | null> {
	if (!pictures.has(externalID)) {
		pictures.set(externalID, await readProfilePicture(mattermost, session, externalID));
	}
	return pictures.get(externalID) ?? null;
}

async function previewOf(link: string): Promise<LinkPreview | null> {
	if (!linkPreviews.has(link)) {
		linkPreviews.set(link, await readLinkPreview(link).catch(() => null));
	}
	return linkPreviews.get(link) ?? null;
}

async function route(method: string, path: string, body: unknown): Promise<unknown> {
	const [route, query] = path.split('?');
	const parameters = new URLSearchParams(query ?? '');
	const parts = route.split('/').filter(Boolean);
	const asked = body as { body?: string; parentID?: string; emoji?: string } | null;

	if (method === 'GET' && parts[0] === 'emoji') return emojiSet();
	if (method === 'GET' && parts[0] === 'link') return previewOf(parameters.get('url') ?? '');
	if (method === 'GET' && parts[0] === 'person' && parts[2] === 'picture') {
		return pictureOf(parts[1]);
	}
	if (method === 'GET' && parts[0] === 'person') return readPeople(mattermost, session);
	if (method === 'GET' && parts.length === 1 && parts[0] === 'channel') return readChannels(mattermost, session);
	if (method === 'POST' && parts[0] === 'channel' && parts[1] === 'direct') {
		const asked = body as { memberIDs?: string[] } | null;
		return openDirectChannel(mattermost, session, await externalIDsOf(asked?.memberIDs ?? []));
	}
	if (method === 'GET' && parts[0] === 'channel' && parts[2] === 'post') {
		return readPosts(mattermost, session, parts[1], parameters.get('before') ?? undefined);
	}
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

async function askForConnection(kind: string): Promise<MattermostSettings> {
	const response = await fetch(`${appURL}/api/agent/connection?kind=${encodeURIComponent(kind)}`, {
		headers: { Authorization: `Bearer ${agentKey}` }
	});
	if (!response.ok) throw new Error(`the central plane has no ${kind} connection for this company (${response.status})`);
	const connection = (await response.json()) as {
		host: string;
		settings: { username?: string };
		secret: string | null;
	};
	if (!connection.settings.username || !connection.secret) {
		throw new Error(`the ${kind} connection is missing an account`);
	}
	return { baseURL: connection.host, email: connection.settings.username, password: connection.secret };
}

async function askForSession(externalID: string): Promise<{ memberID: string; accessToken: string; expiresAt: number }> {
	const response = await fetch(`${appURL}/api/agent/session`, {
		method: 'POST',
		headers: { Authorization: `Bearer ${agentKey}`, 'Content-Type': 'application/json' },
		body: JSON.stringify({ kind: 'mattermost', externalID })
	});
	if (!response.ok) throw new Error(`the central plane refused this agent key (${response.status})`);
	return (await response.json()) as { memberID: string; accessToken: string; expiresAt: number };
}

async function keepSessionFresh(): Promise<void> {
	const secondsLeft = memberSession.expiresAt - Math.floor(Date.now() / 1000);
	if (secondsLeft > 300) return;
	memberSession = await askForSession(session.userID);
	client.realtime.setAuth(memberSession.accessToken);
	console.log('session renewed');
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
	if (people.length !== knownContacts) {
		console.log(`${people.length} contacts recorded`);
		knownContacts = people.length;
	}
}
