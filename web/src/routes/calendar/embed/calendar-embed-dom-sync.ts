import type { CalendarViewType, Event as DayFlowEvent } from '@dayflow/core';
import { syncCalendarAllDayLayout } from './calendar-all-day-layout';
import {
	enhanceDayFlowMiniCalendar,
	type DayFlowMiniCalendarEnhancementContext
} from './calendar-dayflow-mini-calendar-enhancement';
import {
	syncCalendarMobileTwoDayWeekLayout,
	type CalendarMobileTwoDayWeekLayoutContext
} from './calendar-mobile-two-day-week';
import { syncCalendarMultiDayProxyLayout } from './calendar-multi-day-proxy-layout';
import { syncTimelineEventLaneLayout } from './calendar-timeline-event-lanes';

type CalendarDOMSyncTask = () => void;
type CalendarDOMSyncRuntime = {
	requestAnimationFrame: (task: CalendarDOMSyncTask) => void;
	setTimeout: (task: CalendarDOMSyncTask, delay: number) => void;
};
type CalendarDOMSyncScheduler = (task: CalendarDOMSyncTask) => void;

const dayFlowMiniCalendarScheduler = createBrowserCalendarDOMSyncScheduler();
const allDayLayoutScheduler = createBrowserCalendarDOMSyncScheduler();
const multiDayProxyLayoutScheduler = createBrowserCalendarDOMSyncScheduler();
const timelineEventLaneLayoutScheduler = createBrowserCalendarDOMSyncScheduler();
const timelineBoundaryLabelScheduler = createBrowserCalendarDOMSyncScheduler();
const mobileTwoDayWeekLayoutScheduler = createBrowserCalendarDOMSyncScheduler();

export function scheduleDayFlowMiniCalendarEnhancement(context: DayFlowMiniCalendarEnhancementContext): void {
	dayFlowMiniCalendarScheduler(() => enhanceDayFlowMiniCalendar(context));
}

export function scheduleCalendarAllDayLayoutSync(
	stageElement: HTMLElement | null,
	currentView: CalendarViewType,
	currentDate: Date,
	events: () => DayFlowEvent[]
): void {
	allDayLayoutScheduler(() => syncCalendarAllDayLayout(stageElement, currentView, currentDate, events()));
}

export function scheduleCalendarMultiDayProxyLayoutSync(
	stageElement: HTMLElement | null,
	currentView: CalendarViewType,
	currentDate: Date,
	events: () => DayFlowEvent[]
): void {
	multiDayProxyLayoutScheduler(() =>
		syncCalendarMultiDayProxyLayout({
			stageElement,
			currentView,
			currentDate,
			events: events()
		})
	);
}

export function scheduleTimelineEventLaneLayoutSync(
	stageElement: HTMLElement | null,
	currentView: CalendarViewType,
	events: () => DayFlowEvent[]
): void {
	timelineEventLaneLayoutScheduler(() =>
		syncTimelineEventLaneLayout(
			stageElement,
			currentView,
			events()
		)
	);
}

export function scheduleTimelineBoundaryLabelSync(stageElement: HTMLElement | null): void {
	timelineBoundaryLabelScheduler(() => syncTimelineBoundaryLabels(stageElement));
}

export function scheduleCalendarMobileTwoDayWeekLayoutSync(context: CalendarMobileTwoDayWeekLayoutContext): void {
	mobileTwoDayWeekLayoutScheduler(() => syncCalendarMobileTwoDayWeekLayout(context));
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

function syncTimelineBoundaryLabels(stageElement: HTMLElement | null): void {
	if (!stageElement) return;
	for (const label of stageElement.querySelectorAll<HTMLElement>(
		'.df-week-time-grid-boundary-tail > .df-time-label, .df-day-content-grid-boundary-bottom .df-midnight-label'
	)) {
		if (label.textContent?.trim() === '00:00') {
			label.textContent = '24:00';
		}
	}
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
