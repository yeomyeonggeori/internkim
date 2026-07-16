import {
	createAttendanceServerClock,
	type AttendanceServerClock
} from './shared/attendance-server-clock';

export type AttendanceServerClockSyncSource = Readonly<{
	month: string;
	serverTime?: unknown;
}>;

type AttendanceServerClockSyncConfiguration<Summary extends AttendanceServerClockSyncSource> = Readonly<{
	requestSummary: (month: string) => Promise<Summary>;
	applySummarySnapshot: (summary: Summary, serverClock: AttendanceServerClock | null) => void;
	monotonicNow?: () => number;
}>;

type LoadedSummaryApplication<Summary> = Readonly<{
	loadTarget: string;
	loadSequence: number;
	applyLoadedSummary: (summary: Summary) => void;
}>;

const defaultLoadTarget = 'default';

export class AttendanceServerClockSync<Summary extends AttendanceServerClockSyncSource> {
	private readonly requestSummary: (month: string) => Promise<Summary>;
	private readonly applySummarySnapshot: (
		summary: Summary,
		serverClock: AttendanceServerClock | null
	) => void;
	private readonly monotonicNow: () => number;
	private requestSequence = 0;
	private readonly latestLoadSequences = new Map<string, number>();
	private readonly appliedLoadMonths = new Map<string, string>();
	private appliedSnapshotSequence = 0;
	private appliedSummarySnapshot: {
		summary: Summary;
		serverClock: AttendanceServerClock | null;
	} | null = null;
	private refreshPromise: Promise<boolean> | null = null;

	constructor(configuration: AttendanceServerClockSyncConfiguration<Summary>) {
		this.requestSummary = configuration.requestSummary;
		this.applySummarySnapshot = configuration.applySummarySnapshot;
		this.monotonicNow = configuration.monotonicNow ?? (() => performance.now());
	}

	async loadSummary(
		month: string,
		applyLoadedSummary: (summary: Summary) => void,
		loadTarget: string = defaultLoadTarget
	): Promise<Summary | null> {
		const result = await this.requestSummaryAndApplyClock(month, {
			loadTarget,
			loadSequence: this.allocateLoadSequence(loadTarget),
			applyLoadedSummary
		});
		return result?.isApplied ? result.summary : null;
	}

	applySummaryToLoadTarget(
		summary: Summary,
		applyLoadedSummary: (summary: Summary) => void,
		loadTarget: string = defaultLoadTarget
	): void {
		this.applyLoadedSummary(summary, {
			loadTarget,
			loadSequence: this.allocateLoadSequence(loadTarget),
			applyLoadedSummary
		});
	}

	invalidateLoadTarget(loadTarget: string = defaultLoadTarget): void {
		this.allocateLoadSequence(loadTarget);
	}

	refresh(month: string): Promise<boolean> {
		if (this.refreshPromise) return this.refreshPromise;
		const requestPromise = this.requestRefresh(month);
		const refreshPromise = requestPromise.finally(() => {
			if (this.refreshPromise === refreshPromise) this.refreshPromise = null;
		});
		this.refreshPromise = refreshPromise;
		return refreshPromise;
	}

	private async requestSummaryAndApplyClock(
		month: string,
		loadedSummaryApplication?: LoadedSummaryApplication<Summary>
	): Promise<{ summary: Summary; hasServerClock: boolean; isApplied: boolean } | null> {
		const requestSequence = this.allocateRequestSequence();
		try {
			const summary = await this.requestSummary(month);
			const result = this.applySummaryClock(summary, requestSequence, loadedSummaryApplication);
			return { summary, ...result };
		} catch (error) {
			if (
				loadedSummaryApplication &&
				(loadedSummaryApplication.loadSequence <
						(this.latestLoadSequences.get(loadedSummaryApplication.loadTarget) ?? 0) ||
					(this.appliedLoadMonths.get(loadedSummaryApplication.loadTarget) === month &&
						this.appliedSummarySnapshot?.summary.month === month &&
						requestSequence < this.appliedSnapshotSequence))
			) {
				return null;
			}
			throw error;
		}
	}

	private allocateRequestSequence(): number {
		this.requestSequence += 1;
		return this.requestSequence;
	}

	private allocateLoadSequence(loadTarget: string): number {
		const loadSequence = (this.latestLoadSequences.get(loadTarget) ?? 0) + 1;
		this.latestLoadSequences.set(loadTarget, loadSequence);
		return loadSequence;
	}

	private async requestRefresh(month: string): Promise<boolean> {
		try {
			const result = await this.requestSummaryAndApplyClock(month);
			return result?.hasServerClock ?? false;
		} catch (error) {
			if (error instanceof Error) return false;
			throw error;
		}
	}

	private applySummaryClock(
		summary: Summary,
		requestSequence: number,
		loadedSummaryApplication?: LoadedSummaryApplication<Summary>
	): { hasServerClock: boolean; isApplied: boolean } {
		const serverClock = createAttendanceServerClock(summary.serverTime, this.monotonicNow());
		if (
			loadedSummaryApplication &&
			loadedSummaryApplication.loadSequence <
				(this.latestLoadSequences.get(loadedSummaryApplication.loadTarget) ?? 0)
		) {
			return { hasServerClock: serverClock !== null, isApplied: false };
		}
		if (loadedSummaryApplication) {
			this.applyLoadedSummary(summary, loadedSummaryApplication);
		}
		const isSnapshotApplied = this.applySummarySnapshotIfNewer(
			summary,
			serverClock,
			requestSequence
		);
		if (loadedSummaryApplication && !isSnapshotApplied) this.reapplyLatestSummarySnapshot();
		return {
			hasServerClock: serverClock !== null,
			isApplied: loadedSummaryApplication !== undefined || isSnapshotApplied
		};
	}

	private applyLoadedSummary(
		summary: Summary,
		loadedSummaryApplication: LoadedSummaryApplication<Summary>
	): void {
		loadedSummaryApplication.applyLoadedSummary(summary);
		this.appliedLoadMonths.set(loadedSummaryApplication.loadTarget, summary.month);
	}

	private applySummarySnapshotIfNewer(
		summary: Summary,
		serverClock: AttendanceServerClock | null,
		requestSequence: number
	): boolean {
		if (requestSequence <= this.appliedSnapshotSequence) return false;
		this.appliedSnapshotSequence = requestSequence;
		this.appliedSummarySnapshot = { summary, serverClock };
		this.applySummarySnapshot(summary, serverClock);
		return true;
	}

	private reapplyLatestSummarySnapshot(): void {
		if (!this.appliedSummarySnapshot) return;
		this.applySummarySnapshot(
			this.appliedSummarySnapshot.summary,
			this.appliedSummarySnapshot.serverClock
		);
	}
}
