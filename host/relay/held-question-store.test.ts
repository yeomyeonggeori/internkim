import { afterAll, describe, expect, test } from 'bun:test';
import { mkdtempSync, rmSync } from 'node:fs';
import { writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { HeldQuestionStore } from './held-question-store';

const root = mkdtempSync(join(tmpdir(), 'held-question-store-'));
let taken = 0;

function directoryForOneTest(): string {
	taken += 1;
	return join(root, `store-${taken}`);
}

afterAll(() => rmSync(root, { recursive: true, force: true }));

const sampleRequester = { email: 'sample@example.test', name: '이샘플' };
const sampleAddressing = { platform: 'buzz', conversationID: 'conversation-1' };

function aQuestion(overrides: Partial<Parameters<HeldQuestionStore['keep']>[0]> = {}) {
	return {
		toolCallID: 'held-1',
		sessionID: 'session-1',
		requester: sampleRequester,
		addressing: sampleAddressing,
		question: '박예시에게 보낼까요?',
		askedAt: '2026-09-02T00:00:00.000Z',
		...overrides
	};
}

describe('HeldQuestionStore', () => {
	test('a question that was kept comes back unanswered', async () => {
		const store = new HeldQuestionStore({ directoryPath: directoryForOneTest() });

		await store.keep(aQuestion());

		const held = await store.read('held-1');
		expect(held?.toolCallID).toBe('held-1');
		expect(held?.sessionID).toBe('session-1');
		expect(held?.requester).toEqual(sampleRequester);
		expect(held?.addressing).toEqual(sampleAddressing);
		expect(held?.answer).toBeUndefined();
	});

	test('a question asked twice keeps the first record', async () => {
		const store = new HeldQuestionStore({ directoryPath: directoryForOneTest() });

		await store.keep(aQuestion({ question: 'first' }));
		await store.keep(aQuestion({ question: 'a later copy' }));

		expect((await store.read('held-1'))?.question).toBe('first');
	});

	test('the person answering records the raw words against the question already asked', async () => {
		const store = new HeldQuestionStore({ directoryPath: directoryForOneTest() });
		await store.keep(aQuestion());

		await store.answer('held-1', '응 보내줘');

		expect((await store.read('held-1'))?.answer).toBe('응 보내줘');
	});

	test('answering a question nobody asked records nothing', async () => {
		const store = new HeldQuestionStore({ directoryPath: directoryForOneTest() });

		await store.answer('held-1', '응 보내줘');

		expect(await store.read('held-1')).toBeNull();
	});

	test('a forgotten question stops coming back', async () => {
		const store = new HeldQuestionStore({ directoryPath: directoryForOneTest() });
		await store.keep(aQuestion());

		await store.forget('held-1');

		expect(await store.read('held-1')).toBeNull();
	});

	test('forgetting a question nobody kept is not a failure', async () => {
		const store = new HeldQuestionStore({ directoryPath: directoryForOneTest() });

		await store.forget('never-seen');

		expect(await store.read('never-seen')).toBeNull();
	});

	test('every open question survives a restart, asked and answered alike', async () => {
		const directoryPath = directoryForOneTest();
		const beforeRestart = new HeldQuestionStore({ directoryPath });
		await beforeRestart.keep(aQuestion({ toolCallID: 'held-unanswered' }));
		await beforeRestart.keep(aQuestion({ toolCallID: 'held-answered' }));
		await beforeRestart.answer('held-answered', '응 보내줘');

		const afterRestart = new HeldQuestionStore({ directoryPath });
		const all = await afterRestart.all();

		expect(all.map((held) => held.toolCallID).sort()).toEqual(['held-answered', 'held-unanswered']);
		expect(all.find((held) => held.toolCallID === 'held-answered')?.answer).toBe('응 보내줘');
		expect(all.find((held) => held.toolCallID === 'held-unanswered')?.answer).toBeUndefined();
	});

	test('a file that will not parse is reported and the questions beside it still come back', async () => {
		const directoryPath = directoryForOneTest();
		const reported: string[] = [];
		const store = new HeldQuestionStore({ directoryPath, report: (line) => reported.push(line) });
		await store.keep(aQuestion({ toolCallID: 'held-1' }));
		await store.keep(aQuestion({ toolCallID: 'held-2' }));

		await writeFile(join(directoryPath, 'half-written.json'), '{"toolCallID":"held-3');

		const all = await store.all();
		expect(all.map((held) => held.toolCallID).sort()).toEqual(['held-1', 'held-2']);
		expect(reported).toHaveLength(1);
		expect(reported[0]).toContain('half-written.json');
	});

	test('a store whose directory was never made holds nothing', async () => {
		const store = new HeldQuestionStore({ directoryPath: join(directoryForOneTest(), 'never-made') });

		expect(await store.all()).toEqual([]);
	});
});
