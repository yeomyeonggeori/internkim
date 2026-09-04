import { afterAll, describe, expect, test } from 'bun:test';
import { mkdtempSync, rmSync } from 'node:fs';
import { writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { InboundQueue } from './inbound-queue';

const root = mkdtempSync(join(tmpdir(), 'inbound-queue-'));
let taken = 0;

function directoryForOneTest(): string {
	taken += 1;
	return join(root, `queue-${taken}`);
}

afterAll(() => rmSync(root, { recursive: true, force: true }));

const firstKey = 'buzz:conversation-1:message-7';
const secondKey = 'buzz:conversation-1:message-8';

describe('InboundQueue', () => {
	test('an event that was kept comes back with the body the messenger sent', async () => {
		const queue = new InboundQueue({ directoryPath: directoryForOneTest() });

		expect(await queue.keep(firstKey, { text: '오늘 회의 30분 미뤄도 될까요', from: '이샘플' })).toBe(true);

		const waiting = await queue.undelivered();
		expect(waiting).toHaveLength(1);
		expect(waiting[0].key).toBe(firstKey);
		expect(waiting[0].body).toEqual({ text: '오늘 회의 30분 미뤄도 될까요', from: '이샘플' });
		expect(waiting[0].attempts).toBe(0);
		expect(waiting[0].firstQueuedAt).not.toBe('');
	});

	test('the same message arriving twice is kept once and refused the second time', async () => {
		const queue = new InboundQueue({ directoryPath: directoryForOneTest() });

		expect(await queue.keep(firstKey, { text: 'first' })).toBe(true);
		expect(await queue.keep(firstKey, { text: 'a later copy of the same message' })).toBe(false);

		const waiting = await queue.undelivered();
		expect(waiting).toHaveLength(1);
		expect(waiting[0].body).toEqual({ text: 'first' });
	});

	test('a delivered event is forgotten and stops coming back', async () => {
		const queue = new InboundQueue({ directoryPath: directoryForOneTest() });
		await queue.keep(firstKey, { text: 'delivered' });

		await queue.forget(firstKey);

		expect(await queue.undelivered()).toEqual([]);
	});

	test('forgetting an event nobody kept is not a failure', async () => {
		const queue = new InboundQueue({ directoryPath: directoryForOneTest() });

		await queue.forget('buzz:conversation-9:never-seen');

		expect(await queue.undelivered()).toEqual([]);
	});

	test('attempts survive a restart, because the count is on disk and not in the relay', async () => {
		const directoryPath = directoryForOneTest();
		const beforeRestart = new InboundQueue({ directoryPath });
		await beforeRestart.keep(firstKey, { text: 'nobody answered' });

		expect(await beforeRestart.recordAttempt(firstKey)).toBe(1);

		const afterRestart = new InboundQueue({ directoryPath });

		expect(await afterRestart.recordAttempt(firstKey)).toBe(2);

		const waiting = await afterRestart.undelivered();
		expect(waiting).toHaveLength(1);
		expect(waiting[0].attempts).toBe(2);
		expect(waiting[0].body).toEqual({ text: 'nobody answered' });
	});

	test('an attempt against an event already forgotten counts nothing', async () => {
		const queue = new InboundQueue({ directoryPath: directoryForOneTest() });

		expect(await queue.recordAttempt(firstKey)).toBe(0);
	});

	test('an event is exhausted only once it has been through the ceiling', async () => {
		const queue = new InboundQueue({ directoryPath: directoryForOneTest(), attemptCeiling: 3 });
		await queue.keep(firstKey, { text: 'nobody answered' });
		await queue.recordAttempt(firstKey);
		await queue.recordAttempt(firstKey);

		const belowTheCeiling = (await queue.undelivered())[0];
		expect(belowTheCeiling.attempts).toBe(2);
		expect(queue.hasExhausted(belowTheCeiling)).toBe(false);

		await queue.recordAttempt(firstKey);

		const atTheCeiling = (await queue.undelivered())[0];
		expect(atTheCeiling.attempts).toBe(3);
		expect(queue.hasExhausted(atTheCeiling)).toBe(true);
	});

	test('a file that will not parse is reported and the events beside it still come back', async () => {
		const directoryPath = directoryForOneTest();
		const reported: string[] = [];
		const queue = new InboundQueue({ directoryPath, report: (line) => reported.push(line) });
		await queue.keep(firstKey, { text: 'first' });
		await queue.keep(secondKey, { text: 'second' });

		await writeFile(join(directoryPath, 'half-written.json'), '{"key":"buzz:conversa');

		const waiting = await queue.undelivered();
		expect(waiting.map((event) => event.key).sort()).toEqual([firstKey, secondKey]);
		expect(reported).toHaveLength(1);
		expect(reported[0]).toContain('half-written.json');
	});

	test('a queue whose directory was never made has nothing waiting', async () => {
		const queue = new InboundQueue({ directoryPath: join(directoryForOneTest(), 'never-made') });

		expect(await queue.undelivered()).toEqual([]);
	});

	test('events come back oldest first', async () => {
		const queue = new InboundQueue({ directoryPath: directoryForOneTest() });
		await queue.keep(secondKey, { text: 'kept first' });
		await Bun.sleep(2);
		await queue.keep(firstKey, { text: 'kept second' });

		expect((await queue.undelivered()).map((event) => event.key)).toEqual([secondKey, firstKey]);
	});
});
