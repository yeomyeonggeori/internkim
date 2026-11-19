export type DownloadProgress = { receivedBytes: number; totalBytes: number; fraction: number | null };

export type DownloadState = { phase: 'running'; progress: DownloadProgress } | { phase: 'finished' };

export type ReportProgress = (progress: DownloadProgress) => void;

const finishedShownMilliseconds = 1800;
const notStarted: DownloadProgress = { receivedBytes: 0, totalBytes: 0, fraction: 0 };

function fractionOf(doneBytes: number, totalBytes: number): number | null {
	if (totalBytes <= 0) return null;
	return Math.min(1, doneBytes / totalBytes);
}

export function receivingProgress(receivedBytes: number, totalBytes: number): DownloadProgress {
	return { receivedBytes, totalBytes, fraction: fractionOf(receivedBytes, totalBytes) };
}

export function fetchingBeforeHandingOverProgress(receivedBytes: number, totalBytes: number): DownloadProgress {
	const fraction = fractionOf(receivedBytes, totalBytes);
	return { receivedBytes, totalBytes, fraction: fraction === null ? null : fraction / 2 };
}

export function handingOverAfterFetchingProgress(sentBytes: number, totalBytes: number): DownloadProgress {
	const fraction = fractionOf(sentBytes, totalBytes);
	return { receivedBytes: totalBytes, totalBytes, fraction: fraction === null ? null : 0.5 + fraction / 2 };
}

class DownloadProgressStore {
	private stateByIdentity = $state<Map<string, DownloadState>>(new Map());
	private clearTimers = new Map<string, ReturnType<typeof setTimeout>>();

	stateOf(identity: string): DownloadState | null {
		return this.stateByIdentity.get(identity) ?? null;
	}

	isRunning(identity: string): boolean {
		return this.stateOf(identity)?.phase === 'running';
	}

	async run(identity: string, work: (report: ReportProgress) => Promise<void>): Promise<void> {
		if (this.isRunning(identity)) return;
		this.cancelClearing(identity);
		this.set(identity, { phase: 'running', progress: notStarted });
		try {
			await work((progress) => {
				if (this.isRunning(identity)) this.set(identity, { phase: 'running', progress });
			});
		} catch (failure) {
			this.clear(identity);
			throw failure;
		}
		this.set(identity, { phase: 'finished' });
		this.clearTimers.set(
			identity,
			setTimeout(() => this.clear(identity), finishedShownMilliseconds)
		);
	}

	private set(identity: string, state: DownloadState): void {
		this.stateByIdentity = new Map(this.stateByIdentity).set(identity, state);
	}

	private clear(identity: string): void {
		this.cancelClearing(identity);
		const remaining = new Map(this.stateByIdentity);
		remaining.delete(identity);
		this.stateByIdentity = remaining;
	}

	private cancelClearing(identity: string): void {
		const timer = this.clearTimers.get(identity);
		if (timer !== undefined) clearTimeout(timer);
		this.clearTimers.delete(identity);
	}
}

export const downloadProgress = new DownloadProgressStore();
