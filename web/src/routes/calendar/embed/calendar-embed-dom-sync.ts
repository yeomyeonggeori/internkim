import type { CalendarViewType, Event as DayFlowEvent } from '@dayflow/core';
import { syncCalendarAllDayLayout } from './calendar-all-day-layout';
import {
	enhanceDayFlowMiniCalendar,
	type DayFlowMiniCalendarEnhancementContext
} from './calendar-dayflow-mini-calendar-enhancement';
import { syncCalendarMultiDayProxyLayout } from './calendar-multi-day-proxy-layout';
import type { DraftPopoverAnchor } from './calendar-draft-popover-state';

type CalendarDOMSyncTask = () => void;
type CalendarDOMSyncRuntime = {
	requestAnimationFrame: (task: CalendarDOMSyncTask) => void;
	setTimeout: (task: CalendarDOMSyncTask, delay: number) => void;
};
type CalendarDOMSyncScheduler = (task: CalendarDOMSyncTask) => void;

const dayFlowMiniCalendarScheduler = createBrowserCalendarDOMSyncScheduler();
const allDayLayoutScheduler = createBrowserCalendarDOMSyncScheduler();
const multiDayProxyLayoutScheduler = createBrowserCalendarDOMSyncScheduler();

export function scheduleDayFlowMiniCalendarEnhancement(context: DayFlowMiniCalendarEnhancementContext): void {
	dayFlowMiniCalendarScheduler(() => enhanceDayFlowMiniCalendar(context));
}

export function scheduleCalendarAllDayLayoutSync(stageElement: HTMLElement | null, currentView: CalendarViewType): void {
	allDayLayoutScheduler(() => syncCalendarAllDayLayout(stageElement, currentView));
}

export function scheduleCalendarMultiDayProxyLayoutSync(
	stageElement: HTMLElement | null,
	currentView: CalendarViewType,
	currentDate: Date,
	events: () => DayFlowEvent[],
	openEvent: (eventID: string, anchor: DraftPopoverAnchor) => void
): void {
	multiDayProxyLayoutScheduler(() =>
		syncCalendarMultiDayProxyLayout({
			stageElement,
			currentView,
			currentDate,
			events: events(),
			openEvent
		})
	);
}

export function createCalendarDOMSyncScheduler(runtime: CalendarDOMSyncRuntime): CalendarDOMSyncScheduler {
	let generation = 0;
	return (task) => {
		generation += 1;
		const taskGeneration = generation;
		const runLatestTask = () => {
			if (taskGeneration !== generation) return;
			task();
		};
		runtime.requestAnimationFrame(runLatestTask);
		runtime.requestAnimationFrame(() => {
			if (taskGeneration !== generation) return;
			runtime.requestAnimationFrame(runLatestTask);
		});
		for (const delay of [50, 150, 300, 600, 1000]) {
			runtime.setTimeout(runLatestTask, delay);
		}
	};
}

function createBrowserCalendarDOMSyncScheduler(): CalendarDOMSyncScheduler {
	const runtime = browserCalendarDOMSyncRuntime();
	if (!runtime) return () => {};
	return createCalendarDOMSyncScheduler(runtime);
}

function browserCalendarDOMSyncRuntime(): CalendarDOMSyncRuntime | null {
	if (typeof requestAnimationFrame === 'undefined' || typeof window === 'undefined') return null;
	return {
		requestAnimationFrame: (task) => requestAnimationFrame(task),
		setTimeout: (task, delay) => {
			window.setTimeout(task, delay);
		}
	};
}
