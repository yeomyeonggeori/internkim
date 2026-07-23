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
	isLongPressActivation: boolean;
};

type PendingMonthEventTouchDrag = {
	pointerID: number;
	event: DayFlowEvent;
	startClientX: number;
	startClientY: number;
	timeoutID: number;
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

const monthEventTouchLongPressDurationMs = 500;

export function createMonthEventDragController(context: MonthEventDragControllerContext): MonthEventDragController {
	let removeDragListeners: (() => void) | null = null;
	let pendingTouchDrag: PendingMonthEventTouchDrag | null = null;

	function startDrag(pointerEvent: PointerEvent, event: DayFlowEvent): void {
		stop();
		if (pointerEvent.pointerType === 'touch') {
			startTouchDrag(pointerEvent, event);
			return;
		}
		pointerEvent.preventDefault();
		pointerEvent.stopPropagation();
		activateDrag(pointerEvent.pointerId, event, pointerEvent.clientX, pointerEvent.clientY, false);
	}

	function startTouchDrag(pointerEvent: PointerEvent, event: DayFlowEvent): void {
		const timeoutID = window.setTimeout(() => {
			const pendingDrag = pendingTouchDrag;
			if (!pendingDrag || pendingDrag.pointerID !== pointerEvent.pointerId) return;
			pendingTouchDrag = null;
			activateDrag(
				pendingDrag.pointerID,
				pendingDrag.event,
				pendingDrag.startClientX,
				pendingDrag.startClientY,
				true
			);
		}, monthEventTouchLongPressDurationMs);
		pendingTouchDrag = {
			pointerID: pointerEvent.pointerId,
			event,
			startClientX: pointerEvent.clientX,
			startClientY: pointerEvent.clientY,
			timeoutID
		};
		installDragListeners();
	}

	function activateDrag(
		pointerID: number,
		event: DayFlowEvent,
		startClientX: number,
		startClientY: number,
		isLongPressActivation: boolean
	): void {
		context.setDragState({
			pointerID,
			event,
			startClientX,
			startClientY,
			hasMoved: false,
			previewEvent: null,
			isLongPressActivation
		});
		installDragListeners();
	}

	function handlePointerMove(pointerEvent: PointerEvent): void {
		if (cancelPendingTouchDragAfterMovement(pointerEvent)) return;
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
		if (cancelPendingTouchDrag(pointerEvent.pointerId)) return;
		const state = context.getDragState();
		if (!state || state.pointerID !== pointerEvent.pointerId) return;
		pointerEvent.preventDefault();
		stop();
		context.setDragState(null);
		if (state.hasMoved || state.isLongPressActivation) context.suppressNextClick();
		if (!state.hasMoved || !state.previewEvent) return;
		context.clearSelectedEvent();
		void context.saveMovedEvent(state.previewEvent);
	}

	function handlePointerCancel(pointerEvent: PointerEvent): void {
		if (cancelPendingTouchDrag(pointerEvent.pointerId)) return;
		const state = context.getDragState();
		if (!state || state.pointerID !== pointerEvent.pointerId) return;
		pointerEvent.preventDefault();
		stop();
		context.setDragState(null);
	}

	function cancelPendingTouchDragAfterMovement(pointerEvent: PointerEvent): boolean {
		const pendingDrag = pendingTouchDrag;
		if (!pendingDrag || pendingDrag.pointerID !== pointerEvent.pointerId) return false;
		const hasMoved = hasCalendarEventGestureMoved(
			{ clientX: pendingDrag.startClientX, clientY: pendingDrag.startClientY },
			pointerEvent
		);
		if (hasMoved) cancelPendingTouchDrag(pointerEvent.pointerId);
		return true;
	}

	function cancelPendingTouchDrag(pointerID: number): boolean {
		if (!pendingTouchDrag || pendingTouchDrag.pointerID !== pointerID) return false;
		window.clearTimeout(pendingTouchDrag.timeoutID);
		pendingTouchDrag = null;
		stop();
		return true;
	}

	function installDragListeners(): void {
		removeDragListeners?.();
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
		if (pendingTouchDrag) {
			window.clearTimeout(pendingTouchDrag.timeoutID);
			pendingTouchDrag = null;
		}
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
