import type { CompanyEvent } from '$lib/company-event';
import type { HostAnswer, HostCall } from '$lib/host-bridge';

export type TransferProgress = (copiedBytes: number, totalBytes: number) => void;

export type HostConnection = {
	call: (call: HostCall) => Promise<HostAnswer>;
	listen: (listener: (event: CompanyEvent) => void) => () => void;
};

export class TransferFailed extends Error {
	constructor(
		readonly status: number,
		message: string
	) {
		super(message);
		this.name = 'TransferFailed';
	}
}

export const stalledAfterMilliseconds = 90_000;

type TransferAnswer = { state?: unknown; address?: unknown; sizeBytes?: unknown };

function transferOf(body: unknown): TransferAnswer | null {
	const transfer = (body as { transfer?: unknown } | null)?.transfer;
	return typeof transfer === 'object' && transfer !== null ? transfer : null;
}

function refusalOf(answer: HostAnswer): string {
	const said = (answer.body as { error?: unknown } | null)?.error;
	return typeof said === 'string' && said.trim() !== '' ? said : `the company computer answered ${answer.status}`;
}

export function transferThroughTheHost(
	connection: HostConnection,
	call: HostCall,
	onProgress: TransferProgress = () => undefined,
	stallMilliseconds: number = stalledAfterMilliseconds
): Promise<unknown> {
	const transferID = crypto.randomUUID();
	return new Promise<unknown>((resolve, reject) => {
		let stallTimer: ReturnType<typeof setTimeout> | undefined;
		const settle = (finish: () => void): void => {
			clearTimeout(stallTimer);
			stopListening();
			finish();
		};
		const watchForStall = (): void => {
			clearTimeout(stallTimer);
			stallTimer = setTimeout(
				() => settle(() => reject(new TransferFailed(504, 'the company computer stopped saying how the copy is going'))),
				stallMilliseconds
			);
		};
		const stopListening = connection.listen((event) => {
			if (event.transferID !== transferID) return;
			if (event.kind === 'transfer.progress') {
				watchForStall();
				onProgress(event.copiedBytes ?? 0, event.totalBytes ?? 0);
				return;
			}
			if (event.kind === 'transfer.done') settle(() => resolve(event.result));
			if (event.kind === 'transfer.failed') {
				settle(() => reject(new TransferFailed(event.status ?? 502, event.error ?? 'the copy failed')));
			}
		});
		watchForStall();
		connection
			.call({ capability: call.capability, body: { ...call.body, transferID } })
			.then((answer) => {
				if (answer.status >= 400) return settle(() => reject(new TransferFailed(answer.status, refusalOf(answer))));
				const transfer = transferOf(answer.body);
				if (transfer?.state === 'ready') {
					return settle(() => resolve({ address: transfer.address, sizeBytes: transfer.sizeBytes }));
				}
				if (transfer?.state !== 'pending') {
					settle(() => reject(new TransferFailed(502, 'the company computer answered without a transfer')));
				}
			})
			.catch((failure: unknown) => settle(() => reject(failure)));
	});
}
