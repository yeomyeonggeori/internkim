// The two messengers a company might be on, standing in as recorders. Neither
// delivers anything; both remember exactly what they were asked to deliver,
// which is the whole question — a message sent to the wrong one arrives
// somewhere real and looks like a success.

import { MalformedRequest, parsePersonRequest, requireMediaSource } from '../../../.dependency/blueclaw/chatd/src/personal/parse';
import type { CredentialRequirement, KeptMedia } from '../../../.dependency/blueclaw/chatd/src/personal/gateway';
import { messengerIdentityCredentialKind } from '../../src/lib/server/public-api/catalog/credential';
import { parseDirectMessagePostRequest, parseDirectMessageSendRequest, parseMessagePostRequest } from '../../../.dependency/blueclaw/chatd/src/outbound-parse';

export type RecordedCall = { path: string; body: unknown };

type RecordedMedia = { bytes: ArrayBuffer; contentType: string };

export type ARecordingMessenger = {
	url: string;
	calls: RecordedCall[];
	pathsCalled: () => string[];
	stop: () => void;
};

async function bodyOf(request: Request): Promise<unknown> {
	const text = await request.text();
	if (!text) return null;
	try {
		return JSON.parse(text);
	} catch {
		return text;
	}
}

const chatdRequestParsers: Partial<Record<string, (body: unknown) => unknown>> = {
	'dm.post': parseDirectMessagePostRequest,
	'dm.send': parseDirectMessageSendRequest,
	'message.post': parseMessagePostRequest
};

function refusalFromChatd(capability: string, body: unknown): string | null {
	const parse = chatdRequestParsers[capability];
	if (!parse) return null;
	try {
		parse(body);
		return null;
	} catch (error) {
		if (error instanceof Error) return error.message;
		throw error;
	}
}

async function keepUploadedMedia(body: unknown, connectorURL: string, media: Map<string, RecordedMedia>): Promise<Response> {
	try {
		const request = parsePersonRequest(body);
		if (request.actor.kind !== messengerIdentityCredentialKind) {
			throw new Error(`buzz needs a ${messengerIdentityCredentialKind} credential, not ${request.actor.kind}`);
		}
		const source = requireMediaSource(request);
		const response = await fetch(source.url);
		if (!response.ok) return new Response(await response.text(), { status: response.status });
		const bytes = await response.arrayBuffer();
		const digest = new Bun.CryptoHasher('sha256').update(new Uint8Array(bytes)).digest('hex');
		const mediaPath = `/media/${digest}`;
		media.set(mediaPath, { bytes, contentType: source.contentType });
		const keptMedia: KeptMedia = {
			address: `${connectorURL}${mediaPath}`, digest,
			sizeBytes: bytes.byteLength, contentType: source.contentType
		};
		return Response.json({ media: keptMedia });
	} catch (error) {
		if (error instanceof MalformedRequest) return Response.json({ error: error.message }, { status: 400 });
		throw error;
	}
}

export function aConnectorNobodyRuns(): ARecordingMessenger {
	const calls: RecordedCall[] = [];
	const media = new Map<string, RecordedMedia>();
	const server = Bun.serve({
		port: 0,
		fetch: async (request): Promise<Response> => {
			const path = new URL(request.url).pathname;
			const body = await bodyOf(request);
			calls.push({ path, body });
			if (path === '/health' || path === '/healthz') return new Response('ok');
			const kept = media.get(path);
			if (kept) return new Response(kept.bytes, { headers: { 'Content-Type': kept.contentType } });
			const capability = path.split('/')[4] ?? '';
			const refusal = refusalFromChatd(capability, body);
			if (refusal) return Response.json({ error: refusal }, { status: 400 });
			if (capability === 'person.credential.requirement') {
				const requirement: CredentialRequirement = {
					kind: 'secret', credentialKind: messengerIdentityCredentialKind,
					fields: [{ name: 'secret', label: 'Sample messenger secret', isSecret: true }]
				};
				return Response.json(requirement);
			}
			if (capability === 'person.media.upload') {
				return keepUploadedMedia(body, `http://127.0.0.1:${server.port}`, media);
			}
			return Response.json({ channelID: 'channel-nobody-runs', messageID: `message-${calls.length}` });
		}
	});
	return {
		url: `http://127.0.0.1:${server.port}`,
		calls,
		pathsCalled: () => calls.map((call) => call.path),
		stop: () => server.stop(true)
	};
}

export function aMessengerNobodyRuns(): ARecordingMessenger {
	const calls: RecordedCall[] = [];
	const server = Bun.serve({
		port: 0,
		fetch: async (request) => {
			const path = new URL(request.url).pathname;
			calls.push({ path, body: await bodyOf(request) });
			if (path === '/api/v4/users/login') {
				return new Response(JSON.stringify({ id: 'bot-nobody-runs', is_bot: true }), {
					headers: { Token: 'token-nobody-runs', 'Content-Type': 'application/json' }
				});
			}
			if (path === '/api/v4/users/me') {
				return Response.json({ id: 'bot-nobody-runs', username: 'internkim', is_bot: true });
			}
			if (path === '/api/v4/channels/direct') return Response.json({ id: 'direct-nobody-runs' });
			if (path === '/api/v4/posts') return Response.json({ id: `post-${calls.length}` });
			return Response.json({});
		}
	});
	return {
		url: `http://127.0.0.1:${server.port}`,
		calls,
		pathsCalled: () => calls.map((call) => call.path),
		stop: () => server.stop(true)
	};
}

export function postsDelivered(messenger: ARecordingMessenger): RecordedCall[] {
	return messenger.calls.filter((call) => call.path === '/api/v4/posts');
}

export function directMessagesDelivered(connector: ARecordingMessenger): RecordedCall[] {
	return connector.calls.filter((call) => /^\/v1\/platform\/[^/]+\/dm\./.test(call.path));
}

export function messagesPostedTo(connector: ARecordingMessenger, conversationID: string): string[] {
	const posted: string[] = [];
	for (const call of connector.calls) {
		if (!call.path.endsWith('/message.post')) continue;
		if (typeof call.body !== 'object' || call.body === null) continue;
		const document = call.body as Record<string, unknown>;
		if (document.threadID !== conversationID) continue;
		posted.push(typeof document.message === 'string' ? document.message : '');
	}
	return posted;
}

export function platformOf(call: RecordedCall): string {
	return call.path.split('/')[3] ?? '';
}
