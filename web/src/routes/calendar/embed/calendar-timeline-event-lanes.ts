import type { CalendarViewType, Event as DayFlowEvent } from '@dayflow/core';
import { ViewType } from '@dayflow/svelte';
import { dayFlowEventSelectorForID } from './calendar-dayflow-dom-adapter';
import { eventEndDate, eventStartDate } from './calendar-event-mapping';
import {
	timelineEventLanePlacementsFromCandidates,
	type TimelineEventLaneCandidate,
	type TimelineEventLanePlacement
} from './calendar-timeline-lane-model';

export function syncTimelineEventLaneLayout(
	stageElement: HTMLElement | null,
	currentView: CalendarViewType,
	events: DayFlowEvent[]
): void {
	clearTimelineEventLaneLayout(stageElement);
	if (!stageElement || currentView !== ViewType.DAY) return;
	for (const placement of timelineEventLanePlacements(stageElement, events)) {
		for (const element of timelineEventElementsByID(stageElement, placement.eventID)) {
			applyTimelineEventPlacement(element, placement);
		}
	}
}

export function clearTimelineEventLaneLayout(stageElement: HTMLElement | null): void {
	if (!stageElement) return;
	for (const element of stageElement.querySelectorAll<HTMLElement>('.calendar-timeline-lane-adjusted, .calendar-timeline-layered')) {
		element.classList.remove('calendar-timeline-lane-adjusted');
		element.classList.remove('calendar-timeline-layered');
		element.style.removeProperty('--calendar-lane-left');
		element.style.removeProperty('--calendar-lane-width');
		element.style.removeProperty('--calendar-layer-z-index');
	}
}

function timelineEventLanePlacements(stageElement: HTMLElement, events: DayFlowEvent[]): TimelineEventLanePlacement[] {
	const candidates = timelineEventLaneCandidates(stageElement, events);
	return timelineEventLanePlacementsFromCandidates(candidates);
}

function timelineEventLaneCandidates(stageElement: HTMLElement, events: DayFlowEvent[]): TimelineEventLaneCandidate[] {
	return events
		.filter((event) => !event.allDay && !event.id.startsWith('timeline-') && timelineEventElementsByID(stageElement, event.id).length > 0)
		.map((event) => ({
			eventID: event.id,
			title: event.title,
			startDate: eventStartDate(event),
			endDate: eventEndDate(event)
		}))
		.sort((firstEvent, secondEvent) => firstEvent.startDate.getTime() - secondEvent.startDate.getTime() || firstEvent.endDate.getTime() - secondEvent.endDate.getTime());
}

function timelineEventElementsByID(stageElement: HTMLElement, eventID: string): HTMLElement[] {
	return Array.from(stageElement.querySelectorAll<HTMLElement>(dayFlowEventSelectorForID(eventID))).filter(
		(element) =>
			element.classList.contains('df-day-event') &&
			element.classList.contains('df-event-timed') &&
			!element.closest('.df-right-panel-events, .df-right-panel-calendar-shell, .df-mini-calendar')
	);
}

function applyTimelineEventPlacement(element: HTMLElement, placement: TimelineEventLanePlacement): void {
	element.classList.add(placement.kind === 'lane' ? 'calendar-timeline-lane-adjusted' : 'calendar-timeline-layered');
	element.style.setProperty('--calendar-lane-left', `${placement.leftPercent}%`);
	element.style.setProperty('--calendar-lane-width', `${placement.widthPercent}%`);
	element.style.setProperty('--calendar-layer-z-index', String(placement.zIndex));
}
