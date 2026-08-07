//   bun run host/messenger/bridge.ts

import { createClient, type SupabaseClient } from '@supabase/supabase-js';
import { readLinkPreview, type LinkPreview } from './link-preview';
import { largestRawBytesThatFit, oversizeNotice, type Answer } from './answer-size';
import { mintMissingTokens } from './member-tokens';
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

let knownContacts = -1;

const projectURL = required('SUPABASE_URL');
const publishableKey = required('SUPABASE_PUBLISHABLE_KEY');
const agentKey = await agentKeyFromEnvironmentOrFile();
const appURL = required('INTERNKIM_APP_URL');
const answerByteCeiling = Number(process.env.ANSWER_BYTE_CEILING ?? 200_000);
const rejoinDeadlineMilliseconds = 60_000;
const largestPictureBytes = largestRawBytesThatFit(answerByteCeiling);

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

let hostSession = await askForHostSession();
const client = createClient(projectURL, publishableKey, {
	accessToken: async () => hostSession.accessToken
});
const companyID = hostSession.companyID;
console.log(`acting as the host of company ${companyID}`);

await refreshContacts(client, companyID, session);
setInterval(() => void keepGoing('contacts', () => refreshContacts(client, companyID, session)), 600_000);

await keepGoing('credentials', provisionMemberCredentials);
setInterval(() => void keepGoing('credentials', provisionMemberCredentials), 600_000);

client.realtime.setAuth(hostSession.accessToken);
const channel = client.channel(`company:${companyID}`, { config: { private: true } });

channel.on('broadcast', { event: 'call' }, ({ payload }) => {
	void answer(payload as Call);
});

setInterval(() => void keepGoing('session', keepSessionFresh), 60_000);

await new Promise<void>((resolve, reject) => {
	channel.subscribe((status, error) => {
		if (status === 'SUBSCRIBED') resolve();
		if (error) reject(error);
	});
});
await channel.track({ startedAt: new Date().toISOString() });
console.log(`listening on company:${companyID}`);

let lastJoinedAt = Date.now();
setInterval(watchTheChannel, 10_000);

function watchTheChannel(): void {
	if (channel.state === 'joined') {
		lastJoinedAt = Date.now();
		return;
	}
	const awayForMilliseconds = Date.now() - lastJoinedAt;
	if (awayForMilliseconds < rejoinDeadlineMilliseconds) return;
	console.error(`channel has been ${channel.state} for ${Math.round(awayForMilliseconds / 1000)}s, exiting so the supervisor restarts`);
	process.exit(1);
}

async function answer(call: Call): Promise<void> {
	if (typeof call.callID !== 'string' || typeof call.capability !== 'string') return;
	const callID = call.callID;
	try {
		const { status, body, replyTo } = await serveCall(dispatch, call);
		await reply(replyTo ?? reportableTopic(call), { callID, status, body });
	} catch (error) {
		const message = error instanceof Error ? error.message : 'the app could not do that';
		await reply(reportableTopic(call), { callID, status: 500, body: { error: message } });
	}
}

const dispatch = {
	serveAsset: asset,
	askChatd: (capability: string, body: Record<string, unknown>) =>
		forwardToChatd(chatdBaseURL, 'mattermost', capability, body),
	memberOfExternalID: async (externalID: string) => {
		const contact = await client
			.from('contact')
			.select('member_id')
			.eq('company_id', companyID)
			.eq('platform', 'mattermost')
			.eq('external_id', externalID)
			.maybeSingle<{ member_id: string | null }>();
		if (contact.error) throw new Error(contact.error.message);
		return contact.data?.member_id ?? null;
	}
};

async function asset(capability: string, body: Record<string, unknown>): Promise<unknown> {
	if (capability === 'asset.emoji') return emojiSet();
	if (capability === 'asset.picture') return pictureOf(String(body.externalID ?? ''));
	if (capability === 'asset.link') return previewOf(String(body.url ?? ''));
	throw new Error(`the app has nothing called ${capability}`);
}

async function reply(replyTo: string | null, answer: Answer): Promise<void> {
	const notice = oversizeNotice(answer, answerByteCeiling);
	if (notice) console.error(`answer for ${answer.callID} is over the ${answerByteCeiling} byte ceiling`);
	if (!replyTo) return;
	await client
		.channel(`member:${replyTo}`, { config: { private: true } })
		.send({ type: 'broadcast', event: 'answer', payload: notice ?? answer });
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
		pictures.set(externalID, await readProfilePicture(mattermost, session, externalID, largestPictureBytes));
	}
	return pictures.get(externalID) ?? null;
}

async function previewOf(link: string): Promise<LinkPreview | null> {
	if (!linkPreviews.has(link)) {
		linkPreviews.set(link, await readLinkPreview(link).catch(() => null));
	}
	return linkPreviews.get(link) ?? null;
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

async function askForHostSession(): Promise<{ companyID: string; accessToken: string; expiresAt: number }> {
	const response = await fetch(`${appURL}/api/agent/host-session`, {
		method: 'POST',
		headers: { Authorization: `Bearer ${agentKey}` }
	});
	if (!response.ok) throw new Error(`the central plane refused this agent key (${response.status})`);
	return (await response.json()) as { companyID: string; accessToken: string; expiresAt: number };
}

async function keepSessionFresh(): Promise<void> {
	const secondsLeft = hostSession.expiresAt - Math.floor(Date.now() / 1000);
	if (secondsLeft > 300) return;
	hostSession = await askForHostSession();
	await client.realtime.setAuth(hostSession.accessToken);
	console.log('session renewed');
}

async function keepGoing(what: string, work: () => Promise<void>): Promise<void> {
	try {
		await work();
	} catch (error) {
		console.error(`${what} failed, still listening:`, error instanceof Error ? error.message : error);
	}
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
