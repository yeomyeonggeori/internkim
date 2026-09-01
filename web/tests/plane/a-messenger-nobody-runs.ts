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
export function aConnectorNobodyRuns(): ARecordingMessenger {
	const calls: RecordedCall[] = [];
	const server = Bun.serve({
		port: 0,
		fetch: async (request) => {
			const path = new URL(request.url).pathname;
			calls.push({ path, body: await bodyOf(request) });
			if (path === '/health' || path === '/healthz') return new Response('ok');
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

export function platformOf(call: RecordedCall): string {
	return call.path.split('/')[3] ?? '';
}
