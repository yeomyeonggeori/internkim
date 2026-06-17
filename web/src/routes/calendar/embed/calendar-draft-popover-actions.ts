import type { Event as DayFlowEvent } from '@dayflow/core';
import type { CalendarEventActions } from './calendar-event-actions';
import {
	draftPopoverPositionFromAnchor,
	draftPopoverStateFromEvent,
	type DraftPopoverAnchor,
	type DraftPopoverMode,
	type DraftPopoverSize,
	type DraftPopoverState
} from './calendar-draft-popover-state';
import {
	recentCalendarEventAnchorForID
} from './calendar-event-anchor-capture';
import {
	anchorFromElement,
	areSamePopoverAnchors,
	areSamePopoverPositions,
	calendarEventAnchorForEventID,
	clearDraftPopoverElementMotion,
	genericEventAnchorForEventID,
	isAnchorInsideVisibleStage,
	monthDateAnchor,
	shouldShowInitialPopover
} from './calendar-draft-popover-anchor';
import { createCalendarDraftPopoverPersistence } from './calendar-draft-popover-persistence';
import type { MonthRangeSelection } from './calendar-month-range-action';

export type CalendarDraftPopoverActions = {
	createQuickDraftPopover: (event: MouseEvent) => void;
	openMonthSingleDayDraftPopover: (dateKey: string, anchor?: DraftPopoverAnchor | null) => void;
	openMonthRangeDraftPopover: (selection: MonthRangeSelection, anchor?: DraftPopoverAnchor | null) => void;
	openTimelineSingleDraftPopover: (startDate: Date, anchor: DraftPopoverAnchor) => void;
	openTimelineRangeDraftPopover: (firstDate: Date, secondDate: Date, anchor: DraftPopoverAnchor) => void;
	openEventDraftPopover: (event: DayFlowEvent, mode: DraftPopoverMode, anchor?: DraftPopoverAnchor | null) => void;
	refreshDraftPopoverAnchor: (size?: DraftPopoverSize) => boolean;
	repositionDraftPopover: (size: DraftPopoverSize) => void;
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

const renderedEventAnchorRetryDelay = 50;
const renderedEventAnchorRetryLimit = 30;

export function createCalendarDraftPopoverActions(
	context: CalendarDraftPopoverActionsContext
): CalendarDraftPopoverActions {
	const persistence = createCalendarDraftPopoverPersistence(context);

	function createQuickDraftPopover(event: MouseEvent): void {
		const draftEvent = context.eventActions.createQuickEvent();
		openDraftPopoverForEvent(draftEvent, 'create', anchorFromElement(event.currentTarget), false);
	}

	function openMonthSingleDayDraftPopover(dateKey: string, anchor: DraftPopoverAnchor | null = null): void {
		const draftEvent = context.eventActions.createMonthSingleDayEvent(dateKey);
		if (!draftEvent) return;
		openDraftPopoverForEvent(draftEvent, 'create', anchor ?? monthDateAnchor(context.getStageElement(), dateKey), true);
	}

	function openMonthRangeDraftPopover(selection: MonthRangeSelection, anchor: DraftPopoverAnchor | null = null): void {
		const draftEvent = context.eventActions.createMonthRangeEvent(selection);
		openDraftPopoverForEvent(draftEvent, 'create', anchor ?? monthDateAnchor(context.getStageElement(), selection.endDateKey), true);
	}

	function openTimelineSingleDraftPopover(startDate: Date, anchor: DraftPopoverAnchor): void {
		const draftEvent = context.eventActions.createTimelineSingleEvent(startDate);
		if (!draftEvent) return;
		openDraftPopoverForEvent(draftEvent, 'create', anchor, true);
	}

	function openTimelineRangeDraftPopover(firstDate: Date, secondDate: Date, anchor: DraftPopoverAnchor): void {
		const draftEvent = context.eventActions.createTimelineRangeEvent(firstDate, secondDate);
		if (!draftEvent) return;
		openDraftPopoverForEvent(draftEvent, 'create', anchor, true);
	}

	function openEventDraftPopover(event: DayFlowEvent, mode: DraftPopoverMode, anchor: DraftPopoverAnchor | null = null): void {
		const canonicalAnchor = calendarEventAnchorForEventID(context.getStageElement(), event.id);
		openDraftPopoverForEvent(event, mode, canonicalAnchor ?? anchor ?? recentCalendarEventAnchorForID(event.id), false);
	}

	function repositionDraftPopover(size: DraftPopoverSize): void {
		const popover = context.getDraftPopover();
		if (!popover?.anchor || !popover.position.isReady) return;
		const position = draftPopoverPositionFromAnchor(popover.anchor, context.getStageElement(), size);
		if (areSamePopoverPositions(popover.position, position)) return;
		clearDraftPopoverElementMotion();
		context.setDraftPopover({ ...popover, position });
	}

	function refreshDraftPopoverAnchor(size: DraftPopoverSize = {}): boolean {
		const popover = context.getDraftPopover();
		if (!popover) return false;
		const anchor = calendarEventAnchorForEventID(context.getStageElement(), popover.eventID);
		if (!anchor || !isAnchorInsideVisibleStage(context.getStageElement(), anchor)) {
			void persistence.cancelDraftPopover();
			return false;
		}
		const position = draftPopoverPositionFromAnchor(anchor, context.getStageElement(), size);
		if (areSamePopoverAnchors(popover.anchor, anchor) && areSamePopoverPositions(popover.position, position)) return true;
		clearDraftPopoverElementMotion();
		context.setDraftPopover({ ...popover, anchor, position });
		return true;
	}

	function updateDraftPopover(changes: Partial<DraftPopoverState>): void {
		const popover = context.getDraftPopover();
		if (!popover) return;
		context.setDraftPopover({ ...popover, ...changes });
	}

	function openDraftPopoverForEvent(
		event: DayFlowEvent,
		mode: DraftPopoverMode,
		anchor: DraftPopoverAnchor | null,
		shouldWaitForRenderedAnchor: boolean
	): void {
		context.selectEvent(event.id);
		const popover = draftPopoverStateFromEvent(event, mode, anchor, context.getStageElement());
		context.setDraftPopover({
			...popover,
			position: {
				...popover.position,
				isReady: !shouldWaitForRenderedAnchor || shouldShowInitialPopover(mode, anchor)
			}
		});
		scheduleRenderedEventPopoverPosition(event.id, mode, anchor, 0);
	}

	function scheduleRenderedEventPopoverPosition(
		eventID: string,
		mode: DraftPopoverMode,
		anchor: DraftPopoverAnchor | null,
		attempt: number
	): void {
		window.setTimeout(() => {
			updateRenderedEventPopoverPosition(eventID, mode, anchor, attempt);
		}, renderedEventAnchorRetryDelay);
	}

	function updateRenderedEventPopoverPosition(
		eventID: string,
		mode: DraftPopoverMode,
		anchor: DraftPopoverAnchor | null,
		attempt: number
	): void {
		const popover = context.getDraftPopover();
		if (!popover || popover.eventID !== eventID) return;
		const renderedEventAnchor =
			mode === 'edit'
				? (calendarEventAnchorForEventID(context.getStageElement(), eventID) ?? anchor)
				: (calendarEventAnchorForEventID(context.getStageElement(), eventID, anchor) ??
					genericEventAnchorForEventID(context.getStageElement(), eventID));
		if (!renderedEventAnchor) {
			if (attempt < renderedEventAnchorRetryLimit) {
				scheduleRenderedEventPopoverPosition(eventID, mode, anchor, attempt + 1);
			}
			return;
		}
		context.setDraftPopover({
			...popover,
			anchor: renderedEventAnchor,
			position: draftPopoverPositionFromAnchor(renderedEventAnchor, context.getStageElement())
		});
	}

	return {
		createQuickDraftPopover,
		openMonthSingleDayDraftPopover,
		openMonthRangeDraftPopover,
		openTimelineSingleDraftPopover,
		openTimelineRangeDraftPopover,
		openEventDraftPopover,
		refreshDraftPopoverAnchor,
		repositionDraftPopover,
		updateDraftPopover,
		saveDraftPopover: persistence.saveDraftPopover,
		cancelDraftPopover: persistence.cancelDraftPopover,
		deleteDraftPopover: persistence.deleteDraftPopover
	};
}
