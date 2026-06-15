// 캘린더 초안 팝오버의 열기, 저장, 취소, 삭제 동작을 연결합니다.
import { createEvent, type Event as DayFlowEvent } from '@dayflow/core';
import type { CalendarEventActions } from './calendar-event-actions';
import {
	draftPopoverChanges,
	draftPopoverPositionFromAnchor,
	draftPopoverStateFromEvent,
	isDraftPopoverValid,
	type DraftPopoverAnchor,
	type DraftPopoverMode,
	type DraftPopoverState
} from './calendar-draft-popover-state';
import { calendarEventElementsByID } from './calendar-event-elements';
import type { MonthRangeSelection } from './calendar-month-range-action';

export type CalendarDraftPopoverActions = {
	createQuickDraftPopover: (event: MouseEvent) => void;
	openMonthSingleDayDraftPopover: (dateKey: string, anchor?: DraftPopoverAnchor | null) => void;
	openMonthRangeDraftPopover: (selection: MonthRangeSelection, anchor?: DraftPopoverAnchor | null) => void;
	openTimelineSingleDraftPopover: (startDate: Date, anchor: DraftPopoverAnchor) => void;
	openTimelineRangeDraftPopover: (firstDate: Date, secondDate: Date, anchor: DraftPopoverAnchor) => void;
	openEventDraftPopover: (event: DayFlowEvent, mode: DraftPopoverMode) => void;
	updateDraftPopover: (changes: Partial<DraftPopoverState>) => void;
	saveDraftPopover: () => Promise<void>;
	cancelDraftPopover: () => Promise<void>;
	deleteDraftPopover: () => Promise<void>;
};

type CalendarDraftPopoverActionsContext = {
	eventActions: CalendarEventActions;
	getDraftPopover: () => DraftPopoverState | null;
	setDraftPopover: (popover: DraftPopoverState | null) => void;
	getCalendarEvents: () => DayFlowEvent[];
	getStageElement: () => HTMLElement | null;
	selectEvent: (eventID: string) => void;
	replaceLocalEvent: (event: DayFlowEvent) => void;
};

export function createCalendarDraftPopoverActions(
	context: CalendarDraftPopoverActionsContext
): CalendarDraftPopoverActions {
	function createQuickDraftPopover(event: MouseEvent): void {
		const draftEvent = context.eventActions.createQuickEvent();
		openDraftPopoverForEvent(draftEvent, 'create', anchorFromElement(event.currentTarget));
	}

	function openMonthSingleDayDraftPopover(dateKey: string, anchor: DraftPopoverAnchor | null = null): void {
		const draftEvent = context.eventActions.createMonthSingleDayEvent(dateKey);
		if (!draftEvent) return;
		openDraftPopoverForEvent(draftEvent, 'create', anchor ?? monthDateAnchor(dateKey));
	}

	function openMonthRangeDraftPopover(selection: MonthRangeSelection, anchor: DraftPopoverAnchor | null = null): void {
		const draftEvent = context.eventActions.createMonthRangeEvent(selection);
		openDraftPopoverForEvent(draftEvent, 'create', anchor ?? monthDateAnchor(selection.endDateKey));
	}

	function openTimelineSingleDraftPopover(startDate: Date, anchor: DraftPopoverAnchor): void {
		const draftEvent = context.eventActions.createTimelineSingleEvent(startDate);
		if (!draftEvent) return;
		openDraftPopoverForEvent(draftEvent, 'create', anchor);
	}

	function openTimelineRangeDraftPopover(firstDate: Date, secondDate: Date, anchor: DraftPopoverAnchor): void {
		const draftEvent = context.eventActions.createTimelineRangeEvent(firstDate, secondDate);
		if (!draftEvent) return;
		openDraftPopoverForEvent(draftEvent, 'create', anchor);
	}

	function openEventDraftPopover(event: DayFlowEvent, mode: DraftPopoverMode): void {
		openDraftPopoverForEvent(event, mode, eventAnchorForEventID(event.id));
	}

	function updateDraftPopover(changes: Partial<DraftPopoverState>): void {
		const popover = context.getDraftPopover();
		if (!popover) return;
		context.setDraftPopover({ ...popover, ...changes });
	}

	async function saveDraftPopover(): Promise<void> {
		const popover = context.getDraftPopover();
		if (!popover || !isDraftPopoverValid(popover)) return;
		const event = context.getCalendarEvents().find((calendarEvent) => calendarEvent.id === popover.eventID);
		if (!event) return;
		const changes = draftPopoverChanges(popover);
		const updatedEvent = createEvent({
			id: event.id,
			title: changes.title,
			description: changes.description,
			start: changes.start,
			end: changes.end,
			allDay: changes.allDay,
			calendarId: changes.calendarId,
			meta: { ...(event.meta ?? {}), ...changes.meta }
		});
		context.replaceLocalEvent(updatedEvent);
		context.setDraftPopover(null);
		await context.eventActions.saveUpdatedEvent(updatedEvent);
	}

	async function cancelDraftPopover(): Promise<void> {
		const popover = context.getDraftPopover();
		if (!popover) return;
		context.setDraftPopover(null);
		if (popover.mode === 'create') await context.eventActions.deleteEvent(popover.eventID);
	}

	async function deleteDraftPopover(): Promise<void> {
		const popover = context.getDraftPopover();
		if (!popover || popover.mode !== 'edit') return;
		context.setDraftPopover(null);
		await context.eventActions.deleteEvent(popover.eventID);
	}

	function openDraftPopoverForEvent(
		event: DayFlowEvent,
		mode: DraftPopoverMode,
		anchor: DraftPopoverAnchor | null
	): void {
		context.selectEvent(event.id);
		context.setDraftPopover(draftPopoverStateFromEvent(event, mode, anchor, context.getStageElement()));
		window.requestAnimationFrame(() => {
			const popover = context.getDraftPopover();
			if (!popover || popover.eventID !== event.id) return;
			const renderedEventAnchor = eventAnchorForEventID(event.id);
			if (!renderedEventAnchor) return;
			context.setDraftPopover({
				...popover,
				position: draftPopoverPositionFromAnchor(renderedEventAnchor, context.getStageElement())
			});
		});
	}

	function eventAnchorForEventID(eventID: string): DraftPopoverAnchor | null {
		const eventElement = calendarEventElementsByID(context.getStageElement(), eventID)[0];
		return anchorFromElement(eventElement);
	}

	function monthDateAnchor(dateKey: string): DraftPopoverAnchor | null {
		const stageElement = context.getStageElement();
		if (!stageElement) return null;
		const escapedDateKey = window.CSS?.escape(dateKey) ?? dateKey.replaceAll('"', '\\"');
		return anchorFromElement(stageElement.querySelector(`[data-date="${escapedDateKey}"]`));
	}

	function anchorFromElement(element: EventTarget | Element | null): DraftPopoverAnchor | null {
		if (!(element instanceof Element)) return null;
		const rectangle = element.getBoundingClientRect();
		const titleHeight = Math.min(56, rectangle.height);
		return {
			clientX: rectangle.right - Math.min(18, rectangle.width / 2),
			clientY: rectangle.top + Math.min(40, rectangle.height / 2),
			leftClientX: rectangle.left,
			topClientY: rectangle.top,
			bottomClientY: rectangle.top + titleHeight
		};
	}

	return {
		createQuickDraftPopover,
		openMonthSingleDayDraftPopover,
		openMonthRangeDraftPopover,
		openTimelineSingleDraftPopover,
		openTimelineRangeDraftPopover,
		openEventDraftPopover,
		updateDraftPopover,
		saveDraftPopover,
		cancelDraftPopover,
		deleteDraftPopover
	};
}
