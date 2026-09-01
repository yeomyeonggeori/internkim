//   bun run host/relay/relay.ts

import { createClient } from '@supabase/supabase-js';
import { readLinkPreview, type LinkPreview } from './link-preview';
import {
	assetBucket,
	attachmentAddress,
	attachmentAlreadyKept,
	attachmentKind,
	keepMessageAttachment,
	sharedAssetPath
} from './asset-store';
import { defaultAnswerByteCeiling, largestRawBytesThatFit } from './answer-size';
import { positiveNumberSetting } from './settings';
import {
	answerBodyOf,
	defaultAdmindSocketPath,
	forwardToAdmind,
	forwardToAdmindAPI,
	forwardToChatd,
	type AdmindCall,
	type ConnectedAccount,
	type KeptAttachment,
	type KeptFileReference,
	type PublicAPIRequest
} from './forward';
import { readArrivedMessage, tellingOf, type ArrivedMessage } from './arrived';
import { connectToGateway } from './gateway-socket';


const projectURL = required('SUPABASE_URL');
const publishableKey = required('SUPABASE_PUBLISHABLE_KEY');
const agentKey = await agentKeyFromEnvironmentOrFile();
const chatdBaseURL = process.env.CHATD_BASE_URL ?? 'http://127.0.0.1:18090';
const arrivalsPort = positiveNumberSetting('ARRIVALS_PORT', process.env.ARRIVALS_PORT, 18091);
const maildBaseURL = process.env.MAILD_BASE_URL ?? 'http://127.0.0.1:18092';
const admindBaseURL = process.env.ADMIND_BASE_URL ?? 'http://127.0.0.1:18080';
const admindSocketPath = process.env.ADMIND_SOCKET_PATH ?? defaultAdmindSocketPath;
const appURL = required('INTERNKIM_APP_URL');
const messengerPlatform = required('MESSENGER_PLATFORM');
const answerByteCeiling = positiveNumberSetting(
	'ANSWER_BYTE_CEILING',
	process.env.ANSWER_BYTE_CEILING,
	defaultAnswerByteCeiling
);
const largestPictureBytes = largestRawBytesThatFit(answerByteCeiling);
// A file never crosses the gateway — it is kept in the bucket and read from
// there — so this bounds only what the relay will hold in memory while copying
// one across. The largest attachment in a real company's history was 91 MB.
const largestFileBytes = positiveNumberSetting(
	'LARGEST_FILE_BYTES',
	process.env.LARGEST_FILE_BYTES,
	200_000_000
);


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
	askChatd: (capability: string, body: Record<string, unknown>, largestBytes?: number) =>
		forwardToChatd(chatdBaseURL, messengerPlatform, capability, body, largestBytes ?? largestPictureBytes),
	keepAttachment,
	keptAlready,
	keptFileBytes,
	largestFileBytes,
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
	askAdmindAPI: (request: PublicAPIRequest) => forwardToAdmindAPI(admindSocketPath, request),
	askAdmindAsRequester: (call: AdmindCall) => forwardToAdmind(admindSocketPath, call),
	tellAdmindTheDirectoryChanged,
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
		const kind = await messengerCredentialKind();
		const held = await askTheRecord<{ credential?: { kind: string; secret: string } | null }>(
			'GET',
			`/api/agent/messenger-credential?memberID=${encodeURIComponent(memberID)}&kind=${encodeURIComponent(kind)}`
		);
		if (!held.credential) return null;
		return { kind: held.credential.kind, secret: held.credential.secret };
	},
	connectMessengerAccount: async (memberID: string, account: ConnectedAccount) => {
		await askTheRecord('POST', '/api/agent/messenger-account', {
			platform: messengerPlatform,
			memberID,
			...account
		});
	}
};

openGatewayConnection();

let credentialKindAsked: Promise<string> | undefined;

async function messengerCredentialKind(): Promise<string> {
	const asking = credentialKindAsked ?? askWhichCredentialTheMessengerNeeds();
	credentialKindAsked = asking;
	asking.then((kind) => void healMessengerProjection(kind)).catch(() => {
		if (credentialKindAsked === asking) credentialKindAsked = undefined;
	});
	return asking;
}

// The account on the member row is a projection of the issued credential, and a
// credential issued before the projection existed leaves the row behind. The
// record heals the projection whenever this relay learns which credential its
// messenger issues: at boot when the messenger is up, or at first use after.
let isProjectionHealed = false;

async function healMessengerProjection(kind: string): Promise<void> {
	if (isProjectionHealed) return;
	isProjectionHealed = true;
	try {
		const healed = await askTheRecord<{ reconciled?: string[] }>('POST', '/api/agent/messenger-accounts-reconcile', {
			platform: messengerPlatform,
			kind
		});
		if (healed.reconciled?.length) {
			console.log(`messenger accounts reconciled for ${healed.reconciled.length} member(s)`);
		}
	} catch (thrown) {
		isProjectionHealed = false;
		console.log(`messenger account reconcile failed: ${String(thrown)}`);
	}
}

void messengerCredentialKind().catch(() => {});

async function askWhichCredentialTheMessengerNeeds(): Promise<string> {
	const answer = await dispatch.askChatd('person.credential.requirement', {});
	const named = (answer.body as { credentialKind?: unknown } | null)?.credentialKind;
	if (typeof named !== 'string' || named === '') {
		throw new Error(`${messengerPlatform} did not name the credential it needs`);
	}
	return named;
}

async function asset(capability: string, body: Record<string, unknown>): Promise<unknown> {
	if (capability === 'asset.link') return previewOf(String(body.url ?? ''));
	throw new Error(`the app has nothing called ${capability}`);
}

async function keptAlready(digest: string, contentType: string): Promise<KeptAttachment | null> {
	const kept = await attachmentAlreadyKept(client.storage.from(assetBucket), companyID, digest, contentType);
	if (!kept) return null;
	return { address: attachmentAddress(projectURL, kept.path), sizeBytes: kept.sizeBytes, digest };
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

async function keptFileBytes(kept: KeptFileReference): Promise<Uint8Array<ArrayBuffer>> {
	const objectPath = sharedAssetPath(companyID, attachmentKind, kept.digest, kept.contentType);
	const held = await client.storage.from(assetBucket).download(objectPath);
	if (held.error || !held.data) {
		throw new Error(`the asset store holds nothing at ${objectPath}: ${held.error?.message ?? 'no file'}`);
	}
	return new Uint8Array(await held.data.arrayBuffer());
}

const linkPreviews = new Map<string, LinkPreview | null>();

async function previewOf(link: string): Promise<LinkPreview | null> {
	if (!linkPreviews.has(link)) {
		linkPreviews.set(link, await readLinkPreview(link).catch(() => null));
	}
	return linkPreviews.get(link) ?? null;
}

async function tellThoseAddressed(arrived: ArrivedMessage): Promise<number> {
	if (arrived.recipientExternalIDs.length === 0) return 0;
	const spoken = await askTheProject<{ told?: number }>('notify', {
		platform: messengerPlatform,
		externalIDs: arrived.recipientExternalIDs,
		category: 'message',
		conversationID: arrived.conversationID,
		...tellingOf(arrived, await authorNameOf(arrived))
	});
	return spoken.told ?? 0;
}

async function authorNameOf(arrived: ArrivedMessage): Promise<string> {
	if (arrived.authorName) return arrived.authorName;
	return nameOf(arrived.authorExternalID);
}

// A member of the company is named by the member row; anyone else the company
// deals with is named by its address book.
async function nameOf(externalID: string): Promise<string> {
	const member = await client
		.from('member')
		.select('name')
		.eq('company_id', companyID)
		.eq(`messenger->>${messengerPlatform}`, externalID)
		.maybeSingle<{ name: string | null }>();
	if (member.error) throw new Error(member.error.message);
	if (member.data?.name) return member.data.name;

	const contact = await client
		.from('contact')
		.select('name')
		.eq('company_id', companyID)
		.eq(`messenger->>${messengerPlatform}`, externalID)
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

void sayWhereChatdWasLookedFor();

async function sayWhereChatdWasLookedFor(): Promise<void> {
	const answered = await fetch(chatdBaseURL)
		.then(() => true)
		.catch(() => false);
	console.log(
		answered ? `chatd answered at ${chatdBaseURL}` : `chatd did not answer at ${chatdBaseURL}`
	);
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

async function askTheProject<Value>(functionName: string, body: unknown): Promise<Value> {
	const response = await fetch(`${projectURL.replace(/\/+$/, '')}/functions/v1/${functionName}`, {
		method: 'POST',
		headers: { Authorization: `Bearer ${agentKey}`, 'Content-Type': 'application/json' },
		body: JSON.stringify(body)
	});
	if (!response.ok) throw new Error(`the project answered ${response.status} for ${functionName}`);
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

async function tellAdmindTheDirectoryChanged(): Promise<{ status: number; body: unknown }> {
	const response = await fetch(`${admindBaseURL}/admin/api/directory/changed`, { method: 'POST' });
	// The machine records a credential for whoever was just invited before it
	// answers, so the projection on the member rows is healed right behind it.
	isProjectionHealed = false;
	void messengerCredentialKind().catch(() => {});
	return { status: response.status, body: await answerBodyOf(response) };
}
