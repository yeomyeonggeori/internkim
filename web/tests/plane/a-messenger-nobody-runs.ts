// The two messengers a company might be on, standing in as recorders. Neither
// delivers anything; both remember exactly what they were asked to deliver,
// which is the whole question — a message sent to the wrong one arrives
// somewhere real and looks like a success.

export type RecordedCall = { path: string; body: unknown };

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

// chatd is the seam every messenger goes through, and its URL carries the
// platform: /v1/platform/<name>/<capability>. So the platform a message left on
// is readable off the path, without asking any messenger whether it arrived.
//
// It refuses what the real one refuses. A stand-in that accepts anything hands
// back a green run for a call chatd would answer 400 to, which is worse than no
// stand-in at all: the gate would be measuring its own politeness.
// chatd/src/outbound-parse.ts is the contract these mirror.
function refusalFromChatd(capability: string, body: unknown): string | null {
	if (typeof body !== 'object' || body === null) return `expected ${capability} request to be a JSON object`;
	const document = body as Record<string, unknown>;
	const requireText = (field: string): string | null =>
		typeof document[field] === 'string' && (document[field] as string).trim().length > 0
			? null
			: `missing required field ${field}`;

	if (capability === 'dm.post') {
		const missing = requireText('counterpartPubkeyHex') ?? requireText('message');
		if (missing) return missing;
		if (!/^[0-9a-f]{64}$/.test((document.counterpartPubkeyHex as string).toLowerCase())) {
			return 'counterpartPubkeyHex must be 64 hex characters';
		}
		return null;
	}
	if (capability === 'message.post') {
		const missing = requireText('message');
		if (missing) return missing;
		const names = ['threadID', 'channelID', 'channelName'].filter((field) => typeof document[field] === 'string' && document[field] !== '');
		return names.length > 0 ? null : 'message.post requires threadID, channelID, or channelName';
	}
	if (capability === 'dm.send') {
		const missing = requireText('userSecretHex');
		if (missing) return missing;
		const counterpart = document.counterpartPubkeyHex;
		if (counterpart !== undefined && !/^[0-9a-f]{64}$/.test(String(counterpart).toLowerCase())) {
			return 'counterpartPubkeyHex must be 64 hex characters';
		}
		return null;
	}
	return null;
}

export function aConnectorNobodyRuns(): ARecordingMessenger {
	const calls: RecordedCall[] = [];
	const server = Bun.serve({
		port: 0,
		fetch: async (request) => {
			const path = new URL(request.url).pathname;
			const body = await bodyOf(request);
			calls.push({ path, body });
			if (path === '/health' || path === '/healthz') return new Response('ok');
			const capability = path.split('/')[4] ?? '';
			const refusal = refusalFromChatd(capability, body);
			if (refusal) return Response.json({ error: refusal }, { status: 400 });
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
