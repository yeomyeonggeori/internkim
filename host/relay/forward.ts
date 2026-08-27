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
const workspacePrefixes = ['person.memory.', 'person.files.', 'person.runs.', 'person.buzz.'];
export const apiRequestCapability = 'person.api.request';
export const defaultAdmindSocketPath = '/run/internkim/admind.sock';
const requesterEmailHeader = 'X-INTERNKIM-REQUESTER-EMAIL';
const requesterPermissionHeader = 'X-INTERNKIM-REQUESTER-PERMISSION';
// Bun's fetch refuses a body on OPTIONS as well as on GET and HEAD.
const methodsThatCarryNoBody = new Set(['GET', 'HEAD', 'OPTIONS']);

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
	return `http://internkim/api/v1${request.path}${request.query}`;
}

export async function forwardToAdmindAPI(
	socketPath: string,
	request: PublicAPIRequest
): Promise<{ status: number; body: unknown }> {
	const carriesPayload = !methodsThatCarryNoBody.has(request.method) && request.payload !== undefined;
	const response = await fetch(admindAPIURL(request), {
		unix: socketPath,
		method: request.method,
		headers: {
			[requesterEmailHeader]: request.requester,
			[requesterPermissionHeader]: request.permission,
			...(carriesPayload ? { 'Content-Type': 'application/json' } : {})
		},
		body: carriesPayload ? JSON.stringify(request.payload) : undefined
	}).catch((unreachable) => {
		const reason = unreachable instanceof Error ? unreachable.message : String(unreachable);
		throw new Error(`admind did not answer on ${socketPath}: ${reason}`);
	});
	return { status: response.status, body: await answerBodyOf(response) };
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
	serveAsset: (capability: string, body: Record<string, unknown>) => Promise<unknown>;
	askAdmind: (
		capability: string,
		body: Record<string, unknown>,
		requesterEmail: string
	) => Promise<{ status: number; body: unknown }>;
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
		return { ...(await sendKeepingWhatIsRefused(dispatch, body, actor)), replyTo };
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
	return dispatch.askAdmind(capability, body, requesterEmail);
}
