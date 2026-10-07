import { describe, expect, test } from 'bun:test';
import { FairOutbox, type Outlet } from './fair-outbox';

function outletBlockedUntilOpened(): { outlet: Outlet; sent: string[]; open: () => void } {
	const sent: string[] = [];
	let isBlocked = true;
	return {
		sent,
		open: () => {
			isBlocked = false;
		},
		outlet: { send: (document) => sent.push(document), bufferedBytes: () => (isBlocked ? Number.MAX_SAFE_INTEGER : 0) }
	};
}

async function waitUntil(isReady: () => boolean, waitedFor: string): Promise<void> {
	const deadline = Date.now() + 3_000;
	while (Date.now() < deadline) {
		if (isReady()) return;
		await Bun.sleep(1);
	}
	throw new Error(`waited too long for ${waitedFor}`);
}

describe('FairOutbox', () => {
	test('sends straight through while the socket keeps up', () => {
		const sent: string[] = [];
		const outbox = new FairOutbox(() => ({ send: (document) => sent.push(document), bufferedBytes: () => 0 }));
		outbox.push('a', 'a1');
		outbox.push('b', 'b1');
		expect(sent).toEqual(['a1', 'b1']);
		expect(outbox.queuedStreams).toBe(0);
	});

	test('takes turns between streams once the socket backs up, so a busy one cannot starve a quiet one', async () => {
		const { outlet, sent, open } = outletBlockedUntilOpened();
		const outbox = new FairOutbox(() => outlet);
		for (const document of ['a1', 'a2', 'a3', 'a4']) outbox.push('busy', document);
		outbox.push('quiet', 'q1');
		outbox.push('quiet', 'q2');
		expect(sent).toEqual([]);

		open();
		await waitUntil(() => sent.length === 6, 'the backed up streams to drain');
		expect(sent).toEqual(['a1', 'q1', 'a2', 'q2', 'a3', 'a4']);
	});

	test('refuses a stream that has queued more than it may hold, and keeps the others', () => {
		const { outlet } = outletBlockedUntilOpened();
		const outbox = new FairOutbox(() => outlet, 1024, 10);
		expect(outbox.push('greedy', 'x'.repeat(8))).toBe(true);
		expect(outbox.push('greedy', 'x'.repeat(8))).toBe(false);
		expect(outbox.push('modest', 'x'.repeat(8))).toBe(true);
	});

	test('holds nothing for a stream it was told to forget', async () => {
		const { outlet, sent, open } = outletBlockedUntilOpened();
		const outbox = new FairOutbox(() => outlet);
		outbox.push('gone', 'never');
		outbox.push('kept', 'k1');
		outbox.forget('gone');
		open();
		await waitUntil(() => sent.length === 1, 'the kept stream to drain');
		expect(sent).toEqual(['k1']);
	});
});
