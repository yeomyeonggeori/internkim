import { describe, expect, test } from 'bun:test';
import { Transfers, type TransferEvent } from './transfers';

function recordingTransfers(clock: { now: number }) {
	const told: { event: TransferEvent; memberID: string }[] = [];
	const transfers = new Transfers((event, memberID) => told.push({ event, memberID }), () => clock.now);
	return { told, transfers };
}

function settled(): Promise<void> {
	return new Promise((resolve) => setTimeout(resolve, 0));
}

describe('a transfer', () => {
	test('tells each member who asked, by the id they asked with, once it is done', async () => {
		const clock = { now: 0 };
		const { told, transfers } = recordingTransfers(clock);
		let finish: (value: unknown) => void = () => undefined;
		const work = () => new Promise((resolve) => (finish = resolve));

		transfers.start('object-1', { memberID: 'member-1', transferID: 'first' }, work);
		transfers.start('object-1', { memberID: 'member-2', transferID: 'second' }, work);
		expect(transfers.isRunning('object-1')).toBe(true);
		finish({ address: 'kept' });
		await settled();

		expect(told).toEqual([
			{ event: { kind: 'transfer.done', transferID: 'first', result: { address: 'kept' } }, memberID: 'member-1' },
			{ event: { kind: 'transfer.done', transferID: 'second', result: { address: 'kept' } }, memberID: 'member-2' }
		]);
		expect(transfers.isRunning('object-1')).toBe(false);
	});

	test('says how far it has got at most once a second, and not at the end', async () => {
		const clock = { now: 0 };
		const { told, transfers } = recordingTransfers(clock);
		transfers.start('object-1', { memberID: 'member-1', transferID: 'first' }, async (progress) => {
			clock.now = 5_000;
			progress(10, 100);
			progress(20, 100);
			clock.now = 6_500;
			progress(30, 100);
			progress(100, 100);
			return null;
		});
		await settled();

		expect(told.map((one) => one.event)).toEqual([
			{ kind: 'transfer.progress', transferID: 'first', copiedBytes: 10, totalBytes: 100 },
			{ kind: 'transfer.progress', transferID: 'first', copiedBytes: 30, totalBytes: 100 },
			{ kind: 'transfer.done', transferID: 'first', result: null }
		]);
	});

	test('a failure is told with its status and reason, never swallowed', async () => {
		const { told, transfers } = recordingTransfers({ now: 0 });
		transfers.start('object-1', { memberID: 'member-1', transferID: 'first' }, async () => {
			throw Object.assign(new Error('the store refused the bytes from 0'), { status: 413 });
		});
		await settled();

		expect(told).toEqual([
			{
				event: { kind: 'transfer.failed', transferID: 'first', status: 413, error: 'the store refused the bytes from 0' },
				memberID: 'member-1'
			}
		]);
	});
});
