import { binaryBodyFrames, mediaCreditWindowBytes, mediaMessage, type HostMediaMessage, type MediaHead } from './host-protocol';

export const mediaReadsPerCompany = 32;
export const mediaHeadBoundMilliseconds = 60_000;
export const mediaStallBoundMilliseconds = 60_000;
export const relayContentLengthHeader = 'x-internkim-relay-content-length';

export type MediaRequest = { method: string; path: string; host: string; headers: Record<string, string> };

type SendToHost = (document: string) => boolean;

const statusesWithoutBody = new Set([204, 205, 304]);

export class MediaReads {
	private readonly reads = new Map<string, MediaRead>();

	constructor(private readonly sendToHost: SendToHost) {}

	get openCount(): number {
		return this.reads.size;
	}

	get isFull(): boolean {
		return this.reads.size >= mediaReadsPerCompany;
	}

	open(request: MediaRequest): Promise<Response> {
		const streamID = crypto.randomUUID();
		const read = new MediaRead(streamID, request.method, this.sendToHost, () => this.reads.delete(streamID));
		this.reads.set(streamID, read);
		const opened = this.sendToHost(mediaMessage({ kind: 'media.open', streamID, ...request, creditBytes: mediaCreditWindowBytes, bodyFrames: binaryBodyFrames }));
		if (!opened) read.fail(502, 'the company computer could not be asked for this blob');
		return read.response;
	}

	take(message: HostMediaMessage): void {
		const read = this.reads.get(message.streamID);
		if (read) return read.take(message);
		if (message.kind === 'media.head' || message.kind === 'media.body') {
			this.sendToHost(mediaMessage({ kind: 'media.stop', streamID: message.streamID, reason: 'the app has gone' }));
		}
	}

	failEverything(reason: string): void {
		for (const read of [...this.reads.values()]) read.fail(502, reason);
	}
}

class MediaRead {
	readonly response: Promise<Response>;
	private answer: (response: Response) => void = () => undefined;
	private controller: ReadableStreamDefaultController<Uint8Array> | null = null;
	private readonly queued: Uint8Array<ArrayBuffer>[] = [];
	private hasEnded = false;
	private isFinished = false;
	private waitingPull: (() => void) | null = null;
	private bound: ReturnType<typeof setTimeout> | undefined;

	constructor(
		private readonly streamID: string,
		private readonly method: string,
		private readonly sendToHost: SendToHost,
		private readonly forget: () => void
	) {
		this.response = new Promise((resolve) => {
			this.answer = resolve;
		});
		this.bound = setTimeout(
			() => this.fail(504, `the company computer did not start answering within ${mediaHeadBoundMilliseconds / 1000} seconds`),
			mediaHeadBoundMilliseconds
		);
	}

	take(message: HostMediaMessage): void {
		if (message.kind === 'media.fail') return this.fail(502, message.reason || 'the company computer could not read this blob');
		if (message.kind === 'media.head') return this.start(message);
		if (!this.controller) return this.fail(502, 'the company computer sent a body before its status');
		if (message.kind === 'media.body') return this.arrive(message.bytes);
		this.hasEnded = true;
		this.wakeWaitingPull();
	}

	fail(status: number, reason: string): void {
		if (this.isFinished) return;
		if (this.controller) this.controller.error(new Error(reason));
		else this.answer(failureResponse(status, reason));
		this.abandon(reason);
	}

	private abandon(reason: string): void {
		if (this.isFinished) return;
		this.sendToHost(mediaMessage({ kind: 'media.stop', streamID: this.streamID, reason }));
		this.finish();
		this.waitingPull?.();
		this.waitingPull = null;
	}

	private start(head: MediaHead): void {
		if (this.controller || head.status < 200 || head.status > 599) return this.fail(502, `the company computer answered status ${head.status}`);
		clearTimeout(this.bound);
		if (this.method === 'HEAD' || statusesWithoutBody.has(head.status)) {
			this.answer(new Response(null, { status: head.status, headers: bodilessHeadersOf(head.headers) }));
			this.finish();
			return;
		}
		const body = new ReadableStream<Uint8Array>(
			{
				start: (controller) => {
					this.controller = controller;
				},
				pull: () => this.pull(),
				cancel: () => this.abandon('the app stopped reading')
			},
			{ highWaterMark: 0 }
		);
		this.answer(new Response(bodyOfKnownLength(body, head.headers), { status: head.status, headers: head.headers }));
	}

	private pull(): Promise<void> | void {
		if (this.deliverNext()) return;
		return new Promise((resolve) => {
			this.waitingPull = resolve;
			this.bound = setTimeout(
				() => this.fail(504, `the company computer sent nothing for ${mediaStallBoundMilliseconds / 1000} seconds`),
				mediaStallBoundMilliseconds
			);
		});
	}

	private arrive(bytes: Uint8Array<ArrayBuffer>): void {
		this.queued.push(bytes);
		this.wakeWaitingPull();
	}

	private wakeWaitingPull(): void {
		const resolve = this.waitingPull;
		if (!resolve) return;
		clearTimeout(this.bound);
		this.waitingPull = null;
		this.deliverNext();
		resolve();
	}

	private deliverNext(): boolean {
		const next = this.queued.shift();
		if (next) {
			this.controller?.enqueue(next);
			this.sendToHost(mediaMessage({ kind: 'media.credit', streamID: this.streamID, bytes: next.byteLength }));
			return true;
		}
		if (!this.hasEnded) return false;
		this.controller?.close();
		this.finish();
		return true;
	}

	private finish(): void {
		this.isFinished = true;
		clearTimeout(this.bound);
		this.forget();
	}
}

function bodyOfKnownLength(body: ReadableStream<Uint8Array>, headers: Record<string, string>): ReadableStream<Uint8Array> {
	const declared = headers['content-length'];
	const length = Number(declared);
	if (!declared || !Number.isSafeInteger(length) || length < 0) return body;
	return body.pipeThrough(new FixedLengthStream(length));
}

function bodilessHeadersOf(headers: Record<string, string>): Record<string, string> {
	const length = headers['content-length'];
	return length ? { ...headers, [relayContentLengthHeader]: length } : headers;
}

function failureResponse(status: number, reason: string): Response {
	return new Response(JSON.stringify({ error: reason }), { status, headers: { 'Content-Type': 'application/json' } });
}
