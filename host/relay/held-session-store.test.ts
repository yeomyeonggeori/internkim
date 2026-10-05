import { afterAll, expect, spyOn, test } from 'bun:test';
import { mkdtempSync, rmSync } from 'node:fs';
import { open, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { HeldSessionStore } from './held-session-store';

const root = mkdtempSync(join(tmpdir(), 'held-sessions-'));

afterAll(() => rmSync(root, { recursive: true, force: true }));

const sampleRequester = { email: 'sample@example.test', name: '이샘플' };
const firstConversation = { platform: 'buzz', conversationID: 'conversation-1', replyTargetID: 'thread-1', isThread: false };
const secondConversation = { platform: 'buzz', conversationID: 'conversation-2' };

test('the sessions a relay held come back after a restart, one per conversation', async () => {
	const filePath = join(root, 'kept', 'sessions.json');
	const store = new HeldSessionStore({ filePath });
	await store.keep({ sessionID: 'session-1', requester: sampleRequester, addressing: firstConversation });
	await store.keep({ sessionID: 'session-2', requester: sampleRequester, addressing: secondConversation });
	await store.keep({ sessionID: 'session-3', requester: sampleRequester, addressing: firstConversation });

	const afterTheRestart = await new HeldSessionStore({ filePath }).all();

	expect(afterTheRestart.map((held) => held.sessionID)).toEqual(['session-2', 'session-3']);
	expect(afterTheRestart[1].addressing).toEqual(firstConversation);
	expect(afterTheRestart[1].requester).toEqual(sampleRequester);
});

test('nothing is held before the first session, and a file that will not parse is reported and left alone', async () => {
	const reported: string[] = [];
	const missing = new HeldSessionStore({ filePath: join(root, 'missing.json') });
	const brokenPath = join(root, 'broken.json');
	await writeFile(brokenPath, 'not json');
	const broken = new HeldSessionStore({ filePath: brokenPath, report: (line) => reported.push(line) });

	expect(await missing.all()).toEqual([]);
	expect(await broken.all()).toEqual([]);
	expect(reported).toHaveLength(1);
});

test('a binding with a wrong-typed field is dropped, not loaded', async () => {
	const filePath = join(root, 'wrong-types.json');
	const good = { sessionID: 'session-1', requester: sampleRequester, addressing: firstConversation };
	const wrong = [
		{ ...good, sessionID: 7 },
		{ ...good, requester: { ...sampleRequester, name: 5 } },
		{ ...good, addressing: { ...firstConversation, isThread: 'yes' } },
		{ ...good, addressing: { ...firstConversation, replyTargetID: 3 } }
	];
	await writeFile(filePath, JSON.stringify([...wrong, good]));

	expect(await new HeldSessionStore({ filePath }).all()).toEqual([good]);
});

test('a kept binding is written through fsync', async () => {
	const probe = await open(join(root, 'probe'), 'w');
	await probe.close();
	const synced = spyOn(Object.getPrototypeOf(probe), 'sync');
	await new HeldSessionStore({ filePath: join(root, 'durable.json') }).keep({ sessionID: 'session-1', requester: sampleRequester, addressing: secondConversation });
	expect(synced).toHaveBeenCalledTimes(1);
	synced.mockRestore();
});
