import type { Event as DayFlowEvent } from '@dayflow/core';
import { hasCalendarEventGestureMoved } from './calendar-event-gesture';
import { moveMonthEventToDate } from './calendar-month-event-model';

export type MonthEventDragState = {
	pointerID: number;
	event: DayFlowEvent;
	startClientX: number;
	startClientY: number;
	hasMoved: boolean;
	previewEvent: DayFlowEvent | null;
};

type MonthEventDragControllerContext = {
	clearSelectedEvent: () => void;
	getDragState: () => MonthEventDragState | null;
	getStageElement: () => HTMLElement | null;
	saveMovedEvent: (event: DayFlowEvent) => void | Promise<void>;
	setDragState: (state: MonthEventDragState | null) => void;
	suppressNextClick: () => void;
};

export type MonthEventDragController = {
	startDrag: (pointerEvent: PointerEvent, event: DayFlowEvent) => void;
	stop: () => void;
};

export function createMonthEventDragController(context: MonthEventDragControllerContext): MonthEventDragController {
	let removeDragListeners: (() => void) | null = null;

	function startDrag(pointerEvent: PointerEvent, event: DayFlowEvent): void {
		pointerEvent.preventDefault();
		pointerEvent.stopPropagation();
		context.setDragState({
			pointerID: pointerEvent.pointerId,
			event,
			startClientX: pointerEvent.clientX,
			startClientY: pointerEvent.clientY,
			hasMoved: false,
			previewEvent: null
		});
		installDragListeners();
	}

	function handlePointerMove(pointerEvent: PointerEvent): void {
		const state = context.getDragState();
		if (!state || state.pointerID !== pointerEvent.pointerId) return;
		const hasMoved = state.hasMoved || hasMonthEventPointerMoved(state, pointerEvent);
		if (!hasMoved) return;
		const targetDateKey = monthDateKeyFromPoint(context.getStageElement(), pointerEvent.clientX, pointerEvent.clientY);
		if (!targetDateKey) return;
		pointerEvent.preventDefault();
		context.setDragState({
			...state,
			hasMoved: true,
			previewEvent: moveMonthEventToDate(state.event, targetDateKey)
		});
	}

	function handlePointerUp(pointerEvent: PointerEvent): void {
		const state = context.getDragState();
		if (!state || state.pointerID !== pointerEvent.pointerId) return;
		pointerEvent.preventDefault();
		stop();
		context.setDragState(null);
		if (!state.hasMoved || !state.previewEvent) return;
		context.suppressNextClick();
		context.clearSelectedEvent();
		void context.saveMovedEvent(state.previewEvent);
	}

	function handlePointerCancel(pointerEvent: PointerEvent): void {
		const state = context.getDragState();
		if (!state || state.pointerID !== pointerEvent.pointerId) return;
		pointerEvent.preventDefault();
		stop();
		context.setDragState(null);
	}

	function installDragListeners(): void {
		stop();
		window.addEventListener('pointermove', handlePointerMove, true);
		window.addEventListener('pointerup', handlePointerUp, true);
		window.addEventListener('pointercancel', handlePointerCancel, true);
		removeDragListeners = () => {
			window.removeEventListener('pointermove', handlePointerMove, true);
			window.removeEventListener('pointerup', handlePointerUp, true);
			window.removeEventListener('pointercancel', handlePointerCancel, true);
		};
	}

	function stop(): void {
		removeDragListeners?.();
		removeDragListeners = null;
	}

	return {
		startDrag,
		stop
	};
}

export function eventsWithMonthDragPreview(
	calendarEvents: DayFlowEvent[],
	state: MonthEventDragState | null
): DayFlowEvent[] {
	const previewEvent = state?.previewEvent;
	if (!state || !previewEvent) return calendarEvents;
	return calendarEvents.map((calendarEvent) => (calendarEvent.id === state.event.id ? previewEvent : calendarEvent));
}

export function hasMonthEventPointerMoved(state: MonthEventDragState, pointerEvent: PointerEvent): boolean {
	return hasCalendarEventGestureMoved(
		{ clientX: state.startClientX, clientY: state.startClientY },
		pointerEvent
	);
}

export function monthDateKeyFromPoint(
	stageElement: HTMLElement | null,
	clientX: number,
	clientY: number
): string | null {
	if (!stageElement) return null;
	for (const dateCell of stageElement.querySelectorAll<HTMLElement>('.df-month-day-cell[data-date]')) {
		const rectangle = dateCell.getBoundingClientRect();
		if (clientX < rectangle.left || clientX > rectangle.right) continue;
		if (clientY < rectangle.top || clientY > rectangle.bottom) continue;
		return dateCell.dataset.date ?? null;
	}
	return null;
}
