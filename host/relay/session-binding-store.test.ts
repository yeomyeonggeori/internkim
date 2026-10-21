import { afterAll, expect, spyOn, test } from 'bun:test';
import { mkdtempSync, rmSync } from 'node:fs';
import { open, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { SessionBindingStore } from './session-binding-store';

const root = mkdtempSync(join(tmpdir(), 'session-bindings-'));

afterAll(() => rmSync(root, { recursive: true, force: true }));

const sampleRequester = { email: 'sample@example.test', name: '이샘플' };
const firstConversation = { platform: 'buzz', conversationID: 'conversation-1', replyTargetID: 'thread-1', isThread: false };
const secondConversation = { platform: 'buzz', conversationID: 'conversation-2' };

test('the sessions a relay bound come back after a restart, one per person in a conversation', async () => {
	const filePath = join(root, 'kept', 'sessions.json');
	const store = new SessionBindingStore({ filePath });
	await store.save({ sessionID: 'session-1', requester: sampleRequester, addressing: firstConversation, catalogTicket: 'ticket-1' });
	await store.save({ sessionID: 'session-2', requester: sampleRequester, addressing: secondConversation, catalogTicket: 'ticket-2' });
	await store.save({ sessionID: 'session-3', requester: sampleRequester, addressing: firstConversation, catalogTicket: 'ticket-3' });

	const afterTheRestart = await new SessionBindingStore({ filePath }).list();

	expect(afterTheRestart.map((binding) => binding.sessionID)).toEqual(['session-2', 'session-3']);
	expect(afterTheRestart[1].addressing).toEqual(firstConversation);
	expect(afterTheRestart[1].requester).toEqual(sampleRequester);
	expect(afterTheRestart[1].catalogTicket).toBe('ticket-3');
});

test('two people in one conversation keep a session each across a restart', async () => {
	const filePath = join(root, 'two-speakers.json');
	const store = new SessionBindingStore({ filePath });
	const secondSpeaker = { email: 'Second@example.test', name: '박예시' };
	await store.save({ sessionID: 'session-1', requester: sampleRequester, addressing: firstConversation, catalogTicket: 'ticket-1' });
	await store.save({ sessionID: 'session-2', requester: secondSpeaker, addressing: firstConversation, catalogTicket: 'ticket-2' });

	const afterTheRestart = await new SessionBindingStore({ filePath }).list();

	expect(afterTheRestart.map((binding) => [binding.sessionID, binding.requester.email])).toEqual([
		['session-1', sampleRequester.email],
		['session-2', secondSpeaker.email]
	]);
});

test('a binding kept before catalog tickets existed is given one, and keeps it across restarts', async () => {
	const filePath = join(root, 'unticketed.json');
	await writeFile(filePath, JSON.stringify([{ sessionID: 'session-1', requester: sampleRequester, addressing: firstConversation }]));

	const [first] = await new SessionBindingStore({ filePath }).list();
	const [second] = await new SessionBindingStore({ filePath }).list();

	expect(first.catalogTicket).not.toBe('');
	expect(second.catalogTicket).toBe(first.catalogTicket);
});

test('no session is bound before the first session, and a file that will not parse is reported and left alone', async () => {
	const reported: string[] = [];
	const missing = new SessionBindingStore({ filePath: join(root, 'missing.json') });
	const brokenPath = join(root, 'broken.json');
	await writeFile(brokenPath, 'not json');
	const broken = new SessionBindingStore({ filePath: brokenPath, report: (line) => reported.push(line) });

	expect(await missing.list()).toEqual([]);
	expect(await broken.list()).toEqual([]);
	expect(reported).toHaveLength(1);
});

test('a binding with a wrong-typed field is dropped, not loaded', async () => {
	const filePath = join(root, 'wrong-types.json');
	const good = { sessionID: 'session-1', requester: sampleRequester, addressing: firstConversation, catalogTicket: 'ticket-1' };
	const wrong = [
		{ ...good, sessionID: 7 },
		{ ...good, requester: { ...sampleRequester, name: 5 } },
		{ ...good, addressing: { ...firstConversation, isThread: 'yes' } },
		{ ...good, addressing: { ...firstConversation, replyTargetID: 3 } },
		{ ...good, catalogTicket: 4 }
	];
	await writeFile(filePath, JSON.stringify([...wrong, good]));

	expect(await new SessionBindingStore({ filePath }).list()).toEqual([good]);
});

test('a kept binding is written through fsync', async () => {
	const probe = await open(join(root, 'probe'), 'w');
	await probe.close();
	const synced = spyOn(Object.getPrototypeOf(probe), 'sync');
	await new SessionBindingStore({ filePath: join(root, 'durable.json') }).save({ sessionID: 'session-1', requester: sampleRequester, addressing: secondConversation, catalogTicket: 'ticket-1' });
	expect(synced).toHaveBeenCalledTimes(1);
	synced.mockRestore();
});
