import type { Action } from 'svelte/action';
import { taskWeekDateFromISO } from '$lib/task/task-week-code';
import { isTaskRoadmapStartAllowed, taskRoadmapWeekStart } from './task-roadmap-window';

export type TaskViewTab = 'board' | 'list' | 'roadmap';

export type TaskViewMemory = {
	tab: TaskViewTab;
	roadmapStartISO: string;
	roadmapScrollTop: number;
};

type StoredRecord = Record<string, unknown>;

const taskViewMemoryKey = 'internkim:task-view:v1';

export function isTaskViewTab(value: unknown): value is TaskViewTab {
	return value === 'board' || value === 'list' || value === 'roadmap';
}

export function rememberedTaskView(todayISO: string): TaskViewMemory {
	const stored = readStoredRecord();
	return {
		tab: isTaskViewTab(stored.tab) ? stored.tab : 'board',
		roadmapStartISO: rememberedRoadmapStart(stored.roadmapStartISO, todayISO),
		roadmapScrollTop: rememberedScrollTop(stored.roadmapScrollTop)
	};
}

export function rememberTaskView(change: Partial<TaskViewMemory>): void {
	writeStoredRecord({ ...readStoredRecord(), ...change });
}

export const rememberTaskRoadmapScroll: Action<HTMLElement> = (node) => {
	let animationFrameID = requestAnimationFrame(() => {
		node.scrollTop = rememberedScrollTop(readStoredRecord().roadmapScrollTop);
		animationFrameID = 0;
	});
	const saveScrollTop = () => {
		if (animationFrameID) return;
		animationFrameID = requestAnimationFrame(() => {
			rememberTaskView({ roadmapScrollTop: node.scrollTop });
			animationFrameID = 0;
		});
	};
	node.addEventListener('scroll', saveScrollTop, { passive: true });
	return {
		destroy() {
			if (animationFrameID) cancelAnimationFrame(animationFrameID);
			node.removeEventListener('scroll', saveScrollTop);
		}
	};
};

function rememberedRoadmapStart(value: unknown, todayISO: string): string {
	if (typeof value !== 'string' || !taskWeekDateFromISO(value)) return taskRoadmapWeekStart(todayISO);
	if (!isTaskRoadmapStartAllowed(value, todayISO)) return taskRoadmapWeekStart(todayISO);
	return value;
}

function rememberedScrollTop(value: unknown): number {
	if (typeof value !== 'number' || !Number.isFinite(value) || value < 0) return 0;
	return value;
}

function isStoredRecord(value: unknown): value is StoredRecord {
	return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function readStoredRecord(): StoredRecord {
	if (typeof window === 'undefined') return {};
	try {
		const stored = window.sessionStorage.getItem(taskViewMemoryKey);
		if (stored === null) return {};
		const parsed: unknown = JSON.parse(stored);
		return isStoredRecord(parsed) ? parsed : {};
	} catch (error: unknown) {
		console.warn('task view memory unreadable', { key: taskViewMemoryKey, error });
		return {};
	}
}

function writeStoredRecord(record: StoredRecord): void {
	if (typeof window === 'undefined') return;
	try {
		window.sessionStorage.setItem(taskViewMemoryKey, JSON.stringify(record));
	} catch (error: unknown) {
		console.warn('task view memory not saved', { key: taskViewMemoryKey, error });
	}
}
