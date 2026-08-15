//   bun run host/relay/relay.ts

import { createClient } from '@supabase/supabase-js';
import { readLinkPreview, type LinkPreviewImage } from './link-preview';
import { assetBucket, attachmentAddress, keepMessageAttachment, keepSharedAsset } from './asset-store';
import { defaultAnswerByteCeiling, largestRawBytesThatFit } from './answer-size';
import { positiveNumberSetting } from './settings';
import { answerBodyOf, forwardToChatd, type ConnectedAccount, type KeptAttachment } from './forward';
import { readArrivedMessage, tellingOf, type ArrivedMessage } from './arrived';
import { connectToGateway } from './gateway-socket';


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

let hostSession = await askForHostSession();
const client = createClient(projectURL, publishableKey, {
	accessToken: async () => hostSession.accessToken
});
const companyID = hostSession.companyID;
console.log(`acting as the host of company ${companyID}`);

setInterval(() => void keepGoing('session', keepSessionFresh), 60_000);


function openGatewayConnection(): void {
	const gatewayURL = process.env.GATEWAY_URL?.trim();
	const serverKey = process.env.GATEWAY_SERVER_KEY?.trim();
	if (!gatewayURL || !serverKey) return;

	connectToGateway({ gatewayURL, companyID, serverKey, dispatch, byteCeiling: answerByteCeiling });
}

const dispatch = {
	serveAsset: asset,
	askChatd: (capability: string, body: Record<string, unknown>) =>
		forwardToChatd(chatdBaseURL, messengerPlatform, capability, body, largestPictureBytes),
	keepAttachment,
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
	messengerCredentialOf: async (memberID: string) => {
		const held = await askTheRecord<{ credential?: { kind: string; secret: string } | null }>(
			'GET',
			`/api/agent/messenger-credential?memberID=${encodeURIComponent(memberID)}`
		);
		if (!held.credential) return null;
		return { kind: held.credential.kind, secret: held.credential.secret };
	},
	connectMessengerAccount: async (memberID: string, account: ConnectedAccount) => {
		await askTheRecord('POST', '/api/agent/messenger-account', {
			kind: messengerPlatform,
			memberID,
			...account
		});
	}
};

openGatewayConnection();

async function asset(capability: string, body: Record<string, unknown>): Promise<unknown> {
	if (capability === 'asset.link') return previewOf(String(body.url ?? ''));
	throw new Error(`the app has nothing called ${capability}`);
}

async function keepAttachment(contentBase64: string, contentType: string): Promise<KeptAttachment> {
	const bytes = new Uint8Array(Buffer.from(contentBase64, 'base64'));
	const kept = await keepMessageAttachment(
		client.storage.from(assetBucket),
		companyID,
		bytes,
		contentType.trim() || 'application/octet-stream'
	);
	return {
		address: attachmentAddress(projectURL, kept.path),
		sizeBytes: bytes.byteLength,
		digest: kept.digest
	};
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
	console.log('session renewed');
}

async function keepGoing(what: string, work: () => Promise<void>): Promise<void> {
	try {
		await work();
	} catch (error) {
		console.error(`${what} failed, still listening:`, error instanceof Error ? error.message : error);
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
