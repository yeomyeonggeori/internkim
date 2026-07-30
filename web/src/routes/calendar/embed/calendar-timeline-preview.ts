import type { CalendarModelEvent as DayFlowEvent } from './calendar-event-model';
import type { TimelineRangeSelection } from './calendar-timeline-range-action';
import {
	orderedTimelineRangeDates,
	timelineEventsOverlappingRange,
	timelineOverlapWidthPercent,
	timelinePreviewOverlapWidth
} from './calendar-timeline-preview-model';

export { timelineRangePreviewSegments, type TimelineRangePreviewSegment } from './calendar-timeline-preview-model';

export function syncTimelinePreviewOverlapLayout(
	stageElement: HTMLElement | null,
	selection: TimelineRangeSelection | null,
	events: DayFlowEvent[]
): void {
	if (!stageElement) return;
	clearTimelineOverlapLayout(stageElement);
	if (!selection?.hasMoved) return;
	const [startDate, endDate] = orderedTimelineRangeDates(selection.startDate, selection.currentDate);
	const overlappingEvents = timelineEventsOverlappingRange(events, startDate, endDate);
	if (overlappingEvents.length === 0) return;
	const laneCount = overlappingEvents.length + 1;
	const laneWidth = timelineOverlapWidthPercent(laneCount);
	for (const element of timelineRangePreviewElements(stageElement)) {
		applyTimelinePreviewOverlapLane(element, laneCount);
	}
	overlappingEvents.forEach((event, index) => {
		for (const element of timelineEventElementsByID(stageElement, event.id)) {
			applyTimelineOverlapLane(element, index + 1, laneCount, laneWidth);
		}
	});
}

export function clearTimelineOverlapLayout(stageElement: HTMLElement | null): void {
	if (!stageElement) return;
	for (const element of stageElement.querySelectorAll<HTMLElement>('.calendar-timeline-overlap-adjusted')) {
		restoreTimelinePreviewWidth(element);
		element.classList.remove('calendar-timeline-overlap-adjusted');
		element.style.removeProperty('--calendar-overlap-left');
		element.style.removeProperty('--calendar-overlap-width');
	}
}

function timelineEventElementsByID(stageElement: HTMLElement, eventID: string): HTMLElement[] {
	const escapedEventID = window.CSS?.escape(eventID) ?? eventID.replaceAll('"', '\\"');
	return Array.from(stageElement.querySelectorAll<HTMLElement>(`[data-event-id="${escapedEventID}"]`)).filter(
		(element) => !element.closest('.df-right-panel-events, .df-right-panel-calendar-shell, .df-mini-calendar')
	);
}

function timelineRangePreviewElements(stageElement: HTMLElement): HTMLElement[] {
	return Array.from(stageElement.querySelectorAll<HTMLElement>('.timeline-range-preview'));
}

function applyTimelineOverlapLane(element: HTMLElement, laneIndex: number, laneCount: number, laneWidth: number): void {
	element.classList.add('calendar-timeline-overlap-adjusted');
	element.style.setProperty('--calendar-overlap-left', `${(laneIndex * 100) / laneCount}%`);
	element.style.setProperty('--calendar-overlap-width', `${laneWidth}%`);
}

function applyTimelinePreviewOverlapLane(element: HTMLElement, laneCount: number): void {
	const baseWidth = element.dataset.timelinePreviewBaseWidth ?? element.style.width;
	const baseWidthPx = Number.parseFloat(baseWidth);
	if (!Number.isFinite(baseWidthPx)) return;
	element.dataset.timelinePreviewBaseWidth = baseWidth;
	element.classList.add('calendar-timeline-overlap-adjusted');
	element.style.width = `${timelinePreviewOverlapWidth(baseWidthPx, laneCount)}px`;
}

function restoreTimelinePreviewWidth(element: HTMLElement): void {
	const baseWidth = element.dataset.timelinePreviewBaseWidth;
	if (!baseWidth) return;
	element.style.width = baseWidth;
	delete element.dataset.timelinePreviewBaseWidth;
}
