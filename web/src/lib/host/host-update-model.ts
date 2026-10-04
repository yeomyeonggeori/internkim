import type { HostRelease, HostVersion } from './host-version';

export const updatePatienceMilliseconds = 10 * 60 * 1000;
export const updateCheckMilliseconds = 5000;

export type HostActions =
	| { kind: 'updating' }
	| { kind: 'brew' }
	| { kind: 'unpackaged' }
	| { kind: 'offStable' }
	| { kind: 'choices'; update: HostRelease | null; goBack: HostRelease | null };

export type HostReading = { isAnswered: true; version: HostVersion } | { isAnswered: false };

export type UpdateWatch = { fromVersion: string; toVersion: string; startedAt: number };

export type UpdateStage =
	| { kind: 'installing' }
	| { kind: 'restarting' }
	| { kind: 'overdue' }
	| { kind: 'finished' }
	| { kind: 'failed'; reason: string };

export function hostActionsFor(version: HostVersion): HostActions {
	if (version.updateInProgress) return { kind: 'updating' };
	if (version.updateMethod === 'brew') return { kind: 'brew' };
	if (version.updateMethod === '') return { kind: 'unpackaged' };
	if (version.channel !== 'stable') return { kind: 'offStable' };
	return {
		kind: 'choices',
		update: version.isUpdateAvailable ? (version.latestStable ?? null) : null,
		goBack: version.previousStable ?? null
	};
}

export function watchOfTheRunningUpdate(version: HostVersion): UpdateWatch | null {
	const running = version.updateInProgress;
	if (!running) return null;
	return { fromVersion: running.fromVersion, toVersion: running.toVersion, startedAt: Date.parse(running.startedAt) };
}

export function stageOfTheUpdate(watch: UpdateWatch, reading: HostReading, now: number): UpdateStage {
	if (reading.isAnswered && hasArrived(watch, reading.version)) return { kind: 'finished' };
	if (reading.isAnswered && !reading.version.updateInProgress) {
		return { kind: 'failed', reason: failureReasonOf(watch, reading.version) };
	}
	if (now - watch.startedAt >= updatePatienceMilliseconds) return { kind: 'overdue' };
	return reading.isAnswered ? { kind: 'installing' } : { kind: 'restarting' };
}

export function isSettled(stage: UpdateStage): boolean {
	return stage.kind === 'finished' || stage.kind === 'failed';
}

export function refusalMessageOf(
	errorCode: string | undefined,
	refusals: Readonly<Record<string, string>>,
	fallback: string
): string {
	return (errorCode && refusals[errorCode]) || fallback;
}

function hasArrived(watch: UpdateWatch, version: HostVersion): boolean {
	return version.installedVersion === watch.toVersion && !version.updateInProgress;
}

function failureReasonOf(watch: UpdateWatch, version: HostVersion): string {
	const lastUpdate = version.lastUpdate;
	if (!lastUpdate || lastUpdate.toVersion !== watch.toVersion) return '';
	return lastUpdate.error ?? '';
}
