import {
	binaryBodyFrames,
	binaryMediaBody,
	largestStreamFrameBytes,
	mediaBody,
	mediaMessage,
	type GatewayMediaMessage,
	type MediaOpen
} from '../../workers/connection-gateway/src/host-protocol';
import type { FairOutbox, OutboxDocument } from './fair-outbox';

export type RelayFetch = (url: string, init: RequestInit) => Promise<Response>;

type MediaRead = {
	aborter: AbortController;
	credit: number;
	creditArrived: (() => void) | null;
	isStoppedByTheApp: boolean;
};

const gatewayGoneReason = 'the gateway connection went away';

export class MessengerMediaReads {
	private readonly reads = new Map<string, MediaRead>();

	constructor(
		private readonly relayURL: string,
		private readonly outbox: FairOutbox,
		private readonly report: (line: string) => void,
		private readonly fetchRelay: RelayFetch = fetch
	) {}

	get openCount(): number {
		return this.reads.size;
	}

	take(message: GatewayMediaMessage): void {
		if (message.kind === 'media.open') return this.open(message);
		const read = this.reads.get(message.streamID);
		if (!read) return;
		if (message.kind === 'media.credit') return grantCredit(read, message.bytes);
		read.isStoppedByTheApp = true;
		this.outbox.forget(message.streamID);
		read.aborter.abort(new Error(message.reason || 'the app stopped reading'));
	}

	closeEverything(): void {
		for (const read of this.reads.values()) read.aborter.abort(new Error(gatewayGoneReason));
	}

	private open(open: MediaOpen): void {
		if (this.reads.has(open.streamID)) return;
		const read: MediaRead = { aborter: new AbortController(), credit: open.creditBytes, creditArrived: null, isStoppedByTheApp: false };
		this.reads.set(open.streamID, read);
		void this.serve(open, read).finally(() => this.reads.delete(open.streamID));
	}

	private async serve(open: MediaOpen, read: MediaRead): Promise<void> {
		try {
			const response = await this.fetchRelay(`${this.relayURL.replace(/\/+$/, '')}${open.path}`, {
				method: open.method,
				headers: { ...open.headers, Host: open.host },
				redirect: 'manual',
				signal: read.aborter.signal
			});
			this.send(open.streamID, mediaMessage({ kind: 'media.head', streamID: open.streamID, ...decodedHeadOf(response) }));
			if (response.body && open.method !== 'HEAD') await this.sendBody(open, read, response.body);
			this.send(open.streamID, mediaMessage({ kind: 'media.end', streamID: open.streamID }));
		} catch (error) {
			this.sayItFailed(open, read, reasonOf(error));
		}
	}

	private async sendBody(open: MediaOpen, read: MediaRead, body: ReadableStream<Uint8Array>): Promise<void> {
		const bodyFrameOf = open.bodyFrames === binaryBodyFrames ? binaryMediaBody : mediaBody;
		for await (const chunk of body) {
			let remaining = chunk;
			while (remaining.byteLength > 0) {
				await creditFor(read);
				const size = Math.min(remaining.byteLength, read.credit, largestStreamFrameBytes);
				this.send(open.streamID, bodyFrameOf(open.streamID, remaining.subarray(0, size)));
				read.credit -= size;
				remaining = remaining.subarray(size);
			}
		}
	}

	private send(streamID: string, document: OutboxDocument): void {
		if (!this.outbox.push(streamID, document)) throw new Error('the gateway connection is backed up');
	}

	private sayItFailed(open: MediaOpen, read: MediaRead, reason: string): void {
		if (read.isStoppedByTheApp) return;
		this.report(`messenger media ${open.method} ${open.path} failed: ${reason}`);
		if (read.aborter.signal.aborted) return;
		this.outbox.push(open.streamID, mediaMessage({ kind: 'media.fail', streamID: open.streamID, reason }));
	}
}

function grantCredit(read: MediaRead, bytes: number): void {
	read.credit += bytes;
	const arrived = read.creditArrived;
	read.creditArrived = null;
	arrived?.();
}

function creditFor(read: MediaRead): Promise<void> {
	if (read.credit > 0) return Promise.resolve();
	const signal = read.aborter.signal;
	if (signal.aborted) return Promise.reject(signal.reason);
	return new Promise((resolve, reject) => {
		const stop = () => reject(signal.reason);
		signal.addEventListener('abort', stop, { once: true });
		read.creditArrived = () => {
			signal.removeEventListener('abort', stop);
			resolve();
		};
	});
}

function decodedHeadOf(response: Response): { status: number; headers: Record<string, string> } {
	const headers: Record<string, string> = {};
	response.headers.forEach((value, name) => {
		headers[name] = value;
	});
	if (headers['content-encoding']) {
		delete headers['content-encoding'];
		delete headers['content-length'];
	}
	return { status: response.status, headers };
}

function reasonOf(error: unknown): string {
	return error instanceof Error ? error.message : String(error);
}
