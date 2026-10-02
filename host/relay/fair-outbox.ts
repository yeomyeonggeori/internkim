export type Outlet = {
	send: (document: string) => void;
	bufferedBytes: () => number;
};

export const outletHighWaterBytes = 1024 * 1024;
export const largestQueuedBytesPerStream = 8 * 1024 * 1024;
const retryMilliseconds = 5;

type StreamQueue = { documents: string[]; queuedBytes: number };

export class FairOutbox {
	private readonly queues = new Map<string, StreamQueue>();
	private isDrainScheduled = false;

	constructor(
		private readonly outlet: () => Outlet | null,
		private readonly highWaterBytes = outletHighWaterBytes,
		private readonly largestQueuedBytes = largestQueuedBytesPerStream
	) {}

	push(streamID: string, document: string): boolean {
		const queue = this.queues.get(streamID) ?? { documents: [], queuedBytes: 0 };
		if (queue.queuedBytes + document.length > this.largestQueuedBytes) return false;
		queue.documents.push(document);
		queue.queuedBytes += document.length;
		this.queues.set(streamID, queue);
		this.drain();
		return true;
	}

	forget(streamID: string): void {
		this.queues.delete(streamID);
	}

	forgetEverything(): void {
		this.queues.clear();
	}

	get queuedStreams(): number {
		return this.queues.size;
	}

	private drain(): void {
		const outlet = this.outlet();
		if (!outlet) return;
		while (this.queues.size > 0 && outlet.bufferedBytes() < this.highWaterBytes) {
			this.sendFromTheStreamWhoseTurnItIs(outlet);
		}
		if (this.queues.size > 0) this.drainLater();
	}

	private sendFromTheStreamWhoseTurnItIs(outlet: Outlet): void {
		const [streamID, queue] = this.queues.entries().next().value ?? [];
		if (streamID === undefined || !queue) return;
		this.queues.delete(streamID);
		const document = queue.documents.shift();
		if (document === undefined) return;
		queue.queuedBytes -= document.length;
		if (queue.documents.length > 0) this.queues.set(streamID, queue);
		outlet.send(document);
	}

	private drainLater(): void {
		if (this.isDrainScheduled) return;
		this.isDrainScheduled = true;
		setTimeout(() => {
			this.isDrainScheduled = false;
			this.drain();
		}, retryMilliseconds);
	}
}
