import { afterEach, describe, expect, test } from 'bun:test';
import { mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { withChatdServiceToken } from './chatd-reach';
import { forwardToChatd } from './forward';

type Stoppable = { stop: (closeActiveConnections?: boolean) => void };

const started: Stoppable[] = [];

afterEach(() => {
	for (const server of started.splice(0)) server.stop(true);
});

function unusedPort(): number {
	const probe = Bun.listen({ hostname: '127.0.0.1', port: 0, socket: { data() {} } });
	const port = probe.port;
	probe.stop(true);
	return port;
}

function chatdOn(port: number, posted: unknown[]): void {
	started.push(
		Bun.serve({
			hostname: '127.0.0.1',
			port,
			async fetch(request) {
				posted.push(await request.json());
				return Response.json({ messageID: 'posted-1' });
			}
		})
	);
}

describe('a call to chatd while chatd is restarting', () => {
	test('is posted once chatd listens again', async () => {
		const port = unusedPort();
		const posted: unknown[] = [];
		setTimeout(() => chatdOn(port, posted), 700);

		const answered = await forwardToChatd(`http://127.0.0.1:${port}`, 'buzz', 'message.post', { message: 'hello' }, 1024);

		expect(answered).toEqual({ status: 200, body: { messageID: 'posted-1' } });
		expect(posted).toEqual([{ message: 'hello', largestBytes: 1024 }]);
	});

	test('is reported not answered once chatd stays away past the wait', async () => {
		const port = unusedPort();
		const startedAt = Date.now();

		const failure = await forwardToChatd(`http://127.0.0.1:${port}`, 'buzz', 'message.post', {}, 1024, 600).catch(
			(caught: unknown) => caught
		);

		expect(failure).toBeInstanceOf(Error);
		expect(String(failure)).toContain(`http://127.0.0.1:${port} did not answer`);
		expect(Date.now() - startedAt).toBeGreaterThanOrEqual(500);
		expect(Date.now() - startedAt).toBeLessThan(3000);
	});

	test('that reached chatd and lost the connection is not sent again', async () => {
		let connections = 0;
		const dropping = Bun.listen({
			hostname: '127.0.0.1',
			port: 0,
			socket: {
				open() {
					connections += 1;
				},
				data(socket) {
					socket.terminate();
				}
			}
		});
		started.push(dropping);

		const failure = await forwardToChatd(`http://127.0.0.1:${dropping.port}`, 'buzz', 'message.post', {}, 1024).catch(
			(caught: unknown) => caught
		);

		expect(String(failure)).toContain('did not answer');
		expect(connections).toBe(1);
	});
});

describe('a call to chatd as the relay', () => {
	test('carries the token chatd asks of the host services', async () => {
		const directory = mkdtempSync(join(tmpdir(), 'relay-chatd-credential-'));
		const keptAt = join(directory, 'relay-service-token');
		writeFileSync(keptAt, 'kept-for-the-host-services\n');
		const authorizations: (string | null)[] = [];
		const chatd = Bun.serve({
			hostname: '127.0.0.1',
			port: 0,
			fetch(request) {
				authorizations.push(request.headers.get('Authorization'));
				return Response.json({ messageID: 'posted-1' });
			}
		});
		started.push(chatd);
		process.env.CHATD_SERVICE_TOKEN_PATH = keptAt;
		try {
			await forwardToChatd(`http://127.0.0.1:${chatd.port}`, 'buzz', 'message.post', { message: 'hello' }, 1024);
		} finally {
			delete process.env.CHATD_SERVICE_TOKEN_PATH;
			rmSync(directory, { recursive: true, force: true });
		}

		expect(authorizations).toEqual(['Bearer kept-for-the-host-services']);
	});

	test('is not sent when the token cannot be read', async () => {
		const failure = await withChatdServiceToken({ method: 'POST' }, join(tmpdir(), 'no-such-relay-credential')).catch(
			(caught: unknown) => caught
		);

		expect(String(failure)).toContain('could not be read');
	});
});
