export type ActorCredential = { kind: string; secret: string };

export type Call = {
	callID?: string;
	capability?: string;
	replyTo?: string;
	body?: Record<string, unknown>;
};

export type Answer = { callID: string; status: number; body: unknown };

const personPrefix = 'person.';
const registrationPrefix = 'person.credential.';
const issueCapability = 'person.credential.issue';
const mailPrefix = 'person.mail.';
const workspacePrefixes = ['person.memory.', 'person.files.', 'person.tasks.'];

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

export async function answerBodyOf(response: Response): Promise<unknown> {
	const text = await response.text();
	if (!text.trim()) return null;
	try {
		return JSON.parse(text);
	} catch {
		return response.ok ? text : { error: text.trim() };
	}
}

export function reportableTopic(call: Call): string | null {
	const offered = call.replyTo;
	if (typeof offered !== 'string') return null;
	return /^[0-9a-f-]{36}$/i.test(offered) ? offered : null;
}

export function actorOf(call: Call): ActorCredential | null {
	const offered = call.body?.actor;
	if (typeof offered !== 'object' || offered === null) return null;
	const { kind, secret } = offered as { kind?: unknown; secret?: unknown };
	if (typeof kind !== 'string' || typeof secret !== 'string') return null;
	if (!kind.trim() || !secret.trim()) return null;
	return { kind, secret };
}

export async function forwardToChatd(
	chatdBaseURL: string,
	platform: string,
	capability: string,
	body: Record<string, unknown>,
	largestBytes: number
): Promise<{ status: number; body: unknown }> {
	const response = await fetch(
		`${chatdBaseURL}/v1/platform/${encodeURIComponent(platform)}/${encodeURIComponent(capability)}`,
		{
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ ...body, largestBytes })
		}
	);
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
	emailOfMember: (memberID: string) => Promise<string | null>;
	askChatd: (capability: string, body: Record<string, unknown>) => Promise<{ status: number; body: unknown }>;
	askMaild: (operation: string, body: Record<string, unknown>) => Promise<{ status: number; body: unknown }>;
	memberOfExternalID: (externalID: string) => Promise<string | null>;
	mailAccountOf: (memberID: string) => Promise<Record<string, unknown> | null>;
	connectMessengerAccount: (memberID: string, account: ConnectedAccount) => Promise<void>;
};

export type ConnectedAccount = { externalID: string; name: string; secret: string };

export async function serveCall(
	dispatch: Dispatch,
	call: Call,
	channelMemberID?: string
): Promise<Served> {
	const capability = call.capability ?? '';
	const body = call.body ?? {};

	if (isRegistrationCapability(capability)) {
		if (!channelMemberID) {
			return { status: 403, body: { error: 'this call arrived on nobody\'s channel' }, replyTo: null };
		}
		return {
			...(await serveRegistration(dispatch, capability, body, channelMemberID)),
			replyTo: channelMemberID
		};
	}

	const actor = actorOf(call);
	if (!actor) {
		return { status: 400, body: { error: 'this call named no actor' }, replyTo: null };
	}

	const replyTo = await memberHolding(dispatch, actor);
	if (!replyTo) {
		return { status: 403, body: { error: 'that credential belongs to nobody here' }, replyTo: null };
	}

	if (!isPersonCapability(capability)) {
		return { status: 200, body: await dispatch.serveAsset(capability, body), replyTo };
	}

	const operation = mailOperationOf(capability);
	if (operation) return { ...(await serveMail(dispatch, operation, body, replyTo)), replyTo };

	if (isWorkspaceCapability(capability)) {
		return { ...(await serveWorkspace(dispatch, capability, body, replyTo)), replyTo };
	}

	return { ...(await dispatch.askChatd(capability, body)), replyTo };
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
		| { credential?: { secret?: unknown }; identity?: { externalID?: unknown; name?: unknown } }
		| null;
	const secret = issued?.credential?.secret;
	const externalID = issued?.identity?.externalID;
	if (typeof secret !== 'string' || typeof externalID !== 'string') return null;
	const name = issued?.identity?.name;
	return { externalID, name: typeof name === 'string' ? name : '', secret };
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

async function memberHolding(dispatch: Dispatch, actor: ActorCredential): Promise<string | null> {
	const identity = await dispatch.askChatd('person.identity', { actor });
	if (identity.status >= 300) return null;
	const externalID = (identity.body as { externalID?: string } | null)?.externalID;
	if (!externalID) return null;
	return dispatch.memberOfExternalID(externalID);
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
