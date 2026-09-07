import { extensionOf } from './asset-store';

export type Call = {
	callID?: string;
	capability?: string;
	body?: Record<string, unknown>;
};

export type Answer = { callID: string; status: number; body: unknown };

const personPrefix = 'person.';
// Somebody is invited on the company and given a Linux user here, so this
// device has to hear about it rather than wait for its own timer to come round.
export const directoryChangedCapability = 'directory.changed';
const sendCapability = 'person.message.send';
const readCapability = 'person.message.attachment';
const refusedStatus = 415;
const registrationPrefix = 'person.credential.';
const issueCapability = 'person.credential.issue';
const mailPrefix = 'person.mail.';
const workspacePrefixes = ['person.memory.', 'person.files.', 'person.runs.', 'person.buzz.', 'person.task.', 'person.skills.', 'person.persona.', 'person.agent_learning.'];
export const apiRequestCapability = 'person.api.request';
export const apiFileCapability = 'person.api.file';
export const tellCapability = 'person.message.tell';
export const workspaceRootsCapability = 'person.files.roots';
export const defaultAdmindSocketPath = '/run/internkim/admind.sock';
const requesterEmailHeader = 'X-INTERNKIM-REQUESTER-EMAIL';
const requesterPermissionHeader = 'X-INTERNKIM-REQUESTER-PERMISSION';
// Bun's fetch refuses a body on OPTIONS as well as on GET and HEAD.
const methodsThatCarryNoBody = new Set(['GET', 'HEAD', 'OPTIONS']);
const admindHost = 'http://internkim';
const workspaceUploadPath = '/files/api/upload';
const tellPath = '/tell/api/direct-message';

export function mailOperationOf(capability: string): string | null {
	if (!capability.startsWith(mailPrefix)) return null;
	const operation = capability.slice(mailPrefix.length);
	return /^[a-z]+$/.test(operation) ? operation : null;
}

export function isPersonCapability(capability: string): boolean {
	return capability.startsWith(personPrefix);
}

export function isRegistrationCapability(capability: string): boolean {
	return capability.startsWith(registrationPrefix);
}

export function isWorkspaceCapability(capability: string): boolean {
	return workspacePrefixes.some((prefix) => capability.startsWith(prefix));
}

export type PublicAPIRequest = {
	method: string;
	path: string;
	query: string;
	permission: string;
	requester: string;
	payload: unknown;
};

export function publicAPIRequestOf(body: Record<string, unknown>): PublicAPIRequest | null {
	const method = typeof body.method === 'string' ? body.method.trim().toUpperCase() : '';
	const path = typeof body.path === 'string' ? body.path : '';
	const requester = oneHeaderLine(body.requester);
	const permission = oneHeaderLine(body.permission);
	if (!method || !requester || !permission) return null;
	if (!path.startsWith('/')) return null;
	return { method, path, query: queryOf(body.query), permission, requester, payload: body.payload };
}

function oneHeaderLine(given: unknown): string {
	if (typeof given !== 'string') return '';
	const written = given.trim();
	return written.includes('\r') || written.includes('\n') || written.includes('\0') ? '' : written;
}

function queryOf(given: unknown): string {
	if (typeof given !== 'string' || given === '') return '';
	return given.startsWith('?') ? given : `?${given}`;
}

export function admindAPIURL(request: PublicAPIRequest): string {
	return `${admindHost}/api/v1${request.path}${request.query}`;
}

export function workspaceUploadURL(directoryPath: string): string {
	return `${admindHost}${workspaceUploadPath}?path=${encodeURIComponent(directoryPath)}`;
}

export type AdmindCall = {
	method: string;
	url: string;
	requester: string;
	permission?: string;
	contentType?: string;
	body?: FormData | string;
};

export async function forwardToAdmind(
	socketPath: string,
	call: AdmindCall
): Promise<{ status: number; body: unknown }> {
	const response = await fetch(call.url, {
		unix: socketPath,
		method: call.method,
		headers: {
			[requesterEmailHeader]: call.requester,
			...(call.permission ? { [requesterPermissionHeader]: call.permission } : {}),
			...(call.contentType ? { 'Content-Type': call.contentType } : {})
		},
		body: call.body
	}).catch((unreachable) => {
		const reason = unreachable instanceof Error ? unreachable.message : String(unreachable);
		throw new Error(`admind did not answer on ${socketPath}: ${reason}`);
	});
	return { status: response.status, body: await answerBodyOf(response) };
}

export function forwardToAdmindAPI(
	socketPath: string,
	request: PublicAPIRequest
): Promise<{ status: number; body: unknown }> {
	const carriesPayload = !methodsThatCarryNoBody.has(request.method) && request.payload !== undefined;
	return forwardToAdmind(socketPath, {
		method: request.method,
		url: admindAPIURL(request),
		requester: request.requester,
		permission: request.permission,
		contentType: carriesPayload ? 'application/json' : '',
		body: carriesPayload ? JSON.stringify(request.payload) : undefined
	});
}

export async function answerBodyOf(response: Response): Promise<unknown> {
	const text = await response.text();
	if (!text.trim()) return null;
	try {
		return JSON.parse(text);
	} catch {
		return response.ok ? text : { error: text.trim() };
	}
}

export async function forwardToChatd(
	chatdBaseURL: string,
	platform: string,
	capability: string,
	body: Record<string, unknown>,
	largestBytes: number
): Promise<{ status: number; body: unknown }> {
	const url = `${chatdBaseURL}/v1/platform/${encodeURIComponent(platform)}/${encodeURIComponent(capability)}`;
	const response = await fetch(url, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ ...body, largestBytes })
	}).catch((unreachable) => {
		const reason = unreachable instanceof Error ? unreachable.message : String(unreachable);
		throw new Error(`${chatdBaseURL} did not answer: ${reason}`);
	});
	return { status: response.status, body: await response.json().catch(() => null) };
}

export type Served = { status: number; body: unknown; replyTo: string | null };

export type Dispatch = {
	messageArrived: (conversationID: string, messageID: string) => void;
	serveAsset: (capability: string, body: Record<string, unknown>) => Promise<unknown>;
	askAdmindAPI: (request: PublicAPIRequest) => Promise<{ status: number; body: unknown }>;
	emailOfMember: (memberID: string) => Promise<string | null>;
	tellAdmindTheDirectoryChanged: () => Promise<{ status: number; body: unknown }>;
	askChatd: (
		capability: string,
		body: Record<string, unknown>,
		largestBytes?: number
	) => Promise<{ status: number; body: unknown }>;
	keepAttachment: (contentBase64: string, contentType: string) => Promise<KeptAttachment>;
	keptAlready: (digest: string, contentType: string) => Promise<KeptAttachment | null>;
	keptFileBytes: (kept: KeptFileReference) => Promise<Uint8Array<ArrayBuffer>>;
	askAdmindAsRequester: (call: AdmindCall) => Promise<{ status: number; body: unknown }>;
	largestFileBytes: number;
	askMaild: (operation: string, body: Record<string, unknown>) => Promise<{ status: number; body: unknown }>;
	mailAccountOf: (memberID: string) => Promise<Record<string, unknown> | null>;
	connectMessengerAccount: (memberID: string, account: ConnectedAccount) => Promise<void>;
	messengerCredentialOf: (memberID: string) => Promise<ActorCredential | null>;
};

export type ConnectedAccount = { kind: string; externalID: string; name: string; secret: string };

export type ActorCredential = { kind: string; secret: string };

export type SentAttachment = { filename: string; contentType: string; contentBase64: string };

export type KeptAttachment = { address: string; sizeBytes: number; digest: string };

export type KeptFileReference = {
	requester: string;
	permission: string;
	digest: string;
	contentType: string;
	filename: string;
};

type ReadFile = { filename: string; contentType: string; contentBase64: string };

export async function serveCallForMember(
	dispatch: Dispatch,
	call: Call,
	memberID: string
): Promise<Served> {
	const capability = call.capability ?? '';
	const body = call.body ?? {};

	if (isRegistrationCapability(capability)) {
		return { ...(await serveRegistration(dispatch, capability, body, memberID)), replyTo: memberID };
	}
	return serveForMember(dispatch, capability, body, memberID);
}

async function serveForMember(
	dispatch: Dispatch,
	capability: string,
	body: Record<string, unknown>,
	replyTo: string
): Promise<Served> {
	if (capability === directoryChangedCapability) {
		return { ...(await dispatch.tellAdmindTheDirectoryChanged()), replyTo };
	}

	if (!isPersonCapability(capability)) {
		return { status: 200, body: await dispatch.serveAsset(capability, body), replyTo };
	}

	const operation = mailOperationOf(capability);
	if (operation) return { ...(await serveMail(dispatch, operation, body, replyTo)), replyTo };

	if (isWorkspaceCapability(capability)) {
		return { ...(await serveWorkspace(dispatch, capability, body, replyTo)), replyTo };
	}

	// Acting as a person on their messenger means holding their credential.
	// The company's own server resolves it; the browser used to carry it, and
	// that is the whole reason it had to be handed one in the clear.
	const actor = await dispatch.messengerCredentialOf(replyTo);
	if (!actor) {
		return { status: 409, body: { error: 'this member has connected no messenger account' }, replyTo };
	}
	if (capability === sendCapability) {
		const sent = await sendKeepingWhatIsRefused(dispatch, body, actor);
		if (sent.status < 300) dispatch.messageArrived(String(body.conversationID ?? ''), messageIDOf(sent.body));
		return { ...sent, replyTo };
	}
	if (capability === readCapability) {
		return { ...(await keptForReading(dispatch, body, actor)), replyTo };
	}
	return { ...(await dispatch.askChatd(capability, { ...body, actor })), replyTo };
}

// The messenger stores a file on this machine, and the browser asking for it is
// somewhere else entirely. So the answer is never the file: it is an address in
// the company's own bucket, which the reader signs for with their own session.
// The bucket is addressed by content, so a file already kept is answered for
// without the messenger being asked for a single byte.
async function keptForReading(
	dispatch: Dispatch,
	body: Record<string, unknown>,
	actor: ActorCredential
): Promise<{ status: number; body: unknown }> {
	const named = describedFile(body);
	const already = named.digest ? await dispatch.keptAlready(named.digest, named.contentType) : null;
	if (already) return { status: 200, body: { attachment: { ...already, ...named } } };

	const answer = await dispatch.askChatd(readCapability, { ...body, actor }, dispatch.largestFileBytes);
	const read = fileOf(answer);
	if (!read) return { status: answer.status, body: { attachment: null } };

	const kept = await dispatch.keepAttachment(read.contentBase64, read.contentType);
	return {
		status: 200,
		body: { attachment: { ...kept, filename: read.filename, contentType: read.contentType } }
	};
}

function describedFile(body: Record<string, unknown>): { filename: string; contentType: string; digest: string } {
	return {
		filename: typeof body.filename === 'string' ? body.filename : '',
		contentType: typeof body.contentType === 'string' ? body.contentType : '',
		digest: typeof body.digest === 'string' ? body.digest.trim() : ''
	};
}

function fileOf(answer: { status: number; body: unknown }): ReadFile | null {
	if (answer.status !== 200) return null;
	const file = (answer.body as { file?: unknown } | null)?.file as Partial<ReadFile> | null | undefined;
	if (!file || typeof file.contentBase64 !== 'string' || file.contentBase64 === '') return null;
	return {
		filename: typeof file.filename === 'string' ? file.filename : '',
		contentType: typeof file.contentType === 'string' ? file.contentType : 'application/octet-stream',
		contentBase64: file.contentBase64
	};
}

// The messenger's own store takes most files and refuses some by type. One it
// refuses still belongs to the conversation, so it is kept where the company
// can read it and the message is sent again naming it there.
async function sendKeepingWhatIsRefused(
	dispatch: Dispatch,
	body: Record<string, unknown>,
	actor: ActorCredential
): Promise<{ status: number; body: unknown }> {
	const answer = await dispatch.askChatd(sendCapability, { ...body, actor });
	const refused = refusedAttachmentsOf(answer);
	if (refused.length === 0) return answer;

	const attachments = await keptInsteadOfUploaded(dispatch, sentAttachmentsOf(body), refused);
	return dispatch.askChatd(sendCapability, { ...body, attachments, actor });
}

function messageIDOf(body: unknown): string {
	const id = (body as { id?: unknown } | null)?.id;
	return typeof id === 'string' ? id : '';
}

function refusedAttachmentsOf(answer: { status: number; body: unknown }): number[] {

	if (answer.status !== refusedStatus) return [];
	const refused = (answer.body as { refusedAttachments?: unknown } | null)?.refusedAttachments;
	if (!Array.isArray(refused)) return [];
	return refused
		.map((one) => (one as { index?: unknown }).index)
		.filter((index): index is number => typeof index === 'number');
}

function sentAttachmentsOf(body: Record<string, unknown>): SentAttachment[] {
	const sent = body.attachments;
	return Array.isArray(sent) ? (sent as SentAttachment[]) : [];
}

async function keptInsteadOfUploaded(
	dispatch: Dispatch,
	attachments: SentAttachment[],
	refused: number[]
): Promise<(SentAttachment | KeptAttachment)[]> {
	const toKeep = new Set(refused);
	return Promise.all(
		attachments.map(async (attachment, index) => {
			if (!toKeep.has(index)) return attachment;
			const kept = await dispatch.keepAttachment(attachment.contentBase64, attachment.contentType);
			return { filename: attachment.filename, contentType: attachment.contentType, ...kept };
		})
	);
}

async function serveRegistration(
	dispatch: Dispatch,
	capability: string,
	body: Record<string, unknown>,
	memberID: string
): Promise<{ status: number; body: unknown }> {
	const answer = await dispatch.askChatd(capability, body);
	if (answer.status >= 300) return answer;
	if (capability !== issueCapability) return answer;

	const issued = issuedAccountOf(answer.body);
	if (!issued) return { status: 502, body: { error: 'the messenger issued nothing usable' } };
	await dispatch.connectMessengerAccount(memberID, issued);
	return { status: 200, body: { externalID: issued.externalID, name: issued.name } };
}

function issuedAccountOf(body: unknown): ConnectedAccount | null {
	const issued = body as
		| {
				credential?: { kind?: unknown; secret?: unknown };
				identity?: { externalID?: unknown; name?: unknown };
		  }
		| null;
	const kind = issued?.credential?.kind;
	const secret = issued?.credential?.secret;
	const externalID = issued?.identity?.externalID;
	if (typeof kind !== 'string' || kind === '') return null;
	if (typeof secret !== 'string' || typeof externalID !== 'string') return null;
	const name = issued?.identity?.name;
	return { kind, externalID, name: typeof name === 'string' ? name : '', secret };
}

async function serveMail(
	dispatch: Dispatch,
	operation: string,
	body: Record<string, unknown>,
	memberID: string
): Promise<{ status: number; body: unknown }> {
	const account = await dispatch.mailAccountOf(memberID);
	if (!account) {
		return { status: 409, body: { error: 'this member has connected no mail account' } };
	}
	return dispatch.askMaild(operation, { ...body, account });
}

export function tellCallOf(body: Record<string, unknown>): AdmindCall | null {
	const recipientEmail = oneHeaderLine(body.recipientEmail).toLowerCase();
	const message = typeof body.message === 'string' ? body.message.trim() : '';
	if (!recipientEmail || !message) return null;
	return {
		method: 'POST',
		url: `${admindHost}${tellPath}`,
		requester: recipientEmail,
		contentType: 'application/json',
		body: JSON.stringify({ recipientEmail, message })
	};
}

export async function serveTelling(
	dispatch: Dispatch,
	body: Record<string, unknown>
): Promise<{ status: number; body: unknown }> {
	const call = tellCallOf(body);
	if (!call) {
		return { status: 400, body: { error: 'a telling names a recipient and carries a message' } };
	}
	return dispatch.askAdmindAsRequester(call);
}

export async function servePublicAPIRequest(
	dispatch: Dispatch,
	body: Record<string, unknown>
): Promise<{ status: number; body: unknown }> {
	const request = publicAPIRequestOf(body);
	if (!request) {
		return { status: 400, body: { error: 'that is not a public API request the plane resolved' } };
	}
	return dispatch.askAdmindAPI(request);
}

const apiInboxDirectory = 'inbox/api';
const refusedLeafNames = new Set(['', '.', '..', '.blueclaw']);

export function publicAPIFileOf(body: Record<string, unknown>): KeptFileReference | null {
	const requester = oneHeaderLine(body.requester);
	const permission = oneHeaderLine(body.permission);
	const digest = typeof body.digest === 'string' ? body.digest.trim().toLowerCase() : '';
	if (!requester || !permission || !/^[0-9a-f]{64}$/.test(digest)) return null;
	const contentType = typeof body.contentType === 'string' ? body.contentType.trim() : '';
	const filename = typeof body.filename === 'string' ? body.filename : '';
	return {
		requester,
		permission,
		digest,
		contentType: contentType || 'application/octet-stream',
		filename
	};
}

export function leafNameOf(offered: string, fallback: string): string {
	const named = offered.trim().replaceAll('\\', '/').split('/').pop()?.trim() ?? '';
	return refusedLeafNames.has(named) ? fallback : named;
}

export async function servePublicAPIFile(
	dispatch: Dispatch,
	body: Record<string, unknown>
): Promise<{ status: number; body: unknown }> {
	const kept = publicAPIFileOf(body);
	if (!kept) return { status: 400, body: { error: 'that is not a file the plane kept' } };

	const home = await personalHomeOf(dispatch, kept);
	if (!home) return { status: 409, body: { error: 'this address has no home in the workspace' } };

	const directoryPath = `${home}/${apiInboxDirectory}`;
	const bytes = await dispatch.keptFileBytes(kept);
	const filename = leafNameOf(kept.filename, kept.digest + extensionOf(kept.contentType));
	const uploaded = await dispatch.askAdmindAsRequester({
		method: 'POST',
		url: workspaceUploadURL(directoryPath),
		requester: kept.requester,
		permission: kept.permission,
		body: onePartUpload(filename, kept.contentType, bytes)
	});
	if (uploaded.status !== 200) return uploaded;

	const written = writtenNameOf(uploaded.body);
	if (!written) {
		return { status: 502, body: { error: 'the workspace took the call and wrote no file' } };
	}
	return {
		status: 200,
		body: {
			file: {
				path: `${directoryPath}/${written}`,
				sizeBytes: bytes.byteLength,
				digest: kept.digest,
				contentType: kept.contentType
			}
		}
	};
}

function onePartUpload(filename: string, contentType: string, bytes: Uint8Array<ArrayBuffer>): FormData {
	const upload = new FormData();
	upload.append('file', new Blob([bytes], { type: contentType }), filename);
	return upload;
}

function writtenNameOf(body: unknown): string {
	const uploaded = (body as { uploaded?: unknown } | null)?.uploaded;
	if (!Array.isArray(uploaded)) return '';
	const written = uploaded[0];
	return typeof written === 'string' ? written : '';
}

async function personalHomeOf(dispatch: Dispatch, kept: KeptFileReference): Promise<string | null> {
	const call = workspaceCallOf(workspaceRootsCapability, {}, kept.requester);
	if (!call) return null;
	const answer = await dispatch.askAdmindAsRequester({ ...call, permission: kept.permission });
	if (answer.status !== 200) return null;
	const roots = (answer.body as { roots?: unknown } | null)?.roots;
	if (!Array.isArray(roots)) return null;
	const home = roots.find((root) => (root as { kind?: unknown }).kind === 'personal') as
		| { agentPath?: unknown }
		| undefined;
	const agentPath = typeof home?.agentPath === 'string' ? home.agentPath.replace(/\/+$/, '') : '';
	return agentPath.startsWith('/workspace/') ? agentPath : null;
}

async function serveWorkspace(
	dispatch: Dispatch,
	capability: string,
	body: Record<string, unknown>,
	memberID: string
): Promise<{ status: number; body: unknown }> {
	const requesterEmail = await dispatch.emailOfMember(memberID);
	if (!requesterEmail) {
		return { status: 409, body: { error: 'this member has no address the workspace knows' } };
	}
	const call = workspaceCallOf(capability, body, requesterEmail);
	if (!call) return { status: 404, body: { error: `the app has nothing called ${capability}` } };
	return dispatch.askAdmindAsRequester(call);
}

export const workspaceCapabilityPaths: Record<string, string> = {
	'person.memory.graph': '/memory/api/graph',
	'person.memory.schedules': '/memory/api/schedules',
	'person.skills.list': '/skills/api',
	[workspaceRootsCapability]: '/files/api/roots',
	'person.files.list': '/files/api/list',
	'person.runs.list': '/runs/api',
	'person.runs.detail': '/runs/api/detail',
	'person.buzz.claim': '/agent/api/buzz-claim',
	'person.buzz.relay': '/agent/api/buzz-relay-config',
	'person.persona.user': '/persona/api/user',
	'person.persona.identity': '/persona/api/identity',
	'person.agent_learning.skills.list': '/agent-learning/api/skills?includeRetired=true',
	'person.agent_learning.skills.get': '/agent-learning/api/skills',
	'person.agent_learning.settings.get': '/agent-learning/api/settings',
	'person.agent_learning.soul.get': '/agent-learning/api/soul',
	'person.agent_learning.soul.history': '/agent-learning/api/soul/history',
	'person.persona.soul': '/persona/api/soul'
};

export const workspaceWriteCapabilityPaths: Record<string, string> = {
	'person.task.quick_task': '/task/api/tasks/quick',
	'person.runs.approve': '/runs/api/approve',
	'person.memory.episode_delete': '/memory/api/episodes/delete',
	'person.memory.fact_update': '/memory/api/facts/update',
	'person.memory.fact_delete': '/memory/api/facts/delete',
	'person.memory.pinned_update': '/memory/api/pinned/update',
	'person.memory.pinned_delete': '/memory/api/pinned/delete',
	'person.memory.schedule_cancel': '/memory/api/schedules/cancel',
	'person.memory.schedule_delete': '/memory/api/schedules/delete',
	'person.memory.schedule_update': '/memory/api/schedules/update',
	'person.persona.user.update': '/persona/api/user',
	'person.persona.identity.update': '/persona/api/identity',
	'person.agent_learning.settings.update': '/agent-learning/api/settings',
	'person.agent_learning.skills.action': '/agent-learning/api/skills/action'
};

export function workspaceCallOf(
	capability: string,
	body: Record<string, unknown>,
	requester: string
): AdmindCall | null {
	const writePath = workspaceWriteCapabilityPaths[capability];
	if (writePath) {
		const { actor: _actor, ...written } = body;
		const actionPath = capability === 'person.agent_learning.skills.action' && (body.action === 'protect' || body.action === 'retire' || body.action === 'restore') ? `/agent-learning/api/skills/${body.action}` : writePath;
		return {
			method: 'POST',
			url: `${admindHost}${actionPath}`,
			requester,
			contentType: 'application/json',
			body: JSON.stringify(written)
		};
	}
	const path = workspaceCapabilityPaths[capability];
	if (!path) return null;
	const query = new URLSearchParams();
	for (const [name, value] of Object.entries(body)) {
		if (name === 'actor' || value === undefined || value === null) continue;
		query.set(name, String(value));
	}
	const asked = query.toString() ? `${path}?${query}` : path;
	return { method: 'GET', url: `${admindHost}${asked}`, requester };
}
