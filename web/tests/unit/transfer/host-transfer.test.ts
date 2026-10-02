import { describe, expect, test } from 'bun:test';
import type { CompanyEvent } from '$lib/company-event';
import { TransferFailed, transferThroughTheHost, type HostConnection } from '$lib/transfer/host-transfer';

function hostThat(answer: (transferID: string) => { status: number; body: unknown }) {
	const listeners = new Set<(event: CompanyEvent) => void>();
	const asked: { capability: string; body: Record<string, unknown> }[] = [];
	const connection: HostConnection = {
		call: async (call) => {
			asked.push({ capability: call.capability, body: call.body ?? {} });
			return answer(String(call.body?.transferID ?? ''));
		},
		listen: (listener) => {
			listeners.add(listener);
			return () => listeners.delete(listener);
		}
	};
	const tell = (event: CompanyEvent) => {
		for (const listener of listeners) listener(event);
	};
	return { connection, asked, tell, listeners };
}

const pending = { status: 200, body: { transfer: { state: 'pending' } } };

describe('a transfer through the company computer', () => {
	test('a copy already made is answered at once', async () => {
		const { connection, listeners } = hostThat(() => ({
			status: 200,
			body: { transfer: { state: 'ready', address: 'https://store.test/object', sizeBytes: 9 } }
		}));

		expect(await transferThroughTheHost(connection, { capability: 'person.media.prepare', body: {} })).toEqual({
			address: 'https://store.test/object',
			sizeBytes: 9
		});
		expect(listeners.size).toBe(0);
	});

	test('a copy still being made says how far it got, then what it made, under the id the browser chose', async () => {
		const host = hostThat(() => pending);
		const progress: number[][] = [];

		const done = transferThroughTheHost(host.connection, { capability: 'person.files.download', body: { path: '/workspace/a' } }, (copied, total) =>
			progress.push([copied, total])
		);
		await Promise.resolve();
		const transferID = String(host.asked[0]?.body.transferID);
		host.tell({ kind: 'transfer.progress', transferID: 'somebody-else', copiedBytes: 1, totalBytes: 2 });
		host.tell({ kind: 'transfer.progress', transferID, copiedBytes: 6, totalBytes: 12 });
		host.tell({ kind: 'transfer.done', transferID, result: { address: 'https://store.test/copy', sizeBytes: 12 } });

		expect(await done).toEqual({ address: 'https://store.test/copy', sizeBytes: 12 });
		expect(progress).toEqual([[6, 12]]);
		expect(host.asked[0]?.body.path).toBe('/workspace/a');
		expect(host.listeners.size).toBe(0);
	});

	test('a failed copy is a failure the caller sees, with its status and reason', async () => {
		const host = hostThat(() => pending);

		const done = transferThroughTheHost(host.connection, { capability: 'person.media.prepare', body: {} });
		await Promise.resolve();
		host.tell({ kind: 'transfer.failed', transferID: String(host.asked[0]?.body.transferID), status: 413, error: 'too large' });

		const failure = await done.catch((thrown: unknown) => thrown);
		expect(failure).toBeInstanceOf(TransferFailed);
		expect(failure).toMatchObject({ status: 413, message: 'too large' });
	});

	test('a refusal to start is a failure, not a copy that never comes', async () => {
		const host = hostThat(() => ({ status: 403, body: { error: 'workspace path is not accessible' } }));

		const failure = await transferThroughTheHost(host.connection, { capability: 'person.files.download', body: {} }).catch(
			(thrown: unknown) => thrown
		);

		expect(failure).toMatchObject({ status: 403, message: 'workspace path is not accessible' });
	});

	test('a copy the company computer stops speaking about is given up on, visibly', async () => {
		const host = hostThat(() => pending);

		const failure = await transferThroughTheHost(host.connection, { capability: 'person.media.prepare', body: {} }, undefined, 20).catch(
			(thrown: unknown) => thrown
		);

		expect(failure).toMatchObject({ status: 504 });
		expect(host.listeners.size).toBe(0);
	});
});
