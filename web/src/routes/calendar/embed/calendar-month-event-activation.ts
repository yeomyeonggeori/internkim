import { calendarEventAnchorFromElement } from './calendar-event-anchor-capture';
import { calendarEventAnchorForEventID } from './calendar-draft-popover-anchor';
import type { DraftPopoverAnchor } from './calendar-draft-popover-state';
import { isCalendarEventAccessibleClick } from './calendar-event-accessible-click';
import type { MonthEventPlacement } from './calendar-month-event-geometry';
import type { MonthEventSegment } from './calendar-month-event-model';

type CalendarMonthEventActivationTarget = Pick<MonthEventPlacement, 'eventID' | 'startDateKey'>;

type CalendarMonthEventActivationContext = {
	getStageElement: () => HTMLElement | null;
	openEvent: (eventID: string, anchor: DraftPopoverAnchor) => void;
	selectDate: (dateKey: string) => void;
	shouldSuppressEventActivation: () => boolean;
};

export type CalendarMonthEventActivation = {
	handleDirectEventClick: (mouseEvent: MouseEvent, placement: MonthEventPlacement) => void;
	handleDirectEventDoubleClick: (mouseEvent: MouseEvent, placement: MonthEventPlacement) => void;
	handleOverflowEventClick: (mouseEvent: MouseEvent, segment: MonthEventSegment, activeDateKey: string) => void;
	handleOverflowEventDoubleClick: (mouseEvent: MouseEvent, segment: MonthEventSegment, activeDateKey: string) => void;
	selectDate: (target: CalendarMonthEventActivationTarget) => void;
};

export function createCalendarMonthEventActivation(
	context: CalendarMonthEventActivationContext
): CalendarMonthEventActivation {
	function selectDate(target: CalendarMonthEventActivationTarget): void {
		context.selectDate(target.startDateKey);
	}

	function openEvent(
		target: CalendarMonthEventActivationTarget,
		anchor: DraftPopoverAnchor,
		selectedDateKey = target.startDateKey
	): void {
		context.selectDate(selectedDateKey);
		context.openEvent(target.eventID, anchor);
	}

	function handleDirectEventClick(mouseEvent: MouseEvent, placement: MonthEventPlacement): void {
		stopEventActivation(mouseEvent);
		if (context.shouldSuppressEventActivation()) return;
		if (!isCalendarEventAccessibleClick(mouseEvent)) return;
		openDirectEvent(mouseEvent, placement);
	}

	function handleDirectEventDoubleClick(mouseEvent: MouseEvent, placement: MonthEventPlacement): void {
		stopEventActivation(mouseEvent);
		if (context.shouldSuppressEventActivation()) return;
		openDirectEvent(mouseEvent, placement);
	}

	function openDirectEvent(mouseEvent: MouseEvent, placement: MonthEventPlacement): void {
		const eventElement = currentEventElement(mouseEvent);
		if (!eventElement) return;
		const currentAnchor = calendarEventAnchorFromElement(eventElement);
		const canonicalAnchor = calendarEventAnchorForEventID(context.getStageElement(), placement.eventID);
		const anchor = canonicalAnchor
			? { ...canonicalAnchor, originElement: currentAnchor.originElement }
			: currentAnchor;
		openEvent(placement, anchor);
	}

	function handleOverflowEventClick(
		mouseEvent: MouseEvent,
		segment: MonthEventSegment,
		activeDateKey: string
	): void {
		stopEventActivation(mouseEvent);
		if (!isCalendarEventAccessibleClick(mouseEvent)) {
			context.selectDate(activeDateKey);
			return;
		}
		openOverflowEvent(mouseEvent, segment, activeDateKey);
	}

	function handleOverflowEventDoubleClick(
		mouseEvent: MouseEvent,
		segment: MonthEventSegment,
		activeDateKey: string
	): void {
		stopEventActivation(mouseEvent);
		openOverflowEvent(mouseEvent, segment, activeDateKey);
	}

	function openOverflowEvent(mouseEvent: MouseEvent, segment: MonthEventSegment, activeDateKey: string): void {
		const eventElement = currentEventElement(mouseEvent);
		if (!eventElement) return;
		openEvent(segment, calendarEventAnchorFromElement(eventElement), activeDateKey);
	}

	return {
		handleDirectEventClick,
		handleDirectEventDoubleClick,
		handleOverflowEventClick,
		handleOverflowEventDoubleClick,
		selectDate
	};
}

function currentEventElement(mouseEvent: MouseEvent): HTMLElement | null {
	return mouseEvent.currentTarget instanceof HTMLElement ? mouseEvent.currentTarget : null;
}

function stopEventActivation(mouseEvent: MouseEvent): void {
	mouseEvent.preventDefault();
	mouseEvent.stopPropagation();
}
