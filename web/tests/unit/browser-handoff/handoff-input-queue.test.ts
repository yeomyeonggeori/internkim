import { describe, expect, test } from 'bun:test';
import type { HandoffInput } from '../../../src/lib/browser-handoff/handoff-input';
import { HandoffInputQueue } from '../../../src/lib/browser-handoff/handoff-input-queue';

const settle = () => new Promise((resolve) => setTimeout(resolve, 5));

describe('HandoffInputQueue', () => {
	test('inputs pushed while a batch is on its way follow in the next batch, in order', async () => {
		const batches: HandoffInput[][] = [];
		let arrive = () => {};
		const queue = new HandoffInputQueue(
			(batch) => {
				batches.push(batch);
				return new Promise((resolve) => {
					arrive = resolve;
				});
			},
			() => {}
		);

		queue.push({ type: 'text', text: 'a' });
		queue.push({ type: 'text', text: 'b' });
		queue.push({ type: 'reload' });
		arrive();
		await settle();

		expect(batches).toEqual([[{ type: 'text', text: 'a' }], [{ type: 'text', text: 'b' }, { type: 'reload' }]]);
	});

	test('a batch that fails is reported and the queue keeps going', async () => {
		const failures: unknown[] = [];
		const sent: HandoffInput[][] = [];
		const queue = new HandoffInputQueue(
			async (batch) => {
				sent.push(batch);
				if (sent.length === 1) throw new Error('the gateway dropped the call');
			},
			(failure) => failures.push(failure)
		);

		queue.push({ type: 'reload' });
		await settle();
		queue.push({ type: 'text', text: 'x' });
		await settle();

		expect(failures).toHaveLength(1);
		expect(sent).toHaveLength(2);
	});
});
