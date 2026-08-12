//   bun run host/relay/relay.ts

import { createClient, type RealtimeChannel } from '@supabase/supabase-js';
import { readLinkPreview, type LinkPreviewImage } from './link-preview';
import { assetBucket, keepSharedAsset } from './asset-store';
import {
	defaultAnswerByteCeiling,
	largestMessageTheProPlanCarries,
	largestRawBytesThatFit,
	oversizeNotice,
	type Answer
} from './answer-size';
import { positiveNumberSetting } from './settings';
import { mintMissingTokens } from './member-tokens';
import {
	answerBodyOf,
	forwardToChatd,
	reportableTopic,
	serveCall,
	type Call,
	type ConnectedAccount
} from './forward';
import { readArrivedMessage, tellingOf, type ArrivedMessage } from './arrived';
import { keepMessengerAccount, type MessengerAccount } from './messenger-account';
import { readPeople, signIn, type MattermostSettings } from './mattermost';


let knownContacts = -1;

const projectURL = required('SUPABASE_URL');
const publishableKey = required('SUPABASE_PUBLISHABLE_KEY');
const agentKey = await agentKeyFromEnvironmentOrFile();
const chatdBaseURL = process.env.CHATD_BASE_URL ?? 'http://127.0.0.1:18090';
const arrivalsPort = positiveNumberSetting('ARRIVALS_PORT', process.env.ARRIVALS_PORT, 18091);
const maildBaseURL = process.env.MAILD_BASE_URL ?? 'http://127.0.0.1:18092';
const admindBaseURL = process.env.ADMIND_BASE_URL ?? 'http://127.0.0.1:18080';
const appURL = required('INTERNKIM_APP_URL');
const messengerPlatform = required('MESSENGER_PLATFORM');
const answerByteCeiling = positiveNumberSetting(
	'ANSWER_BYTE_CEILING',
	process.env.ANSWER_BYTE_CEILING,
	defaultAnswerByteCeiling
);
const rejoinDeadlineMilliseconds = 60_000;
const largestPictureBytes = largestRawBytesThatFit(answerByteCeiling);
const platformTheDirectoryClientServes = 'mattermost';

if (answerByteCeiling > largestMessageTheProPlanCarries) {
	console.warn(
		`ANSWER_BYTE_CEILING is ${answerByteCeiling}, past the ${largestMessageTheProPlanCarries} a Pro project carries; if this project carries no more, answers over that vanish instead of coming back 413`
	);
}

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

const messenger = keepMessengerAccount(
	() => askForConnection(messengerPlatform),
	async (settings) => {
		const session = await signIn(settings);
		console.log(`${messengerPlatform} ready as ${settings.email}`);
		return session;
	}
);

let hostSession = await askForHostSession();
const client = createClient(projectURL, publishableKey, {
	accessToken: async () => hostSession.accessToken
});
const companyID = hostSession.companyID;
console.log(`acting as the host of company ${companyID}`);

client.realtime.setAuth(hostSession.accessToken);
const presence = client.channel(`company:${companyID}`, { config: { private: true } });
const listeningTo = new Map<string, RealtimeChannel>();

async function listenTo(memberID: string): Promise<RealtimeChannel> {
	const known = listeningTo.get(memberID);
	if (known) return known;
	const theirs = client.channel(`member:${memberID}`, { config: { private: true } });
	theirs.on('broadcast', { event: 'call' }, ({ payload }) => {
		void answer(payload as Call, memberID);
	});
	listeningTo.set(memberID, theirs);
	await join(theirs);
	return theirs;
}

async function listenToEveryMember(): Promise<void> {
	const members = await client.from('member').select('id').returns<{ id: string }[]>();
	if (members.error) throw new Error(members.error.message);
	for (const member of members.data) await listenTo(member.id);
}

setInterval(() => void keepGoing('session', keepSessionFresh), 60_000);

await join(presence);
await presence.track({ startedAt: new Date().toISOString() });
await listenToEveryMember();
setInterval(() => void keepGoing('members', listenToEveryMember), 600_000);
console.log(`listening to ${listeningTo.size} members of company ${companyID}`);

function join(channel: RealtimeChannel): Promise<void> {
	return new Promise<void>((resolve, reject) => {
		channel.subscribe((status, error) => {
			if (status === 'SUBSCRIBED') resolve();
			if (error) reject(error);
		});
	});
}

let lastJoinedAt = Date.now();
setInterval(watchTheChannel, 10_000);

function watchTheChannel(): void {
	if (presence.state === 'joined' && [...listeningTo.values()].every((theirs) => theirs.state === 'joined')) {
		lastJoinedAt = Date.now();
		return;
	}
	const awayForMilliseconds = Date.now() - lastJoinedAt;
	if (awayForMilliseconds < rejoinDeadlineMilliseconds) return;
	console.error(`presence is ${presence.state} and some member channel is not joined, ${Math.round(awayForMilliseconds / 1000)}s past the deadline; exiting so the supervisor restarts`);
	process.exit(1);
}

async function answer(call: Call, channelMemberID: string): Promise<void> {
	if (typeof call.callID !== 'string' || typeof call.capability !== 'string') return;
	const callID = call.callID;
	try {
		const { status, body, replyTo } = await serveCall(dispatch, call, channelMemberID);
		await reply(replyTo ?? reportableTopic(call), { callID, status, body });
	} catch (error) {
		const message = error instanceof Error ? error.message : 'the app could not do that';
		await reply(reportableTopic(call), { callID, status: 500, body: { error: message } });
	}
}

const dispatch = {
	serveAsset: asset,
	askChatd: (capability: string, body: Record<string, unknown>) =>
		forwardToChatd(chatdBaseURL, messengerPlatform, capability, body, largestPictureBytes),
	askMaild: async (operation: string, body: Record<string, unknown>) => {
		const response = await fetch(`${maildBaseURL}/v1/mail/${encodeURIComponent(operation)}`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify(body)
		});
		return { status: response.status, body: await answerBodyOf(response) };
	},
	mailAccountOf: async (memberID: string) => {
		const held = await askTheRecord<{ account?: Record<string, unknown> | null }>(
			'GET',
			`/api/agent/mail-account?memberID=${encodeURIComponent(memberID)}`
		);
		return held.account ?? null;
	},
	askAdmind,
	emailOfMember: async (memberID: string) => {
		const member = await client
			.from('member')
			.select('email')
			.eq('id', memberID)
			.maybeSingle<{ email: string | null }>();
		if (member.error) throw new Error(member.error.message);
		return member.data?.email ?? null;
	},
	connectMessengerAccount: async (memberID: string, account: ConnectedAccount) => {
		await askTheRecord('POST', '/api/agent/messenger-account', {
			kind: messengerPlatform,
			memberID,
			...account
		});
	},
	memberOfExternalID: async (externalID: string) => {
		const contact = await client
			.from('contact')
			.select('member_id')
			.eq('company_id', companyID)
			.eq('platform', messengerPlatform)
			.eq('external_id', externalID)
			.maybeSingle<{ member_id: string | null }>();
		if (contact.error) throw new Error(contact.error.message);
		return contact.data?.member_id ?? null;
	}
};

async function asset(capability: string, body: Record<string, unknown>): Promise<unknown> {
	if (capability === 'asset.link') return previewOf(String(body.url ?? ''));
	throw new Error(`the app has nothing called ${capability}`);
}

async function reply(replyTo: string | null, answer: Answer): Promise<void> {
	const notice = oversizeNotice(answer, answerByteCeiling);
	if (notice) console.error(`answer for ${answer.callID} is over the ${answerByteCeiling} byte ceiling`);
	if (!replyTo) return;
	try {
		const theirs = await listenTo(replyTo);
		await theirs.httpSend('answer', notice ?? answer);
	} catch (error) {
		const refusal = error instanceof Error ? error.message : 'the record would not take it';
		console.error(`answer for ${answer.callID} never reached member:${replyTo}: ${refusal}`);
	}
}

type ServedLinkPreview = {
	url: string;
	title: string;
	description: string;
	siteName: string;
	imageURL: string;
};

const linkPreviews = new Map<string, ServedLinkPreview | null>();

async function previewOf(link: string): Promise<ServedLinkPreview | null> {
	if (!linkPreviews.has(link)) {
		linkPreviews.set(link, await servePreview(link).catch(() => null));
	}
	return linkPreviews.get(link) ?? null;
}

async function servePreview(link: string): Promise<ServedLinkPreview | null> {
	const preview = await readLinkPreview(link);
	if (!preview) return null;
	const { image, ...described } = preview;
	return { ...described, imageURL: image ? await storedImageURL(image) : '' };
}

async function storedImageURL(image: LinkPreviewImage): Promise<string> {
	const store = client.storage.from(assetBucket);
	return keepSharedAsset(store, companyID, 'link', image.bytes, image.contentType).catch((error: unknown) => {
		console.error(`link preview image not stored: ${error instanceof Error ? error.message : error}`);
		return '';
	});
}

async function tellThoseAddressed(arrived: ArrivedMessage): Promise<number> {
	if (arrived.recipientExternalIDs.length === 0) return 0;
	const spoken = await askTheRecord<{ told?: number }>('POST', '/api/agent/notify', {
		platform: messengerPlatform,
		externalIDs: arrived.recipientExternalIDs,
		category: 'message',
		...tellingOf(arrived, await authorNameOf(arrived))
	});
	return spoken.told ?? 0;
}

async function authorNameOf(arrived: ArrivedMessage): Promise<string> {
	if (arrived.authorName) return arrived.authorName;
	return nameOf(arrived.authorExternalID);
}

async function nameOf(externalID: string): Promise<string> {
	const contact = await client
		.from('contact')
		.select('name')
		.eq('company_id', companyID)
		.eq('platform', messengerPlatform)
		.eq('external_id', externalID)
		.maybeSingle<{ name: string | null }>();
	if (contact.error) throw new Error(contact.error.message);
	return contact.data?.name ?? '';
}

Bun.serve({
	hostname: '127.0.0.1',
	port: arrivalsPort,
	fetch: async (request) => {
		if (request.method !== 'POST') return new Response('post an arrival', { status: 405 });
		const arrived = readArrivedMessage(await request.json().catch(() => null));
		if (!arrived) return new Response('that is not a message', { status: 400 });
		const told = await tellThoseAddressed(arrived).catch((error) => {
			console.error('arrival not told:', error instanceof Error ? error.message : error);
			return 0;
		});
		return Response.json({ told });
	}
});
console.log(`arrivals accepted on 127.0.0.1:${arrivalsPort}`);

if (messengerPlatform === platformTheDirectoryClientServes) {
	await keepGoing('contacts', refreshContacts);
	setInterval(() => void keepGoing('contacts', refreshContacts), 600_000);
	await keepGoing('credentials', provisionMemberCredentials);
	setInterval(() => void keepGoing('credentials', provisionMemberCredentials), 600_000);
} else {
	console.log(`no directory client here for ${messengerPlatform}; chatd answers for its people`);
}

async function askTheRecord<Value>(method: string, path: string, body?: unknown): Promise<Value> {
	const response = await fetch(`${appURL}${path}`, {
		method,
		headers: {
			Authorization: `Bearer ${agentKey}`,
			...(body === undefined ? {} : { 'Content-Type': 'application/json' })
		},
		body: body === undefined ? undefined : JSON.stringify(body)
	});
	if (!response.ok) throw new Error(`the central plane answered ${response.status} for ${path}`);
	return (await response.json()) as Value;
}

async function asTheMessengerAdmin<Value>(work: (account: MessengerAccount) => Promise<Value>): Promise<Value> {
	const account = await messenger.admin();
	try {
		return await work(account);
	} catch (error) {
		messenger.forgetSession();
		throw error;
	}
}

async function provisionMemberCredentials(): Promise<void> {
	const held = await askTheRecord<{ have?: string[] }>(
		'GET',
		`/api/agent/messenger-credentials?kind=${encodeURIComponent(messengerPlatform)}`
	);
	const { credentials, report } = await asTheMessengerAdmin(async ({ settings, session }) => {
		const people = await readPeople(settings, session.token);
		return mintMissingTokens(settings, session.token, people, new Set(held.have ?? []));
	});

	if (credentials.length > 0) {
		const kept = await askTheRecord<{ kept?: number }>('POST', '/api/agent/messenger-credentials', {
			kind: messengerPlatform,
			credentials
		});
		console.log(`${kept.kept ?? 0} member credentials recorded, ${report.alreadyHeld} already held`);
	}
	if (report.refused.length > 0) {
		console.error(`the messenger refused a token for ${report.refused.length} of its people`);
	}
}

function refusalOf(kind: string, response: Response): string {
	// fetch drops Authorization across origins, so a redirect turns a good key into no key.
	const moved = response.redirected ? ` after being sent to ${response.url}` : '';
	if (response.status === 401) return `the central plane got no agent key${moved}`;
	if (response.status === 403) return `the central plane refused this agent key${moved}`;
	if (response.status === 404) return `this company has no ${kind} connection${moved}`;
	return `the central plane answered ${response.status} for the ${kind} connection${moved}`;
}

async function askForConnection(kind: string): Promise<MattermostSettings> {
	const response = await fetch(`${appURL}/api/agent/connection?kind=${encodeURIComponent(kind)}`, {
		headers: { Authorization: `Bearer ${agentKey}` }
	});
	if (!response.ok) throw new Error(refusalOf(kind, response));
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

async function refreshContacts(): Promise<void> {
	const people = await asTheMessengerAdmin(({ settings, session }) => readPeople(settings, session.token));
	const members = await client.from('member').select('id, email').returns<{ id: string; email: string | null }[]>();
	if (members.error) throw new Error(members.error.message);
	const memberByEmail = new Map(members.data.map((entry) => [entry.email ?? '', entry.id]));

	const { error } = await client.from('contact').upsert(
		people.map((person) => ({
			company_id: companyID,
			platform: messengerPlatform,
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

const workspacePaths: Record<string, string> = {
	'person.memory.graph': '/memory/api/graph',
	'person.memory.schedules': '/memory/api/schedules',
	'person.files.roots': '/files/api/roots',
	'person.files.list': '/files/api/list',
	'person.tasks.list': '/tasks/api/runs',
	'person.tasks.detail': '/tasks/api/run-detail'
};

async function askAdmind(
	capability: string,
	body: Record<string, unknown>,
	requesterEmail: string
): Promise<{ status: number; body: unknown }> {
	const path = workspacePaths[capability];
	if (!path) return { status: 404, body: { error: `the app has nothing called ${capability}` } };

	const query = new URLSearchParams();
	for (const [name, value] of Object.entries(body)) {
		if (name === 'actor' || value === undefined || value === null) continue;
		query.set(name, String(value));
	}
	const asked = query.toString() ? `${path}?${query}` : path;
	const response = await fetch(`${admindBaseURL}${asked}`, {
		headers: { 'X-INTERNKIM-REQUESTER-EMAIL': requesterEmail }
	});
	return { status: response.status, body: await answerBodyOf(response) };
}
