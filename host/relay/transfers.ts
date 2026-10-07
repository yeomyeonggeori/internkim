import { reasonOf } from './failure';
import type { CopyProgress } from './transfer-store';

export type TransferWatcher = { memberID: string; transferID: string };

export type TransferEvent =
	| { kind: 'transfer.progress'; transferID: string; copiedBytes: number; totalBytes: number }
	| { kind: 'transfer.done'; transferID: string; result: unknown }
	| { kind: 'transfer.failed'; transferID: string; status: number; error: string };

export type DeliverToMember = (event: TransferEvent, memberID: string) => void;

export type TransferWork = (progress: CopyProgress) => Promise<unknown>;

type Job = {
	watchers: TransferWatcher[];
	copiedBytes: number;
	totalBytes: number;
	toldAt: number;
};

const progressIntervalMilliseconds = 1_000;
const stillGoingIntervalMilliseconds = 15_000;

export class Transfers {
	private readonly running = new Map<string, Job>();

	constructor(
		private readonly deliver: DeliverToMember,
		private readonly now: () => number = Date.now,
		private readonly stillGoingEvery: number = stillGoingIntervalMilliseconds
	) {}

	isRunning(key: string): boolean {
		return this.running.has(key);
	}

	start(key: string, watcher: TransferWatcher, work: TransferWork): void {
		const already = this.running.get(key);
		if (already) {
			already.watchers.push(watcher);
			if (already.totalBytes > 0) this.tellProgress(watcher, already);
			return;
		}
		const job: Job = { watchers: [watcher], copiedBytes: 0, totalBytes: 0, toldAt: 0 };
		this.running.set(key, job);
		void this.run(key, job, work);
	}

	private async run(key: string, job: Job, work: TransferWork): Promise<void> {
		const stillGoing = setInterval(() => {
			for (const watcher of job.watchers) this.tellProgress(watcher, job);
		}, this.stillGoingEvery);
		try {
			const result = await work((copiedBytes, totalBytes) => this.advance(job, copiedBytes, totalBytes));
			this.running.delete(key);
			for (const watcher of job.watchers) {
				this.deliver({ kind: 'transfer.done', transferID: watcher.transferID, result }, watcher.memberID);
			}
		} catch (failure) {
			this.running.delete(key);
			const status = statusOf(failure);
			const error = reasonOf(failure);
			for (const watcher of job.watchers) {
				this.deliver({ kind: 'transfer.failed', transferID: watcher.transferID, status, error }, watcher.memberID);
			}
		} finally {
			clearInterval(stillGoing);
		}
	}

	private advance(job: Job, copiedBytes: number, totalBytes: number): void {
		job.copiedBytes = copiedBytes;
		job.totalBytes = totalBytes;
		const isDue = this.now() - job.toldAt >= progressIntervalMilliseconds;
		if (!isDue || copiedBytes >= totalBytes) return;
		job.toldAt = this.now();
		for (const watcher of job.watchers) this.tellProgress(watcher, job);
	}

	private tellProgress(watcher: TransferWatcher, job: Job): void {
		this.deliver(
			{ kind: 'transfer.progress', transferID: watcher.transferID, copiedBytes: job.copiedBytes, totalBytes: job.totalBytes },
			watcher.memberID
		);
	}
}

function statusOf(failure: unknown): number {
	const status = (failure as { status?: unknown } | null)?.status;
	return typeof status === 'number' && status >= 400 ? status : 502;
}
