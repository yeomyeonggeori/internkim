import type { TaskState } from './task-types';

const storageKey = 'internkim:task-snapshot:v1';
const boardStorageKey = 'internkim:task-board-snapshots:v1';
const maximumBoards = 6;
const maximumAge = 24 * 60 * 60 * 1000;
let generation = 0;
const invalidationListeners = new Set<() => void>();

export function taskSnapshotGeneration(): number { return generation; }

export function subscribeTaskSnapshotInvalidation(listener: () => void): () => void {
	invalidationListeners.add(listener);
	return () => { invalidationListeners.delete(listener); };
}

type Snapshot = { version: 1; scope: string; savedAt: number; state: TaskState };

export function storedTaskSnapshot(scope: string, now = Date.now()): Snapshot | null {
	if (!scope || typeof window === 'undefined') return null;
	try {
		const written = window.localStorage.getItem(storageKey);
		if (!written || written.length > 4_000_000) return null;
		const value = JSON.parse(written) as Snapshot;
		if (value.version !== 1 || value.scope !== scope || !Number.isFinite(value.savedAt) || now < value.savedAt || now - value.savedAt > maximumAge || !isTaskState(value.state) || value.state.completeness !== 'full') {
			window.localStorage.removeItem(storageKey);
			return null;
		}
		return value;
	} catch { return null; }
}

export function storeTaskSnapshot(scope: string, state: TaskState, savedAt = Date.now()): void {
	if (!scope || typeof window === 'undefined' || !isTaskState(state) || !state.peopleReady || state.completeness !== 'full') return;
	try { window.localStorage.setItem(storageKey, JSON.stringify({ version: 1, scope, savedAt, state })); } catch {}
}

export function clearStoredTaskSnapshot(scope?: string): void {
	generation++;
	for (const listener of invalidationListeners) listener();
	if (typeof window === 'undefined') return;
	try {
		if (!scope || JSON.parse(window.localStorage.getItem(storageKey) ?? 'null')?.scope === scope) window.localStorage.removeItem(storageKey);
	} catch {}
	try {
		const kept = scope ? storedBoards().filter(snapshot => snapshot.scope !== scope) : [];
		if (kept.length) window.localStorage.setItem(boardStorageKey, JSON.stringify(kept));
		else window.localStorage.removeItem(boardStorageKey);
	} catch {}
}

export function storedTaskBoardSnapshot(scope: string, boardWeek: string, now = Date.now()): Snapshot | null {
	return storedBoards(now).find(snapshot => snapshot.scope === scope && snapshot.state.boardWeek === boardWeek) ?? null;
}

export function storeTaskBoardSnapshot(scope: string, state: TaskState, savedAt = Date.now()): void {
	if (!scope || typeof window === 'undefined' || !isTaskState(state) || !state.peopleReady || state.completeness !== 'board' || !state.boardWeek) return;
	const kept = storedBoards(savedAt).filter(snapshot => snapshot.scope !== scope || snapshot.state.boardWeek !== state.boardWeek);
	const snapshots: Snapshot[] = [...kept, { version: 1, scope, savedAt, state }];
	try { window.localStorage.setItem(boardStorageKey, JSON.stringify(snapshots.slice(-maximumBoards))); } catch {}
}

function storedBoards(now = Date.now()): Snapshot[] {
	if (typeof window === 'undefined') return [];
	try {
		const written = window.localStorage.getItem(boardStorageKey);
		if (!written || written.length > 4_000_000) return [];
		const values: unknown = JSON.parse(written);
		if (!Array.isArray(values)) return [];
		return values.filter((value): value is Snapshot => isRecord(value)
			&& value.version === 1 && typeof value.scope === 'string' && typeof value.savedAt === 'number'
			&& Number.isFinite(value.savedAt) && now >= value.savedAt && now - value.savedAt <= maximumAge
			&& isTaskState(value.state) && value.state.completeness === 'board' && typeof value.state.boardWeek === 'string').slice(-maximumBoards);
	} catch { return []; }
}

function isRecord(value: unknown): value is Record<string, unknown> {
	return value !== null && typeof value === 'object';
}

function isTaskState(value: unknown): value is TaskState {
	if (!value || typeof value !== 'object') return false;
	const state = value as TaskState;
	if (state.peopleReady !== true) return false;
	const strings = (value: unknown): value is string[] => Array.isArray(value) && value.every(item => typeof item === 'string');
	const optionalString = (value: unknown) => value === undefined || typeof value === 'string';
	return (state.completeness === 'full' || state.completeness === 'board') && Array.isArray(state.tasks) && state.tasks.every(task => task && typeof task.id === 'string' && typeof task.content === 'string' && typeof task.status === 'string' && typeof task.ownerID === 'string' && typeof task.ownerName === 'string' && typeof task.size === 'string' && typeof task.weekCode === 'string' && optionalString(task.startDate) && optionalString(task.endDate) && optionalString(task.createdAt) && optionalString(task.parentTaskID) && optionalString(task.requesterID) && optionalString(task.requesterName) && (task.business === null || typeof task.business === 'string') && (task.type === null || typeof task.type === 'string') && strings(task.participantIDs) && strings(task.participantNames))
		&& Array.isArray(state.members) && state.members.every(member => member && typeof member.id === 'string' && typeof member.email === 'string' && typeof member.name === 'string' && typeof member.role === 'string' && Number.isFinite(member.activeTaskCount) && Number.isFinite(member.completeTaskCount))
		&& Array.isArray(state.statusOptions) && state.statusOptions.every(value => typeof value === 'string')
		&& !!state.metrics && typeof state.metrics.totalTasks === 'number' && !!state.metrics.statusCounts
		&& !!state.definitions && strings(state.definitions.categories) && strings(state.definitions.types) && Array.isArray(state.definitions.sizes) && state.definitions.sizes.every(size => size && typeof size.name === 'string' && typeof size.label === 'string' && Number.isFinite(size.score) && Number.isFinite(size.distanceKm) && Number.isFinite(size.maxHours))
		&& typeof state.currentUserEmail === 'string' && typeof state.currentUserName === 'string' && typeof state.isAdmin === 'boolean';
}
