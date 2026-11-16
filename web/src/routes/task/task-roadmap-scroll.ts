import type { Action } from 'svelte/action';
import type { TaskRoadmapRow } from './task-roadmap-model';

const rowSelector = '[data-roadmap-task]';
const userScrollEvents = ['wheel', 'touchstart', 'pointerdown', 'keydown'] as const;

export const onUserScrollIntent: Action<HTMLElement, () => void> = (node, handler) => {
	let currentHandler = handler;
	const handle = () => currentHandler();
	for (const eventName of userScrollEvents) node.addEventListener(eventName, handle, { passive: true });
	return {
		update(nextHandler) {
			currentHandler = nextHandler;
		},
		destroy() {
			for (const eventName of userScrollEvents) node.removeEventListener(eventName, handle);
		}
	};
};

export function taskRoadmapRevealedTaskID(rows: TaskRoadmapRow[]): string | undefined {
	const overlapping = rows.find((row) => row.bar.kind === 'span');
	if (overlapping) return overlapping.task.id;
	return rows.find((row) => row.bar.kind === 'after')?.task.id;
}

export function scrollTaskRoadmapRowToTop(frame: HTMLElement, taskID: string, reduceMotion: boolean): void {
	const firstRow = frame.querySelector(rowSelector);
	const row = frame.querySelector(`[data-roadmap-task="${CSS.escape(taskID)}"]`);
	if (!(firstRow instanceof HTMLElement) || !(row instanceof HTMLElement)) return;
	const top = row.getBoundingClientRect().top - firstRow.getBoundingClientRect().top;
	frame.scrollTo({ top, behavior: reduceMotion ? 'auto' : 'smooth' });
}
