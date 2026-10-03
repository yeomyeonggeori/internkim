// One member's half of the native-install rig's step 5, over the wire the
// browser uses and no other. It signs in through GoTrue with a password,
// carries the access token to the connection gateway as the
// `internkim.bearer.` subprotocol a browser is forced to use, and then asks the
// company the same two calls `web/src/lib/messenger/messenger-api.ts` asks to
// put a message in a conversation.
//
// What it observes is printed as one JSON document on stdout, so the rig reads
// what happened rather than an exit code. The gateway's admin token and the
// server key are deliberately absent: this session authenticates the way a
// person does, and every answer it receives came back over the socket the
// guest's own relay is holding.

import { typingEventKind } from '../web/src/lib/messenger/typing-signal';
import { transferThroughTheHost, type HostConnection } from '../web/src/lib/transfer/host-transfer';
import { uploadToStore } from '../web/src/lib/transfer/store-upload';

type Frame = Record<string, unknown>;

type Answer = { status: number; body: unknown };

type AttachmentFile = { path: string; filename: string; contentType: string };

type KeptAttachment = { address: string; digest: string; sizeBytes: number; filename: string; contentType: string };

const settings = {
	projectURL: required('MEMBER_SESSION_PROJECT_URL'),
	publishableKey: required('MEMBER_SESSION_PUBLISHABLE_KEY'),
	gatewayURL: required('MEMBER_SESSION_GATEWAY_URL'),
	companyID: required('MEMBER_SESSION_COMPANY_ID'),
	email: required('MEMBER_SESSION_EMAIL'),
	password: required('MEMBER_SESSION_PASSWORD'),
	messageText: required('MEMBER_SESSION_MESSAGE'),
	answerTimeoutMilliseconds: Number(process.env.MEMBER_SESSION_ANSWER_TIMEOUT_MS ?? 60_000),
	typingWaitMilliseconds: Number(process.env.MEMBER_SESSION_TYPING_WAIT_MS ?? 0),
	memberID: process.env.MEMBER_SESSION_MEMBER_ID?.trim() ?? '',
	attachmentFiles: JSON.parse(process.env.MEMBER_SESSION_ATTACHMENTS ?? '[]') as AttachmentFile[]
};

function required(name: string): string {
	const value = process.env[name]?.trim();
	if (!value) throw new Error(`set ${name}`);
	return value;
}

async function signIn(): Promise<{ accessToken: string; algorithm: string }> {
	const response = await fetch(`${settings.projectURL}/auth/v1/token?grant_type=password`, {
		method: 'POST',
		headers: { apikey: settings.publishableKey, 'Content-Type': 'application/json' },
		body: JSON.stringify({ email: settings.email, password: settings.password })
	});
	const document = (await response.json()) as { access_token?: string };
	if (!response.ok || !document.access_token) {
		throw new Error(`${settings.email} could not sign in: ${response.status} ${JSON.stringify(document)}`);
	}
	return { accessToken: document.access_token, algorithm: algorithmOf(document.access_token) };
}

function algorithmOf(token: string): string {
	const header = JSON.parse(Buffer.from(token.split('.')[0] ?? '', 'base64url').toString()) as {
		alg?: string;
	};
	return header.alg ?? '';
}

class MemberConnection {
	private readonly waiting = new Map<string, (answer: Answer) => void>();
	private readonly listeners = new Set<(event: Frame) => void>();
	readonly delivered: Frame[] = [];
	private presence: Frame | null = null;

	private constructor(private readonly socket: WebSocket) {}

	static async open(accessToken: string): Promise<MemberConnection> {
		const url = `${settings.gatewayURL.replace(/\/+$/, '')}/company/${encodeURIComponent(settings.companyID)}/client`;
		const socket = new WebSocket(url, [`internkim.bearer.${accessToken}`]);
		const connection = new MemberConnection(socket);
		socket.addEventListener('message', (message) => connection.receive(message.data));
		await new Promise<void>((resolve, reject) => {
			const refuse = setTimeout(
				() => reject(new Error('the gateway never opened the member socket')),
				30_000
			);
			socket.addEventListener('open', () => {
				clearTimeout(refuse);
				resolve();
			});
			socket.addEventListener('error', () => {
				clearTimeout(refuse);
				reject(new Error('the gateway refused the member socket'));
			});
		});
		return connection;
	}

	private receive(data: unknown): void {
		if (typeof data !== 'string') return;
		let frame: Frame;
		try {
			frame = JSON.parse(data) as Frame;
		} catch {
			return;
		}
		if (frame.kind === 'presence') {
			this.presence = frame;
			return;
		}
		if (frame.kind === 'deliver') {
			this.delivered.push(frame);
			const event = frame.event;
			if (typeof event === 'object' && event !== null) {
				for (const listener of this.listeners) listener(event as Frame);
			}
			return;
		}
		if (frame.kind !== 'result' || typeof frame.requestID !== 'string') return;
		const settle = this.waiting.get(frame.requestID);
		if (!settle) return;
		this.waiting.delete(frame.requestID);
		settle({ status: typeof frame.status === 'number' ? frame.status : 0, body: frame.body });
	}

	// The gateway sends this unasked the moment the socket is accepted, and it
	// is the only thing on the wire that tells a member whether the company's
	// own machine is holding the other half.
	async whetherTheHostIsConnected(): Promise<Frame> {
		const deadline = Date.now() + 10_000;
		while (Date.now() < deadline) {
			if (this.presence) return this.presence;
			await Bun.sleep(100);
		}
		throw new Error('the gateway sent no presence frame');
	}

	ask(capability: string, body: Record<string, unknown> = {}): Promise<Answer> {
		const requestID = crypto.randomUUID();
		const answered = new Promise<Answer>((resolve) => {
			this.waiting.set(requestID, resolve);
			setTimeout(() => {
				if (!this.waiting.delete(requestID)) return;
				resolve({ status: 0, body: { error: `${capability} was never answered` } });
			}, settings.answerTimeoutMilliseconds);
		});
		this.socket.send(JSON.stringify({ kind: 'call', requestID, capability, body }));
		return answered;
	}

	listen(listener: (event: Frame) => void): () => void {
		this.listeners.add(listener);
		return () => {
			this.listeners.delete(listener);
		};
	}

	async untilSomeoneTypesIn(conversationID: string, waitMilliseconds: number): Promise<void> {
		const deadline = Date.now() + waitMilliseconds;
		while (Date.now() < deadline) {
			if (this.delivered.some((frame) => isTypingIn(frame, conversationID))) return;
			await Bun.sleep(100);
		}
	}

	close(): void {
		this.socket.close();
	}
}

function isTypingIn(frame: Frame, conversationID: string): boolean {
	const event = frame.event;
	if (typeof event !== 'object' || event === null) return false;
	return 'kind' in event && event.kind === typingEventKind && 'conversationID' in event && event.conversationID === conversationID;
}

function answerField(answer: Answer, name: string): string {
	const held = (answer.body as Record<string, unknown> | null)?.[name];
	return typeof held === 'string' ? held : '';
}

function keptAttachmentOf(result: unknown, file: AttachmentFile): KeptAttachment {
	const kept = (result as { attachment?: Partial<KeptAttachment> } | null)?.attachment;
	if (!kept || typeof kept.address !== 'string' || typeof kept.digest !== 'string') {
		throw new Error(`the company computer kept ${file.filename} and said nothing about where: ${JSON.stringify(result)}`);
	}
	return {
		address: kept.address,
		digest: kept.digest,
		sizeBytes: typeof kept.sizeBytes === 'number' ? kept.sizeBytes : 0,
		filename: typeof kept.filename === 'string' ? kept.filename : file.filename,
		contentType: typeof kept.contentType === 'string' ? kept.contentType : file.contentType
	};
}

async function keptAttachment(connection: MemberConnection, bearer: string, file: AttachmentFile): Promise<KeptAttachment> {
	const storeSession = {
		projectURL: settings.projectURL,
		publishableKey: settings.publishableKey,
		accessToken: async () => bearer,
		companyID: settings.companyID,
		memberID: settings.memberID
	};
	const object = await uploadToStore(storeSession, Bun.file(file.path), file.contentType);
	const hostConnection: HostConnection = {
		call: (call) => connection.ask(call.capability, call.body ?? {}),
		listen: (listener) => connection.listen((event) => listener(event as Parameters<typeof listener>[0]))
	};
	const result = await transferThroughTheHost(hostConnection, {
		capability: 'person.media.upload',
		body: { filename: file.filename, object, contentType: file.contentType }
	});
	return keptAttachmentOf(result, file);
}

const session = await signIn();
const connection = await MemberConnection.open(session.accessToken);
const presence = await connection.whetherTheHostIsConnected();
const attachments: KeptAttachment[] = [];
for (const file of settings.attachmentFiles) {
	attachments.push(await keptAttachment(connection, session.accessToken, file));
}

const conversation = await connection.ask('person.dm.ensure', { counterpartExternalIDs: [] });
const conversationID = answerField(conversation, 'id');
const sent = await connection.ask('person.message.send', {
	conversationID,
	body: settings.messageText,
	...(attachments.length > 0 ? { attachments } : {})
});
await connection.untilSomeoneTypesIn(conversationID, settings.typingWaitMilliseconds);
connection.close();

console.log(
	JSON.stringify({
		tokenAlgorithm: session.algorithm,
		presence,
		conversation: { status: conversation.status, id: conversationID, body: conversation.body },
		sent: { status: sent.status, messageID: answerField(sent, 'id'), body: sent.body },
		attachments,
		delivered: connection.delivered,
		typing: connection.delivered.filter((frame) => isTypingIn(frame, conversationID)).map((frame) => frame.event)
	})
);
