export type QueuedWork = {
	requestID: string;
	affinityKey: string;
	onlyBoxKey?: string;
	work: unknown;
	deadline: number;
};

export type Dispatch = { boxKey: string; item: QueuedWork };

export class WorkQueue {
	private readonly waiting: QueuedWork[] = [];
	private readonly freeSlots = new Map<string, number>();
	private readonly lastBoxOf = new Map<string, string>();

	get waitingCount(): number {
		return this.waiting.length;
	}

	setFree(boxKey: string, free: number): Dispatch[] {
		this.freeSlots.set(boxKey, Math.max(0, Math.floor(free)));
		return this.dispatchWaiting();
	}

	forget(boxKey: string): QueuedWork[] {
		this.freeSlots.delete(boxKey);
		const stranded = this.waiting.filter((item) => item.onlyBoxKey === boxKey);
		for (const item of stranded) this.withdraw(item.requestID);
		return stranded;
	}

	post(item: QueuedWork): Dispatch | null {
		const boxKey = this.boxFor(item);
		if (!boxKey) {
			this.waiting.push(item);
			return null;
		}
		return this.take(boxKey, item);
	}

	withdraw(requestID: string): boolean {
		const index = this.waiting.findIndex((item) => item.requestID === requestID);
		if (index < 0) return false;
		this.waiting.splice(index, 1);
		return true;
	}

	expire(now: number): QueuedWork[] {
		const expired = this.waiting.filter((item) => item.deadline <= now);
		for (const item of expired) this.withdraw(item.requestID);
		return expired;
	}

	private dispatchWaiting(): Dispatch[] {
		const dispatched: Dispatch[] = [];
		for (const item of [...this.waiting]) {
			const boxKey = this.boxFor(item);
			if (!boxKey) continue;
			this.withdraw(item.requestID);
			dispatched.push(this.take(boxKey, item));
		}
		return dispatched;
	}

	private boxFor(item: QueuedWork): string | null {
		if (item.onlyBoxKey) return this.hasRoom(item.onlyBoxKey) ? item.onlyBoxKey : null;
		const last = this.lastBoxOf.get(item.affinityKey);
		if (last && this.hasRoom(last)) return last;
		for (const [boxKey, free] of this.freeSlots) {
			if (free > 0) return boxKey;
		}
		return null;
	}

	private hasRoom(boxKey: string): boolean {
		return (this.freeSlots.get(boxKey) ?? 0) > 0;
	}

	private take(boxKey: string, item: QueuedWork): Dispatch {
		this.freeSlots.set(boxKey, (this.freeSlots.get(boxKey) ?? 1) - 1);
		this.lastBoxOf.set(item.affinityKey, boxKey);
		return { boxKey, item };
	}
}
