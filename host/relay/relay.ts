//   bun run host/relay/relay.ts

import { createClient } from '@supabase/supabase-js';
import { readLinkPreview, type LinkPreview } from './link-preview';
import {
	assetBucket,
	attachmentAddress,
	keepSharedAsset,
	personPictureKind,
	sharedAssetKeptAs
} from './asset-store';
import {
	prepareMedia,
	prepareWorkspaceFile,
	removeExpiredCopies,
	takeUploadIntoMessenger,
	takeUploadIntoWorkspace,
	writeKeptFileIntoWorkspace,
	type ActorCredential,
	type FileTransferDependencies,
	type KeptFileReference
} from './file-transfer';
import type { StoreAccess } from './transfer-store';
import { Transfers } from './transfers';
import { PersonPictures, type PictureRequest } from './person-picture';
import { defaultAnswerByteCeiling, largestRawBytesThatFit } from './answer-size';
import { positiveNumberSetting } from './settings';
import {
	answerBodyOf,
	askAdmind,
	defaultAdmindSocketPath,
	forwardToAdmind,
	forwardToAdmindAPI,
	forwardToChatd,
	workspaceCallOf,
	workspaceDownloadURL,
	workspaceFileURL,
	type AdmindCall,
	type ConnectedAccount,
	type PublicAPIRequest
} from './forward';
import { notifyRequestOf, readArrivedMessage, type ArrivedMessage } from './arrived';
import { connectToGateway, type GatewayConnection } from './gateway-socket';
import { MessengerRelay } from './messenger-calls';
import { messengerStoreOf } from './messenger-store';
import { CredentialCache } from './credential-cache';
import { BlueclawACPClient, defaultBlueclawACPSocketPath } from './acp-session';
import { RecordCatalogs, ticketOf } from './record-catalog';
import { displayNameForRequester, readInboundMessage } from './inbound-message';
import { InboundQueue } from './inbound-queue';
import { InboundTurns } from './inbound-turn';
import { conversationPoster } from './conversation-post';
import { HeldQuestionStore } from './held-question-store';
import { activeMemberIDsOf, arrivalsPath, keepWatchingArrivals } from './arrival-watchers';
import { readTyping, typingPath, typingTeller } from './typing';


const projectURL = required('SUPABASE_URL');
const publishableKey = required('SUPABASE_PUBLISHABLE_KEY');
await hostCredential();
const chatdBaseURL = process.env.CHATD_BASE_URL ?? 'http://127.0.0.1:18090';
const arrivalsPort = positiveNumberSetting('ARRIVALS_PORT', process.env.ARRIVALS_PORT, 18091);
const maildBaseURL = process.env.MAILD_BASE_URL ?? 'http://127.0.0.1:18092';
const messengerRelayURL = process.env.MESSENGER_RELAY_URL ?? 'ws://127.0.0.1:3000';
const admindBaseURL = process.env.ADMIND_BASE_URL ?? 'http://127.0.0.1:18080';
const admindSocketPath = process.env.ADMIND_SOCKET_PATH ?? defaultAdmindSocketPath;
const blueclawACPSocketPath = process.env.BLUECLAW_ACP_SOCKET_PATH ?? defaultBlueclawACPSocketPath;
const workspaceRootPath = process.env.WORKSPACE_ROOT_PATH ?? '/workspace';
const relayStateDirectory = process.env.RELAY_STATE_DIR ?? '/var/lib/internkim/relay';
const appURL = required('INTERNKIM_APP_URL');
const messengerPlatform = required('MESSENGER_PLATFORM');
const answerByteCeiling = positiveNumberSetting(
	'ANSWER_BYTE_CEILING',
	process.env.ANSWER_BYTE_CEILING,
	defaultAnswerByteCeiling
);
const largestPictureBytes = largestRawBytesThatFit(answerByteCeiling);
const transferCopiesKeptDays = 7;


function required(name: string): string {
	const value = process.env[name];
	if (!value) throw new Error(`set ${name}`);
	return value;
}

async function hostCredential(): Promise<string> {
	const given = process.env.AGENT_API_KEY?.trim();
	if (given) return given;
	const keptAt = process.env.AGENT_API_KEY_PATH?.trim();
	if (!keptAt) throw new Error('set AGENT_API_KEY or AGENT_API_KEY_PATH');
	const kept = (await Bun.file(keptAt).text()).trim();
	if (!kept) throw new Error(`${keptAt} holds no company computer credential`);
	return kept;
}

let hostSession = await askForHostSession();
const client = createClient(projectURL, publishableKey, {
	accessToken: async () => hostSession.accessToken
});
const companyID = hostSession.companyID;
console.log(`acting as the host of company ${companyID}`);

setInterval(() => void keepGoing('session', keepSessionFresh), 60_000);
setInterval(() => void keepGoing('expiring transfer copies', letExpiredCopiesGo), 60 * 60_000);


function openGatewayConnection(): GatewayConnection | null {
	const gatewayURL = process.env.GATEWAY_URL?.trim();
	const serverKey = process.env.GATEWAY_SERVER_KEY?.trim();
	if (!gatewayURL) return null;

	return connectToGateway({
		gatewayURL,
		companyID,
		...(serverKey ? { serverKey } : { hostAccessToken: freshHostAccessToken }),
		dispatch,
		byteCeiling: answerByteCeiling,
		messengerRelayURL
	});
}

async function freshHostAccessToken(): Promise<string> {
	await keepSessionFresh();
	return hostSession.accessToken;
}

let gateway: GatewayConnection | null = null;

function tellBrowsers(conversationID: string, messageID: string): void {
	if (!conversationID) return;
	gateway?.deliver({ kind: 'message.arrived', conversationID, messageID });
}

const credentials = new CredentialCache(readMessengerCredential);

async function readMessengerCredential(memberID: string): Promise<{ kind: string; secret: string } | null> {
	const kind = await messengerCredentialKind();
	const held = await askTheRecord<{ credential?: { kind: string; secret: string } | null }>(
		'GET',
		`/api/agent/messenger-credential?memberID=${encodeURIComponent(memberID)}&kind=${encodeURIComponent(kind)}`
	);
	if (!held.credential) return null;
	return { kind: held.credential.kind, secret: held.credential.secret };
}

const hostAccess: StoreAccess = { projectURL, apiKey: publishableKey, accessToken: freshHostAccessToken };

const memberSessions = new Map<string, Promise<{ accessToken: string; expiresAt: number }>>();

async function memberAccessToken(requesterEmail: string): Promise<string> {
	const held = await memberSessions.get(requesterEmail)?.catch(() => undefined);
	if (held && held.expiresAt - Math.floor(Date.now() / 1000) > 300) return held.accessToken;
	const asking = askForMemberSession(requesterEmail);
	memberSessions.set(requesterEmail, asking);
	return (await asking).accessToken;
}

const transfers = new Transfers((event, memberID) => gateway?.deliver(event, [memberID]));

const fileTransfer: FileTransferDependencies = {
	companyID,
	hostAccess,
	memberAccess: (requesterEmail) => ({
		projectURL,
		apiKey: publishableKey,
		accessToken: () => memberAccessToken(requesterEmail)
	}),
	transfers,
	readMediaRange: (actor, mediaURL, rangeHeader) => askChatdRaw('person.media.read', { actor, mediaURL, range: rangeHeader }),
	uploadMedia: (actor, sourceURL, contentType) => dispatch.askChatd('person.media.upload', { actor, sourceURL, contentType }),
	listWorkspaceDirectory: (requesterEmail, directoryPath) => {
		const call = workspaceCallOf('person.files.list', { path: directoryPath }, requesterEmail);
		if (!call) throw new Error('the relay lost its own route to a workspace listing');
		return forwardToAdmind(admindSocketPath, call);
	},
	readWorkspaceRange: (requesterEmail, path, rangeHeader) =>
		askAdmind(admindSocketPath, { method: 'GET', url: workspaceDownloadURL(path), requester: requesterEmail, range: rangeHeader }),
	writeWorkspaceFile: async (write, body) => {
		const response = await askAdmind(admindSocketPath, {
			method: 'PUT',
			url: workspaceFileURL(write.path),
			requester: write.requester,
			permission: write.permission,
			contentType: 'application/octet-stream',
			body
		});
		return { status: response.status, body: await answerBodyOf(response) };
	},
	report: (line) => console.log(`transfer: ${line}`)
};

function askChatdRaw(capability: string, body: Record<string, unknown>): Promise<Response> {
	const url = `${chatdBaseURL}/v1/platform/${encodeURIComponent(messengerPlatform)}/${encodeURIComponent(capability)}`;
	return fetch(url, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
}

const messenger = new MessengerRelay(messengerRelayURL.replace(/^ws/, 'http'), messengerStoreOf(companyID, hostAccess));

const dispatch = {
	messageArrived: tellBrowsers,
	serveAsset: asset,
	serveMessenger: (capability: string, body: Record<string, unknown>) => messenger.serve(capability, body),
	askChatd: (capability: string, body: Record<string, unknown>, largestBytes?: number) =>
		forwardToChatd(chatdBaseURL, messengerPlatform, capability, body, largestBytes ?? largestPictureBytes),
	transfer: {
		prepareMedia: (memberID: string, actor: ActorCredential, body: Record<string, unknown>) =>
			prepareMedia(fileTransfer, memberID, actor, body),
		takeUploadIntoMessenger: (memberID: string, actor: ActorCredential, requesterEmail: string, body: Record<string, unknown>) =>
			takeUploadIntoMessenger(fileTransfer, memberID, actor, requesterEmail, body),
		prepareWorkspaceFile: (memberID: string, requesterEmail: string, body: Record<string, unknown>) =>
			prepareWorkspaceFile(fileTransfer, memberID, requesterEmail, body),
		takeUploadIntoWorkspace: (memberID: string, requesterEmail: string, body: Record<string, unknown>) =>
			takeUploadIntoWorkspace(fileTransfer, memberID, requesterEmail, body),
		writeKeptFileIntoWorkspace: (kept: KeptFileReference, path: string) => writeKeptFileIntoWorkspace(fileTransfer, kept, path)
	},
	keptPersonPicture: async (request: PictureRequest) => {
		const path = await personPictures.keptPathOf(request);
		return path ? attachmentAddress(projectURL, path) : '';
	},
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
	messengerCredentialOf: (memberID: string) => credentials.credentialOf(memberID),
	connectMessengerAccount: async (memberID: string, account: ConnectedAccount) => {
		await askTheRecord('POST', '/api/agent/messenger-account', {
			platform: messengerPlatform,
			memberID,
			...account
		});
		credentials.forget(memberID);
	},
};

gateway = openGatewayConnection();

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

const linkPreviews = new Map<string, LinkPreview | null>();

async function previewOf(link: string): Promise<LinkPreview | null> {
	if (!linkPreviews.has(link)) {
		linkPreviews.set(link, await readLinkPreview(link).catch(() => null));
	}
	return linkPreviews.get(link) ?? null;
}

async function tellThoseAddressed(arrived: ArrivedMessage): Promise<number> {
	if (arrived.recipientExternalIDs.length === 0) return 0;
	const [authorName, senderPicturePath] = await Promise.all([
		authorNameOf(arrived),
		personPictures.pathForNotification(arrived.authorExternalID)
	]);
	const spoken = await askTheProject<{ told?: number }>(
		'notify',
		notifyRequestOf(arrived, authorName, messengerPlatform, senderPicturePath)
	);
	return spoken.told ?? 0;
}

const personPictures = new PersonPictures({
	keptAlready: (digest) => sharedAssetKeptAs(client.storage.from(assetBucket), companyID, personPictureKind, digest),
	askChatd: (capability, body) => dispatch.askChatd(capability, body),
	keep: async (bytes, contentType) =>
		(await keepSharedAsset(client.storage.from(assetBucket), companyID, personPictureKind, bytes, contentType)).path,
	memberIDOf,
	credentialOf: (memberID) => credentials.credentialOf(memberID),
	report: (line) => console.log(line)
});

async function memberIDOf(externalID: string): Promise<string | null> {
	const member = await client
		.from('member')
		.select('id')
		.eq('company_id', companyID)
		.eq(`messenger->>${messengerPlatform}`, externalID)
		.maybeSingle<{ id: string }>();
	if (member.error) throw new Error(member.error.message);
	return member.data?.id ?? null;
}

async function memberIDsOf(externalIDs: string[]): Promise<Map<string, string>> {
	const members = await client
		.from('member')
		.select(`id, externalID:messenger->>${messengerPlatform}`)
		.eq('company_id', companyID)
		.in(`messenger->>${messengerPlatform}`, externalIDs)
		.returns<{ id: string; externalID: string }[]>();
	if (members.error) throw new Error(members.error.message);
	return new Map(members.data.map((member) => [member.externalID, member.id]));
}

const tellTypingTo = typingTeller({
	memberIDsOf,
	deliver: (event, audienceMemberIDs) => gateway?.deliver(event, audienceMemberIDs),
	now: () => Date.now()
});

async function tellTyping(offered: unknown): Promise<Response> {
	const typing = readTyping(offered);
	if (!typing) return new Response('that is not someone typing', { status: 400 });
	const told = await tellTypingTo(typing).catch((error) => {
		console.error('typing not told:', error instanceof Error ? error.message : error);
		return 0;
	});
	return Response.json({ told });
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

const recordCatalogs = new RecordCatalogs({
	appURL,
	loopbackURL: `http://127.0.0.1:${arrivalsPort}`,
	mintFor: (requesterEmail) => askForMemberSession(requesterEmail),
	report: (line) => console.log(`catalog: ${line}`)
});
type InboundMemberName = {
	name: string | null;
	company: { locale: string | null } | null;
};

async function inboundBodyWithDisplayName(offered: unknown): Promise<unknown> {
	const inbound = readInboundMessage(offered);
	if (!inbound) return offered;
	const member = await client
		.from('member')
		.select('name, company (locale)')
		.eq('company_id', companyID)
		.eq('email', inbound.requester.email)
		.maybeSingle<InboundMemberName>();
	if (member.error) throw new Error(member.error.message);
	if (!member.data?.name) return offered;
	const displayName = displayNameForRequester(
		member.data.name,
		member.data.company?.locale ?? '',
		inbound.addressing.responseLanguage ?? ''
	);
	if (!isRecord(offered)) return offered;
	const context = offered.context;
	if (!isRecord(context) || !isRecord(context.sender)) return offered;
	const sender = context.sender;
	return {
		...offered,
		context: { ...context, sender: { ...sender, name: displayName } }
	};
}

function isRecord(offered: unknown): offered is Record<string, unknown> {
	return typeof offered === 'object' && offered !== null;
}

const inboundTurns: InboundTurns = new InboundTurns({
	client: new BlueclawACPClient({
		socketPath: blueclawACPSocketPath,
		workspaceRootPath,
		catalogFor: (requesterEmail, conversationID) =>
			recordCatalogs.serversFor(requesterEmail, conversationID),
		questions: new HeldQuestionStore({
			directoryPath: `${relayStateDirectory}/questions`,
			report: (line) => console.log(`questions: ${line}`)
		}),
		askThePerson: (asked, addressing) => inboundTurns.askThePerson(asked, addressing),
		awaitAnAlreadyAskedQuestion: (addressing) => inboundTurns.awaitAnAlreadyAskedQuestion(addressing),
		report: (line) => console.log(`acp: ${line}`)
	}),
	queue: new InboundQueue({
		directoryPath: `${relayStateDirectory}/inbound`,
		report: (line) => console.log(`inbound: ${line}`)
	}),
	postToConversation: conversationPoster({
		askChatd: (capability, body) => dispatch.askChatd(capability, body),
		tellBrowsers,
		report: (line) => console.log(`reply: ${line}`)
	}),
	report: (line) => console.log(`acp: ${line}`)
});

async function keepInboundMessage(offered: unknown): Promise<Response> {
	const localizedOffered = await inboundBodyWithDisplayName(offered);
	const inbound = readInboundMessage(localizedOffered);
	if (!inbound) return new Response('that is not a message the agent can answer', { status: 400 });
	const isNew = await inboundTurns.keep(inbound.key, localizedOffered);
	tellBrowsers(inbound.addressing.conversationID, inbound.messageID);
	return Response.json({ queued: isNew, key: inbound.key }, { status: 202 });
}

// Whatever a stopped relay had asked and not yet delivered is on disk; put it
// back in play before /inbound answers anything.
await inboundTurns.restoreHeldQuestions();

Bun.serve({
	hostname: '127.0.0.1',
	port: arrivalsPort,
	fetch: async (request) => {
		if (request.method !== 'POST') return new Response('post an arrival', { status: 405 });
		const pathname = new URL(request.url).pathname;
		const catalogTicket = ticketOf(pathname);
		if (catalogTicket) return recordCatalogs.serve(request, catalogTicket);
		const offered = await request.json().catch(() => null);
		if (pathname === '/inbound') return keepInboundMessage(offered);
		if (pathname === typingPath) return tellTyping(offered);
		const arrived = readArrivedMessage(offered);
		if (!arrived) return new Response('that is not a message', { status: 400 });
		const told = await tellThoseAddressed(arrived).catch((error) => {
			console.error('arrival not told:', error instanceof Error ? error.message : error);
			return 0;
		});
		return Response.json({ told });
	}
});
console.log(`arrivals accepted on 127.0.0.1:${arrivalsPort}`);

keepWatchingArrivals({
	activeMemberIDs: () => activeMemberIDsOf(client, companyID),
	credentialOf: (memberID) => credentials.credentialOf(memberID),
	askChatd: (capability, body) => dispatch.askChatd(capability, body),
	arrivalsURL: `http://127.0.0.1:${arrivalsPort}${arrivalsPath}`,
	typingURL: `http://127.0.0.1:${arrivalsPort}${typingPath}`,
	report: (line) => console.log(line)
});

// Whatever the last relay took and had not delivered is still on disk.
inboundTurns.startDraining();

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
			Authorization: `Bearer ${await hostCredential()}`,
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
		headers: { Authorization: `Bearer ${await hostCredential()}`, 'Content-Type': 'application/json' },
		body: JSON.stringify(body)
	});
	if (!response.ok) throw new Error(`the project answered ${response.status} for ${functionName}`);
	return (await response.json()) as Value;
}

async function askForMemberSession(
	requesterEmail: string
): Promise<{ accessToken: string; expiresAt: number }> {
	const response = await fetch(`${appURL}/api/agent/session`, {
		method: 'POST',
		headers: { Authorization: `Bearer ${await hostCredential()}`, 'Content-Type': 'application/json' },
		body: JSON.stringify({ kind: 'email', externalID: requesterEmail })
	});
	if (!response.ok) {
		throw new Error(`the central plane would not sign in ${requesterEmail} (${response.status})`);
	}
	return (await response.json()) as { accessToken: string; expiresAt: number };
}

async function askForHostSession(): Promise<{ companyID: string; accessToken: string; expiresAt: number }> {
	const response = await fetch(`${appURL}/api/agent/host-session`, {
		method: 'POST',
		headers: { Authorization: `Bearer ${await hostCredential()}` }
	});
	if (!response.ok) throw new Error(`the central plane refused this company computer credential (${response.status})`);
	return (await response.json()) as { companyID: string; accessToken: string; expiresAt: number };
}

async function letExpiredCopiesGo(): Promise<void> {
	const expired = await client.rpc('expired_transfer_copies', { kept_days: transferCopiesKeptDays });
	if (expired.error) throw new Error(expired.error.message);
	const paths = (expired.data ?? []).filter((path: unknown): path is string => typeof path === 'string');
	const removed = await removeExpiredCopies(fileTransfer, paths);
	if (removed > 0) console.log(`let ${removed} transfer copies older than ${transferCopiesKeptDays} days go`);
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
	credentials.forgetEveryone();

	void keepGoing('healing the messenger projection after the directory changed', async () => {
		await messengerCredentialKind();
	});
	return { status: response.status, body: await answerBodyOf(response) };
}
